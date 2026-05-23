package tui

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dilitS/webox/config"
	"github.com/dilitS/webox/providers"
	"github.com/dilitS/webox/wizard"
)

func TestProjectWizardKeymapMatrix(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		SchemaVersion: config.Current,
		Profiles: []config.Profile{
			{Alias: "main", Type: "smallhost", Host: "s1.small.pl", User: "demo"},
		},
	}
	tests := []struct {
		name     string
		step     ProjectWizardStep
		key      tea.KeyMsg
		wantStep ProjectWizardStep
		wantText string
	}{
		{name: "profile enter advances", step: ProjectStepProfile, key: tea.KeyMsg{Type: tea.KeyEnter}, wantStep: ProjectStepStack},
		{name: "stack down changes stack only", step: ProjectStepStack, key: tea.KeyMsg{Type: tea.KeyDown}, wantStep: ProjectStepStack, wantText: "vite-react"},
		{name: "domain j is text input", step: ProjectStepDomain, key: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}, wantStep: ProjectStepDomain, wantText: "j"},
		{name: "db name k is text input", step: ProjectStepDBName, key: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")}, wantStep: ProjectStepDBName, wantText: "k"},
		{name: "failure n exits to dashboard", step: ProjectStepFailure, key: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}, wantStep: ProjectStepDone},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := New(Options{WizardRunner: noopRunner{}})
			m.cfg = cfg
			m.state = StateProjectWizard
			m.projectForm = newProjectWizardForm(cfg)
			m.projectForm.step = tt.step
			updated, _ := m.updateProjectWizardKey(tt.key)
			got := updated.(Model)
			if tt.name == "failure n exits to dashboard" {
				if got.State() != StateDashboard {
					t.Fatalf("state = %s, want Dashboard", got.State())
				}
				return
			}
			if got.projectForm.step != tt.wantStep {
				t.Fatalf("step = %v, want %v", got.projectForm.step, tt.wantStep)
			}
			switch tt.step {
			case ProjectStepDomain:
				if got.projectForm.domain != tt.wantText {
					t.Fatalf("domain = %q, want %q", got.projectForm.domain, tt.wantText)
				}
			case ProjectStepDBName:
				if got.projectForm.dbName != tt.wantText {
					t.Fatalf("dbName = %q, want %q", got.projectForm.dbName, tt.wantText)
				}
			case ProjectStepStack:
				if got.projectForm.stack != tt.wantText {
					t.Fatalf("stack = %q, want %q", got.projectForm.stack, tt.wantText)
				}
			}
		})
	}
}

type noopRunner struct{}

func (noopRunner) Preflight(context.Context, config.Profile) (*providers.ProviderStatus, error) {
	return &providers.ProviderStatus{SSHConnected: true, CLIInstalled: true}, nil
}

func (noopRunner) CheckDomainAvailable(context.Context, config.Profile, string) error { return nil }

func (noopRunner) Execute(context.Context, config.Profile, wizard.ProvisionPlan, *wizard.Stack) (*wizard.ProvisionReport, error) {
	return &wizard.ProvisionReport{}, nil
}

func (noopRunner) Rollback(context.Context, config.Profile, *wizard.Stack) ([]wizard.CleanupResult, error) {
	return nil, nil
}

func (noopRunner) RestartApp(context.Context, config.Profile, string) error { return nil }

func (noopRunner) RenewSSL(context.Context, config.Profile, string) error { return nil }

func (noopRunner) TailLog(context.Context, config.Profile, string, int) ([]byte, error) {
	return nil, nil
}

func (noopRunner) ListProviderSubdomains(context.Context, config.Profile) ([]providers.Subdomain, error) {
	return nil, nil
}
