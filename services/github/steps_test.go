package github

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCLITransport_GetWorkflowSteps_FlattensJobsAndSteps(t *testing.T) {
	t.Parallel()

	payload := `{
		"jobs": [{
			"id": 1,
			"name": "build",
			"steps": [
				{"name": "Set up", "status": "completed", "conclusion": "success", "number": 1,
				 "started_at": "2026-05-23T09:00:00Z", "completed_at": "2026-05-23T09:00:05Z"},
				{"name": "Test", "status": "in_progress", "conclusion": "", "number": 2,
				 "started_at": "2026-05-23T09:00:05Z", "completed_at": "0001-01-01T00:00:00Z"}
			]
		}, {
			"id": 2,
			"name": "deploy",
			"steps": [
				{"name": "Push", "status": "queued", "conclusion": "", "number": 1}
			]
		}]
	}`

	runner := commandRunnerFunc(func(_ context.Context, name string, args []string, _ []byte) ([]byte, []byte, error) {
		if name != "gh" {
			t.Fatalf("expected gh invocation, got %q", name)
		}
		joined := strings.Join(args, " ")
		if !strings.Contains(joined, "/actions/runs/42/jobs") {
			t.Fatalf("expected path with run id, got %q", joined)
		}
		return []byte(payload), nil, nil
	})

	transport := NewCLITransport(runner)
	steps, err := transport.GetWorkflowSteps(context.Background(), RepoRef{Owner: "dilitS", Name: "webox"}, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(steps) != 3 {
		t.Fatalf("expected 3 flattened steps, got %d", len(steps))
	}
	if steps[0].JobName != "build" || steps[0].Name != "Set up" {
		t.Errorf("first step mismatch: %+v", steps[0])
	}
	if steps[0].DurationMs != 5_000 {
		t.Errorf("expected duration 5000ms, got %d", steps[0].DurationMs)
	}
	if steps[2].JobID != 2 || steps[2].JobName != "deploy" {
		t.Errorf("expected deploy job for step #3, got %+v", steps[2])
	}
}

func TestCLITransport_GetWorkflowSteps_HTTP404MapsToRunNotFound(t *testing.T) {
	t.Parallel()

	runner := commandRunnerFunc(func(_ context.Context, _ string, _ []string, _ []byte) ([]byte, []byte, error) {
		return nil, []byte("HTTP 404: Not Found"), errors.New("exit 1")
	})
	transport := NewCLITransport(runner)
	_, err := transport.GetWorkflowSteps(context.Background(), RepoRef{Owner: "o", Name: "r"}, 7)
	if !errors.Is(err, ErrRunNotFound) {
		t.Fatalf("expected ErrRunNotFound, got %v", err)
	}
}

func TestCLITransport_GetWorkflowSteps_InvalidRunID(t *testing.T) {
	t.Parallel()

	transport := NewCLITransport(commandRunnerFunc(func(context.Context, string, []string, []byte) ([]byte, []byte, error) {
		t.Fatal("runner should not be invoked")
		return nil, nil, nil
	}))
	_, err := transport.GetWorkflowSteps(context.Background(), RepoRef{Owner: "o", Name: "r"}, 0)
	if !errors.Is(err, ErrRunNotFound) {
		t.Fatalf("expected ErrRunNotFound for zero runID, got %v", err)
	}
}

func TestCLITransport_GetWorkflowSteps_EmptyJobsAreRunNotFound(t *testing.T) {
	t.Parallel()

	runner := commandRunnerFunc(func(context.Context, string, []string, []byte) ([]byte, []byte, error) {
		return []byte(`{"jobs": []}`), nil, nil
	})
	transport := NewCLITransport(runner)
	_, err := transport.GetWorkflowSteps(context.Background(), RepoRef{Owner: "o", Name: "r"}, 99)
	if !errors.Is(err, ErrRunNotFound) {
		t.Fatalf("expected ErrRunNotFound, got %v", err)
	}
}

func TestCLITransport_GetWorkflowLogs_TailAndRedact(t *testing.T) {
	t.Parallel()

	raw := strings.Join([]string{
		"build\tStep1\t2026-05-23T09:00:00Z Build starting",
		"build\tStep1\t2026-05-23T09:00:01Z token=ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
		"build\tStep1\t2026-05-23T09:00:02Z Build finished",
	}, "\n")

	runner := commandRunnerFunc(func(_ context.Context, _ string, args []string, _ []byte) ([]byte, []byte, error) {
		joined := strings.Join(args, " ")
		if !strings.Contains(joined, "run view") || !strings.Contains(joined, "--log") {
			t.Fatalf("expected run view --log invocation, got %q", joined)
		}
		return []byte(raw), nil, nil
	})

	transport := NewCLITransport(runner)
	lines, err := transport.GetWorkflowLogs(context.Background(), RepoRef{Owner: "o", Name: "r"}, 1, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("expected tail of 2 lines, got %d", len(lines))
	}
	if strings.Contains(lines[0].Raw, "ghp_") {
		t.Errorf("expected redaction of PAT, got %q", lines[0].Raw)
	}
	if !strings.Contains(lines[0].Raw, "REDACTED") {
		t.Errorf("expected REDACTED marker in line, got %q", lines[0].Raw)
	}
	if lines[1].JobName != "build" || lines[1].StepName != "Step1" {
		t.Errorf("expected job/step parsed, got %+v", lines[1])
	}
}

func TestCLITransport_GetWorkflowLogs_NotFoundMapsToErrRunNotFound(t *testing.T) {
	t.Parallel()

	runner := commandRunnerFunc(func(context.Context, string, []string, []byte) ([]byte, []byte, error) {
		return nil, []byte("could not find run not found"), errors.New("exit 1")
	})
	transport := NewCLITransport(runner)
	_, err := transport.GetWorkflowLogs(context.Background(), RepoRef{Owner: "o", Name: "r"}, 1, 50)
	if !errors.Is(err, ErrRunNotFound) {
		t.Fatalf("expected ErrRunNotFound, got %v", err)
	}
}

func TestRESTTransport_GetWorkflowSteps_404Sentinel(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "Not Found", http.StatusNotFound)
	}))
	defer srv.Close()

	transport := NewRESTTransport(RESTOptions{
		BaseURL:     srv.URL,
		TokenSource: tokenSourceFunc(func(context.Context) (string, error) { return "ghp_fake_token_1234567890", nil }),
		Sleep:       func(context.Context, time.Duration) error { return nil },
	})
	_, err := transport.GetWorkflowSteps(context.Background(), RepoRef{Owner: "o", Name: "r"}, 1)
	if !errors.Is(err, ErrRunNotFound) {
		t.Fatalf("expected ErrRunNotFound on 404, got %v", err)
	}
}

