// Package tui hosts the Bubble Tea state machine that drives the Webox
// terminal cockpit.
//
// The package follows strict Model-View-Update (MVU) rules described in
// docs/DESIGN.md §2.3: Update is a pure function, View is a pure renderer,
// and every I/O effect is dispatched through tea.Cmd values returned by
// Update. The state enumeration and transition table live in states.go;
// per-screen rendering helpers live under tui/views.
package tui
