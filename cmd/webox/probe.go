package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/dilitS/webox/presets"
)

// probeOutputPreviewLimit caps the number of stdout/stderr bytes shown
// per probe in the human-readable formatter. JSON output is always
// full-fidelity; only the text formatter truncates so noisy commands
// (e.g. `devil www list` on a 200-domain account) do not blow the
// terminal width. The constant lives outside the function body so
// tests assert against it directly.
const probeOutputPreviewLimit = 240

// confidenceTotal is the upper bound on the confidence score (0-100).
// Pulled into a named constant so the rounding direction is explicit.
const confidenceTotal = 100

// probeOpts collects the inputs required to execute `webox doctor
// preset --probe`. The factory seam takes a copy of probeOpts so a
// production SSH runner can resolve target / pool from it without
// re-parsing CLI flags.
type probeOpts struct {
	id      string
	host    string
	port    int
	user    string
	timeout time.Duration
	json    bool
}

// ProbeStatus is the per-probe verdict surfaced to the operator.
type ProbeStatus string

// Recognised verdicts. Documented inline so the formatter never has
// to guess what the strings mean — they go straight into stdout / JSON.
const (
	// ProbeOK — runner returned exit code 0. Preset metadata is
	// at least surface-level consistent with the live host (we do
	// not parse the output yet; that lands when typed Probes ship).
	ProbeOK ProbeStatus = "OK"
	// ProbeMismatch — runner returned a non-zero exit code. The
	// command exists on the host but produced an unexpected
	// result. Operator should investigate.
	ProbeMismatch ProbeStatus = "MISMATCH"
	// ProbeFailed — the runner itself errored before producing
	// an exit code (network failure, auth refused, timeout).
	// Distinct from MISMATCH so operators can tell "the panel
	// disagreed with the preset" from "we never reached the
	// panel".
	ProbeFailed ProbeStatus = "FAILED"
)

// ProbeResult is a single probe's outcome.
type ProbeResult struct {
	Command  string        `json:"command"`
	Stdout   string        `json:"stdout,omitempty"`
	Stderr   string        `json:"stderr,omitempty"`
	ExitCode int           `json:"exit_code"`
	Duration time.Duration `json:"duration_ns"`
	Err      error         `json:"-"`
	ErrMsg   string        `json:"error,omitempty"`
	Status   ProbeStatus   `json:"status"`
}

// ProbeSummary is the rolled-up outcome surfaced to operators. The
// confidence score is the share of OK probes among total probes
// (rounded down so the formatter never inflates the result).
type ProbeSummary struct {
	PresetID    string        `json:"preset_id"`
	PresetName  string        `json:"preset_name"`
	Host        string        `json:"host"`
	User        string        `json:"user"`
	Confidence  int           `json:"confidence"`
	OK          int           `json:"ok_count"`
	Mismatch    int           `json:"mismatch_count"`
	Failed      int           `json:"failed_count"`
	GeneratedAt time.Time     `json:"generated_at"`
	Results     []ProbeResult `json:"results"`
}

// ProbeRunner abstracts probe execution so the dispatcher tests can
// inject a fake without spinning up an SSH server. The production
// wiring (newSSHProbeRunner) dials a real cPanel / Devil / DirectAdmin
// host via the ssh package; tests substitute fakeProbeRunner.
type ProbeRunner interface {
	Run(ctx context.Context, command string) (ProbeResult, error)
}

// probeRunnerFactory is the seam through which the CLI selects a
// runner. Production callers wire it to newSSHProbeRunner; tests
// inject a fake that returns a canned ProbeRunner without dialing.
type probeRunnerFactory func(probeOpts) (ProbeRunner, error)

// presetRegistryProvider is reused from preset.go; both files share the
// same seam so tests do not have to introduce a parallel hierarchy.

// summarizeProbe converts a slice of ProbeResult into the rollup
// surfaced to the operator. Status is filled in if the caller did not
// set it (the SSH runner can leave it blank and trust the summarizer
// to derive it from ExitCode / Err).
func summarizeProbe(presetID, presetName, host, user string, results []ProbeResult) ProbeSummary {
	s := ProbeSummary{
		PresetID:    presetID,
		PresetName:  presetName,
		Host:        host,
		User:        user,
		GeneratedAt: time.Now().UTC(),
		Results:     make([]ProbeResult, len(results)),
	}
	for i, r := range results {
		if r.Status == "" {
			r.Status = deriveStatus(r)
		}
		if r.Err != nil && r.ErrMsg == "" {
			r.ErrMsg = r.Err.Error()
		}
		switch r.Status {
		case ProbeOK:
			s.OK++
		case ProbeMismatch:
			s.Mismatch++
		case ProbeFailed:
			s.Failed++
		}
		s.Results[i] = r
	}
	total := len(results)
	if total == 0 {
		s.Confidence = 0
		return s
	}
	s.Confidence = (s.OK * confidenceTotal) / total
	return s
}

