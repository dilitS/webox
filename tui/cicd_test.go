package tui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dilitS/webox/config"
	ghsvc "github.com/dilitS/webox/services/github"
	"github.com/dilitS/webox/status"
	"github.com/dilitS/webox/tui/bento"
)

func testGitHubProject() config.Project {
	return config.Project{
		ID:           "p1",
		ProfileAlias: "main",
		Domain:       "app.example.com",
		Repo:         "dilitS/webox",
		NodeVersion:  "22",
	}
}

func TestCICDStatusFromGitHub_MapsKnownPairs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		status     string
		conclusion string
		want       bento.CICDStatus
	}{
		{"completed", "success", bento.CICDStatusSuccess},
		{"completed", "failure", bento.CICDStatusFailure},
		{"completed", "timed_out", bento.CICDStatusFailure},
		{"completed", "cancelled", bento.CICDStatusCancelled},
		{"completed", "skipped", bento.CICDStatusSkipped},
		{"in_progress", "", bento.CICDStatusInProgress},
		{"queued", "", bento.CICDStatusQueued},
		{"", "", bento.CICDStatusUnknown},
	}
	for _, tc := range cases {
		got := cicdStatusFromGitHub(tc.status, tc.conclusion)
		if got != tc.want {
			t.Errorf("(%q,%q) got %v want %v", tc.status, tc.conclusion, got, tc.want)
		}
	}
}

func TestFormatStepDuration(t *testing.T) {
	t.Parallel()

	if got := formatStepDuration(0, "completed"); got != "" {
		t.Errorf("zero ms should return empty, got %q", got)
	}
	if got := formatStepDuration(500, "completed"); got != "<1s" {
		t.Errorf("sub-second should be <1s, got %q", got)
	}
	if got := formatStepDuration(5_000, "completed"); got != "5s" {
		t.Errorf("5000ms should be 5s, got %q", got)
	}
	if got := formatStepDuration(0, "in_progress"); got != "running" {
		t.Errorf("in_progress should be running, got %q", got)
	}
}

func TestFormatResetDelta(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input time.Duration
		want  string
	}{
		{0, "<1min"},
		{30 * time.Second, "<1min"},
		{2 * time.Minute, "2min"},
		{90 * time.Minute, "1h30m"},
		{2 * time.Hour, "2h"},
	}
	for _, tc := range cases {
		got := formatResetDelta(tc.input)
		if got != tc.want {
			t.Errorf("%v: got %q want %q", tc.input, got, tc.want)
		}
	}
}

func TestCICDPipelineCacheKey_UsesPrefixAndWorkflow(t *testing.T) {
	t.Parallel()

	got := cicdPipelineCacheKey(ghsvc.RepoRef{Owner: "owner", Name: "repo"}, "deploy.yml")
	want := status.PrefixGitHubSteps + "owner/repo:deploy.yml"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	// Empty workflow falls back to default file.
	got = cicdPipelineCacheKey(ghsvc.RepoRef{Owner: "owner", Name: "repo"}, "")
	if !strings.HasSuffix(got, ":"+DefaultWorkflowFile) {
		t.Fatalf("expected default workflow suffix, got %q", got)
	}
}

func TestBuildCICDPipelineSnapshot_Placeholder(t *testing.T) {
	t.Parallel()

	m := New(Options{ConfigPath: "/tmp/cfg.json"})
	m.cfg = &config.Config{Projects: []config.Project{testGitHubProject()}}
	m.selectedIndex = 0
	snap, ok := buildCICDPipelineSnapshot(m)
	if !ok {
		t.Fatal("expected snapshot for GitHub-linked project")
	}
	if snap.ProjectAlias != "app.example.com" || snap.WorkflowName != DefaultWorkflowFile {
		t.Errorf("unexpected snapshot: %+v", snap)
	}
	if snap.RunNumber != 0 {
		t.Errorf("expected zero run number before first poll, got %d", snap.RunNumber)
	}
}

