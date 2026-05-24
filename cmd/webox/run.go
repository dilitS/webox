package main

import (
	"context"
	"fmt"
	"io"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dilitS/webox/internal/version"
	ghsvc "github.com/dilitS/webox/services/github"
	"github.com/dilitS/webox/tui"
)

// Exit codes follow the POSIX convention: 0 success, 1 general error,
// 2 command-line misuse (unknown flag, bad arg).
const (
	exitOK     = 0
	exitMisuse = 2
)

const helpText = `webox — keyboard-driven cockpit for shared-hosting deployments

Usage:
  webox                       launch the cockpit (TUI; arrives with v0.1 MVP)
  webox doctor                run local diagnostics and print a text report
  webox doctor --json         run local diagnostics and print JSON
  webox doctor github         run read-only GitHub integration diagnostics
  webox doctor github --json  run GitHub integration diagnostics as JSON
  webox --version             print build metadata and exit
  webox --help                print this help and exit

Flags:
  --debug          enable verbose diagnostic logging

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
	doctor       bool
	doctorJSON   bool
	doctorTarget string // "" | "github"
}

// doctorDispatcher is the seam that lets tests run the CLI router
// against a deterministic doctor implementation without touching
// package-level state.
type doctorDispatcher func(jsonOutput bool, stdout, stderr io.Writer) int

type tuiDispatcher func(stdout, stderr io.Writer) int

// Run dispatches the command implied by args (without the program name)
// and returns the process exit code. Output is written to the supplied
// writers so tests can capture it without touching os.Stdout/os.Stderr.
func Run(args []string, stdout, stderr io.Writer) int {
	return runWithFullDeps(args, stdout, stderr, runDoctor, runDoctorGitHub, runTUI)
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
	return runWithFullDeps(args, stdout, stderr, dispatch, runDoctorGitHub, startTUI)
}

func runWithFullDeps(
	args []string,
	stdout,
	stderr io.Writer,
	dispatchCore doctorDispatcher,
	dispatchGitHub doctorDispatcher,
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
		default:
			return dispatchCore(parsed.doctorJSON, stdout, stderr)
		}
	}

	// The --debug modifier is parsed (so order-independent invocations such
	// as `webox --debug --version` work today) but log-level routing lands
	// with the diagnostics wiring.
	_ = parsed.debug
	return startTUI(stdout, stderr)
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
// runTUI a stub instead. The wrapper is one line on purpose: every
// other function-shape adapter in main has the same shape and the
// repetition keeps the seam grep-able.
func realTeaProgram(model tea.Model, stdout io.Writer) teaRunner {
	return tea.NewProgram(model, tea.WithOutput(stdout))
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
		switch arg {
		case "doctor":
			parsed.doctor = true
		case "github":
			if !parsed.doctor {
				return opts{}, "webox: `github` is only valid after `doctor`."
			}
			if parsed.doctorTarget != "" && parsed.doctorTarget != "github" {
				return opts{}, fmt.Sprintf("webox: doctor target already set to %q.", parsed.doctorTarget)
			}
			parsed.doctorTarget = "github"
		case "--version":
			parsed.showVersion = true
		case "--help", "-h":
			parsed.showHelp = true
		case "--debug":
			parsed.debug = true
		case "--json":
			parsed.doctorJSON = true
		default:
			return opts{}, fmt.Sprintf(
				"webox: unknown argument %q. Run `webox --help` for usage.",
				arg,
			)
		}
	}
	if parsed.doctorJSON && !parsed.doctor {
		return opts{}, "webox: --json is only valid with `webox doctor`."
	}
	return parsed, ""
}