// deriveStatus returns the implicit verdict for a probe whose caller
// did not set ProbeResult.Status explicitly. Order matters: an Err
// trumps any exit code (we never claim OK on a runner-level failure).
func deriveStatus(r ProbeResult) ProbeStatus {
	if r.Err != nil {
		return ProbeFailed
	}
	if r.ExitCode == 0 {
		return ProbeOK
	}
	return ProbeMismatch
}

// truncate caps s at probeOutputPreviewLimit bytes and appends a
// `(truncated)` marker when it had to cut. Used only by the text
// formatter — the JSON output is always full-fidelity.
func truncate(s string) string {
	s = strings.TrimRight(s, "\n")
	if len(s) <= probeOutputPreviewLimit {
		return s
	}
	return s[:probeOutputPreviewLimit] + "… (truncated)"
}

// formatProbeText renders a ProbeSummary as the default human-readable
// console output. Layout: header (preset + host + user), per-probe
// block (command, status, exit, duration, stdout/stderr previews),
// trailer (counters + confidence). Lines are kept under 100 chars
// so they wrap cleanly inside the operator's terminal.
func formatProbeText(s ProbeSummary) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Preset: %s (%s)\n", s.PresetID, s.PresetName)
	fmt.Fprintf(&b, "Host:   %s\n", s.Host)
	fmt.Fprintf(&b, "User:   %s\n\n", s.User)
	for i, r := range s.Results {
		fmt.Fprintf(&b, "[%d/%d] %s\n", i+1, len(s.Results), r.Command)
		fmt.Fprintf(&b, "      status   %s\n", r.Status)
		fmt.Fprintf(&b, "      exit     %d\n", r.ExitCode)
		fmt.Fprintf(&b, "      time     %s\n", r.Duration.Round(time.Millisecond))
		if r.ErrMsg != "" {
			fmt.Fprintf(&b, "      error    %s\n", r.ErrMsg)
		}
		if r.Stdout != "" {
			fmt.Fprintf(&b, "      stdout   %s\n", truncate(r.Stdout))
		}
		if r.Stderr != "" {
			fmt.Fprintf(&b, "      stderr   %s\n", truncate(r.Stderr))
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "Summary: OK=%d  Mismatch=%d  Failed=%d  Total=%d\n", s.OK, s.Mismatch, s.Failed, len(s.Results))
	fmt.Fprintf(&b, "Confidence: %d / %d\n", s.Confidence, confidenceTotal)
	return b.String()
}

// formatProbeJSON renders ProbeSummary as indented JSON suitable for
// piping into `jq` or capturing in CI logs.
func formatProbeJSON(s ProbeSummary) ([]byte, error) {
	return json.MarshalIndent(s, "", "  ")
}

// runPresetProbe is the dispatch entry for `webox doctor preset --id=<id>
// --probe --host=<h> --user=<u>`. It is split out of preset.go so the
// stub-vs-real branching stays readable: preset.go owns the read-only
// surface, probe.go owns the live-execution surface.
func runPresetProbe(
	opts probeOpts,
	stdout, stderr io.Writer,
	provider presetRegistryProvider,
	factory probeRunnerFactory,
) int {
	if opts.id == "" || opts.host == "" || opts.user == "" {
		fmt.Fprintln(stderr, "webox: --probe requires --id, --host, and --user (all three).")
		return exitMisuse
	}

	reg, err := provider()
	if err != nil {
		fmt.Fprintf(stderr, "webox: load preset registry: %v\n", err)
		return exitMisuse
	}
	p, err := reg.Get(opts.id)
	if err != nil {
		fmt.Fprintf(stderr, "webox: %v\n", err)
		return exitMisuse
	}
	if len(p.Probes) == 0 {
		fmt.Fprintf(stderr, "webox: preset %q declares no probes; nothing to verify.\n", p.ID)
		return exitMisuse
	}

	runner, err := factory(opts)
	if err != nil {
		fmt.Fprintf(stderr, "webox: open probe runner: %v\n", err)
		return exitGeneric
	}

	timeout := opts.timeout
	if timeout <= 0 {
		timeout = defaultProbeTimeout
	}

	results := make([]ProbeResult, 0, len(p.Probes))
	for _, cmd := range p.Probes {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		r, runErr := runner.Run(ctx, cmd)
		cancel()
		r.Command = cmd
		if runErr != nil {
			r.Err = runErr
			r.ErrMsg = runErr.Error()
		}
		r.Status = deriveStatus(r)
		results = append(results, r)
	}

	summary := summarizeProbe(p.ID, p.DisplayName, opts.host, opts.user, results)
	return emitProbeSummary(opts.json, summary, stdout, stderr)
}

