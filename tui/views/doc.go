// Package views contains the pure render functions used by the Webox TUI.
//
// Each view takes a snapshot of the tui.Model and returns a string built
// with Lipgloss styles. Views never mutate state and never start I/O —
// they are deterministic functions, which makes teatest snapshot tests
// reliable. See docs/UX.md §11 for the screen catalog and docs/DESIGN.md
// §2.3 for the MVU contract.
package views
