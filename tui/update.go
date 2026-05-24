package tui

import (
	"errors"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dilitS/webox/providers"
	ghsvc "github.com/dilitS/webox/services/github"
	"github.com/dilitS/webox/status"
	"github.com/dilitS/webox/tui/components"
	"github.com/dilitS/webox/wizard"
)

// Update is pure state transition logic. I/O is represented only as tea.Cmd.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) { //nolint:gocyclo // Top-level MVU router stays flat so every message route is visible in one place.
	if _, isKey := msg.(tea.KeyMsg); isKey {
		m.alert = ""
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		previous := m.BentoMode()
		m.width = msg.Width
		m.height = msg.Height
		const chromeLines = 2
		body := m.renderRootBody(m.screen())
		available := m.height - chromeLines
		if available < 1 {
			available = 1
		}
		m.viewportOffsetY = clampViewportOffset(m.viewportOffsetY, available, len(viewportLines(body)))
		if next := m.BentoMode(); next != previous {
			m.spinner.Spinner = components.SpinnerStyle(next.String())
		}
		return m, nil
	case ConfigLoadedMsg:
		if m.resumeForm.snapshot != nil || m.resumeForm.loadErr != nil {
			m.cfg = msg.Config
			m.state = StateResumeWizard
			return m, nil
		}
		if msg.Missing {
			m.cfg = msg.Config
			m.state = StateInitWizard
			m.initForm = newInitWizardForm()
			return m, nil
		}
		m.cfg = msg.Config
		m.state = StateDashboard
		m.selectedIndex = clampIndex(m.selectedIndex, len(cfgProjects(m.cfg)))
		cmds := []tea.Cmd{refreshVisibleProjectsCmd(m), scheduleRefresh(m.refreshInterval)}
		if m.cicdFetcher != nil {
			cmds = append(cmds, scheduleCICDTick(status.GitHubStepsTTL))
			if poll := pollCICDPipelineCmd(m); poll != nil {
				cmds = append(cmds, poll)
			}
		}
		return m, tea.Batch(cmds...)
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
	case PendingLoadedMsg:
		return m.applyPendingLoaded(msg), nil
	case PendingDiscardedMsg:
		return m.applyPendingDiscarded(msg), nil
	case ProjectActionCompletedMsg:
		return m.applyProjectAction(msg)
	case ImportScanCompletedMsg:
		return m.applyImportScan(msg), nil
	case ImportPersistedMsg:
		return m.applyImportPersisted(msg)
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
	case CICDTickMsg:
		return m.applyCICDTick()
	case CICDFetchedMsg:
		return m.applyCICDFetched(msg)
	case CICDLogsFetchedMsg:
		return m.applyCICDLogsFetched(msg), nil
	case tea.MouseMsg:
		return m.updateMouse(msg)
	case tea.KeyMsg:
		return m.updateKey(msg)
	default:
		return m, nil
	}
}

// mouseWheelStep is how many lines a single wheel tick moves the
// viewport. We keep it small enough to feel precise on a trackpad
// and large enough that a flick on a real wheel does not feel slow.
const mouseWheelStep = 3

// updateMouse handles the scroll-wheel cases that the Sprint-13 chrome
// contract surfaces inside the body slot. Click / drag events are
// reserved for future per-tile click-through and currently no-op.
//
// Bubble Tea v1.3+ split `MouseMsg.Type` into the orthogonal
// `MouseAction` (press/release/motion) + `MouseButton`
// (left/middle/wheel-up/wheel-down/…) pair. We only react to *press*
// actions on the two scroll-wheel buttons so a long mouse drag does
// not accidentally fire repeated viewport jumps.
func (m Model) updateMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Action != tea.MouseActionPress {
		return m, nil
	}
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		m.scrollViewportBy(-mouseWheelStep)
	case tea.MouseButtonWheelDown:
		m.scrollViewportBy(mouseWheelStep)
	}
	return m, nil
}

func (m Model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.cancel()
		return m, tea.Quit
	case "?":
		m.helpVisible = !m.helpVisible
		m.viewportOffsetY = 0
		return m, nil
	}

	if handled := m.handleViewportKey(msg); handled {
		return m, nil
	}

	m.viewportOffsetY = 0

	switch m.state {
	case StateInitWizard:
		return m.updateInitWizardKey(msg)
	case StateDashboard:
		return m.updateDashboardKey(msg)
	case StateProjectDetail:
		return m.updateProjectDetailKey(msg)
	case StateProjectWizard:
		return m.updateProjectWizardKey(msg)
	case StateResumeWizard:
		return m.updateResumeWizardKey(msg)
	case StateImportPreview:
		return m.updateImportPreviewKey(msg)
	default:
		return m, nil
	}
}