// emitProbeSummary writes the summary in either text or JSON form and
// returns the appropriate process exit code. We treat any FAILED probe
// as a non-zero exit so CI scripts can detect dial / network errors,
// even when the rest of the run was nominally OK.
func emitProbeSummary(asJSON bool, s ProbeSummary, stdout, stderr io.Writer) int {
	if asJSON {
		payload, err := formatProbeJSON(s)
		if err != nil {
			fmt.Fprintf(stderr, "webox: encode probe summary json: %v\n", err)
			return exitGeneric
		}
		_, _ = stdout.Write(payload)
		_, _ = io.WriteString(stdout, "\n")
	} else {
		_, _ = io.WriteString(stdout, formatProbeText(s))
	}
	if s.Failed > 0 {
		return exitGeneric
	}
	if s.Mismatch > 0 {
		// Mismatches still warrant a non-zero exit so operators
		// can wire `webox doctor preset --probe` into CI gates,
		// but we use a softer code than FAILED to let callers
		// distinguish "the panel disagreed" from "we never
		// reached the panel".
		return exitMisuse
	}
	return exitOK
}

// defaultProbeTimeout caps a single probe command at 30 s. Commands
// that take longer almost certainly indicate a hung session — better
// to abort and report than to block the whole CLI.
const defaultProbeTimeout = 30 * time.Second

// sshExecRunner is the production ProbeRunner. It shells out to the
// operator's native `ssh` binary so authentication delegates to
// `~/.ssh/config`, `ssh-agent`, and the system known_hosts file (no
// custom auth surface to maintain in this CLI subcommand). We use
// BatchMode=yes so the CLI never prompts for a password mid-probe
// (operator gets a clean FAILED probe instead of a hung session) and
// StrictHostKeyChecking=accept-new so the first connect TOFUs the
// fingerprint while mismatches still strict-block per security
// charter.
//
// The probe command itself is treated as a single argv element when
// invoking ssh; the remote shell on the panel does the parsing. The
// command source is the embedded preset registry (signed by the
// project at compile time), not operator input, so no local shell
// injection is possible.
type sshExecRunner struct {
	host string
	user string
	port int
}

// Run satisfies the ProbeRunner contract. Errors from exec.Command
// other than exit-code != 0 are surfaced as ProbeFailed via the
// returned error; non-zero exits land as ProbeMismatch via ExitCode.
func (s *sshExecRunner) Run(ctx context.Context, command string) (ProbeResult, error) {
	args := []string{
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "ConnectTimeout=10",
	}
	if s.port > 0 {
		args = append(args, "-p", strconv.Itoa(s.port))
	}
	args = append(args, fmt.Sprintf("%s@%s", s.user, s.host), command)

	start := time.Now()
	//nolint:gosec // command originates from the embedded preset registry
	// (curated, compile-time-signed source), not from operator input.
	// `args` are hard-coded; the trailing element is the remote command
	// passed verbatim to the SSH session. No local shell expansion runs.
	cmd := exec.CommandContext(ctx, "ssh", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	duration := time.Since(start)

	r := ProbeResult{
		Command:  command,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
	}
	if err == nil {
		return r, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		r.ExitCode = exitErr.ExitCode()
		return r, nil
	}
	r.Err = err
	r.ErrMsg = err.Error()
	return r, err
}

// newSSHProbeRunner is the production probeRunnerFactory. It returns
// a runner without dialing — the dial happens on the first Run call
// and is gated by ssh's own ConnectTimeout (10 s above), so callers
// see a per-probe timeout consistent with the rest of the suite.
func newSSHProbeRunner(opts probeOpts) (ProbeRunner, error) {
	if _, err := exec.LookPath("ssh"); err != nil {
		return nil, fmt.Errorf("ssh binary not found on PATH: %w", err)
	}
	return &sshExecRunner{
		host: opts.host,
		user: opts.user,
		port: opts.port,
	}, nil
}

// _ keeps presets imported even if a future refactor moves all
// preset.Probes references out of this file.
var _ = presets.ErrPresetNotFound
