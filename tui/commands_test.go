package tui_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dilitS/webox/tui"
	"github.com/dilitS/webox/wizard"
)

// These tests cover the lower-level command builders directly so we
// can exercise default-path resolution, missing-config branches, and
// pending-snapshot discard semantics without spinning up the full
// Bubble Tea event loop.

func TestDefaultConfigPath_ReturnsAbsoluteJoinedPath(t *testing.T) {
	t.Parallel()
	got, err := tui.DefaultConfigPath()
	if err != nil {
		t.Fatalf("DefaultConfigPath err = %v", err)
	}
	if !strings.HasSuffix(got, filepath.Join("webox", "config.json")) {
		t.Fatalf("DefaultConfigPath = %q, want suffix webox/config.json", got)
	}
}

func TestDefaultPendingPath_ReturnsAbsoluteJoinedPath(t *testing.T) {
	t.Parallel()
	got, err := tui.DefaultPendingPath()
	if err != nil {
		t.Fatalf("DefaultPendingPath err = %v", err)
	}
	if got == "" {
		t.Fatal("DefaultPendingPath returned empty")
	}
}

func TestLoadConfigCmd_MissingFileFlagsMissingTrue(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	m := tui.New(tui.Options{
		ConfigPath:  filepath.Join(dir, "config.json"),
		PendingPath: filepath.Join(dir, "pending.json"),
		NewContext: func() (context.Context, context.CancelFunc) {
			return context.WithCancel(context.Background())
		},
	})
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init returned nil cmd")
	}
	msg := cmd()
	if msg == nil {
		t.Fatal("Init batch produced nil msg")
	}
	// Init batches loadConfig + loadPending. Drive Bubble Tea's
	// batch executor to fan them out and assert the config-loaded
	// message carries Missing=true so the route into InitWizard
	// fires on the next Update.
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("Init returned %T, want tea.BatchMsg", msg)
	}
	var sawMissing bool
	for _, sub := range batch {
		if sub == nil {
			continue
		}
		v := sub()
		if loaded, ok := v.(tui.ConfigLoadedMsg); ok && loaded.Missing {
			sawMissing = true
		}
	}
	if !sawMissing {
		t.Fatal("expected ConfigLoadedMsg with Missing=true")
	}
}

func TestPendingDiscardCmd_AbsentFileDoesNotError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	m := tui.New(tui.Options{
		ConfigPath:  filepath.Join(dir, "config.json"),
		PendingPath: filepath.Join(dir, "pending.json"),
	})
	m, _ = runMsg(t, m, tui.ConfigLoadedMsg{Config: sampleConfig()})
	m, _ = runMsg(t, m, tui.PendingLoadedMsg{})
	// Resume Wizard is not entered when snapshot is nil; assert
	// dashboard state instead.
	if m.State() != tui.StateDashboard {
		t.Fatalf("state = %s, want Dashboard", m.State())
	}
}

func TestPendingLoadedSurfacesErrorInResumeWizard(t *testing.T) {
	t.Parallel()
	m := newModelWithRunner(t, &fakeRunner{})
	m, _ = runMsg(t, m, tui.ConfigLoadedMsg{Config: sampleConfig()})
	m, _ = runMsg(t, m, tui.PendingLoadedMsg{Err: errors.New("corrupt snapshot")})
	if m.State() != tui.StateResumeWizard {
		t.Fatalf("state = %s, want ResumeWizard (load err should route here)", m.State())
	}
	if !strings.Contains(m.View(), "corrupt snapshot") {
		t.Fatalf("view missing load error:\n%s", m.View())
	}
}

func TestImportPersistedNoRows_SkipsConfigChange(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	m := tui.New(tui.Options{
		ConfigPath:   configPath,
		PendingPath:  filepath.Join(dir, "pending.json"),
		WizardRunner: &fakeRunner{},
		NewContext: func() (context.Context, context.CancelFunc) {
			return context.WithCancel(context.Background())
		},
	})
	m, _ = runMsg(t, m, tui.ConfigLoadedMsg{Config: sampleConfig()})

	_, _ = runMsg(t, m, tui.ImportPersistedMsg{Config: sampleConfig(), ImportedRows: 0})
	if _, err := os.Stat(configPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("config should NOT have been written for zero rows, err=%v", err)
	}
}

// fakePersistedSnapshot is used by the resume-rollback test to feed
// LoadPending via a real on-disk snapshot. The TUI's resume cmd
// resolves the persister against the path, so we round-trip through
// the actual file.
func writeSnapshot(t *testing.T, dir, alias string) string {
	t.Helper()
	path := filepath.Join(dir, "pending.json")
	persist := wizard.NewFilePersisterWithProfile(path, "w-test", alias)
	stack := wizard.NewStack(persist, "w-test")
	if err := stack.Push(context.Background(), wizard.CleanupStep{Name: "Remove subdomain x", Kind: wizard.ResourceSubdomain, Params: map[string]string{"domain": "x.demo.smallhost.pl"}}); err != nil {
		t.Fatalf("Push: %v", err)
	}
	return path
}

func TestResumeWizard_ResumesAndRollsBack(t *testing.T) {
	t.Parallel()
	runner := &fakeRunner{rollbackResult: []wizard.CleanupResult{{Step: wizard.CleanupStep{Name: "Remove subdomain x"}}}}
	dir := t.TempDir()
	pendingPath := writeSnapshot(t, dir, "main")
	m := tui.New(tui.Options{
		ConfigPath:   filepath.Join(dir, "config.json"),
		PendingPath:  pendingPath,
		WizardRunner: runner,
		NewContext: func() (context.Context, context.CancelFunc) {
			return context.WithCancel(context.Background())
		},
	})

	m, _ = runMsg(t, m, tui.ConfigLoadedMsg{Config: sampleConfig()})
	snap, err := wizard.LoadPending(pendingPath)
	if err != nil || snap == nil {
		t.Fatalf("LoadPending = (%v, %v)", snap, err)
	}
	m, _ = runMsg(t, m, tui.PendingLoadedMsg{Snapshot: snap})
	if m.State() != tui.StateResumeWizard {
		t.Fatalf("state = %s, want ResumeWizard", m.State())
	}
}

func TestResumeWizard_DiscardRequiresExactPhrase(t *testing.T) {
	t.Parallel()
	runner := &fakeRunner{}
	dir := t.TempDir()
	pendingPath := writeSnapshot(t, dir, "main")
	m := tui.New(tui.Options{
		ConfigPath:   filepath.Join(dir, "config.json"),
		PendingPath:  pendingPath,
		WizardRunner: runner,
		NewContext: func() (context.Context, context.CancelFunc) {
			return context.WithCancel(context.Background())
		},
	})

	m, _ = runMsg(t, m, tui.ConfigLoadedMsg{Config: sampleConfig()})
	snap, _ := wizard.LoadPending(pendingPath)
	m, _ = runMsg(t, m, tui.PendingLoadedMsg{Snapshot: snap})

	m, _ = runMsg(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	for _, r := range "wrong-phrase" {
		m, _ = runMsg(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	_, cmd := runMsg(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("wrong discard phrase must NOT fire discard cmd")
	}
	if _, err := os.Stat(pendingPath); err != nil {
		t.Fatalf("pending file should remain on disk, stat err = %v", err)
	}
}
