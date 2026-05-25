package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dilitS/webox/internal/telemetry"
	"github.com/dilitS/webox/internal/version"
	"github.com/dilitS/webox/presets"
	ghsvc "github.com/dilitS/webox/services/github"
	"github.com/dilitS/webox/tui"
)

// Exit codes follow the POSIX convention: 0 success, 1 general error
// (e.g. probe runner could not dial, registry load failed at runtime),
// 2 command-line misuse (unknown flag, bad arg, or — for probe runs —
// preset mismatches that still warrant a non-zero gate exit).
const (
	exitOK      = 0
	exitGeneric = 1
	exitMisuse  = 2
)

// maxTCPPort caps the SSH --port value at the legal upper bound for
// well-formed TCP. Pulled out as a constant so the parser error
// message stays consistent with the validation logic.
const maxTCPPort = 65535

const helpText = `webox — keyboard-driven cockpit for shared-hosting deployments

Usage:
  webox                                  launch the cockpit (TUI)
  webox --mock                           launch the cockpit with seeded demo data (offline)
  webox doctor                           run local diagnostics and print a text report
  webox doctor --json                    run local diagnostics and print JSON
  webox doctor github                    run read-only GitHub integration diagnostics
  webox doctor github --json             run GitHub integration diagnostics as JSON
  webox doctor preset                    list all embedded provider presets
  webox doctor preset --id=ID            show preset details (use --json for machine output)
  webox provider new <name> [--preset=PRESET] scaffold a new hosting provider adapter
  webox --version                        print build metadata and exit
  webox --help                           print this help and exit

Flags:
  --debug              enable verbose diagnostic logging
  --mock               boot the cockpit with deterministic mock data;
                       no SSH, no HTTP probes, no GitHub calls. Useful
                       for demos, screenshots, and offline UI iteration.
                       Equivalent to WEBOX_MOCK=1.
  --debug-trace=PATH   record local-only JSONL trace events (state
                       transitions, SSH metrics, error categories) to
                       PATH. Strictly local; no network. The file is
                       created with mode 0600 and every line is passed
                       through the redactor before write — see
                       docs/SECURITY.md §6 for the policy.
  --preset=PRESET      (only with ` + "`provider new`" + `) seed the generated
                       skeleton with vendor-specific scaffolding. Supported:
                       blank (default), cpanel-uapi, directadmin, cyberpanel.
  --dry-run            (only with ` + "`provider new`" + `) report what would be
                       written without touching the filesystem.
  --id=ID              (only with ` + "`doctor preset`" + `) show details for the
                       preset with the given id (e.g. cpanel-generic).
  --probe              (only with ` + "`doctor preset --id=ID`" + `) execute the
                       preset's probe commands over SSH (Sprint 21+). Requires
                       --host=HOST and --user=USER. Without those flags it
                       falls back to a documentation dump of the declarative
                       metadata and explains how to enable live execution.
  --host=HOST          (only with ` + "`doctor preset --probe`" + `) target host for
                       live preset probing (e.g. --host=panel.example.com).
  --user=USER          (only with ` + "`doctor preset --probe`" + `) SSH user for
                       live preset probing (e.g. --user=operator).
  --port=N             (only with ` + "`doctor preset --probe`" + `) SSH port. Defaults
                       to the value in ~/.ssh/config when omitted (typically 22).
  --timeout=DUR        (only with ` + "`doctor preset --probe`" + `) per-probe
                       timeout in Go duration form (default 30s).

Documentation:
  https://github.com/dilitS/webox/tree/main/docs
`

// opts holds the parsed startup flags. The TUI lands in Sprint 04, so
// the debug field is parsed but not yet wired into slog level routing —
// keeping it as a struct field (rather than a discarded local) means
// the linter does not flag it and future wiring is a one-line change.
type opts struct {
	showVersion  bool
	showHelp     bool
	debug        bool
	mock         bool
	doctor       bool
	doctorJSON   bool
	doctorTarget string // "" | "github" | "preset"
	// debugTracePath, when non-empty, instructs the launcher to open
	// a [telemetry.FileSink] at the given path and route cockpit /
	// SSH / doctor events into it. Empty value keeps the default
	// [telemetry.Disabled] no-op sink so production runs never touch
	// disk (TASK-14.6).
	debugTracePath string
	// providerNew toggles the `webox provider new <name>` generator
	// path. The remaining provider* fields hold the subcommand's
	// own options (name + preset + dry-run); they are zero values
	// when providerNew is false.
	providerNew     bool
	providerNewName string
	providerPreset  string
	providerDryRun  bool
	// presetID and presetProbe are the sub-flags that target a
	// specific preset under `webox doctor preset`. presetProbe
	// turns live SSH execution on; --host / --user / --port /
	// --timeout supply the connection parameters.
	presetID      string
	presetProbe   bool
	presetHost    string
	presetUser    string
	presetPort    int
	presetTimeout time.Duration
}

