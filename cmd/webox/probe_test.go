package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/dilitS/webox/presets"
)

// TestSummarizeProbe_EmptyInput documents the contract for the zero-probe
// edge case: callers pass an empty results slice (preset declared no
// probes, or all probes were skipped). The summary must surface that
// state explicitly instead of returning a misleading 100 % confidence
// score from a 0/0 ratio.
func TestSummarizeProbe_EmptyInput(t *testing.T) {
	t.Parallel()

	s := summarizeProbe("smallhost-devil", "small.pl / Devil", "demo.smallhost.pl", "operator", nil)
	if s.OK != 0 || s.Mismatch != 0 || s.Failed != 0 {
		t.Fatalf("expected zero counters, got OK=%d Mismatch=%d Failed=%d", s.OK, s.Mismatch, s.Failed)
	}
	if s.Confidence != 0 {
		t.Fatalf("expected 0 confidence for empty input, got %d", s.Confidence)
	}
	if s.PresetID != "smallhost-devil" || s.Host != "demo.smallhost.pl" || s.User != "operator" {
		t.Fatalf("unexpected metadata: %+v", s)
	}
}

// TestSummarizeProbe_AllOK exercises the happy path: every probe returned
// exit code 0, so the confidence score is 100 and the OK counter equals
// the input length. This is the v0.2 baseline acceptance signal — preset
// metadata matches the live host.
func TestSummarizeProbe_AllOK(t *testing.T) {
	t.Parallel()

	results := []ProbeResult{
		{Command: "node --version", ExitCode: 0, Stdout: "v20.11.0"},
		{Command: "php --version", ExitCode: 0, Stdout: "PHP 8.2.10"},
		{Command: "nginx -v", ExitCode: 0, Stderr: "nginx version: nginx/1.24.0"},
	}
	s := summarizeProbe("preset-x", "Preset X", "h.example", "u", results)

	if s.OK != 3 || s.Mismatch != 0 || s.Failed != 0 {
		t.Fatalf("counters: %+v", s)
	}
	if s.Confidence != 100 {
		t.Fatalf("expected 100 confidence, got %d", s.Confidence)
	}
	for i, r := range s.Results {
		if r.Status != ProbeOK {
			t.Fatalf("result[%d] status = %q, want %q", i, r.Status, ProbeOK)
		}
	}
}

// TestSummarizeProbe_MixedExitCodes covers the realistic operator
// scenario: 2/3 probes succeed, 1 returns non-zero. Confidence is the
// integer ratio of OK probes to total, rounded down — 66 not 67.
// Rounding choice is documented so the formatter never bumps the score
// to flatter the operator.
func TestSummarizeProbe_MixedExitCodes(t *testing.T) {
	t.Parallel()

	results := []ProbeResult{
		{Command: "node --version", ExitCode: 0, Stdout: "v20.11.0"},
		{Command: "php --version", ExitCode: 0, Stdout: "PHP 8.2.10"},
		{Command: "nginx -v", ExitCode: 1, Stderr: "command not found"},
	}
	s := summarizeProbe("preset-x", "Preset X", "h.example", "u", results)

	if s.OK != 2 || s.Mismatch != 1 || s.Failed != 0 {
		t.Fatalf("counters: OK=%d Mismatch=%d Failed=%d", s.OK, s.Mismatch, s.Failed)
	}
	if s.Confidence != 66 {
		t.Fatalf("expected 66 confidence (round down), got %d", s.Confidence)
	}
	if s.Results[2].Status != ProbeMismatch {
		t.Fatalf("result[2] status = %q, want %q", s.Results[2].Status, ProbeMismatch)
	}
}

// TestSummarizeProbe_ExecutionError separates a non-zero exit (mismatch
// in declarative-vs-live) from an outright SSH / network failure
// (Err set). Failed probes are counted distinctly so the operator
// can tell "the panel ran the command and disagreed with the preset"
// from "we never reached the panel".
func TestSummarizeProbe_ExecutionError(t *testing.T) {
	t.Parallel()

	results := []ProbeResult{
		{Command: "node --version", ExitCode: 0, Stdout: "v20"},
		{Command: "php --version", Err: errors.New("dial tcp: connection refused")},
	}
	s := summarizeProbe("preset-x", "Preset X", "h", "u", results)

	if s.OK != 1 || s.Mismatch != 0 || s.Failed != 1 {
		t.Fatalf("counters: %+v", s)
	}
	if s.Confidence != 50 {
		t.Fatalf("expected 50 confidence, got %d", s.Confidence)
	}
	if s.Results[1].Status != ProbeFailed {
		t.Fatalf("result[1] status = %q, want %q", s.Results[1].Status, ProbeFailed)
	}
}

