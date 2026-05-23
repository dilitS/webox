package main

import (
	"fmt"
	"io"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dilitS/webox/internal/version"
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
  webox              launch the cockpit (TUI; arrives with v0.1 MVP)
  webox doctor       run local diagnostics and print a text report
  webox doctor --json
                     run local diagnostics and print JSON
  webox --version    print build metadata and exit
  webox --help       print this help and exit

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
	showVersion bool
	showHelp    bool
	debug       bool
	doctor      bool
	doctorJSON  bool
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
	return runWithDeps(args, stdout, stderr, runDoctor, runTUI)
}

func runWithDeps(
	args []string,
	stdout,
	stderr io.Writer,
	dispatch doctorDispatcher,
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
		return dispatch(parsed.doctorJSON, stdout, stderr)
	}

	// The --debug modifier is parsed (so order-independent invocations such
	// as `webox --debug --version` work today) but log-level routing lands
	// with the diagnostics wiring.
	_ = parsed.debug
	return startTUI(stdout, stderr)
}

func runTUI(stdout, stderr io.Writer) int {
	cfgPath, err := tui.DefaultConfigPath()
	if err != nil {
		fmt.Fprintf(stderr, "webox: resolve config path: %v\n", err)
		return exitMisuse
	}
	program := tea.NewProgram(
		tui.New(tui.Options{ConfigPath: cfgPath}),
		tea.WithOutput(stdout),
	)
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(stderr, "webox: TUI failed: %v\n", err)
		return exitMisuse
	}
	return exitOK
}

func parseArgs(args []string) (parsed opts, errMsg string) {
	for _, arg := range args {
		switch arg {
		case "doctor":
			parsed.doctor = true
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
