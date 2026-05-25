// Command screenshot renders a TUI snapshot to stdout for the
// docs/screenshots/* fixtures. It is a development helper, not
// shipped in the production binary.
//
// Usage:
//
//	go run ./cmd/screenshot --mode catalog --width 120 --height 35
//
// The probe drives [tui.Model] through a deterministic sequence
// of messages — `tea.WindowSizeMsg` for the canvas, then a
// fixture-defined keypress trail — and writes the resulting
// `View()` output. The mock fixture (the same one tests use)
// keeps the sample data stable across runs.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dilitS/webox/tui"
)

// Default viewport for the screenshot probe matches the
// canonical Bento Ultra cockpit (120x35) so a flag-less run
// produces a sensible regression sample.
const (
	defaultWidth    = 120
	defaultHeight   = 35
	exitUsageError  = 2
	exitInternalErr = 1
)

func main() {
	mode := flag.String("mode", "catalog", "mode: catalog|catalog-detail|help-dashboard|help-detail|standard")
	width := flag.Int("width", defaultWidth, "viewport width")
	height := flag.Int("height", defaultHeight, "viewport height")
	flag.Parse()

	model := tui.New(tui.MockOptions(""))
	var m tea.Model = model
	m, _ = m.Update(tea.WindowSizeMsg{Width: *width, Height: *height})

	switch *mode {
	case "catalog":
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	case "catalog-detail":
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	case "help-dashboard":
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	case "help-detail":
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	case "standard":
		// Pure dashboard at the requested dimensions, no
		// keypresses — useful when a layout regression
		// requires re-baselining the mini-bento screenshot.
	default:
		fmt.Fprintf(os.Stderr, "unknown mode: %q\n", *mode)
		os.Exit(exitUsageError)
	}
	out := strings.TrimRight(m.View(), "\n") + "\n"
	if _, err := os.Stdout.WriteString(out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(exitInternalErr)
	}
}
