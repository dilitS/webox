package doctor

import (
	"bytes"
	"context"
	"io"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/fatih/color"

	"github.com/dilitS/webox/internal/version"
)

func TestNew_FillsDefaultsWhenOptionsZero(t *testing.T) {
	t.Parallel()

	doctor := New(nil, Options{})
	report := doctor.Run(context.Background())

	if report.Platform.OS != runtime.GOOS {
		t.Errorf("Platform.OS = %q, want runtime.GOOS=%q", report.Platform.OS, runtime.GOOS)
	}
	if report.Platform.Arch != runtime.GOARCH {
		t.Errorf("Platform.Arch = %q, want runtime.GOARCH=%q", report.Platform.Arch, runtime.GOARCH)
	}
	if report.Platform.GoVersion == "" {
		t.Error("Platform.GoVersion should default to runtime.Version()")
	}
	if report.WeboxVersion == "" {
		t.Error("WeboxVersion should default to effective version, got empty")
	}
	if report.GeneratedAt.IsZero() {
		t.Error("GeneratedAt should default to time.Now(), got zero value")
	}
}

func TestEffectiveWeboxVersion(t *testing.T) {
	original := version.Version
	t.Cleanup(func() { version.Version = original })

	version.Version = ""
	if got := effectiveWeboxVersion(); got != version.DefaultVersion {
		t.Errorf("empty version → %q, want %q", got, version.DefaultVersion)
	}

	version.Version = "v9.9.9-test"
	if got := effectiveWeboxVersion(); got != "v9.9.9-test" {
		t.Errorf("set version → %q, want v9.9.9-test", got)
	}
}

func TestNewDefault_SmokeRunReturnsReport(t *testing.T) {
	t.Parallel()

	report := NewDefault().Run(context.Background())
	if report.SchemaVersion != reportSchemaVersion {
		t.Fatalf("SchemaVersion = %d, want %d", report.SchemaVersion, reportSchemaVersion)
	}
	if len(report.Checks) == 0 {
		t.Fatal("default doctor produced zero checks")
	}
	total := report.Summary.OK + report.Summary.Warn + report.Summary.Fail + report.Summary.Skipped
	if total != len(report.Checks) {
		t.Fatalf("summary total %d != checks %d", total, len(report.Checks))
	}
}

func TestColorEnabled(t *testing.T) {
	if ColorEnabled(io.Discard) {
		t.Error("io.Discard is not a terminal, ColorEnabled should be false")
	}

	original := isTerminal
	t.Cleanup(func() { isTerminal = original })

	f, err := os.CreateTemp(t.TempDir(), "tty-probe-*")
	if err != nil {
		t.Fatalf("CreateTemp = %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })

	isTerminal = func(int) bool { return true }
	if !ColorEnabled(f) {
		t.Error("ColorEnabled should return true when writer is *os.File and isTerminal stub returns true")
	}

	isTerminal = func(int) bool { return false }
	if ColorEnabled(f) {
		t.Error("ColorEnabled should return false when isTerminal stub returns false")
	}
}

func TestStatusLabel_AllStatuses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status Status
		want   string
	}{
		{StatusOK, "OK"},
		{StatusWarn, "WARN"},
		{StatusFail, "FAIL"},
		{StatusSkipped, "SKIP"},
		{Status("unknown"), "SKIP"},
	}
	for _, tt := range tests {
		got, _ := statusLabel(tt.status)
		if got != tt.want {
			t.Errorf("statusLabel(%q) = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestWriteText_ColorEnabledEmitsAnsi(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("CLICOLOR_FORCE", "1")
	previous := color.NoColor
	color.NoColor = false
	t.Cleanup(func() { color.NoColor = previous })

	report := Report{
		WeboxVersion: "v0.1.0",
		Platform:     Platform{OS: "linux", Arch: "amd64", GoVersion: "go1.24.0"},
		GeneratedAt:  time.Date(2026, 5, 23, 0, 32, 0, 0, time.UTC),
		Checks: []Result{
			{ID: "x.fail", Category: "system", Severity: SeverityFatal, Status: StatusFail, Message: "boom", Hint: "see docs"},
		},
		Summary: Summary{Fail: 1},
	}

	var out bytes.Buffer
	if err := WriteText(&out, report, TextOptions{Color: true}); err != nil {
		t.Fatalf("WriteText(color=true) = %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "\x1b[") {
		t.Errorf("expected ANSI escapes in colored output, got %q", text)
	}
	if !strings.Contains(text, "FAIL") {
		t.Errorf("expected FAIL label in output, got %q", text)
	}
	if !strings.Contains(text, "(hint: see docs)") {
		t.Errorf("expected hint suffix in output, got %q", text)
	}
}

func TestWriteText_RenderErrorPropagates(t *testing.T) {
	t.Parallel()

	report := Report{
		Checks: []Result{
			{ID: "x", Status: StatusOK, Message: "ok"},
		},
	}

	cases := []struct {
		name string
		w    io.Writer
	}{
		{"header fails", brokenWriterAfter(0)},
		{"check line fails", brokenWriterAfter(1)},
		{"summary fails", brokenWriterAfter(2)},
	}
	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := WriteText(tt.w, report, TextOptions{Color: false}); err == nil {
				t.Fatalf("WriteText(%s) returned nil, want error", tt.name)
			}
		})
	}
}

// brokenWriterAfter returns an io.Writer that succeeds for the first n
// Write calls and fails afterwards. Used to drive each error branch in
// WriteText / writeTextCheck independently.
func brokenWriterAfter(n int) io.Writer {
	return &countingFailWriter{successesLeft: n}
}

type countingFailWriter struct {
	successesLeft int
}

func (c *countingFailWriter) Write(p []byte) (int, error) {
	if c.successesLeft <= 0 {
		return 0, io.ErrShortWrite
	}
	c.successesLeft--
	return len(p), nil
}
