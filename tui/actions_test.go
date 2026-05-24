package tui_test

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dilitS/webox/config"
	"github.com/dilitS/webox/tui"
)

func seedWithProject(t *testing.T, runner tui.WizardRunner) tui.Model {
	t.Helper()
	cfg := sampleConfig()
	cfg.Projects = []config.Project{{
		ID:           "p-1",
		Domain:       "app.demo.smallhost.pl",
		ProfileAlias: "main",
		Stack:        "node-express",
		NodeVersion:  "22",
	}}
	m := newModelWithRunner(t, runner)
	m, _ = runMsg(t, m, tui.ConfigLoadedMsg{Config: cfg})
	enter := tea.KeyMsg{Type: tea.KeyEnter}
	m, _ = runMsg(t, m, enter)
	return m
}

func TestProjectDetailRestartDispatchesRunner(t *testing.T) {
	t.Parallel()
	runner := &fakeRunner{}
	m := seedWithProject(t, runner)
	if m.State() != tui.StateProjectDetail {
		t.Fatalf("state = %s, want ProjectDetail", m.State())
	}

	m, cmd := runMsg(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	if cmd == nil {
		t.Fatal("r should dispatch restart command")
	}
	m = runUntilNil(t, m, cmd())

	if runner.restartCalls != 1 {
		t.Fatalf("restartCalls = %d, want 1", runner.restartCalls)
	}
	if !strings.Contains(m.Alert(), "restart succeeded") {
		t.Fatalf("alert = %q, want restart succeeded", m.Alert())
	}
}

func TestProjectDetailRestartFailureSurfacedAsAlert(t *testing.T) {
	t.Parallel()
	runner := &fakeRunner{restartErr: errors.New("panel offline")}
	m := seedWithProject(t, runner)
	m, cmd := runMsg(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	m = runUntilNil(t, m, cmd())

	if runner.restartCalls != 1 {
		t.Fatalf("restartCalls = %d, want 1", runner.restartCalls)
	}
	if !strings.Contains(m.Alert(), "restart failed: panel offline") {
		t.Fatalf("alert = %q, want restart failed", m.Alert())
	}
}

func TestProjectDetailSSLRenewDispatchesRunner(t *testing.T) {
	t.Parallel()
	runner := &fakeRunner{}
	m := seedWithProject(t, runner)
	m, cmd := runMsg(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	m = runUntilNil(t, m, cmd())
	if runner.renewCalls != 1 {
		t.Fatalf("renewCalls = %d, want 1", runner.renewCalls)
	}
	if !strings.Contains(m.Alert(), "ssl_renew succeeded") {
		t.Fatalf("alert = %q, want ssl_renew succeeded", m.Alert())
	}
}

func TestProjectDetailLogsRendersTailOutput(t *testing.T) {
	t.Parallel()
	logBody := "[info] booted on :3000\n[info] GET / 200\n[error] db connection slow\n"
	runner := &fakeRunner{tailOutput: []byte(logBody)}
	m := seedWithProject(t, runner)
	m, cmd := runMsg(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")})
	m = runUntilNil(t, m, cmd())

	if runner.tailCalls != 1 {
		t.Fatalf("tailCalls = %d, want 1", runner.tailCalls)
	}
	view := m.View()
	for _, needle := range []string{"booted on :3000", "GET / 200", "db connection slow", "logs (last"} {
		if !strings.Contains(view, needle) {
			t.Fatalf("view missing %q:\n%s", needle, view)
		}
	}
}

func TestProjectDetailLogsClampsLongOutput(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	const totalLines = 50
	for i := 0; i < totalLines; i++ {
		b.WriteString("line\n")
	}
	runner := &fakeRunner{tailOutput: []byte(b.String())}
	m := seedWithProject(t, runner)
	m, cmd := runMsg(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")})
	m = runUntilNil(t, m, cmd())

	view := m.View()
	if !strings.Contains(view, "older lines omitted") {
		t.Fatalf("view missing omitted-lines hint:\n%s", view)
	}
}

func TestProjectDetailRestartWithoutProjectDoesNotDispatch(t *testing.T) {
	t.Parallel()
	runner := &fakeRunner{}
	m := newModelWithRunner(t, runner)
	cfg := sampleConfig()
	cfg.Projects = nil
	m, _ = runMsg(t, m, tui.ConfigLoadedMsg{Config: cfg})

	// Dashboard ignores `r` outside of detail; this asserts the
	// guard actually fires (no silent restart dispatch when there
	// is no project to act on).
	_, _ = runMsg(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	if runner.restartCalls != 0 {
		t.Fatalf("restartCalls = %d, want 0 (guard should block dispatch)", runner.restartCalls)
	}
}

func TestProjectDetailDoesNotDispatchTwice(t *testing.T) {
	t.Parallel()
	runner := &fakeRunner{}
	m := seedWithProject(t, runner)

	m, cmd := runMsg(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	// Don't complete the first command; immediately press r again.
	m, secondCmd := runMsg(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	if secondCmd != nil {
		t.Fatal("second restart while running should be ignored")
	}
	_ = runUntilNil(t, m, cmd())
	if runner.restartCalls != 1 {
		t.Fatalf("restartCalls = %d, want 1", runner.restartCalls)
	}
}
