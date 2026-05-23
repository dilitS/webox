package main

import (
	"bytes"
	"context"
	"io"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/dilitS/webox/internal/version"
	doctorservice "github.com/dilitS/webox/services/doctor"
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
		name string
		args []string
	}{
		{"alone", []string{"--debug"}},
		{"before version", []string{"--debug", "--version"}},
		{"after version", []string{"--version", "--debug"}},
		{"twice", []string{"--debug", "--debug", "--help"}},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			got := Run(tt.args, &stdout, &stderr)
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
	got := Run(nil, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("exit code = %d, want 0", got)
	}
	if stdout.Len() == 0 {
		t.Error("stdout should not be empty (TUI stub or help)")
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
	got := runWith([]string{"doctor"}, &stdout, &stderr, stub)
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
			got := runWith(tt.args, &stdout, &stderr, stub)
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
