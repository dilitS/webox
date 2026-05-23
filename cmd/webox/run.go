package main

import (
	"fmt"
	"io"

	"github.com/dilitS/webox/internal/version"
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

// Run dispatches the command implied by args (without the program name)
// and returns the process exit code. Output is written to the supplied
// writers so tests can capture it without touching os.Stdout/os.Stderr.
func Run(args []string, stdout, stderr io.Writer) int {
	return runWith(args, stdout, stderr, runDoctor)
}

// runWith is the testable CLI router. Production wires it to runDoctor;
// table-driven tests pass a stub dispatcher to verify the dispatch
// surface in isolation from the doctor service.
func runWith(args []string, stdout, stderr io.Writer, dispatch doctorDispatcher) int {
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

	// `webox` alone shows help until the TUI is wired in Sprint 04. The
	// --debug modifier is parsed (so order-independent invocations such
	// as `webox --debug --version` work today) but has no observable
	// effect yet.
	_ = parsed.debug
	fmt.Fprint(stdout, helpText)
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