func TestRESTTransport_GetWorkflowSteps_Success(t *testing.T) {
	t.Parallel()

	payload := `{"jobs":[{"id":1,"name":"build","steps":[{"name":"Compile","status":"completed","conclusion":"success","number":1}]}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	transport := NewRESTTransport(RESTOptions{
		BaseURL:     srv.URL,
		TokenSource: tokenSourceFunc(func(context.Context) (string, error) { return "ghp_fake_token_1234567890", nil }),
		Sleep:       func(context.Context, time.Duration) error { return nil },
	})
	steps, err := transport.GetWorkflowSteps(context.Background(), RepoRef{Owner: "o", Name: "r"}, 9)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(steps) != 1 || steps[0].Name != "Compile" {
		t.Errorf("unexpected steps: %+v", steps)
	}
}

func TestRESTTransport_GetWorkflowLogs_UnsupportedFallback(t *testing.T) {
	t.Parallel()

	transport := NewRESTTransport(RESTOptions{
		BaseURL:     "https://example.invalid",
		TokenSource: tokenSourceFunc(func(context.Context) (string, error) { return "x", nil }),
		Sleep:       func(context.Context, time.Duration) error { return nil },
	})
	_, err := transport.GetWorkflowLogs(context.Background(), RepoRef{Owner: "o", Name: "r"}, 1, 10)
	if !errors.Is(err, ErrPATScopeInsufficient) {
		t.Fatalf("expected ErrPATScopeInsufficient sentinel, got %v", err)
	}
}

func TestParseGHLogLines_TabSeparation(t *testing.T) {
	t.Parallel()

	raw := []byte("job1\tstep1\tmsg one\njob2\tstep2\tmsg two")
	lines := parseGHLogLines(raw, 0)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[0].JobName != "job1" || lines[0].StepName != "step1" || lines[0].Raw != "msg one" {
		t.Errorf("unexpected parse: %+v", lines[0])
	}
}

func TestParseGHLogLines_RedactsSecrets(t *testing.T) {
	t.Parallel()

	raw := []byte("job\tstep\ttoken=ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx")
	lines := parseGHLogLines(raw, 0)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if strings.Contains(lines[0].Raw, "ghp_") {
		t.Errorf("expected redaction, got %q", lines[0].Raw)
	}
}

func TestWorkflowRunSummary_IsTerminal(t *testing.T) {
	t.Parallel()

	if (WorkflowRunSummary{Status: "in_progress"}).IsTerminal() {
		t.Error("in_progress must not be terminal")
	}
	if !(WorkflowRunSummary{Status: "completed"}).IsTerminal() {
		t.Error("completed must be terminal")
	}
}

func TestClient_GetWorkflowSteps_FallbackOnGHUnavailable(t *testing.T) {
	t.Parallel()

	primary := transportFunc{
		getWorkflowSteps: func(context.Context, RepoRef, int64) ([]Step, error) {
			return nil, ErrGHUnavailable
		},
	}
	fallback := transportFunc{
		getWorkflowSteps: func(context.Context, RepoRef, int64) ([]Step, error) {
			return []Step{{JobName: "build", Name: "fallback"}}, nil
		},
	}

	client := NewClient(Options{Primary: primary, Fallback: fallback})
	steps, err := client.GetWorkflowSteps(context.Background(), RepoRef{Owner: "o", Name: "r"}, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(steps) != 1 || steps[0].Name != "fallback" {
		t.Errorf("expected fallback step, got %+v", steps)
	}
}
