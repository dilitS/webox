package tui_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/dilitS/webox/config"
	"github.com/dilitS/webox/providers"
	"github.com/dilitS/webox/tui"
)

func seedDashboardForImport(t *testing.T, runner *fakeRunner) (model tui.Model, configPath string) {
	t.Helper()
	cfg := sampleConfig()
	cfg.Projects = []config.Project{{
		ID:           uuid.NewString(),
		Domain:       "managed.demo.smallhost.pl",
		ProfileAlias: "main",
		Stack:        "node-express",
		NodeVersion:  "20",
	}}
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	if err := config.Save(context.Background(), cfgPath, cfg); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	m := tui.New(tui.Options{
		ConfigPath:   cfgPath,
		PendingPath:  filepath.Join(dir, "pending.json"),
		WizardRunner: runner,
		NewContext: func() (context.Context, context.CancelFunc) {
			return context.WithCancel(context.Background())
		},
	})
	m, _ = runMsg(t, m, tui.ConfigLoadedMsg{Config: cfg})
	if m.State() != tui.StateDashboard {
		t.Fatalf("seed state = %s, want Dashboard", m.State())
	}
	return m, cfgPath
}

func TestImport_DashboardKeyDispatchesScanCmd(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{listSubdomains: []providers.Subdomain{{Domain: "x.smallhost.pl", Type: "nodejs"}}}
	m, _ := seedDashboardForImport(t, runner)

	got, cmd := runMsg(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
	if cmd == nil {
		t.Fatal("`i` from dashboard must return a tea.Cmd to start the scan")
	}
	if got.State() != tui.StateImportPreview {
		t.Fatalf("state after `i` = %s, want ImportPreview", got.State())
	}
}

func TestImport_ScanResultPopulatesPreview(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{}
	m, _ := seedDashboardForImport(t, runner)
	m, _ = runMsg(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})

	msg := tui.ImportScanCompletedMsg{
		Rows: []tui.ImportRow{
			{Domain: "managed.demo.smallhost.pl", Type: "nodejs", NodeVersion: "20", ProfileAlias: "main", Managed: true},
			{Domain: "ghost.demo.smallhost.pl", Type: "nodejs", NodeVersion: "22", ProfileAlias: "main"},
			{Domain: "static.demo.smallhost.pl", Type: "static", ProfileAlias: "main"},
		},
	}
	got, _ := runMsg(t, m, msg)

	snap, ok := got.ImportSnapshot()
	if !ok {
		t.Fatal("ImportSnapshot should be available after scan completes")
	}
	if snap.Total != 3 {
		t.Fatalf("Total = %d, want 3", snap.Total)
	}
	if snap.Unmanaged != 2 {
		t.Fatalf("Unmanaged = %d, want 2", snap.Unmanaged)
	}
	if snap.Managed != 1 {
		t.Fatalf("Managed = %d, want 1", snap.Managed)
	}
}

func TestImport_ScanErrorSurfacesAlertAndReturnsToDashboard(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{}
	m, _ := seedDashboardForImport(t, runner)
	m, _ = runMsg(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})

	got, _ := runMsg(t, m, tui.ImportScanCompletedMsg{Err: errors.New("ssh unavailable")})
	if got.State() != tui.StateDashboard {
		t.Fatalf("state after scan error = %s, want Dashboard", got.State())
	}
	if !strings.Contains(got.Alert(), "import scan failed") {
		t.Fatalf("alert=%q, want import scan failed", got.Alert())
	}
}

func TestImport_EscReturnsToDashboard(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{}
	m, _ := seedDashboardForImport(t, runner)
	m, _ = runMsg(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
	m, _ = runMsg(t, m, tui.ImportScanCompletedMsg{Rows: []tui.ImportRow{{Domain: "x.smallhost.pl"}}})

	got, _ := runMsg(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if got.State() != tui.StateDashboard {
		t.Fatalf("state after esc = %s, want Dashboard", got.State())
	}
}

func TestImport_AcceptAppendsUnmanagedProjectsAndPersists(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{}
	m, cfgPath := seedDashboardForImport(t, runner)
	m, _ = runMsg(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
	m, _ = runMsg(t, m, tui.ImportScanCompletedMsg{
		Rows: []tui.ImportRow{
			{Domain: "managed.demo.smallhost.pl", Type: "nodejs", NodeVersion: "20", ProfileAlias: "main", Managed: true},
			{Domain: "ghost.demo.smallhost.pl", Type: "nodejs", NodeVersion: "22", ProfileAlias: "main"},
		},
	})

	m, cmd := runMsg(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	if cmd == nil {
		t.Fatal("`a` should return a tea.Cmd that persists imports")
	}
	_ = runUntilNil(t, m, cmd())

	persisted, err := config.Load(context.Background(), cfgPath)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(persisted.Projects) != 2 {
		t.Fatalf("projects after import = %d, want 2; %+v", len(persisted.Projects), persisted.Projects)
	}
	found := false
	for _, project := range persisted.Projects {
		if project.Domain == "ghost.demo.smallhost.pl" {
			found = true
			if project.ImportedAt == nil {
				t.Fatal("ImportedAt should be set on imported project")
			}
		}
	}
	if !found {
		t.Fatalf("imported project not found in persisted config: %+v", persisted.Projects)
	}
}
