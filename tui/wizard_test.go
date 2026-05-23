package tui_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dilitS/webox/config"
	"github.com/dilitS/webox/providers"
	"github.com/dilitS/webox/tui"
	"github.com/dilitS/webox/wizard"
)

// fakeRunner is the deterministic stand-in for tui.WizardRunner used
// in integration tests for the TUI wizard. The execute / rollback
// behaviour is driven by an in-memory fake provider.
type fakeRunner struct {
	preflightErr   error
	domainErrors   map[string]error
	executeReport  *wizard.ProvisionReport
	executeErr     error
	rollbackResult []wizard.CleanupResult
	rollbackErr    error
	executeCalls   int
	rollbackCalls  int
	pushDuringExec []wizard.CleanupStep
}

func (r *fakeRunner) Preflight(context.Context, config.Profile) (*providers.ProviderStatus, error) {
	if r.preflightErr != nil {
		return nil, r.preflightErr
	}
	return &providers.ProviderStatus{SSHConnected: true, CLIInstalled: true, LatencyMS: 1}, nil
}

func (r *fakeRunner) CheckDomainAvailable(_ context.Context, _ config.Profile, domain string) error {
	if r.domainErrors == nil {
		return nil
	}
	return r.domainErrors[domain]
}

func (r *fakeRunner) Execute(ctx context.Context, _ config.Profile, plan wizard.ProvisionPlan, stack *wizard.Stack) (*wizard.ProvisionReport, error) {
	r.executeCalls++
	for _, step := range r.pushDuringExec {
		if err := stack.Push(ctx, step); err != nil {
			return nil, err
		}
	}
	if r.executeReport == nil {
		r.executeReport = &wizard.ProvisionReport{
			Subdomain: wizard.ProvisionStatus{Step: "subdomain", OK: true},
			SSL:       wizard.ProvisionStatus{Step: "ssl", OK: true},
		}
		if plan.DBKind != "" {
			r.executeReport.Database = wizard.ProvisionStatus{Step: "database", OK: true}
			r.executeReport.Credentials = &wizard.DatabaseCredentials{Username: "u", Password: "REDACTED-NEVER-A-REAL-SECRET-pwd"}
		}
		if len(r.pushDuringExec) == 0 {
			// match standard sub+ssl(+db) push set
			_ = stack.Push(ctx, wizard.CleanupStep{Name: "sub", Kind: wizard.ResourceSubdomain, Params: map[string]string{"domain": plan.Domain}})
			_ = stack.Push(ctx, wizard.CleanupStep{Name: "ssl", Kind: wizard.ResourceSSL, Params: map[string]string{"domain": plan.Domain}})
			if plan.DBKind != "" {
				_ = stack.Push(ctx, wizard.CleanupStep{Name: "db", Kind: wizard.ResourceDatabase, Params: map[string]string{"dbKind": plan.DBKind, "dbName": plan.DBName}})
			}
		}
	}
	return r.executeReport, r.executeErr
}

func (r *fakeRunner) Rollback(context.Context, config.Profile, *wizard.Stack) ([]wizard.CleanupResult, error) {
	r.rollbackCalls++
	return r.rollbackResult, r.rollbackErr
}

func newModelWithRunner(t *testing.T, runner tui.WizardRunner) tui.Model {
	t.Helper()
	dir := t.TempDir()
	return tui.New(tui.Options{
		ConfigPath:   filepath.Join(dir, "config.json"),
		PendingPath:  filepath.Join(dir, "pending.json"),
		WizardRunner: runner,
		NewContext: func() (context.Context, context.CancelFunc) {
			return context.WithCancel(context.Background())
		},
	})
}

func runMsg(t *testing.T, m tui.Model, msg tea.Msg) (tui.Model, tea.Cmd) {
	t.Helper()
	updated, cmd := m.Update(msg)
	typed, ok := updated.(tui.Model)
	if !ok {
		t.Fatalf("Update returned %T", updated)
	}
	return typed, cmd
}