func TestBuildCICDPipelineSnapshot_NoGitHubRepo_ReturnsFalse(t *testing.T) {
	t.Parallel()

	m := New(Options{ConfigPath: "/tmp/cfg.json"})
	m.cfg = &config.Config{Projects: []config.Project{{ID: "p1", Domain: "x"}}}
	m.selectedIndex = 0
	if _, ok := buildCICDPipelineSnapshot(m); ok {
		t.Fatal("expected no snapshot for project without Repo field")
	}
}

func TestApplyCICDFetched_RateLimitedKeepsCachedSteps(t *testing.T) {
	t.Parallel()

	project := testGitHubProject()
	m := New(Options{ConfigPath: "/tmp/cfg.json"})
	m.cfg = &config.Config{Projects: []config.Project{project}}
	m.selectedIndex = 0

	now := time.Now()
	m.cicdSnapshots[project.ID] = cicdSnapshotEntry{
		Run:   &cicdRunSummary{RunID: 7, RunNumber: 7, Status: "completed", Conclusion: "success", HeaderTime: now},
		Steps: []ghsvc.Step{{Number: 1, Name: "Build", Status: "completed", Conclusion: "success"}},
	}

	updated, _ := m.applyCICDFetched(CICDFetchedMsg{
		ProjectID: project.ID,
		Err:       ghsvc.ErrRateLimited,
		FetchedAt: now,
	})
	updatedM := updated.(Model)
	entry := updatedM.cicdSnapshots[project.ID]
	if !entry.RateLimited {
		t.Fatal("rate-limit response should flip RateLimited flag")
	}
	if entry.Run == nil || len(entry.Steps) != 1 {
		t.Fatal("rate-limit response must preserve cached run/steps (SWR)")
	}

	snap, _ := buildCICDPipelineSnapshot(updatedM)
	if !snap.RateLimited {
		t.Fatal("snapshot should propagate rate-limit flag to renderer")
	}
}

func TestApplyCICDFetched_RunNotFoundClearsCachedRun(t *testing.T) {
	t.Parallel()

	project := testGitHubProject()
	m := New(Options{ConfigPath: "/tmp/cfg.json"})
	m.cfg = &config.Config{Projects: []config.Project{project}}
	m.selectedIndex = 0
	m.cicdSnapshots[project.ID] = cicdSnapshotEntry{
		Run: &cicdRunSummary{RunID: 1, RunNumber: 1},
		Err: "stale error",
	}
	updated, _ := m.applyCICDFetched(CICDFetchedMsg{
		ProjectID: project.ID,
		Err:       ghsvc.ErrRunNotFound,
		FetchedAt: time.Now(),
	})
	entry := updated.(Model).cicdSnapshots[project.ID]
	if entry.Run != nil || entry.Err != "" {
		t.Fatalf("ErrRunNotFound should clear cached run/error, got %+v", entry)
	}
}

func TestApplyCICDFetched_SuccessReplacesCache(t *testing.T) {
	t.Parallel()

	project := testGitHubProject()
	m := New(Options{ConfigPath: "/tmp/cfg.json"})
	m.cfg = &config.Config{Projects: []config.Project{project}}
	m.selectedIndex = 0

	started := time.Now().Add(-2 * time.Minute)
	run := &ghsvc.WorkflowRun{
		ID:         42,
		RunNumber:  413,
		Status:     "completed",
		Conclusion: "success",
		UpdatedAt:  time.Now(),
		StartedAt:  &started,
	}
	updated, _ := m.applyCICDFetched(CICDFetchedMsg{
		ProjectID: project.ID,
		Result: PipelineFetchResult{
			Run:   run,
			Steps: []ghsvc.Step{{Number: 1, Name: "Lint", Status: "completed", Conclusion: "success", DurationMs: 5000}},
		},
		FetchedAt: time.Now(),
	})
	entry := updated.(Model).cicdSnapshots[project.ID]
	if entry.Run == nil || entry.Run.RunNumber != 413 {
		t.Fatalf("expected RunNumber 413, got %+v", entry.Run)
	}
	if len(entry.Steps) != 1 || entry.Steps[0].Name != "Lint" {
		t.Fatalf("expected Lint step, got %+v", entry.Steps)
	}
}