// TestFormatProbeText_ContainsAllKeyFields locks in the human-readable
// output contract: every probe shows command + status + truncated
// stdout/stderr, the trailer carries the confidence score, and the
// preset id appears in the header.
func TestFormatProbeText_ContainsAllKeyFields(t *testing.T) {
	t.Parallel()

	s := summarizeProbe(
		"smallhost-devil",
		"small.pl / Devil",
		"demo.smallhost.pl",
		"operator",
		[]ProbeResult{
			{Command: "devil --version", ExitCode: 0, Stdout: "Devil CLI 2.4.1", Duration: 142 * time.Millisecond},
			{Command: "devil node list", ExitCode: 0, Stdout: "node20 v20.11.0\nnode18 v18.18.0", Duration: 380 * time.Millisecond},
		},
	)
	out := formatProbeText(s)

	for _, needle := range []string{
		"smallhost-devil",
		"small.pl / Devil",
		"demo.smallhost.pl",
		"operator",
		"devil --version",
		"devil node list",
		"Devil CLI 2.4.1",
		"Confidence: 100",
		"OK",
	} {
		if !strings.Contains(out, needle) {
			t.Fatalf("formatProbeText missing %q in:\n%s", needle, out)
		}
	}
}

// TestFormatProbeText_TruncatesLongStdout caps each probe's stdout/stderr
// preview at probeOutputPreviewLimit characters so a noisy command (e.g.
// `devil www list` on a 200-domain account) does not blow the terminal
// width when --json is not used.
func TestFormatProbeText_TruncatesLongStdout(t *testing.T) {
	t.Parallel()

	huge := strings.Repeat("x", probeOutputPreviewLimit*3)
	s := summarizeProbe(
		"preset-x", "Preset X", "h", "u",
		[]ProbeResult{
			{Command: "noisy", ExitCode: 0, Stdout: huge},
		},
	)
	out := formatProbeText(s)
	if !strings.Contains(out, "(truncated)") {
		t.Fatalf("expected `(truncated)` marker for stdout > probeOutputPreviewLimit, got:\n%s", out)
	}
	if strings.Count(out, "x") > probeOutputPreviewLimit*2 {
		t.Fatalf("formatter did not actually truncate; output length = %d", len(out))
	}
}

