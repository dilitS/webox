package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync/atomic"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dilitS/webox/internal/version"
	doctorservice "github.com/dilitS/webox/services/doctor"
	ghsvc "github.com/dilitS/webox/services/github"
	"github.com/dilitS/webox/tui"
)

func TestRun_Version_PrintsVersionLine(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	got := Run([]string{"--version"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", got, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr should be empty for --version, got %q", stderr.String())
	}

	want := version.String() + "\n"
	if stdout.String() != want {
		t.Errorf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestRun_Help_PrintsUsageAndDocsLink(t *testing.T) {
	t.Parallel()

	cases := []string{"--help", "-h"}
	for _, flag := range cases {
		flag := flag
		t.Run(flag, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			got := Run([]string{flag}, &stdout, &stderr)

			if got != 0 {
				t.Fatalf("exit code = %d, want 0; stderr=%q", got, stderr.String())
			}
			out := stdout.String()
			for _, needle := range []string{"webox", "Usage", "--version", "--help", "--debug", "doctor", "docs"} {
				if !strings.Contains(out, needle) {
					t.Errorf("help output missing %q\n--- output ---\n%s", needle, out)
				}
			}
		})
	}
}

func TestRun_Debug_AcceptedAsModifier_DoesNotEscape(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		useStubTUI bool
	}{
		{"alone", []string{"--debug"}, true},
		{"before version", []string{"--debug", "--version"}, false},
		{"after version", []string{"--version", "--debug"}, false},
		{"twice", []string{"--debug", "--debug", "--help"}, false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			var got int
			if tt.useStubTUI {
				got = runWithDeps(tt.args, &stdout, &stderr, runDoctor, func(stdout, stderr io.Writer) int {
					_, _ = stdout.Write([]byte("tui started\n"))
					return 0
				})
			} else {
				got = Run(tt.args, &stdout, &stderr)
			}
			if got != 0 {
				t.Fatalf("Run(%v) exit = %d, want 0; stderr=%q", tt.args, got, stderr.String())
			}
			if stderr.Len() != 0 {
				t.Errorf("Run(%v) stderr should be empty, got %q", tt.args, stderr.String())
			}
		})
	}
}

func TestRun_NoArgs_PrintsHelpStubExitsZero(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	stub := func(stdout, stderr io.Writer) int {
		_, _ = stdout.Write([]byte("tui started\n"))
		return 0
	}
	got := runWithDeps(nil, &stdout, &stderr, runDoctor, stub)
	if got != 0 {
		t.Fatalf("exit code = %d, want 0", got)
	}
	if stdout.String() != "tui started\n" {
		t.Fatalf("stdout = %q, want TUI stub output", stdout.String())
	}
}

func TestRun_UnknownFlag_ReturnsTwoAndExplains(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{"unknown long flag", []string{"--bogus"}},
		{"unknown short flag", []string{"-x"}},
		{"unknown positional", []string{"unknown-cmd"}},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			got := Run(tt.args, &stdout, &stderr)
			if got != 2 {
				t.Fatalf("Run(%v) exit = %d, want 2; stderr=%q", tt.args, got, stderr.String())
			}
			if !strings.Contains(stderr.String(), "webox") {
				t.Errorf("stderr should mention webox, got %q", stderr.String())
			}
			if !strings.Contains(stderr.String(), "--help") {
				t.Errorf("stderr should hint at --help, got %q", stderr.String())
			}
		})
	}
}

