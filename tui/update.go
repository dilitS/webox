package tui

import (
	"errors"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dilitS/webox/providers"
	"github.com/dilitS/webox/wizard"
)

// Update is pure state transition logic. I/O is represented only as tea.Cmd.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.alert = ""

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case ConfigLoadedMsg:
		if msg.Missing {
			m.cfg = msg.Config
			m.state = StateInitWizard
			m.initForm = newInitWizardForm()
			return m, nil
		}
		m.cfg = msg.Config
		m.state = StateDashboard
		m.selectedIndex = clampIndex(m.selectedIndex, len(cfgProjects(m.cfg)))
		return m, tea.Batch(refreshVisibleProjectsCmd(m), scheduleRefresh(m.refreshInterval))
	case ConfigLoadFailedMsg:
		m.state = StateInitWizard
		m.initForm = newInitWizardForm()
		m.alert = fmt.Sprintf("config load failed: %v", msg.Err)
		return m, nil
	case ProfileSavedMsg:
		m.cfg = msg.Config
		m.state = StateDashboard
		m.initForm = initWizardForm{step: InitStepDone}
		m.selectedIndex = clampIndex(m.selectedIndex, len(cfgProjects(m.cfg)))
		m.alert = "profile saved"
		return m, tea.Batch(refreshVisibleProjectsCmd(m), scheduleRefresh(m.refreshInterval))
	case ProfileSaveFailedMsg:
		m.initForm.saving = false
		m.initForm.err = fmt.Sprintf("save failed: %v", msg.Err)
		m.initForm.step = InitStepReview
		return m, nil
	case ProjectWizardPreflightMsg:
		return m.applyPreflight(msg)
	case ProjectWizardDomainCheckedMsg:
		return m.applyDomainCheck(msg), nil
	case ProjectWizardExecutedMsg:
		return m.applyExecution(msg)
	case ProjectWizardRolledBackMsg:
		return m.applyRollback(msg), nil
	case StatusRefreshedMsg:
		if m.statuses == nil {
			m.statuses = make(map[string]ProjectStatus)
		}
		for _, refreshed := range msg.Statuses {
			m.statuses[refreshed.ProjectID] = refreshed
		}
		return m, nil
	case StatusRefreshFailedMsg:
		m.alert = "status refresh failed; showing cached data"
		return m, scheduleRefresh(m.refreshInterval)
	case RefreshTickMsg:
		if m.state == StateDashboard || m.state == StateProjectDetail {
			return m, tea.Batch(refreshVisibleProjectsCmd(m), scheduleRefresh(m.refreshInterval))
		}
		return m, scheduleRefresh(m.refreshInterval)
	case tea.KeyMsg:
		return m.updateKey(msg)
	default:
		return m, nil
	}
}

func (m Model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.cancel()
		return m, tea.Quit
	case "?":
		m.helpVisible = !m.helpVisible
		return m, nil
	}

	switch m.state {
	case StateInitWizard:
		return m.updateInitWizardKey(msg)
	case StateDashboard:
		return m.updateDashboardKey(msg)
	case StateProjectDetail:
		return m.updateProjectDetailKey(msg)
	case StateProjectWizard:
		return m.updateProjectWizardKey(msg)
	default:
		return m, nil
	}
}

func (m Model) updateInitWizardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.initForm.saving {
		return m, nil
	}
	switch msg.String() {
	case "esc":
		if m.initForm.step == InitStepWelcome {
			m.cancel()
			return m, tea.Quit
		}
		m.initForm = m.initForm.stepBack()
		return m, nil
	case "shift+tab":
		m.initForm = m.initForm.stepBack()
		return m, nil
	case "tab", "enter":
		if m.initForm.step == InitStepReview {
			m.initForm.saving = true
			profile := m.initForm.toProfile()
			return m, saveProfileCmd(m.ctx, m.configPath, profile, m.cfg)
		}
		m.initForm = m.initForm.validateAndAdvance()
		return m, nil
	case "backspace", "ctrl+h":
		m.initForm = m.initForm.backspaceInput()
		return m, nil
	default:
		if msg.Type == tea.KeyRunes {
			m.initForm = m.initForm.appendInput(string(msg.Runes))
		}
		return m, nil
	}
}