func TestOpenCICDLogsModal_NoRun(t *testing.T) {
	t.Parallel()

	m := New(Options{
		ConfigPath: "/tmp/cfg.json",
		GitHubLogs: func(context.Context, ghsvc.RepoRef, int64, int) ([]ghsvc.WorkflowLogLine, error) {
			return nil, nil
		},
	})
	m.cfg = &config.Config{Projects: []config.Project{testGitHubProject()}}
	m.selectedIndex = 0
	updated, _ := m.openCICDLogsModal()
	if updated.(Model).cicdModal.Open {
		t.Fatal("modal must remain closed when no run is cached")
	}
	if !strings.Contains(updated.(Model).alert, "no workflow run") {
		t.Errorf("expected alert about missing run, got %q", updated.(Model).alert)
	}
}

func TestOpenCICDLogsModal_OpensWhenRunAvailable(t *testing.T) {
	t.Parallel()

	project := testGitHubProject()
	logsCalled := make(chan struct{}, 1)
	m := New(Options{
		ConfigPath: "/tmp/cfg.json",
		GitHubLogs: func(_ context.Context, _ ghsvc.RepoRef, runID int64, _ int) ([]ghsvc.WorkflowLogLine, error) {
			if runID != 99 {
				t.Errorf("expected runID 99, got %d", runID)
			}
			select {
			case logsCalled <- struct{}{}:
			default:
			}
			return []ghsvc.WorkflowLogLine{{JobName: "build", StepName: "Lint", Raw: "ok"}}, nil
		},
	})
	m.cfg = &config.Config{Projects: []config.Project{project}}
	m.selectedIndex = 0
	m.cicdSnapshots[project.ID] = cicdSnapshotEntry{
		Run: &cicdRunSummary{RunID: 99, RunNumber: 99, Status: "completed", Conclusion: "success"},
	}

	updated, cmd := m.openCICDLogsModal()
	if !updated.(Model).cicdModal.Open {
		t.Fatal("modal should be open after openCICDLogsModal")
	}
	if cmd == nil {
		t.Fatal("expected a command to load logs")
	}
	msg := cmd()
	logsMsg, ok := msg.(CICDLogsFetchedMsg)
	if !ok {
		t.Fatalf("expected CICDLogsFetchedMsg, got %T", msg)
	}
	if logsMsg.RunID != 99 || len(logsMsg.Lines) != 1 {
		t.Fatalf("unexpected logs payload: %+v", logsMsg)
	}
	final := updated.(Model).applyCICDLogsFetched(logsMsg)
	if final.cicdModal.Loading {
		t.Error("modal should not stay in loading state after logs arrive")
	}
	if len(final.cicdModal.Lines) != 1 || final.cicdModal.Lines[0].Text != "ok" {
		t.Errorf("expected logs to populate, got %+v", final.cicdModal.Lines)
	}
}

func TestUpdateDashboardKey_F8OpensModal(t *testing.T) {
	t.Parallel()

	project := testGitHubProject()
	m := New(Options{
		ConfigPath: "/tmp/cfg.json",
		GitHubLogs: func(context.Context, ghsvc.RepoRef, int64, int) ([]ghsvc.WorkflowLogLine, error) {
			return nil, nil
		},
	})
	m.cfg = &config.Config{Projects: []config.Project{project}}
	m.selectedIndex = 0
	m.cicdSnapshots[project.ID] = cicdSnapshotEntry{
		Run: &cicdRunSummary{RunID: 5, RunNumber: 5, Status: "completed", Conclusion: "success"},
	}
	updated, _ := m.updateDashboardKey(tea.KeyMsg{Type: tea.KeyF8})
	if !updated.(Model).cicdModal.Open {
		t.Fatal("F8 should open the CI/CD logs modal")
	}
}