const viewportPageMargin = 4

func (m *Model) handleViewportKey(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "pgup":
		m.scrollViewportBy(-m.viewportPageStep())
		return true
	case "pgdown":
		m.scrollViewportBy(m.viewportPageStep())
		return true
	case "home":
		m.viewportOffsetY = 0
		return true
	case "end":
		m.viewportOffsetY = m.maxViewportOffset()
		return true
	default:
		return false
	}
}

func (m *Model) viewportPageStep() int {
	step := m.height - viewportPageMargin
	if step < 1 {
		return 1
	}
	return step
}

func (m *Model) scrollViewportBy(delta int) {
	m.viewportOffsetY += delta
	const chromeLines = 2
	body := m.renderRootBody(m.screen())
	available := m.height - chromeLines
	if available < 1 {
		available = 1
	}
	m.viewportOffsetY = clampViewportOffset(m.viewportOffsetY, available, len(viewportLines(body)))
}

func (m Model) updateImportPreviewKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.importForm.Loading || m.importForm.Saving {
		return m, nil
	}
	switch msg.String() {
	case "esc", "left", "q":
		m.importForm = importPreviewForm{}
		m.state = StateDashboard
		return m, nil
	case "a", "A":
		unmanaged := unmanagedRows(m.importForm.Rows)
		if len(unmanaged) == 0 {
			m.alert = "no unmanaged subdomains to import"
			return m, nil
		}
		m.importForm.Saving = true
		return m, importPersistCmd(m.ctx, m.configPath, m.cfg, unmanaged)
	}
	return m, nil
}

func (m Model) applyImportScan(msg ImportScanCompletedMsg) Model {
	if msg.Err != nil {
		m.alert = fmt.Sprintf("import scan failed: %v", msg.Err)
		m.importForm = importPreviewForm{}
		m.state = StateDashboard
		return m
	}
	m.importForm = importPreviewForm{Rows: msg.Rows}
	m.state = StateImportPreview
	return m
}

func (m Model) applyImportPersisted(msg ImportPersistedMsg) (Model, tea.Cmd) {
	m.importForm.Saving = false
	if msg.Err != nil {
		m.importForm.Err = fmt.Sprintf("import save failed: %v", msg.Err)
		m.alert = m.importForm.Err
		return m, nil
	}
	if msg.Config != nil {
		m.cfg = msg.Config
	}
	m.importForm = importPreviewForm{}
	m.state = StateDashboard
	if msg.ImportedRows == 1 {
		m.alert = "imported 1 subdomain"
	} else {
		m.alert = fmt.Sprintf("imported %d subdomains", msg.ImportedRows)
	}
	return m, tea.Batch(refreshVisibleProjectsCmd(m), scheduleRefresh(m.refreshInterval))
}

func unmanagedRows(rows []ImportRow) []ImportRow {
	out := make([]ImportRow, 0, len(rows))
	for _, row := range rows {
		if !row.Managed {
			out = append(out, row)
		}
	}
	return out
}

func (m Model) applyPendingLoaded(msg PendingLoadedMsg) Model {
	if msg.Err == nil && msg.Snapshot == nil {
		return m
	}
	m.resumeForm = resumeWizardForm{snapshot: msg.Snapshot, loadErr: msg.Err}
	m.state = StateResumeWizard
	if msg.Err != nil {
		m.resumeForm.err = fmt.Sprintf("pending cleanup cannot be loaded: %v", msg.Err)
	}
	return m
}

func (m Model) applyPendingDiscarded(msg PendingDiscardedMsg) Model {
	if msg.Err != nil {
		m.resumeForm.err = fmt.Sprintf("discard failed: %v", msg.Err)
		return m
	}
	m.resumeForm = resumeWizardForm{}
	m.state = StateDashboard
	m.alert = "pending cleanup snapshot discarded"
	return m
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
	if m.cicdModal.Open {
		return m.updateCICDModalKey(msg)
	}
	projectCount := len(cfgProjects(m.cfg))
	previousIndex := m.selectedIndex
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
	case "i":
		return m.beginImportScan()
	case "f8":
		return m.openCICDLogsModal()
	}
	if m.selectedIndex != previousIndex {
		return m.onDashboardSelectionChanged()
	}
	return m, nil
}

// onDashboardSelectionChanged is invoked whenever the operator moves
// the selection cursor to a different project. Sprint 10 §TASK-10.4
// requires the CI/CD tile cache to be invalidated so the next poll
// fetches fresh data for the newly highlighted project.
func (m Model) onDashboardSelectionChanged() (tea.Model, tea.Cmd) {
	project, ok := m.selectedProject()
	if !ok {
		return m, nil
	}
	invalidateCICDCacheForProject(m.cache, project)
	if m.cicdFetcher == nil {
		return m, nil
	}
	return m, pollCICDPipelineCmd(m)
}

