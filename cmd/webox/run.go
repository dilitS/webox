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
  webox            launch the cockpit (TUI; arrives with v0.1 MVP)
  webox --version  print build metadata and exit
  webox --help     print this help and exit

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
}

// Run dispatches the command implied by args (without the program name)
// and returns the process exit code. Output is written to the supplied
// writers so tests can capture it without touching os.Stdout/os.Stderr.
func Run(args []string, stdout, stderr io.Writer) int {
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
		case "--version":
			parsed.showVersion = true
		case "--help", "-h":
			parsed.showHelp = true
		case "--debug":
			parsed.debug = true
		default:
			return opts{}, fmt.Sprintf(
				"webox: unknown argument %q. Run `webox --help` for usage.",
				arg,
			)
		}
	}
	return parsed, ""
}