func TestUpdateCICDModalKey_EscClosesModal(t *testing.T) {
	t.Parallel()

	m := New(Options{ConfigPath: "/tmp/cfg.json"})
	m.cicdModal = cicdLogsModalForm{Open: true, ProjectID: "p1"}
	updated, _ := m.updateCICDModalKey(tea.KeyMsg{Type: tea.KeyEsc})
	if updated.(Model).cicdModal.Open {
		t.Fatal("Esc should dismiss the modal")
	}
}

func TestUpdateCICDModalKey_ArrowKeysScroll(t *testing.T) {
	t.Parallel()

	m := New(Options{ConfigPath: "/tmp/cfg.json"})
	m.cicdModal = cicdLogsModalForm{
		Open:  true,
		Lines: []CICDLogLineSnapshot{{Text: "a"}, {Text: "b"}, {Text: "c"}},
	}
	down, _ := m.updateCICDModalKey(tea.KeyMsg{Type: tea.KeyDown})
	if down.(Model).cicdModal.ScrollOffset != 1 {
		t.Fatalf("expected scroll offset 1, got %d", down.(Model).cicdModal.ScrollOffset)
	}
	up, _ := down.(Model).updateCICDModalKey(tea.KeyMsg{Type: tea.KeyUp})
	if up.(Model).cicdModal.ScrollOffset != 0 {
		t.Fatalf("expected scroll offset 0 after Up, got %d", up.(Model).cicdModal.ScrollOffset)
	}
}

func TestInvalidateCICDCacheForProject_RemovesEntry(t *testing.T) {
	t.Parallel()

	cache := status.NewCache(status.Options{})
	project := testGitHubProject()
	ref, ok := parseRepoRef(project.Repo)
	if !ok {
		t.Fatal("parseRepoRef failed for test project")
	}
	key := cicdPipelineCacheKey(ref, DefaultWorkflowFile)
	_, _, err := status.GetOrFetch[int](cache, key, time.Minute, func(context.Context) (int, error) { return 1, nil }, context.Background())
	if err != nil {
		t.Fatalf("seed cache: %v", err)
	}
	invalidateCICDCacheForProject(cache, project)
	// Second fetch should miss → call fetcher again.
	called := false
	_, _, err = status.GetOrFetch[int](cache, key, time.Minute, func(context.Context) (int, error) {
		called = true
		return 2, nil
	}, context.Background())
	if err != nil {
		t.Fatalf("refetch: %v", err)
	}
	if !called {
		t.Fatal("invalidateCICDCacheForProject should force a refetch")
	}
}

func TestExtractRateLimitInfo_GenericFallback(t *testing.T) {
	t.Parallel()

	limited, hint := extractRateLimitInfo(errors.New("non-rate limit error"))
	if limited {
		t.Error("non-rate-limit error must not set limited flag")
	}
	if hint != "" {
		t.Error("non-rate-limit error must not produce hint")
	}

	limited, hint = extractRateLimitInfo(ghsvc.ErrRateLimited)
	if !limited || hint == "" {
		t.Errorf("ErrRateLimited should produce hint, got limited=%v hint=%q", limited, hint)
	}
}

func TestApplyCICDFetched_GenericErrorPreservesPreviousData(t *testing.T) {
	t.Parallel()

	project := testGitHubProject()
	m := New(Options{ConfigPath: "/tmp/cfg.json"})
	m.cfg = &config.Config{Projects: []config.Project{project}}
	m.cicdSnapshots[project.ID] = cicdSnapshotEntry{
		Run: &cicdRunSummary{RunID: 1, RunNumber: 1, Status: "completed", Conclusion: "success"},
	}
	updated, _ := m.applyCICDFetched(CICDFetchedMsg{
		ProjectID: project.ID,
		Err:       errors.New("transport boom"),
		FetchedAt: time.Now(),
	})
	entry := updated.(Model).cicdSnapshots[project.ID]
	if entry.Run == nil || entry.Run.RunNumber != 1 {
		t.Fatalf("generic error must preserve previous run, got %+v", entry.Run)
	}
	if !strings.Contains(entry.Err, "transport boom") {
		t.Fatalf("expected error message stored, got %q", entry.Err)
	}
	if !entry.Stale {
		t.Error("generic error should flip Stale flag for renderer badge")
	}
}
