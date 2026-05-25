package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestRenderChromeBottom_FocusedTileShowsScopedHint verifies that
// the cockpit footer swaps the global PgUp/PgDn hint for a tile-
// scoped one when a tile is focused. We exercise renderChromeBottom
// indirectly via View() so the test reflects what an operator
// actually sees.
func TestRenderChromeBottom_FocusedTileShowsScopedHint(t *testing.T) {
	t.Parallel()

	m := New(MockOptions(""))

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(Model)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)

	if m.focusedTile == nil {
		t.Fatal("setup: Tab did not focus a tile")
	}

	got := m.View()
	if !strings.Contains(got, "focus:") {
		t.Errorf("focused View missing 'focus:' hint\n--- view ---\n%s", got)
	}
	if !strings.Contains(got, "[Esc] release") {
		t.Errorf("focused View missing '[Esc] release' hint")
	}
}