// doctorDispatcher is the seam that lets tests run the CLI router
// against a deterministic doctor implementation without touching
// package-level state.
type doctorDispatcher func(jsonOutput bool, stdout, stderr io.Writer) int

type presetDispatcher func(opts presetOpts, stdout, stderr io.Writer) int

type tuiDispatcher func(stdout, stderr io.Writer) int

// Run dispatches the command implied by args (without the program name)
// and returns the process exit code. Output is written to the supplied
// writers so tests can capture it without touching os.Stdout/os.Stderr.
func Run(args []string, stdout, stderr io.Writer) int {
	return runWithFullDeps(args, stdout, stderr, runDoctor, runDoctorGitHub, defaultPresetDispatcher, runTUI)
}

// runWithDeps preserves the legacy two-dispatcher seam used by existing
// tests that only care about the core doctor path. New tests should use
// [runWithFullDeps] so they can stub the GitHub doctor independently.
func runWithDeps(
	args []string,
	stdout,
	stderr io.Writer,
	dispatch doctorDispatcher,
	startTUI tuiDispatcher,
) int {
	return runWithFullDeps(args, stdout, stderr, dispatch, runDoctorGitHub, defaultPresetDispatcher, startTUI)
}

func runWithFullDeps(
	args []string,
	stdout,
	stderr io.Writer,
	dispatchCore doctorDispatcher,
	dispatchGitHub doctorDispatcher,
	dispatchPreset presetDispatcher,
	startTUI tuiDispatcher,
) int {
	parsed, errMsg := parseArgs(args)
	if errMsg != "" {
		fmt.Fprintln(stderr, errMsg)
		return exitMisuse
	}

	switch {
	case parsed.showVersion:
		fmt.Fprintln(stdout, version.String())
		return exitOK
	case parsed.showHelp:
		fmt.Fprint(stdout, helpText)
		return exitOK
	case parsed.doctor:
		switch parsed.doctorTarget {
		case "github":
			return dispatchGitHub(parsed.doctorJSON, stdout, stderr)
		case "preset":
			return dispatchPreset(presetOpts{
				id:      parsed.presetID,
				json:    parsed.doctorJSON,
				probe:   parsed.presetProbe,
				host:    parsed.presetHost,
				user:    parsed.presetUser,
				port:    parsed.presetPort,
				timeout: parsed.presetTimeout,
			}, stdout, stderr)
		default:
			return dispatchCore(parsed.doctorJSON, stdout, stderr)
		}
	case parsed.providerNew:
		return runProviderNew(providerNewOpts{
			name:   parsed.providerNewName,
			preset: parsed.providerPreset,
			dryRun: parsed.providerDryRun,
		}, stdout, stderr)
	}

	// The --debug modifier is parsed (so order-independent invocations such
	// as `webox --debug --version` work today) but log-level routing lands
	// with the diagnostics wiring.
	_ = parsed.debug

	sink, sinkCleanup := openTraceSink(parsed.debugTracePath, stderr)
	defer sinkCleanup()

	if parsed.mock || mockEnvActive() {
		return runMockTUIWithTrace(stdout, stderr, sink)
	}
	return runTUIWithTrace(stdout, stderr, sink, startTUI)
}

// runTUIWithTrace is a thin shim that keeps the legacy
// `tuiDispatcher` signature alive (used by tests) while still
// threading the trace sink into the production launcher. For the
// default `tuiDispatcher == runTUI`, the shim calls `runTUIWith`
// directly so the sink reaches `tui.Options.Trace`. For test
// dispatchers (which already inject everything they need via
// closure capture), the shim invokes the dispatcher unchanged.
func runTUIWithTrace(stdout, stderr io.Writer, sink telemetry.Sink, dispatch tuiDispatcher) int {
	if !sink.Enabled() {
		return dispatch(stdout, stderr)
	}
	// Production fast path: when the trace sink is live we bypass
	// the legacy dispatcher seam and go straight to runTUIWith so
	// the sink reaches the cockpit. Tests stub `dispatch` to a
	// no-op so this branch is never exercised by them.
	client := ghsvc.NewClient(ghsvc.Options{})
	return runTUIWithDeps(
		stdout, stderr, sink,
		tui.DefaultConfigPath,
		realTeaProgram,
		lastDeployFetcherFor(client),
		pipelineFetcherFor(client),
		logsFetcherFor(client),
	)
}