// applyCICDTick triggers a poll for the active project and schedules
// the next tick. The tick fires regardless of selection so background
// data stays warm even when the operator is in a wizard.
func (m Model) applyCICDTick() (tea.Model, tea.Cmd) {
	if m.cicdFetcher == nil {
		return m, scheduleCICDTick(status.GitHubStepsTTL)
	}
	cmds := []tea.Cmd{scheduleCICDTick(status.GitHubStepsTTL)}
	if poll := pollCICDPipelineCmd(m); poll != nil {
		cmds = append(cmds, poll)
	}
	return m, tea.Batch(cmds...)
}

// applyCICDFetched merges a CI/CD poll result into the per-project
// snapshot cache. Rate-limit responses preserve any previously
// successful data (SWR semantics): the tile renders the cached steps
// with a [LIMITED] badge instead of clearing them.
func (m Model) applyCICDFetched(msg CICDFetchedMsg) (tea.Model, tea.Cmd) {
	if m.cicdSnapshots == nil {
		m.cicdSnapshots = make(map[string]cicdSnapshotEntry)
	}
	previous := m.cicdSnapshots[msg.ProjectID]

	if msg.Err != nil {
		entry := previous
		entry.Stale = true
		entry.FetchedAt = msg.FetchedAt
		switch {
		case errors.Is(msg.Err, ghsvc.ErrRateLimited):
			entry.RateLimited = true
			_, hint := extractRateLimitInfo(msg.Err)
			entry.RateLimitHint = hint
		case errors.Is(msg.Err, ghsvc.ErrRunNotFound):
			entry.Run = nil
			entry.Steps = nil
			entry.RateLimited = false
			entry.Err = ""
		default:
			entry.Err = msg.Err.Error()
		}
		m.cicdSnapshots[msg.ProjectID] = entry
		return m, nil
	}

	entry := cicdSnapshotEntry{
		Run:       summarizeRun(msg.Result.Run),
		Steps:     append([]ghsvc.Step(nil), msg.Result.Steps...),
		FetchedAt: msg.FetchedAt,
	}
	if msg.Result.RateLimitHint != "" {
		entry.RateLimitHint = msg.Result.RateLimitHint
	}
	m.cicdSnapshots[msg.ProjectID] = entry
	return m, nil
}

// applyCICDLogsFetched populates the F8 modal with the requested log
// tail. Errors stay in the modal so the operator can read them, then
// hit `esc` to dismiss.
func (m Model) applyCICDLogsFetched(msg CICDLogsFetchedMsg) Model {
	if !m.cicdModal.Open || m.cicdModal.ProjectID != msg.ProjectID {
		return m
	}
	m.cicdModal.Loading = false
	if msg.Err != nil {
		m.cicdModal.Err = msg.Err.Error()
		return m
	}
	m.cicdModal.Lines = msg.Lines
	m.cicdModal.Err = ""
	return m
}

// openCICDLogsModal initialises the F8 modal for the active project +
// run id. Returns a no-op when the project has no run yet, no GitHub
// link, or no logs fetcher configured.
func (m Model) openCICDLogsModal() (tea.Model, tea.Cmd) {
	project, ok := m.selectedProject()
	if !ok {
		m.alert = "no project selected"
		return m, nil
	}
	entry, has := m.cicdSnapshots[project.ID]
	if !has || entry.Run == nil || entry.Run.RunID <= 0 {
		m.alert = "no workflow run available yet"
		return m, nil
	}
	if m.cicdLogsFetcher == nil {
		m.alert = "logs fetcher unavailable (install gh CLI)"
		return m, nil
	}
	m.cicdModal = cicdLogsModalForm{
		Open:         true,
		ProjectID:    project.ID,
		ProjectAlias: project.Domain,
		RunID:        entry.Run.RunID,
		RunNumber:    entry.Run.RunNumber,
		RunStatus:    cicdStatusFromGitHub(entry.Run.Status, entry.Run.Conclusion),
		Loading:      true,
	}
	return m, loadCICDLogsCmd(m, entry.Run.RunID)
}

// updateCICDModalKey handles keypresses while the F8 modal is open.
// The modal is the only consumer of `↑/↓` while open so the dashboard
// router defers to it before applying dashboard navigation.
func (m Model) updateCICDModalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "f8":
		m.cicdModal = cicdLogsModalForm{}
		return m, nil
	case "up", "k":
		if m.cicdModal.ScrollOffset > 0 {
			m.cicdModal.ScrollOffset--
		}
		return m, nil
	case "down", "j":
		if m.cicdModal.ScrollOffset < len(m.cicdModal.Lines)-1 {
			m.cicdModal.ScrollOffset++
		}
		return m, nil
	}
	return m, nil
}

