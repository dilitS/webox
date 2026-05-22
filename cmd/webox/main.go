// Command webox is the entry point of the Webox TUI.
//
// Webox is a keyboard-driven cockpit that orchestrates GitHub-driven
// deployments to shared hosting (small.pl/Devil in MVP, additional
// providers in v0.2+). Operator workflows live behind the Bubble Tea
// TUI; only `webox doctor` and a small set of startup/debug flags
// (`--version`, `--help`, `--debug`) are exposed as non-interactive
// commands per ADR-0001.
package main

import "os"

func main() {
	os.Exit(Run(os.Args[1:], os.Stdout, os.Stderr))
}