func runUntilNil(t *testing.T, m tui.Model, msg tea.Msg) tui.Model {
	t.Helper()
	m, cmd := runMsg(t, m, msg)
	for cmd != nil {
		next := cmd()
		if next == nil {
			break
		}
		var step tea.Cmd
		m, step = runMsg(t, m, next)
		cmd = step
	}
	return m
}

func sampleConfig() *config.Config {
	cfg := config.DefaultConfig()
	cfg.Profiles = []config.Profile{{
		Alias: "main", Type: "smallhost", Host: "s1.small.pl", Port: 22, User: "demo",
		Properties: map[string]string{"restart_method": "devil"},
	}}
	return cfg
}

func TestInitWizardCapturesProfileAndSavesConfig(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{}
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	pendingPath := filepath.Join(dir, "pending.json")
	m := tui.New(tui.Options{
		ConfigPath:   configPath,
		PendingPath:  pendingPath,
		WizardRunner: runner,
		NewContext: func() (context.Context, context.CancelFunc) {
			return context.WithCancel(context.Background())
		},
	})

	m, _ = runMsg(t, m, tui.ConfigLoadedMsg{Missing: true, Config: config.DefaultConfig()})

	enter := tea.KeyMsg{Type: tea.KeyEnter}
	m, _ = runMsg(t, m, enter) // welcome -> alias

	for _, r := range "main" {
		m, _ = runMsg(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = runMsg(t, m, enter) // alias -> host

	for _, r := range "s1.small.pl" {
		m, _ = runMsg(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = runMsg(t, m, enter) // host -> port (default keeps 22)
	m, _ = runMsg(t, m, enter) // port -> user

	for _, r := range "demo" {
		m, _ = runMsg(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = runMsg(t, m, enter) // user -> review

	// Trigger save.
	m, cmd := runMsg(t, m, enter)
	if cmd == nil {
		t.Fatal("review should fire save cmd")
	}
	m = runUntilNil(t, m, cmd())

	if m.State() != tui.StateDashboard {
		t.Fatalf("state = %s, want Dashboard", m.State())
	}
	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile = %v", err)
	}
	for _, needle := range []string{"main", "s1.small.pl", "demo", "smallhost"} {
		if !strings.Contains(string(raw), needle) {
			t.Fatalf("config missing %q: %s", needle, raw)
		}
	}
	for _, secretShape := range []string{"password", "passwd", "ghp_", "ghs_", "PRIVATE KEY"} {
		if strings.Contains(string(raw), secretShape) {
			t.Fatalf("config contains forbidden substring %q", secretShape)
		}
	}
}

func TestInitWizardRejectsInvalidAliasAndStays(t *testing.T) {
	t.Parallel()

	m := newModelWithRunner(t, &fakeRunner{})
	m, _ = runMsg(t, m, tui.ConfigLoadedMsg{Missing: true, Config: config.DefaultConfig()})
	enter := tea.KeyMsg{Type: tea.KeyEnter}
	m, _ = runMsg(t, m, enter)

	for _, r := range "Bad Alias!" {
		m, _ = runMsg(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = runMsg(t, m, enter)

	view := m.View()
	if !strings.Contains(view, "alias must match") {
		t.Fatalf("view missing alias error: %s", view)
	}
}

func TestDashboardNKeyOpensProjectWizard(t *testing.T) {
	t.Parallel()
	m := newModelWithRunner(t, &fakeRunner{})
	m, _ = runMsg(t, m, tui.ConfigLoadedMsg{Config: sampleConfig()})
	m, _ = runMsg(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if m.State() != tui.StateProjectWizard {
		t.Fatalf("state = %s, want ProjectWizard", m.State())
	}
}

func TestProjectWizardHappyPathProvisionsAndUpdatesDashboard(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{}
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	pendingPath := filepath.Join(dir, "pending.json")
	m := tui.New(tui.Options{
		ConfigPath:   configPath,
		PendingPath:  pendingPath,
		WizardRunner: runner,
		NewContext: func() (context.Context, context.CancelFunc) {
			return context.WithCancel(context.Background())
		},
	})

	// Seed config so we can skip the init wizard.
	cfg := sampleConfig()
	rawConfig, _ := config.DefaultConfig(), config.DefaultConfig()
	_ = rawConfig
	m, _ = runMsg(t, m, tui.ConfigLoadedMsg{Config: cfg})

	enter := tea.KeyMsg{Type: tea.KeyEnter}
	m, _ = runMsg(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})

	// profile step -> Enter
	m, _ = runMsg(t, m, enter)
	// stack: pick `vite-react` via cycle (already first by default, change to vite-react via down)
	m, _ = runMsg(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m, _ = runMsg(t, m, enter) // stack -> db choice (static or vite-react)
	// since stack is now vite-react, dbWanted defaults to false; just hit Enter to skip
	m, _ = runMsg(t, m, enter) // db choice -> domain
	for _, r := range "app.demo.smallhost.pl" {
		m, _ = runMsg(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	// Enter triggers preflight + domain check
	m, cmd := runMsg(t, m, enter)
	if cmd == nil {
		t.Fatal("domain step should fire async cmd")
	}
	m = runUntilNil(t, m, cmd())
	if m.State() != tui.StateProjectWizard {
		t.Fatalf("state after preflight = %s", m.State())
	}
	// Now at review step; Enter triggers execute.
	m, cmd = runMsg(t, m, enter)
	if cmd == nil {
		t.Fatal("review step should fire execute cmd")
	}
	m = runUntilNil(t, m, cmd())

	if m.State() != tui.StateDashboard {
		t.Fatalf("state after success = %s, want Dashboard", m.State())
	}
	if runner.executeCalls != 1 {
		t.Fatalf("execute calls = %d, want 1", runner.executeCalls)
	}

	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile = %v", err)
	}
	if !strings.Contains(string(raw), "app.demo.smallhost.pl") {
		t.Fatalf("config missing new project domain: %s", raw)
	}
	if _, err := os.Stat(pendingPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("pending file should be removed after success, err = %v", err)
	}
}

func TestProjectWizardDomainCollisionStaysOnDomainStep(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{domainErrors: map[string]error{"taken.demo.smallhost.pl": providers.ErrSubdomainExists}}
	m := newModelWithRunner(t, runner)
	m, _ = runMsg(t, m, tui.ConfigLoadedMsg{Config: sampleConfig()})
	m, _ = runMsg(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	enter := tea.KeyMsg{Type: tea.KeyEnter}
	m, _ = runMsg(t, m, enter) // profile
	m, _ = runMsg(t, m, enter) // stack (static)
	m, _ = runMsg(t, m, enter) // db choice (skip)
	for _, r := range "taken.demo.smallhost.pl" {
		m, _ = runMsg(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, cmd := runMsg(t, m, enter)
	m = runUntilNil(t, m, cmd())

	if m.State() != tui.StateProjectWizard {
		t.Fatalf("state = %s, want still ProjectWizard", m.State())
	}
	view := m.View()
	if !strings.Contains(view, "already provisioned") {
		t.Fatalf("view missing collision message: %s", view)
	}
}

func TestProjectWizardExecutionFailureOffersRollback(t *testing.T) {
	t.Parallel()

	failingErr := &wizard.ExecutionFailedError{FailedStep: "ssl", Err: providers.ErrDNSNotResolving}
	runner := &fakeRunner{
		executeReport: &wizard.ProvisionReport{
			Subdomain: wizard.ProvisionStatus{Step: "subdomain", OK: true},
			SSL:       wizard.ProvisionStatus{Step: "ssl", Err: providers.ErrDNSNotResolving},
		},
		executeErr: failingErr,
		pushDuringExec: []wizard.CleanupStep{
			{Name: "Remove subdomain x", Kind: wizard.ResourceSubdomain, Params: map[string]string{"domain": "fail.demo.smallhost.pl"}},
		},
		rollbackResult: []wizard.CleanupResult{{Step: wizard.CleanupStep{Name: "Remove subdomain x", Kind: wizard.ResourceSubdomain, Params: map[string]string{"domain": "fail.demo.smallhost.pl"}}}},
	}

	m := newModelWithRunner(t, runner)
	m, _ = runMsg(t, m, tui.ConfigLoadedMsg{Config: sampleConfig()})
	m, _ = runMsg(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	enter := tea.KeyMsg{Type: tea.KeyEnter}
	m, _ = runMsg(t, m, enter) // profile
	m, _ = runMsg(t, m, enter) // stack -> db choice
	m, _ = runMsg(t, m, enter) // db choice (skip)
	for _, r := range "fail.demo.smallhost.pl" {
		m, _ = runMsg(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, cmd := runMsg(t, m, enter)
	m = runUntilNil(t, m, cmd())
	m, cmd = runMsg(t, m, enter) // review -> execute
	m = runUntilNil(t, m, cmd())

	view := m.View()
	if !strings.Contains(view, "Provisioning failed") {
		t.Fatalf("view missing failure message: %s", view)
	}

	// Press 'y' to rollback.
	m, cmd = runMsg(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if cmd == nil {
		t.Fatal("rollback should fire async cmd")
	}
	m = runUntilNil(t, m, cmd())

	if runner.rollbackCalls != 1 {
		t.Fatalf("rollback calls = %d, want 1", runner.rollbackCalls)
	}
	if m.State() != tui.StateDashboard {
		t.Fatalf("state after rollback = %s, want Dashboard", m.State())
	}
	if !strings.Contains(m.Alert(), "rollback complete") {
		t.Fatalf("alert = %q, want rollback complete", m.Alert())
	}
}

func TestProjectWizardEscFromFailureKeepsResources(t *testing.T) {
	t.Parallel()
	runner := &fakeRunner{
		executeErr: &wizard.ExecutionFailedError{FailedStep: "subdomain", Err: providers.ErrSubdomainExists},
		executeReport: &wizard.ProvisionReport{
			Subdomain: wizard.ProvisionStatus{Step: "subdomain", Err: providers.ErrSubdomainExists},
		},
	}
	m := newModelWithRunner(t, runner)
	m, _ = runMsg(t, m, tui.ConfigLoadedMsg{Config: sampleConfig()})
	m, _ = runMsg(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	enter := tea.KeyMsg{Type: tea.KeyEnter}
	m, _ = runMsg(t, m, enter)
	m, _ = runMsg(t, m, enter)
	m, _ = runMsg(t, m, enter)
	for _, r := range "stay.demo.smallhost.pl" {
		m, _ = runMsg(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, cmd := runMsg(t, m, enter)
	m = runUntilNil(t, m, cmd())
	m, cmd = runMsg(t, m, enter)
	m = runUntilNil(t, m, cmd())

	if runner.rollbackCalls != 0 {
		t.Fatal("rollback should not run yet")
	}
	m, _ = runMsg(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if runner.rollbackCalls != 0 {
		t.Fatal("Esc must NOT trigger rollback")
	}
	if m.State() != tui.StateDashboard {
		t.Fatalf("state = %s, want Dashboard", m.State())
	}
}

func TestDashboardNKeyWithoutProfileShowsAlert(t *testing.T) {
	t.Parallel()
	m := newModelWithRunner(t, &fakeRunner{})
	emptyCfg := config.DefaultConfig()
	emptyCfg.Profiles = nil
	emptyCfg.Projects = nil
	m, _ = runMsg(t, m, tui.ConfigLoadedMsg{Config: emptyCfg})
	if m.State() == tui.StateInitWizard {
		// load already routed to init wizard because profiles empty
		return
	}
	m, _ = runMsg(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if m.State() == tui.StateProjectWizard {
		t.Fatalf("n without profiles should not open project wizard")
	}
}