func (m Model) beginImportScan() (tea.Model, tea.Cmd) {
	if m.cfg == nil || len(m.cfg.Profiles) == 0 {
		m.alert = "create a profile first (init wizard)"
		return m, nil
	}
	m.importForm = importPreviewForm{Loading: true}
	m.state = StateImportPreview
	return m, importScanCmd(m.ctx, m.wizardRunner, m.cfg)
}

func (m Model) updateProjectDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.activeTab == TabLogs {
		return m.updateLiveLogsKey(msg)
	}
	switch msg.String() {
	case "left", "esc":
		m.state = StateDashboard
		return m, nil
	case "1":
		m.activeTab = TabOverview
		return m, nil
	case "4":
		return m.enterLiveLogsTab()
	case "2", "3", "h", "l":
		m.alert = "tab available in v0.2"
		return m, nil
	case "r":
		return m.dispatchProjectAction(ProjectActionRestart)
	case "s":
		return m.dispatchProjectAction(ProjectActionSSLRenew)
	case "v":
		return m.dispatchProjectAction(ProjectActionLogs)
	}
	return m, nil
}

func (m Model) dispatchProjectAction(kind ProjectActionKind) (tea.Model, tea.Cmd) {
	if m.actionForm.Running {
		m.alert = "another action is in flight"
		return m, nil
	}
	projects := cfgProjects(m.cfg)
	if len(projects) == 0 || m.selectedIndex < 0 || m.selectedIndex >= len(projects) {
		m.alert = "no project selected"
		return m, nil
	}
	project := projects[m.selectedIndex]
	profile, ok := ProfileByAlias(m.cfg, project.ProfileAlias)
	if !ok {
		m.alert = "profile for project not found"
		return m, nil
	}
	m.actionForm = projectActionForm{Kind: kind, ProjectID: project.ID, Running: true}
	return m, projectActionCmd(m.ctx, m.wizardRunner, kind, profile, project, m.cache)
}

func (m Model) applyProjectAction(msg ProjectActionCompletedMsg) (Model, tea.Cmd) {
	m.actionForm.Running = false
	m.actionForm.Kind = msg.Kind
	m.actionForm.ProjectID = msg.ProjectID
	m.actionForm.Output = msg.Output
	m.actionForm.Err = msg.Err
	switch {
	case msg.Err != nil:
		m.alert = fmt.Sprintf("%s failed: %v", msg.Kind, msg.Err)
	case msg.Kind == ProjectActionLogs:
		m.alert = "log tail captured"
	default:
		m.alert = string(msg.Kind) + " succeeded"
	}
	if msg.Err == nil && (msg.Kind == ProjectActionRestart || msg.Kind == ProjectActionSSLRenew) {
		return m, refreshVisibleProjectsCmd(m)
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

func (m Model) updateResumeWizardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	form := m.resumeForm
	if form.discarding {
		switch msg.String() {
		case "backspace", "ctrl+h":
			form.confirmInput = trimRight(form.confirmInput)
			m.resumeForm = form
			return m, nil
		case "enter":
			if form.confirmInput != form.discardPhrase() {
				form.err = "confirmation phrase does not match"
				m.resumeForm = form
				return m, nil
			}
			return m, pendingDiscardCmd(m.pendingPath)
		}
		if msg.Type == tea.KeyRunes {
			form.confirmInput += string(msg.Runes)
			m.resumeForm = form
		}
		return m, nil
	}
	switch msg.String() {
	case "r", "R":
		if form.snapshot == nil || form.loadErr != nil {
			form.err = "cannot roll back until the snapshot loads cleanly"
			m.resumeForm = form
			return m, nil
		}
		form.rollingBack = true
		form.err = ""
		m.resumeForm = form
		return m, resumeRollbackCmd(m.ctx, m.wizardRunner, m.cfg, form.snapshot, m.pendingPath)
	case "k", "K":
		m.cancel()
		return m, tea.Quit
	case "d", "D":
		form.discarding = true
		form.confirmInput = ""
		form.err = "type confirmation phrase to discard"
		m.resumeForm = form
		return m, nil
	case "enter":
		return m, nil
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
	if m.state == StateResumeWizard {
		m.resumeForm.rollingBack = false
		m.resumeForm.results = msg.Results
		if msg.Err != nil {
			m.resumeForm.err = fmt.Sprintf("rollback finished with errors: %v", msg.Err)
			return m
		}
		m.resumeForm = resumeWizardForm{}
		m.state = StateDashboard
		m.alert = "resume rollback complete"
		return m
	}
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