func (m Model) updateDashboardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	projectCount := len(cfgProjects(m.cfg))
	switch msg.String() {
	case "up", "k":
		m.selectedIndex = clampIndex(m.selectedIndex-1, projectCount)
	case "down", "j":
		m.selectedIndex = clampIndex(m.selectedIndex+1, projectCount)
	case "right", "tab":
		if projectCount > 0 {
			m.state = StateProjectDetail
			m.activeTab = TabOverview
		}
	case "enter":
		if projectCount > 0 {
			m.state = StateProjectDetail
			m.activeTab = TabOverview
		}
	case "n":
		if m.cfg == nil || len(m.cfg.Profiles) == 0 {
			m.alert = "create a profile first (init wizard)"
			return m, nil
		}
		m.projectForm = newProjectWizardForm(m.cfg)
		m.state = StateProjectWizard
		m.wizardStack = newStackSlot()
	}
	return m, nil
}

func (m Model) updateProjectDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "left", "esc":
		m.state = StateDashboard
	case "1":
		m.activeTab = TabOverview
	case "2", "3", "4", "h", "l":
		m.alert = "tab available in v0.2"
	case "r", "s", "v":
		m.alert = "action available in Sprint 06+"
	}
	return m, nil
}

func (m Model) updateProjectWizardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	form := m.projectForm
	key := msg.String()

	switch key {
	case "esc":
		switch form.step {
		case ProjectStepExecuting, ProjectStepRollingBack:
			return m, nil
		case ProjectStepFailure:
			form.step = ProjectStepDone
			m.projectForm = form
			m.state = StateDashboard
			m.alert = "wizard exited; resources kept on panel"
			return m, nil
		default:
			m.state = StateDashboard
			m.alert = "wizard cancelled"
			return m, nil
		}
	case "shift+tab":
		form = form.stepBack()
		m.projectForm = form
		return m, nil
	case "backspace", "ctrl+h":
		form = form.backspaceInput()
		m.projectForm = form
		return m, nil
	case "tab", "enter":
		updated, needsAsync := form.validateAndAdvance(m.cfg)
		m.projectForm = updated
		if !needsAsync {
			return m, nil
		}
		return m.dispatchProjectWizardAsync()
	}

	if isPickerStep(form.step) {
		switch key {
		case "up", "k", "left":
			form = form.cycleSelection(m.cfg, -1)
			m.projectForm = form
			return m, nil
		case "down", "j", "right":
			form = form.cycleSelection(m.cfg, 1)
			m.projectForm = form
			return m, nil
		}
		if form.step == ProjectStepDBChoice {
			switch key {
			case "y", "Y":
				form.dbWanted = true
				m.projectForm = form
				return m, nil
			case "n", "N":
				form.dbWanted = false
				m.projectForm = form
				return m, nil
			}
		}
	}

	if form.step == ProjectStepFailure {
		switch key {
		case "y", "Y":
			profile, _ := ProfileByAlias(m.cfg, form.profileAlias)
			form.step = ProjectStepRollingBack
			m.projectForm = form
			return m, wizardRollbackCmd(m.ctx, m.wizardRunner, profile, m.wizardStack, m.pendingPath)
		case "n", "N":
			form.step = ProjectStepDone
			m.projectForm = form
			m.state = StateDashboard
			m.alert = "wizard exited; resources kept on panel"
			return m, nil
		}
	}

	if msg.Type == tea.KeyRunes && isInputStep(form.step) {
		form = form.appendInput(string(msg.Runes))
		m.projectForm = form
	}
	return m, nil
}

func isPickerStep(step ProjectWizardStep) bool {
	switch step {
	case ProjectStepProfile, ProjectStepStack, ProjectStepDBChoice, ProjectStepDBKind:
		return true
	}
	return false
}

func isInputStep(step ProjectWizardStep) bool {
	switch step {
	case ProjectStepDomain, ProjectStepDBName:
		return true
	}
	return false
}

