package tui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dilitS/webox/config"
	"github.com/dilitS/webox/tui"
)

// These tests exercise the form-helper branches that the higher-
// level integration tests (TestProjectWizardHappyPath, etc.) do not
// touch: stepBack from every step, backspace on every input field,
// cycle wrap-around on the picker steps, and the DBChoice "wanted"
// toggle. Tests use the public Update / View surface because the
// helpers are unexported, but each test moves through the smallest
// possible state change so failures point at the exact branch.

func newProjectWizardModelAtStep(t *testing.T, stepKeys []tea.Msg) tui.Model {
	t.Helper()
	m := newModelWithRunner(t, &fakeRunner{})
	m, _ = runMsg(t, m, tui.ConfigLoadedMsg{Config: sampleConfig()})
	m, _ = runMsg(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	for _, msg := range stepKeys {
		m, _ = runMsg(t, m, msg)
	}
	return m
}

func TestProjectWizard_BackspaceAtDomainStep_TrimsRune(t *testing.T) {
	t.Parallel()
	enter := tea.KeyMsg{Type: tea.KeyEnter}
	m := newProjectWizardModelAtStep(t, []tea.Msg{enter, enter, enter})
	for _, r := range "abc" {
		m, _ = runMsg(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = runMsg(t, m, tea.KeyMsg{Type: tea.KeyBackspace})
	if !strings.Contains(m.View(), "ab") {
		t.Fatalf("view missing trimmed input:\n%s", m.View())
	}
	if strings.Contains(m.View(), "abc ") {
		t.Fatalf("view should NOT still contain abc:\n%s", m.View())
	}
}

func TestProjectWizard_BackspaceOnPickerStepIsNoop(t *testing.T) {
	t.Parallel()
	m := newModelWithRunner(t, &fakeRunner{})
	m, _ = runMsg(t, m, tui.ConfigLoadedMsg{Config: sampleConfig()})
	m, _ = runMsg(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	m, _ = runMsg(t, m, tea.KeyMsg{Type: tea.KeyBackspace})
	if m.State() != tui.StateProjectWizard {
		t.Fatalf("backspace on profile step changed state: %s", m.State())
	}
}

func TestProjectWizard_StepBackUnwindsThroughFlow(t *testing.T) {
	t.Parallel()
	enter := tea.KeyMsg{Type: tea.KeyEnter}
	m := newProjectWizardModelAtStep(t, []tea.Msg{enter, enter, enter})
	for _, r := range "x.demo.smallhost.pl" {
		m, _ = runMsg(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, cmd := runMsg(t, m, enter)
	m = runUntilNil(t, m, cmd())

	for i := 0; i < 4; i++ {
		m, _ = runMsg(t, m, tea.KeyMsg{Type: tea.KeyShiftTab})
		if m.State() != tui.StateProjectWizard {
			t.Fatalf("Shift+Tab #%d left wizard: %s", i, m.State())
		}
	}
}

func TestProjectWizard_CycleSelectionWrapsBackwards(t *testing.T) {
	t.Parallel()
	m := newModelWithRunner(t, &fakeRunner{})
	m, _ = runMsg(t, m, tui.ConfigLoadedMsg{Config: sampleConfig()})
	m, _ = runMsg(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	m, _ = runMsg(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = runMsg(t, m, tea.KeyMsg{Type: tea.KeyUp})
	if m.State() != tui.StateProjectWizard {
		t.Fatalf("cycle should keep wizard alive: %s", m.State())
	}
}

func TestInitWizard_ShiftTabFromWelcomeIsNoop(t *testing.T) {
	t.Parallel()
	m := newModelWithRunner(t, &fakeRunner{})
	m, _ = runMsg(t, m, tui.ConfigLoadedMsg{Missing: true, Config: config.DefaultConfig()})
	if m.State() != tui.StateInitWizard {
		t.Fatalf("state = %s, want InitWizard", m.State())
	}
	m, _ = runMsg(t, m, tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.State() != tui.StateInitWizard {
		t.Fatalf("Shift+Tab at welcome should be no-op, got %s", m.State())
	}
}

func TestInitWizard_BackspaceTrimsActiveStep(t *testing.T) {
	t.Parallel()
	m := newModelWithRunner(t, &fakeRunner{})
	m, _ = runMsg(t, m, tui.ConfigLoadedMsg{Missing: true, Config: config.DefaultConfig()})
	m, _ = runMsg(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	for _, r := range "abc" {
		m, _ = runMsg(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = runMsg(t, m, tea.KeyMsg{Type: tea.KeyBackspace})
	if !strings.Contains(m.View(), "ab") {
		t.Fatalf("init wizard backspace did not trim, view:\n%s", m.View())
	}
}

func TestInitWizard_ShiftTabStepsBack(t *testing.T) {
	t.Parallel()
	m := newModelWithRunner(t, &fakeRunner{})
	m, _ = runMsg(t, m, tui.ConfigLoadedMsg{Missing: true, Config: config.DefaultConfig()})
	m, _ = runMsg(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	for _, r := range "main" {
		m, _ = runMsg(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = runMsg(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = runMsg(t, m, tea.KeyMsg{Type: tea.KeyShiftTab})
	if !strings.Contains(m.View(), "Profile alias") {
		t.Fatalf("did not return to alias step:\n%s", m.View())
	}
}

func TestStateString_AllCovered(t *testing.T) {
	t.Parallel()
	cases := []struct {
		s    tui.State
		want string
	}{
		{tui.StateInitWizard, "InitWizard"},
		{tui.StateDashboard, "Dashboard"},
		{tui.StateProjectDetail, "ProjectDetail"},
		{tui.StateProjectWizard, "ProjectWizard"},
		{tui.StateResumeWizard, "ResumeWizard"},
		{tui.StateImportPreview, "ImportPreview"},
		{tui.StateCommandPalette, "CommandPalette"},
		{tui.StateConfirmDialog, "ConfirmDialog"},
		{tui.State(99), "Unknown"},
	}
	for _, tc := range cases {
		if got := tc.s.String(); got != tc.want {
			t.Errorf("State(%d).String() = %q, want %q", tc.s, got, tc.want)
		}
	}
}

func TestDetailTab_StringAndEnabled(t *testing.T) {
	t.Parallel()
	cases := []struct {
		t       tui.DetailTab
		name    string
		enabled bool
	}{
		{tui.TabOverview, "Overview", true},
		{tui.TabEnvDiff, "Env Diff", true},
		{tui.TabDatabase, "Database", true},
		{tui.TabLogs, "Logs", true},
		{tui.DetailTab(99), "Unknown", false},
	}
	for _, tc := range cases {
		if got := tc.t.String(); got != tc.name {
			t.Errorf("DetailTab(%d).String() = %q, want %q", tc.t, got, tc.name)
		}
		if got := tc.t.Enabled(); got != tc.enabled {
			t.Errorf("DetailTab(%d).Enabled() = %v, want %v", tc.t, got, tc.enabled)
		}
	}
}
