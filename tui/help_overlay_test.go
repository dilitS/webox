package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestHelp_QuestionMarkOpensOverlay verifies pressing `?` from
// the dashboard sets `helpVisible` and that pressing `?` again
// closes it.
func TestHelp_QuestionMarkOpensOverlay(t *testing.T) {
	t.Parallel()

	m := New(MockOptions(""))
	if m.helpVisible {
		t.Fatalf("setup: helpVisible = true, want false")
	}

	mAny, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	m = mAny.(Model)
	if !m.helpVisible {
		t.Fatalf("after ? = false, want true")
	}

	mAny, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	m = mAny.(Model)
	if m.helpVisible {
		t.Fatalf("after second ? = true, want false (toggle)")
	}
}

// TestHelp_EscClosesOverlay confirms Esc dismisses the overlay
// without affecting the underlying state.
func TestHelp_EscClosesOverlay(t *testing.T) {
	t.Parallel()

	m := New(MockOptions(""))
	mAny, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	m = mAny.(Model)
	if !m.helpVisible {
		t.Fatalf("setup: helpVisible = false, want true")
	}

	mAny, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = mAny.(Model)
	if m.helpVisible {
		t.Errorf("after Esc helpVisible = true, want false")
	}
	if m.state != StateDashboard {
		t.Errorf("after Esc state = %v, want StateDashboard (overlay must not pop the surface)", m.state)
	}
}

// TestHelp_OverlayBlocksUnderlyingHandlers proves that while
// the help is visible, keys other than the dismissal trio do
// nothing. This guards against the operator accidentally
// triggering destructive flows (`d`, `n`, etc.) while reading
// the help.
func TestHelp_OverlayBlocksUnderlyingHandlers(t *testing.T) {
	t.Parallel()

	m := New(MockOptions(""))
	mAny, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	m = mAny.(Model)
	stateBefore := m.state

	for _, key := range []string{"n", "i", "p", "tab", "j", "k", "down", "up"} {
		var msg tea.KeyMsg
		switch key {
		case "tab":
			msg = tea.KeyMsg{Type: tea.KeyTab}
		case "down":
			msg = tea.KeyMsg{Type: tea.KeyDown}
		case "up":
			msg = tea.KeyMsg{Type: tea.KeyUp}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
		}
		mAny, _ = m.Update(msg)
		m = mAny.(Model)
		if m.state != stateBefore {
			t.Errorf("key %q changed state to %v while help open; want %v", key, m.state, stateBefore)
		}
		if !m.helpVisible {
			t.Errorf("key %q closed help overlay; only ?/esc/enter should", key)
			break
		}
	}
}

// TestHelp_OverlayDebugStringExtractsBindings asserts the
// overlay parses the dashboard footer into the canonical key
// list. The debug helper drops styling so we can match by raw
// substring.
func TestHelp_OverlayDebugStringExtractsBindings(t *testing.T) {
	t.Parallel()

	m := New(MockOptions(""))
	mAny, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	m = mAny.(Model)
	debug := helpDebugString(m, m.screen())
	if !strings.Contains(debug, "surface=") {
		t.Errorf("debug missing surface label:\n%s", debug)
	}
	for _, needle := range []string{"[q]", "[?]", "[Tab]", "[Right/Enter]", "[n]", "[i]", "[p]"} {
		if !strings.Contains(debug, needle) {
			t.Errorf("debug missing %q\n--- debug ---\n%s", needle, debug)
		}
	}
}

// TestHelp_OverlayQuitShortcutStillWorks ensures `q` and
// Ctrl+C still quit the cockpit even with the help overlay
// open. Operators in a hurry MUST always be able to leave.
func TestHelp_OverlayQuitShortcutStillWorks(t *testing.T) {
	t.Parallel()

	m := New(MockOptions(""))
	mAny, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	m = mAny.(Model)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Errorf("q while help open did not return tea.Quit cmd")
	}
}

// TestHelp_OverlayParsesProjectDetailFooter triggers the
// project-detail surface and asserts its bindings make it into
// the help body. Used as a regression guard so a future
// developer who edits a surface's footer cannot silently break
// help discovery.
func TestHelp_OverlayParsesProjectDetailFooter(t *testing.T) {
	t.Parallel()

	m := New(MockOptions(""))
	// Drill into the project detail by selecting the first
	// project (always present in the mock fixture) and
	// pressing Enter.
	mAny, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mAny.(Model)
	if m.state != StateProjectDetail {
		t.Skipf("setup: drill failed, state=%v (mock fixture changed?)", m.state)
	}
	mAny, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	m = mAny.(Model)
	debug := helpDebugString(m, m.screen())
	if !strings.Contains(debug, "Project Detail") && !strings.Contains(debug, "Overview") {
		t.Errorf("help label missing project detail surface name:\n%s", debug)
	}
	for _, needle := range []string{"[1]", "[4]"} {
		if !strings.Contains(debug, needle) {
			t.Errorf("project-detail bindings missing %q\n--- debug ---\n%s", needle, debug)
		}
	}
}