// defaultPresetDispatcher is the production wiring for `webox
// doctor preset`. It binds the runtime registry resolver to
// presets.Default(), keeping the dispatch surface stub-able from
// tests via [runWithFullDeps].
func defaultPresetDispatcher(opts presetOpts, stdout, stderr io.Writer) int {
	return runPresetDoctor(opts, stdout, stderr, defaultPresetRegistry)
}

// defaultPresetRegistry returns the singleton preset registry for
// production callers. The seam is exported as a package-level
// variable so a future test that wants to swap it can do so by
// assigning before runPresetDoctor is invoked.
var defaultPresetRegistry presetRegistryProvider = presets.Default

// runMockTUIWithTrace mirrors runMockTUI but injects a trace sink
// into the mock cockpit. Used when an operator runs
// `webox --mock --debug-trace=PATH` to debug renderer transitions
// without touching real servers.
func runMockTUIWithTrace(stdout, stderr io.Writer, sink telemetry.Sink) int {
	fmt.Fprintln(stderr, "webox: starting in MOCK mode — no servers are contacted")
	opts := tui.MockOptions("")
	opts.Trace = sink
	program := realTeaProgram(tui.New(opts), stdout)
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(stderr, "webox: TUI failed: %v\n", err)
		return exitMisuse
	}
	return exitOK
}

// openTraceSink resolves the `--debug-trace=PATH` flag into either a
// [telemetry.FileSink] or the canonical [telemetry.Disabled] no-op
// sink. The function NEVER returns an error to the caller — trace
// is a debugging convenience, not a critical path, so a failed open
// only emits a one-line warning on stderr and degrades to the no-op
// sink. The returned cleanup closes the file (if any) and is safe
// to defer regardless of which sink was selected.
func openTraceSink(path string, stderr io.Writer) (sink telemetry.Sink, cleanup func()) {
	if path == "" {
		return telemetry.Disabled, func() {}
	}
	opened, err := telemetry.OpenFileSink(path, telemetry.FileSinkPolicy{})
	if err != nil {
		fmt.Fprintf(stderr, "webox: --debug-trace disabled: %v\n", err)
		return telemetry.Disabled, func() {}
	}
	fmt.Fprintf(stderr, "webox: --debug-trace writing to %s (local only, 0600)\n", path)
	if closer, ok := opened.(io.Closer); ok {
		return opened, func() { _ = closer.Close() }
	}
	return opened, func() {}
}

// mockEnv is the environment variable the launcher honours as an
// alternative to the `--mock` CLI flag. Setting it to any non-empty
// value other than "0" / "false" boots the cockpit in mock mode.
const mockEnv = "WEBOX_MOCK"

func mockEnvActive() bool {
	v := os.Getenv(mockEnv)
	return v != "" && v != "0" && v != "false" && v != "FALSE"
}

// configPathResolver is the package-private seam that returns the
// `config.json` path the TUI should load. Production wires it to
// [tui.DefaultConfigPath]; tests substitute a deterministic stub so
// the path-resolution error branch in [runTUI] is reachable without
// touching the real XDG / HOME environment.
type configPathResolver func() (string, error)

// teaProgramFactory is the seam that builds the Bubble Tea program
// runner. Production returns a real [*tea.Program]; tests return an
// in-memory stub that records the model and reports a scripted
// outcome. The TUI dispatcher (`runTUI`) owns the only call site so
// the seam stays narrow.
type teaProgramFactory func(model tea.Model, stdout io.Writer) teaRunner

// teaRunner abstracts the subset of [*tea.Program] runTUI cares
// about. Keeping it a one-method interface means a test stub fits in
// 5 lines.
type teaRunner interface {
	Run() (tea.Model, error)
}

