package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"go.uber.org/goleak"
)

func TestQuitTransitionDoesNotLeakGoroutines(t *testing.T) {
	defer goleak.VerifyNone(t)

	m := New(Options{}).withConfig(fixtureConfig())
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("quit command is nil")
	}
}