// TestFormatProbeJSON_Structure documents the machine-readable schema.
// Consumers (CI scripts, future cockpit panels) rely on these field
// names; bumping them is a breaking change that requires a CHANGELOG
// entry and ideally a `--json --schema-version` bump.
func TestFormatProbeJSON_Structure(t *testing.T) {
	t.Parallel()

	s := summarizeProbe(
		"smallhost-devil", "small.pl / Devil", "demo.smallhost.pl", "operator",
		[]ProbeResult{
			{Command: "devil --version", ExitCode: 0, Stdout: "Devil CLI 2.4.1", Duration: 100 * time.Millisecond, Status: ProbeOK},
		},
	)
	raw, err := formatProbeJSON(s)
	if err != nil {
		t.Fatalf("formatProbeJSON: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"preset_id", "preset_name", "host", "user", "confidence", "ok_count", "mismatch_count", "failed_count", "results"} {
		if _, ok := parsed[key]; !ok {
			t.Fatalf("json missing key %q. payload: %s", key, raw)
		}
	}
	results, ok := parsed["results"].([]any)
	if !ok || len(results) != 1 {
		t.Fatalf("results must be a 1-element array, got %T %v", parsed["results"], parsed["results"])
	}
}

// fakeProbeRunner is a deterministic ProbeRunner for tests. It returns
// canned responses keyed by command, and records the order they were
// invoked so we can assert ordering matches preset.Probes.
type fakeProbeRunner struct {
	responses map[string]ProbeResult
	executed  []string
	err       error
}

func (f *fakeProbeRunner) Run(_ context.Context, command string) (ProbeResult, error) {
	f.executed = append(f.executed, command)
	if f.err != nil {
		return ProbeResult{Command: command}, f.err
	}
	r, ok := f.responses[command]
	if !ok {
		return ProbeResult{Command: command, ExitCode: 127, Stderr: "fake: no canned response"}, nil
	}
	r.Command = command
	return r, nil
}

// fakePresetRegistry builds a registry from an in-memory fstest map so
// the test never touches the real embedded catalog. The registry is
// constructed with one preset whose probes we control exactly.
func fakePresetRegistry(t *testing.T, presetID string, probes []string) *presets.Registry {
	t.Helper()

	payload := fmt.Sprintf(`{
  "schema_version": 1,
  "id": %q,
  "display_name": "Preset Under Test",
  "provider_type": "fake",
  "status": "research",
  "markets": ["PL"],
  "panel": {"name": "fake", "api": "ssh_only", "ssh_required": true},
  "capabilities": {
    "node_runtime": "version_manager",
    "restart_method": "kill_and_restart",
    "ssl_provider": "panel_managed",
    "database_engines": ["mysql"],
    "git_available": true,
    "logs_path_known": true,
    "safe_restart": true
  },
  "paths": {
    "deploy_path_template": "/home/{{user}}/domains/{{domain}}/public_nodejs",
    "log_path_template": "/home/{{user}}/.logs/{{domain}}.log"
  },
  "probes": %s
}`, presetID, jsonStringArray(probes))

	fsys := fstest.MapFS{
		"presets/" + presetID + ".json": &fstest.MapFile{Data: []byte(payload)},
	}
	res, err := presets.LoadFrom(fsys, "presets")
	if err != nil {
		t.Fatalf("loadFrom: %v", err)
	}
	return presets.NewRegistryFromResult(res)
}

func jsonStringArray(ss []string) string {
	b, _ := json.Marshal(ss)
	return string(b)
}

// TestRunPresetProbe_HappyPath drives runPresetProbe end-to-end with a
// fake runner. Every probe succeeds, so the exit code is OK and the
// human output ends with `Confidence: 100`. We also verify the runner
// was called in the order the preset declared its probes — operators
// expect deterministic execution so they can compare runs across time.
func TestRunPresetProbe_HappyPath(t *testing.T) {
	t.Parallel()

	reg := fakePresetRegistry(t, "preset-x", []string{
		"node --version",
		"php --version",
	})
	runner := &fakeProbeRunner{responses: map[string]ProbeResult{
		"node --version": {ExitCode: 0, Stdout: "v20.11.0"},
		"php --version":  {ExitCode: 0, Stdout: "PHP 8.2.10"},
	}}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	rc := runPresetProbe(
		probeOpts{
			id:      "preset-x",
			host:    "h.example",
			user:    "operator",
			timeout: 5 * time.Second,
		},
		stdout, stderr,
		func() (*presets.Registry, error) { return reg, nil },
		func(probeOpts) (ProbeRunner, error) { return runner, nil },
	)

	if rc != exitOK {
		t.Fatalf("exit = %d, stderr=%q", rc, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Confidence: 100") {
		t.Fatalf("missing confidence line in stdout:\n%s", stdout.String())
	}
	if diff := cmp(runner.executed, []string{"node --version", "php --version"}); diff != "" {
		t.Fatalf("probe order: %s", diff)
	}
}

// TestRunPresetProbe_RunnerFactoryError documents the contract for SSH
// dial failure: factory returns an error → exit code != OK and the
// failure surfaces on stderr (not silent). No probes execute.
func TestRunPresetProbe_RunnerFactoryError(t *testing.T) {
	t.Parallel()

	reg := fakePresetRegistry(t, "preset-x", []string{"node --version"})
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	rc := runPresetProbe(
		probeOpts{id: "preset-x", host: "unreachable", user: "u"},
		stdout, stderr,
		func() (*presets.Registry, error) { return reg, nil },
		func(probeOpts) (ProbeRunner, error) {
			return nil, errors.New("dial tcp: connection refused")
		},
	)

	if rc == exitOK {
		t.Fatalf("expected non-zero exit, got OK; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "connection refused") {
		t.Fatalf("missing dial error in stderr: %q", stderr.String())
	}
}

// TestRunPresetProbe_PresetWithoutProbes is an edge case that surfaced
// during Sprint 19: a `mock` preset has no probes at all. The command
// should not crash, should not pretend success — it surfaces an
// explicit `no probes declared` notice and returns a non-zero exit so
// CI scripts can detect "nothing was actually verified".
func TestRunPresetProbe_PresetWithoutProbes(t *testing.T) {
	t.Parallel()

	reg := fakePresetRegistry(t, "preset-x", []string{})
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	rc := runPresetProbe(
		probeOpts{id: "preset-x", host: "h", user: "u"},
		stdout, stderr,
		func() (*presets.Registry, error) { return reg, nil },
		func(probeOpts) (ProbeRunner, error) {
			return &fakeProbeRunner{}, nil
		},
	)
	if rc == exitOK {
		t.Fatalf("expected non-zero exit for preset with no probes")
	}
	if !strings.Contains(stderr.String(), "no probes") {
		t.Fatalf("missing notice: %q", stderr.String())
	}
}

// cmp is a tiny diff helper for []string slices; we don't pull
// google/go-cmp here to keep the test binary lean — preset.go and
// probe.go are deliberately dependency-free.
func cmp(got, want []string) string {
	if len(got) != len(want) {
		return fmt.Sprintf("len mismatch got=%v want=%v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			return fmt.Sprintf("index %d: got=%q want=%q", i, got[i], want[i])
		}
	}
	return ""
}

// assertSink is a tiny io.Writer that records the last write so the
// test can quickly check "did anything land here at all". Used in
// edge-case tests where the assertion is "stderr is non-empty".
type assertSink struct{ buf bytes.Buffer }

func (s *assertSink) Write(p []byte) (int, error) { return s.buf.Write(p) }
func (s *assertSink) String() string              { return s.buf.String() }

var _ io.Writer = (*assertSink)(nil)