// realTeaProgram wraps the standard `*tea.Program` so tests can hand
// runTUI a stub instead.
//
// As of the 2026-05-24 UX refresh we run inside Bubble Tea's alternate
// screen buffer ([tea.WithAltScreen]). The cockpit is a full-screen
// app (like vim, htop, lazygit): screen swaps replace the active frame
// instead of scrolling the host terminal's history. The buffer is
// released on quit so the operator returns to a clean prompt.
//
// [tea.WithMouseCellMotion] enables fine-grained mouse routing so
// future surfaces (CI/CD step click-through, log scroll) can opt in
// without bumping the program options. Mouse handlers stay opt-in per
// surface; no global behaviour change today.
func realTeaProgram(model tea.Model, stdout io.Writer) teaRunner {
	return tea.NewProgram(
		model,
		tea.WithOutput(stdout),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
}

func runTUI(stdout, stderr io.Writer) int {
	client := ghsvc.NewClient(ghsvc.Options{})
	return runTUIWith(
		stdout,
		stderr,
		tui.DefaultConfigPath,
		realTeaProgram,
		lastDeployFetcherFor(client),
		pipelineFetcherFor(client),
		logsFetcherFor(client),
	)
}

func runTUIWith(
	stdout,
	stderr io.Writer,
	resolveConfig configPathResolver,
	makeProgram teaProgramFactory,
	fetcher tui.GitHubLastDeployFetcher,
	pipeline tui.GitHubPipelineFetcher,
	logs tui.GitHubLogsFetcher,
) int {
	return runTUIWithDeps(
		stdout, stderr, telemetry.Disabled,
		resolveConfig, makeProgram, fetcher, pipeline, logs,
	)
}

// runTUIWithDeps is the trace-aware TUI launcher used by both the
// production fast path (when `--debug-trace=PATH` is set) and the
// legacy `runTUIWith` shim (which passes [telemetry.Disabled]). The
// extra parameter avoids changing the signature of `runTUIWith`
// (still used by existing test helpers) while keeping the new wire
// in a single place.
func runTUIWithDeps(
	stdout,
	stderr io.Writer,
	sink telemetry.Sink,
	resolveConfig configPathResolver,
	makeProgram teaProgramFactory,
	fetcher tui.GitHubLastDeployFetcher,
	pipeline tui.GitHubPipelineFetcher,
	logs tui.GitHubLogsFetcher,
) int {
	cfgPath, err := resolveConfig()
	if err != nil {
		fmt.Fprintf(stderr, "webox: resolve config path: %v\n", err)
		return exitMisuse
	}
	program := makeProgram(
		tui.New(tui.Options{
			ConfigPath:       cfgPath,
			GitHubLastDeploy: fetcher,
			GitHubPipeline:   pipeline,
			GitHubLogs:       logs,
			Trace:            sink,
		}),
		stdout,
	)
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(stderr, "webox: TUI failed: %v\n", err)
		return exitMisuse
	}
	return exitOK
}

// lastDeployFetcherFor wires the dashboard last-deploy fetcher against
// the supplied GitHub client. Sharing the same client across fetchers
// reuses the gh CLI's cached auth state for the duration of the run.
func lastDeployFetcherFor(client *ghsvc.Client) tui.GitHubLastDeployFetcher {
	return func(ctx context.Context, ref ghsvc.RepoRef, workflow string) (*ghsvc.WorkflowRun, error) {
		return client.GetLatestRun(ctx, ref, ghsvc.LatestRunRequest{Event: "workflow_dispatch"})
	}
}

// pipelineFetcherFor wires the CI/CD pipeline tile against the
// shared GitHub client. The fetcher first calls `GetLatestRun` to
// resolve the active run id, then `GetWorkflowSteps` to populate the
// step list. Rate-limit errors propagate so the tile can degrade
// gracefully (Sprint 10 plan §TASK-10.5).
func pipelineFetcherFor(client *ghsvc.Client) tui.GitHubPipelineFetcher {
	return func(ctx context.Context, ref ghsvc.RepoRef, workflow string) (tui.PipelineFetchResult, error) {
		run, err := client.GetLatestRun(ctx, ref, ghsvc.LatestRunRequest{})
		if err != nil {
			return tui.PipelineFetchResult{}, err
		}
		if run == nil {
			return tui.PipelineFetchResult{}, ghsvc.ErrRunNotFound
		}
		steps, err := client.GetWorkflowSteps(ctx, ref, run.ID)
		if err != nil {
			return tui.PipelineFetchResult{Run: run}, err
		}
		return tui.PipelineFetchResult{Run: run, Steps: steps}, nil
	}
}

// logsFetcherFor wires the F8 modal against the shared GitHub client.
// All lines come back already redacted at the transport boundary.
func logsFetcherFor(client *ghsvc.Client) tui.GitHubLogsFetcher {
	return func(ctx context.Context, ref ghsvc.RepoRef, runID int64, maxLines int) ([]ghsvc.WorkflowLogLine, error) {
		return client.GetWorkflowLogs(ctx, ref, runID, maxLines)
	}
}

