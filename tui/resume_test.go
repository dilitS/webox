package tui_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dilitS/webox/tui"
	"github.com/dilitS/webox/wizard"
)

func TestPendingLoadedOpensResumeWizard(t *testing.T) {
	t.Parallel()

	m := newModelWithRunner(t, &fakeRunner{})
	m, _ = runMsg(t, m, tui.ConfigLoadedMsg{Config: sampleConfig()})
	m, _ = runMsg(t, m, tui.PendingLoadedMsg{Snapshot: samplePending()})

	if m.State() != tui.StateResumeWizard {
		t.Fatalf("state = %s, want ResumeWizard", m.State())
	}
	view := m.View()
	for _, needle := range []string{"Resume Wizard", "wizard-1", "main", "Remove SSL"} {
		if !strings.Contains(view, needle) {
			t.Fatalf("view missing %q:\n%s", needle, view)
		}
	}
}

func TestResumeRollbackRunsLoadedSnapshot(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{}
	m := newModelWithRunner(t, runner)
	m, _ = runMsg(t, m, tui.ConfigLoadedMsg{Config: sampleConfig()})
	m, _ = runMsg(t, m, tui.PendingLoadedMsg{Snapshot: samplePending()})

	m, cmd := runMsg(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	if cmd == nil {
		t.Fatal("rollback key should start rollback command")
	}
	m = runUntilNil(t, m, cmd())
	if runner.rollbackCalls != 1 {
		t.Fatalf("rollback calls = %d, want 1", runner.rollbackCalls)
	}
	if m.State() != tui.StateDashboard {
		t.Fatalf("state = %s, want Dashboard", m.State())
	}
	if !strings.Contains(m.Alert(), "resume rollback complete") {
		t.Fatalf("alert = %q, want resume rollback complete", m.Alert())
	}
}

func TestResumeDiscardRequiresPhraseAndRemovesSnapshot(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pendingPath := filepath.Join(dir, "pending.json")
	if err := wizard.SavePending(context.Background(), pendingPath, samplePending()); err != nil {
		t.Fatalf("SavePending = %v", err)
	}
	m := tui.New(tui.Options{
		ConfigPath:    filepath.Join(dir, "config.json"),
		PendingPath:   pendingPath,
		WizardRunner:  &fakeRunner{},
		InitialWidth:  100,
		InitialHeight: 30,
		NewContext: func() (context.Context, context.CancelFunc) {
			return context.WithCancel(context.Background())
		},
	})
	m, _ = runMsg(t, m, tui.ConfigLoadedMsg{Config: sampleConfig()})
	m, _ = runMsg(t, m, tui.PendingLoadedMsg{Snapshot: samplePending()})
	m, _ = runMsg(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	for _, r := range "discard wizard-1" {
		m, _ = runMsg(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, cmd := runMsg(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("matching discard phrase should return a delete command")
	}
	m = runUntilNil(t, m, cmd())
	if m.State() != tui.StateDashboard {
		t.Fatalf("state = %s, want Dashboard", m.State())
	}
	if _, err := os.Stat(pendingPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("pending file should be removed, stat err = %v", err)
	}
}

func TestPendingLoadErrorOpensActionableResumeWizard(t *testing.T) {
	t.Parallel()

	m := newModelWithRunner(t, &fakeRunner{})
	m, _ = runMsg(t, m, tui.ConfigLoadedMsg{Config: sampleConfig()})
	m, _ = runMsg(t, m, tui.PendingLoadedMsg{Err: wizard.ErrCorruptedSnapshot})
	if m.State() != tui.StateResumeWizard {
		t.Fatalf("state = %s, want ResumeWizard", m.State())
	}
	if !strings.Contains(m.View(), "pending cleanup cannot be loaded") {
		t.Fatalf("view missing actionable load error:\n%s", m.View())
	}
}

func samplePending() *wizard.PendingCleanups {
	return &wizard.PendingCleanups{
		SchemaVersion: wizard.PendingSchemaVersion,
		WizardID:      "wizard-1",
		ProfileAlias:  "main",
		UpdatedAt:     time.Date(2026, 5, 23, 3, 0, 0, 0, time.UTC),
		Steps: []wizard.CleanupStep{
			{Name: "Remove subdomain app.demo.smallhost.pl", Kind: wizard.ResourceSubdomain, Params: map[string]string{"domain": "app.demo.smallhost.pl"}},
			{Name: "Remove SSL app.demo.smallhost.pl", Kind: wizard.ResourceSSL, Params: map[string]string{"domain": "app.demo.smallhost.pl"}},
		},
	}
}