func TestRunWith_DoctorDispatchesText(t *testing.T) {
	t.Parallel()

	stub, calls := stubDispatcher(t, 1, false, "doctor text\n", "")

	var stdout, stderr bytes.Buffer
	got := runWithDeps([]string{"doctor"}, &stdout, &stderr, stub, brokenTUI)
	if got != 1 {
		t.Fatalf("runWith(doctor) exit = %d, want 1", got)
	}
	if calls.Load() != 1 {
		t.Fatalf("dispatcher called %d times, want 1", calls.Load())
	}
	if stdout.String() != "doctor text\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunWith_DoctorDispatchesJSON(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
	}{
		{"flag after doctor", []string{"doctor", "--json"}},
		{"flag before doctor", []string{"--json", "doctor"}},
	}
	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stub, calls := stubDispatcher(t, 0, true, "{\"summary\":{}}\n", "")

			var stdout, stderr bytes.Buffer
			got := runWithDeps(tt.args, &stdout, &stderr, stub, brokenTUI)
			if got != 0 {
				t.Fatalf("runWith(%v) exit = %d, want 0; stderr=%q", tt.args, got, stderr.String())
			}
			if calls.Load() != 1 {
				t.Fatalf("dispatcher called %d times, want 1", calls.Load())
			}
			if !strings.Contains(stdout.String(), `"summary"`) {
				t.Fatalf("stdout = %q", stdout.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}

func TestRunWith_DoctorGitHubRoutesToGitHubDispatcher(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		args     []string
		wantJSON bool
	}{
		{"text", []string{"doctor", "github"}, false},
		{"json after target", []string{"doctor", "github", "--json"}, true},
		{"json before target", []string{"doctor", "--json", "github"}, true},
	}
	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var ghCalls atomic.Int32
			var gotJSON atomic.Bool
			ghStub := func(jsonOutput bool, stdout, stderr io.Writer) int {
				ghCalls.Add(1)
				gotJSON.Store(jsonOutput)
				_, _ = stdout.Write([]byte("gh doctor\n"))
				return 0
			}
			coreStub, coreCalls := stubDispatcher(t, 0, false, "core doctor\n", "")

			var stdout, stderr bytes.Buffer
			got := runWithFullDeps(tt.args, &stdout, &stderr, coreStub, ghStub, brokenTUI)
			if got != 0 {
				t.Fatalf("exit = %d, want 0; stderr=%q", got, stderr.String())
			}
			if ghCalls.Load() != 1 {
				t.Fatalf("github dispatcher called %d, want 1", ghCalls.Load())
			}
			if coreCalls.Load() != 0 {
				t.Fatalf("core dispatcher should not be called for github target, got %d", coreCalls.Load())
			}
			if gotJSON.Load() != tt.wantJSON {
				t.Fatalf("json flag forwarded = %t, want %t", gotJSON.Load(), tt.wantJSON)
			}
		})
	}
}

func TestRun_JSONWithoutDoctorReturnsMisuse(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	got := Run([]string{"--json"}, &stdout, &stderr)
	if got != exitMisuse {
		t.Fatalf("Run(--json) exit = %d, want %d", got, exitMisuse)
	}
	if !strings.Contains(stderr.String(), "only valid with `webox doctor`") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunDoctorWith_PropagatesExitCodeAndChoosesRenderer(t *testing.T) {
	t.Parallel()

	report := doctorservice.Report{Summary: doctorservice.Summary{Warn: 1}}

	t.Run("text", func(t *testing.T) {
		t.Parallel()

		var stdout, stderr bytes.Buffer
		got := runDoctorWith(false, &stdout, &stderr, stubRunner{report: report})
		if got != 1 {
			t.Fatalf("runDoctorWith(text) exit = %d, want 1", got)
		}
		if !strings.Contains(stdout.String(), "webox doctor") {
			t.Fatalf("text stdout missing header: %q", stdout.String())
		}
		if !strings.Contains(stdout.String(), "summary: 0 ok, 1 warn") {
			t.Fatalf("text stdout missing summary line: %q", stdout.String())
		}
	})

	t.Run("json", func(t *testing.T) {
		t.Parallel()

		var stdout, stderr bytes.Buffer
		got := runDoctorWith(true, &stdout, &stderr, stubRunner{report: report})
		if got != 1 {
			t.Fatalf("runDoctorWith(json) exit = %d, want 1", got)
		}
		if !strings.Contains(stdout.String(), `"warn": 1`) {
			t.Fatalf("json stdout missing summary: %q", stdout.String())
		}
		if stderr.Len() != 0 {
			t.Fatalf("stderr = %q, want empty", stderr.String())
		}
	})
}

func TestRunDoctorWith_RenderErrorGoesToStderr(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	got := runDoctorWith(false, brokenWriter{}, &stderr, stubRunner{})
	if got != exitMisuse {
		t.Fatalf("exit = %d, want %d", got, exitMisuse)
	}
	if !strings.Contains(stderr.String(), "render report") {
		t.Fatalf("stderr = %q, want render-report message", stderr.String())
	}
}

func stubDispatcher(t *testing.T, exit int, wantJSON bool, stdoutOut, stderrOut string) (doctorDispatcher, *atomic.Int32) {
	t.Helper()

	var calls atomic.Int32
	dispatch := func(jsonOutput bool, stdout, stderr io.Writer) int {
		calls.Add(1)
		if jsonOutput != wantJSON {
			t.Errorf("dispatcher called with json=%t, want %t", jsonOutput, wantJSON)
		}
		if stdoutOut != "" {
			_, _ = stdout.Write([]byte(stdoutOut))
		}
		if stderrOut != "" {
			_, _ = stderr.Write([]byte(stderrOut))
		}
		return exit
	}
	return dispatch, &calls
}

func brokenTUI(stdout, stderr io.Writer) int {
	_, _ = stderr.Write([]byte("unexpected tui dispatch"))
	return 99
}

type stubRunner struct {
	report doctorservice.Report
}

func (s stubRunner) Run(_ context.Context) doctorservice.Report {
	return s.report
}

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) {
	return 0, io.ErrShortWrite
}