func parseArgs(args []string) (parsed opts, errMsg string) {
	for _, arg := range args {
		if errMsg = applySimpleFlag(&parsed, arg); errMsg != "" {
			return opts{}, errMsg
		}
		if simpleFlagHandled(arg) {
			continue
		}
		if errMsg = applyPrefixedFlag(&parsed, arg); errMsg != "" {
			return opts{}, errMsg
		}
		if prefixedFlagHandled(arg) {
			continue
		}
		if parsed.providerNew && parsed.providerNewName == "" && !strings.HasPrefix(arg, "-") {
			parsed.providerNewName = arg
			continue
		}
		return opts{}, fmt.Sprintf("webox: unknown argument %q. Run `webox --help` for usage.", arg)
	}
	if errMsg := postParseValidation(parsed); errMsg != "" {
		return opts{}, errMsg
	}
	return parsed, ""
}

// applySimpleFlag handles the closed-set of literal token / flag args.
// It returns a non-empty `errMsg` only when the token is recognised
// AND the operator combined it incorrectly (e.g. `--dry-run` outside
// `provider new`). It returns "" both for "handled" and "not mine";
// the caller disambiguates via [simpleFlagHandled].
//
// Cyclomatic complexity is kept under control by routing
// context-sensitive tokens (`github`, `preset`, `new`, `--probe`,
// `--dry-run`) through helper functions; standalone tokens stay
// in the switch arm.
func applySimpleFlag(parsed *opts, arg string) string {
	switch arg {
	case "doctor":
		parsed.doctor = true
	case "github", "preset", "new":
		return applyContextualToken(parsed, arg)
	case "provider":
		parsed.providerNew = true
	case "--version":
		parsed.showVersion = true
	case "--help", "-h":
		parsed.showHelp = true
	case "--debug":
		parsed.debug = true
	case "--mock":
		parsed.mock = true
	case "--json":
		parsed.doctorJSON = true
	case "--probe":
		return applyProbeFlag(parsed)
	case "--dry-run":
		return applyDryRunFlag(parsed)
	}
	return ""
}

// applyContextualToken handles tokens that are only valid in a
// specific subcommand context. Kept separate from applySimpleFlag
// so the parent stays under the gocyclo budget.
func applyContextualToken(parsed *opts, arg string) string {
	switch arg {
	case "github":
		return setDoctorTarget(parsed, "github", "`github` is only valid after `doctor`.")
	case "preset":
		return setDoctorTarget(parsed, "preset", "`preset` is only valid after `doctor` (usage: webox doctor preset).")
	case "new":
		if !parsed.providerNew {
			return "webox: `new` is only valid after `provider` (usage: webox provider new <name>)."
		}
	}
	return ""
}

func setDoctorTarget(parsed *opts, target, missingDoctorMsg string) string {
	if !parsed.doctor {
		return "webox: " + missingDoctorMsg
	}
	if parsed.doctorTarget != "" && parsed.doctorTarget != target {
		return fmt.Sprintf("webox: doctor target already set to %q.", parsed.doctorTarget)
	}
	parsed.doctorTarget = target
	return ""
}

func applyProbeFlag(parsed *opts) string {
	if parsed.doctorTarget != "preset" {
		return "webox: --probe is only valid with `webox doctor preset`."
	}
	parsed.presetProbe = true
	return ""
}

func applyDryRunFlag(parsed *opts) string {
	if !parsed.providerNew {
		return "webox: --dry-run is only valid with `provider new`."
	}
	parsed.providerDryRun = true
	return ""
}

// simpleFlagHandled returns true when applySimpleFlag consumed the
// argument, false otherwise. The split keeps parseArgs's main loop
// flat — without it, gocyclo flags the function as too complex.
func simpleFlagHandled(arg string) bool {
	switch arg {
	case "doctor", "github", "preset", "provider", "new",
		"--version", "--help", "-h", "--debug",
		"--mock", "--json", "--probe", "--dry-run":
		return true
	}
	return false
}