func (m Model) dispatchProjectWizardAsync() (tea.Model, tea.Cmd) {
	profile, ok := ProfileByAlias(m.cfg, m.projectForm.profileAlias)
	if !ok {
		m.projectForm.err = "profile not found"
		return m, nil
	}
	switch m.projectForm.step {
	case ProjectStepDomain:
		if !m.projectForm.preflightDone {
			return m, wizardPreflightCmd(m.ctx, m.wizardRunner, profile, m.projectForm.profileAlias)
		}
		return m, wizardDomainCheckCmd(m.ctx, m.wizardRunner, profile, m.projectForm.domain)
	case ProjectStepExecuting:
		return m, wizardExecuteCmd(m.ctx, m.wizardRunner, profile, m.projectForm.plan(), m.wizardStack, m.pendingPath, m.cfg, m.configPath)
	}
	return m, nil
}

func (m Model) applyPreflight(msg ProjectWizardPreflightMsg) (Model, tea.Cmd) {
	if msg.Err != nil {
		m.projectForm.err = fmt.Sprintf("preflight failed: %v", msg.Err)
		return m, nil
	}
	m.projectForm.preflightDone = true
	profile, _ := ProfileByAlias(m.cfg, msg.ProfileAlias)
	return m, wizardDomainCheckCmd(m.ctx, m.wizardRunner, profile, m.projectForm.domain)
}

func (m Model) applyDomainCheck(msg ProjectWizardDomainCheckedMsg) Model {
	if msg.Err != nil {
		if errors.Is(msg.Err, providers.ErrSubdomainExists) {
			m.projectForm.err = "domain is already provisioned on the panel; pick another"
		} else {
			m.projectForm.err = fmt.Sprintf("domain check failed: %v", msg.Err)
		}
		return m
	}
	if !msg.Available {
		m.projectForm.err = "domain is not available"
		return m
	}
	m.projectForm.step = ProjectStepReview
	m.projectForm.err = ""
	return m
}

func (m Model) applyExecution(msg ProjectWizardExecutedMsg) (Model, tea.Cmd) {
	m.projectForm.report = msg.Report
	m.projectForm.execErr = msg.Err
	if msg.Err != nil {
		m.projectForm.executing = false
		m.projectForm.step = ProjectStepFailure
		var execErr *wizard.ExecutionFailedError
		if errors.As(msg.Err, &execErr) {
			m.projectForm.err = execErr.Error()
		} else {
			m.projectForm.err = msg.Err.Error()
		}
		return m, nil
	}
	if msg.SaveErr != nil {
		m.projectForm.err = fmt.Sprintf("project saved partially: %v", msg.SaveErr)
		m.projectForm.step = ProjectStepFailure
		return m, nil
	}
	if msg.ProjectCfg != nil {
		m.cfg = msg.ProjectCfg
	}
	if msg.ProjectID != "" {
		for idx, project := range cfgProjects(m.cfg) {
			if project.ID == msg.ProjectID {
				m.selectedIndex = idx
				break
			}
		}
	}
	if m.cache != nil && msg.Plan.Domain != "" {
		m.cache.Invalidate("http:" + msg.Plan.Domain)
		m.cache.Invalidate("ssl:" + msg.Plan.Domain)
		m.cache.Invalidate("ssh:node:" + msg.Plan.Domain)
	}
	m.projectForm.executing = false
	m.projectForm.step = ProjectStepDone
	m.state = StateDashboard
	m.alert = "project created"
	return m, tea.Batch(refreshVisibleProjectsCmd(m), scheduleRefresh(m.refreshInterval))
}

func (m Model) applyRollback(msg ProjectWizardRolledBackMsg) Model {
	m.projectForm.rolledBack = true
	m.projectForm.rollbackResults = msg.Results
	m.projectForm.rollbackErr = msg.Err
	m.projectForm.step = ProjectStepDone
	m.state = StateDashboard
	if msg.Err != nil {
		m.alert = fmt.Sprintf("rollback finished with errors: %v", msg.Err)
	} else {
		m.alert = "rollback complete"
	}
	return m
}