// stubTeaRunner is the in-memory teaRunner used by runTUIWith tests
// to assert the TUI dispatcher wires the model + writers without
// spinning up a real Bubble Tea program (which would race against
// stdin).
type stubTeaRunner struct {
	err error
}

func (s stubTeaRunner) Run() (tea.Model, error) { return nil, s.err }

func TestParseArgs_DoctorTargetCannotBeChangedTwice(t *testing.T) {
	t.Parallel()
	_, errMsg := parseArgs([]string{"doctor", "github", "github"})
	if errMsg != "" {
		t.Fatalf("second `github` should be idempotent, got %q", errMsg)
	}
}

func TestParseArgs_GithubBeforeDoctorIsMisuse(t *testing.T) {
	t.Parallel()
	_, errMsg := parseArgs([]string{"github"})
	if !strings.Contains(errMsg, "only valid after `doctor`") {
		t.Fatalf("errMsg = %q, want hint about doctor", errMsg)
	}
}

func TestRunTUIWith_ResolveConfigFailureReturnsMisuse(t *testing.T) {
	t.Parallel()

	resolveErr := errors.New("home directory unset")
	resolve := func() (string, error) { return "", resolveErr }
	stubProgram := func(_ tea.Model, _ io.Writer) teaRunner { return stubTeaRunner{} }

	var stdout, stderr bytes.Buffer
	got := runTUIWith(&stdout, &stderr, resolve, stubProgram, nil)
	if got != exitMisuse {
		t.Fatalf("exit = %d, want %d", got, exitMisuse)
	}
	if !strings.Contains(stderr.String(), "resolve config path") {
		t.Fatalf("stderr = %q, want resolve hint", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestRunTUIWith_ProgramFailureReturnsMisuse(t *testing.T) {
	t.Parallel()

	resolve := func() (string, error) { return "/tmp/config.json", nil }
	stubProgram := func(_ tea.Model, _ io.Writer) teaRunner {
		return stubTeaRunner{err: errors.New("tea: program crashed")}
	}

	var stdout, stderr bytes.Buffer
	got := runTUIWith(&stdout, &stderr, resolve, stubProgram, nil)
	if got != exitMisuse {
		t.Fatalf("exit = %d, want %d", got, exitMisuse)
	}
	if !strings.Contains(stderr.String(), "TUI failed") {
		t.Fatalf("stderr = %q, want TUI failed hint", stderr.String())
	}
}

func TestRunTUIWith_HappyPathReturnsOK(t *testing.T) {
	t.Parallel()

	resolve := func() (string, error) { return "/tmp/config.json", nil }

	var receivedModel tea.Model
	stubProgram := func(model tea.Model, _ io.Writer) teaRunner {
		receivedModel = model
		return stubTeaRunner{}
	}

	var stdout, stderr bytes.Buffer
	got := runTUIWith(&stdout, &stderr, resolve, stubProgram, nil)
	if got != exitOK {
		t.Fatalf("exit = %d, want %d", got, exitOK)
	}
	if receivedModel == nil {
		t.Fatal("teaProgramFactory was never called with a model")
	}
}

func TestDefaultGitHubLastDeployFetcher_ProducesNonNilFunc(t *testing.T) {
	t.Parallel()

	fetcher := defaultGitHubLastDeployFetcher()
	if fetcher == nil {
		t.Fatal("defaultGitHubLastDeployFetcher returned nil")
	}

	// Invoke with a deliberately bogus ref so we exercise the
	// transport error path rather than relying on live GitHub.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := fetcher(ctx, ghsvc.RepoRef{Owner: "owner", Name: "name"}, "deploy.yml")
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
}

// Ensure the seam types are still exported through compile-time
// usage so the linter does not flag them as unused.
var _ = tui.GitHubLastDeployFetcher(nil)