// applyPrefixedFlag handles `--key=value` form options. Returns
// non-empty `errMsg` on validation failure; otherwise "" (handled or
// not mine — caller checks [prefixedFlagHandled]). Kept under
// gocyclo's threshold by delegating each flag family to a helper.
func applyPrefixedFlag(parsed *opts, arg string) string {
	if path, ok := strings.CutPrefix(arg, "--debug-trace="); ok {
		if path == "" {
			return "webox: --debug-trace requires a non-empty PATH (e.g. --debug-trace=/tmp/webox.jsonl)"
		}
		parsed.debugTracePath = path
		return ""
	}
	if preset, ok := strings.CutPrefix(arg, "--preset="); ok {
		if !parsed.providerNew {
			return "webox: --preset is only valid with `provider new`."
		}
		if preset == "" {
			return "webox: --preset requires a value (one of blank, cpanel-uapi, directadmin, cyberpanel)."
		}
		parsed.providerPreset = preset
		return ""
	}
	if msg, handled := applyPresetIDFlag(parsed, arg); handled {
		return msg
	}
	if msg, handled := applyPresetProbeFlag(parsed, arg); handled {
		return msg
	}
	return ""
}

// applyPresetIDFlag handles --id=ID for the preset subcommand.
// Returns (errMsg, handled). `handled=true` means caller should
// stop scanning further prefix matchers.
func applyPresetIDFlag(parsed *opts, arg string) (string, bool) {
	id, ok := strings.CutPrefix(arg, "--id=")
	if !ok {
		return "", false
	}
	if parsed.doctorTarget != "preset" {
		return "webox: --id is only valid with `webox doctor preset`.", true
	}
	if id == "" {
		return "webox: --id requires a value (e.g. --id=cpanel-generic).", true
	}
	parsed.presetID = id
	return "", true
}

// applyPresetProbeFlag handles the four flags that parameterise a
// live `doctor preset --probe` run: --host, --user, --port, --timeout.
// Each one independently asserts that the doctor target is "preset"
// so the parser surfaces a focused error message when an operator
// types --host before --preset.
func applyPresetProbeFlag(parsed *opts, arg string) (string, bool) {
	if host, ok := strings.CutPrefix(arg, "--host="); ok {
		if parsed.doctorTarget != "preset" {
			return "webox: --host is only valid with `webox doctor preset --probe`.", true
		}
		if host == "" {
			return "webox: --host requires a value (e.g. --host=panel.example.com).", true
		}
		parsed.presetHost = host
		return "", true
	}
	if user, ok := strings.CutPrefix(arg, "--user="); ok {
		if parsed.doctorTarget != "preset" {
			return "webox: --user is only valid with `webox doctor preset --probe`.", true
		}
		if user == "" {
			return "webox: --user requires a value (e.g. --user=operator).", true
		}
		parsed.presetUser = user
		return "", true
	}
	if portStr, ok := strings.CutPrefix(arg, "--port="); ok {
		if parsed.doctorTarget != "preset" {
			return "webox: --port is only valid with `webox doctor preset --probe`.", true
		}
		port, err := strconv.Atoi(portStr)
		if err != nil || port <= 0 || port > maxTCPPort {
			return fmt.Sprintf("webox: --port=%q must be a positive integer <= %d.", portStr, maxTCPPort), true
		}
		parsed.presetPort = port
		return "", true
	}
	if timeoutStr, ok := strings.CutPrefix(arg, "--timeout="); ok {
		if parsed.doctorTarget != "preset" {
			return "webox: --timeout is only valid with `webox doctor preset --probe`.", true
		}
		dur, err := time.ParseDuration(timeoutStr)
		if err != nil || dur <= 0 {
			return fmt.Sprintf("webox: --timeout=%q must be a positive Go duration (e.g. 5s, 1m).", timeoutStr), true
		}
		parsed.presetTimeout = dur
		return "", true
	}
	return "", false
}

func prefixedFlagHandled(arg string) bool {
	return strings.HasPrefix(arg, "--debug-trace=") ||
		strings.HasPrefix(arg, "--preset=") ||
		strings.HasPrefix(arg, "--id=") ||
		strings.HasPrefix(arg, "--host=") ||
		strings.HasPrefix(arg, "--user=") ||
		strings.HasPrefix(arg, "--port=") ||
		strings.HasPrefix(arg, "--timeout=")
}

// postParseValidation enforces cross-flag invariants that cannot be
// decided in a single token's switch arm.
func postParseValidation(parsed opts) string {
	if parsed.doctorJSON && !parsed.doctor {
		return "webox: --json is only valid with `webox doctor`."
	}
	if parsed.presetProbe && parsed.presetID == "" {
		return "webox: --probe requires --id=<preset-id> (usage: webox doctor preset --id=<id> --probe)."
	}
	if parsed.providerNew && parsed.providerNewName == "" {
		return "webox: `provider new` requires a name (usage: webox provider new <name> [--preset=PRESET])."
	}
	return ""
}
