package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/dilitS/webox/internal/version"
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
			for _, needle := range []string{"webox", "Usage", "--version", "--help", "--debug", "docs"} {
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
