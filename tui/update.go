package tui

import (
	"errors"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dilitS/webox/providers"
	ghsvc "github.com/dilitS/webox/services/github"
	"github.com/dilitS/webox/status"
	"github.com/dilitS/webox/tui/bento"
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
		m.emit("status.refresh_failed", map[string]any{
			"err_class": classifyErrForTrace(msg.Err),
		})
		if m.tryRaiseHostKeyModal(msg.Err) {
			m.emit("modal.hostkey_open", map[string]any{"kind": m.hostKeyModal.Kind})
			return m, scheduleRefresh(m.refreshInterval)
		}
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

// updateMouse handles mouse events surfaced by Bubble Tea's
// `tea.WithMouseCellMotion` opt-in. The cockpit understands two
// button families:
//
//   - **Wheel up / wheel down** — scrolls the focused tile (when one
//     is Tab-focused) or the global body viewport (otherwise),
//     mirroring `PgUp`/`PgDn` / `Home`/`End`. Step size is
//     [mouseWheelStep] lines.
//   - **Left button press** — layout-aware hit testing routes the
//     click into one of three behaviours:
//   - clicking the row Y of the Projects tile sets selectedIndex
//     to that row AND drills into project detail.
//   - clicking on a non-scrollable tile (Overview, Topology,
//     Metrics) drills into project detail (mirrors Enter).
//   - clicking on a scrollable tile (Logs, CI/CD) focuses that
//     tile so wheel events scroll its body instead of the
//     viewport.
//   - clicking the project detail body returns to the dashboard.
//   - clicking on the status bar or any region outside a slot
//     is a no-op (chrome should be informational only).
//   - while a tile is already Tab-focused, left-click is a no-op
//     so the operator does not lose scroll context — Esc is the
//     explicit release path.
//
// Bubble Tea v1.3+ split `MouseMsg.Type` into the orthogonal
// `MouseAction` (press/release/motion) + `MouseButton`
// (left/middle/wheel-up/wheel-down/…) pair. We only react to *press*
// actions so a long mouse drag does not fire repeated jumps.
func (m Model) updateMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Action != tea.MouseActionPress {
		return m, nil
	}
	if m.focusedTile != nil {
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.scrollFocusedTileBy(-mouseWheelStep)
		case tea.MouseButtonWheelDown:
			m.scrollFocusedTileBy(mouseWheelStep)
		}
		return m, nil
	}
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		m.scrollViewportBy(-mouseWheelStep)
		return m, nil
	case tea.MouseButtonWheelDown:
		m.scrollViewportBy(mouseWheelStep)
		return m, nil
	case tea.MouseButtonLeft:
		return m.handleLeftClick(msg.X, msg.Y), nil
	}
	return m, nil
}

// handleLeftClick implements the layout-aware click router
// documented on [updateMouse]. It is a pure model transform; all
// I/O stays in commands.
//
// The dashboard branch resolves the click coordinate against
// [Model.dashboardLayout] so the slot under the cursor decides
// whether the cockpit drills, focuses, or stays put. Non-dashboard
// states use coarse semantics: project detail clicks return to
// the dashboard; wizards and modals ignore clicks because their
// keyboard-driven flow would be derailed by stray pointer events.
func (m Model) handleLeftClick(x, y int) Model {
	switch m.state {
	case StateDashboard:
		if m.cicdModal.Open || m.hostKeyModal.Open {
			return m
		}
		layout := m.dashboardLayout()
		slot, ok := layout.SlotAt(x, y)
		if !ok {
			return m
		}
		return m.routeDashboardClick(slot, x, y, layout)
	case StateProjectDetail:
		if m.activeTab == TabLogs {
			return m
		}
		m.state = StateDashboard
		return m
	}
	return m
}

// routeDashboardClick decides what to do once the layout map has
// resolved a click into a [bento.Slot]. Split out from
// [handleLeftClick] so the router stays narratable: each slot has
// its own one-line case.
func (m Model) routeDashboardClick(slot bento.Slot, _, y int, layout bento.LayoutMap) Model {
	switch slot {
	case bento.SlotProjects:
		projects := cfgProjects(m.cfg)
		if len(projects) == 0 {
			return m
		}
		// Inside the Projects tile: skip the top border + tile
		// header (2 chrome rows) to get the body offset, then
		// each subsequent row maps to one project. The clamp
		// prevents an out-of-range index when the click lands on
		// the bottom border or the empty padding area.
		const chromeRows = 2
		rowOffset := y - layout.Slots[slot].Y - chromeRows
		if rowOffset < 0 {
			rowOffset = 0
		}
		if rowOffset >= len(projects) {
			rowOffset = len(projects) - 1
		}
		m.selectedIndex = rowOffset
		m.state = StateProjectDetail
		m.activeTab = TabOverview
		return m
	case bento.SlotLogs, bento.SlotCICD:
		// Scrollable tile → focus it directly. Tab cycle still
		// works; this is the click shortcut that lets the
		// operator pick a panel without remembering its position
		// in the rotation.
		if !m.slotIsScrollableInDashboard(slot) {
			// Defensive guard: in case a future engine change
			// reuses these slot ids for non-scrollable tiles,
			// fall back to the drill behaviour rather than
			// silently breaking.
			return m.drillIntoSelectedProject()
		}
		focus := slot
		m.focusedTile = &focus
		return m
	case bento.SlotOverview, bento.SlotTopology, bento.SlotMetrics:
		return m.drillIntoSelectedProject()
	}
	return m
}

// drillIntoSelectedProject is the shared "click drilled, no row
// selection" path. The current selectedIndex is preserved.
func (m Model) drillIntoSelectedProject() Model {
	if len(cfgProjects(m.cfg)) == 0 {
		return m
	}
	m.state = StateProjectDetail
	m.activeTab = TabOverview
	return m
}

// slotIsScrollableInDashboard reports whether the dashboard frame
// would expose `slot` as Tab-focusable (i.e. the underlying tile
// implements [bento.ScrollableTile]). Used by
// [routeDashboardClick] so a click on a scrollable tile lands
// directly on focus regardless of the current Tab cycle position.
func (m Model) slotIsScrollableInDashboard(slot bento.Slot) bool {
	for _, tile := range m.dashboardBentoTiles() {
		if tile.Slot() != slot {
			continue
		}
		_, ok := tile.(bento.ScrollableTile)
		return ok
	}
	return false
}

// dashboardLayout returns the [bento.LayoutMap] for the current
// viewport. Computed deterministically from `m.width`, `m.height`,
// and the active [bento.Mode] so the result tracks whatever
// [Engine.RenderMode] would actually paint. Centralised here so
// both the click router and any future test helper can ask for
// the same answer without reaching into the engine themselves.
func (m Model) dashboardLayout() bento.LayoutMap {
	return bento.NewEngine("Webox Cockpit v0.1", nil).
		ComputeLayout(m.width, m.height, m.BentoMode())
}

func (m Model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Host-key modal is a strict-block overlay: while it is open we
	// only accept `Esc` (close) and `q`/`Ctrl+C` (quit). Any other
	// key would silently route to the underlying state — confusing
	// at best, security-relevant at worst (e.g. accidentally
	// triggering a wizard step while a MITM warning is on screen).
	if m.hostKeyModal.Open {
		switch msg.String() {
		case "q", "ctrl+c":
			m.cancel()
			return m, tea.Quit
		case "esc", "enter":
			// Inline mutation rather than calling a pointer-
			// receiver helper: Bubble Tea's MVU contract returns
			// a value copy; we want the dismissal visible in
			// the returned Model regardless of how Go handles
			// the implicit address-of in the method call. Keeps
			// the data-flow obvious to readers.
			m.hostKeyModal = hostKeyModalForm{}
			return m, nil
		default:
			return m, nil
		}
	}

	switch msg.String() {
	case "q", "ctrl+c":
		m.cancel()
		return m, tea.Quit
	case "?":
		m.helpVisible = !m.helpVisible
		m.viewportOffsetY = 0
		return m, nil
	}

	// Sprint 14 TASK-14.2 — focus rotation MUST take precedence over
	// the legacy `Tab → Project Detail` routing on the dashboard so
	// the operator can cycle scrollable tiles. The router consumes
	// Tab / Shift+Tab when the dashboard is the active state and
	// hands the keystroke off to `cycleFocusedTile`. Other states
	// (wizards, detail, etc.) keep their pre-existing Tab semantics.
	if m.state == StateDashboard {
		switch msg.String() {
		case "tab":
			m.cycleFocusedTile(+1)
			return m, nil
		case "shift+tab":
			m.cycleFocusedTile(-1)
			return m, nil
		case "esc":
			if m.focusedTile != nil {
				m.focusedTile = nil
				return m, nil
			}
		}
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

// tileScrollStep is the number of lines a single PgUp / PgDn moves a
// focused tile. Smaller than the viewport page step because tiles
// are shorter than the full body — flicking a wheel feels jerky if
// the offset jumps half the buffer at once.
const tileScrollStep = 5

func (m *Model) handleViewportKey(msg tea.KeyMsg) bool {
	// Sprint 14 TASK-14.2 — when a tile is focused the scroll keys
	// move that tile's offset instead of the global body viewport.
	// Non-scrollable tiles never appear in `focusedTile` (the
	// rotation skips them) so we can treat focus as a strict signal.
	if m.focusedTile != nil {
		switch msg.String() {
		case "pgup":
			m.scrollFocusedTileBy(-tileScrollStep)
			return true
		case "pgdown":
			m.scrollFocusedTileBy(tileScrollStep)
			return true
		case "home":
			m.tileScrollOffsets[*m.focusedTile] = 0
			return true
		case "end":
			m.scrollFocusedTileBy(maxTileScrollOffset)
			return true
		}
	}
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

// maxTileScrollOffset is a sentinel "End" value passed to
// scrollFocusedTileBy. The renderer clamps to the actual maximum
// (live-log buffer length minus one row) on the next frame, so we
// can be generous here without overshooting.
const maxTileScrollOffset = 1 << 16

// cycleFocusedTile advances the focus across scrollable tiles.
// `direction` is +1 (Tab) or -1 (Shift+Tab). The cycle includes a
// "no-focus" slot so the operator can return to global viewport
// scrolling without leaving the dashboard.
//
// Sequence: nil → Logs → CICD → nil → Logs … (when both are
// scrollable). Non-scrollable slots are silently skipped, satisfying
// the AC's "fokus na nie-scrollowalnym kafelku" guard: the rotation
// never lands on a tile that cannot consume PgUp / PgDn.
func (m *Model) cycleFocusedTile(direction int) {
	cycle := m.scrollableSlotCycle()
	if len(cycle) == 0 {
		return
	}
	// Position 0 in cycle == "no focus" sentinel. Translating
	// `m.focusedTile` into a cycle index keeps the rotation logic a
	// single integer addition modulo len(cycle).
	current := 0
	if m.focusedTile != nil {
		for i, slot := range cycle {
			if i == 0 {
				continue
			}
			if slot == *m.focusedTile {
				current = i
				break
			}
		}
	}
	next := (current + direction + len(cycle)) % len(cycle)
	if next == 0 {
		m.focusedTile = nil
		return
	}
	slot := cycle[next]
	m.focusedTile = &slot
}

// scrollableSlotCycle returns the rotation order: a "no focus"
// sentinel followed by every scrollable slot present in the current
// frame's tiles. Computed per call so a future tile registry change
// does not require model surgery.
//
// The sentinel `bento.SlotProjects` value at index 0 is a placeholder
// — index 0 always means "no focus", regardless of the slot value.
func (m *Model) scrollableSlotCycle() []bento.Slot {
	tiles := m.dashboardBentoTiles()
	cycle := make([]bento.Slot, 0, len(tiles)+1)
	// Sentinel: index 0 == no focus. Slot value is a don't-care.
	cycle = append(cycle, bento.SlotProjects)
	for _, tile := range tiles {
		if _, ok := tile.(bento.ScrollableTile); ok {
			cycle = append(cycle, tile.Slot())
		}
	}
	if len(cycle) == 1 {
		// No scrollable tiles in this frame — disable the cycle so
		// Tab / Shift+Tab become a no-op rather than infinitely
		// landing on the sentinel.
		return nil
	}
	return cycle
}

// scrollFocusedTileBy advances the offset for the currently focused
// tile by `delta`, clamping to its scrollable range. The clamp uses
// the freshly built tile so the bound always reflects the latest
// snapshot (e.g. the live-log buffer growing between frames).
func (m *Model) scrollFocusedTileBy(delta int) {
	if m.focusedTile == nil {
		return
	}
	if m.tileScrollOffsets == nil {
		m.tileScrollOffsets = make(map[bento.Slot]int)
	}
	target := *m.focusedTile
	current := m.tileScrollOffsets[target]
	candidate := current + delta
	if candidate < 0 {
		candidate = 0
	}
	for _, tile := range m.dashboardBentoTiles() {
		if tile.Slot() != target {
			continue
		}
		s, ok := tile.(bento.ScrollableTile)
		if !ok {
			return
		}
		clamped := s.Scroll(candidate - s.ScrollOffset()).ScrollOffset()
		m.tileScrollOffsets[target] = clamped
		return
	}
	m.tileScrollOffsets[target] = candidate
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
	case "right":
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
	case "left", "esc", "tab":
		// Sprint 20 — Tab joins Esc/Left as a back-nav alias so
		// the operator's muscle-memory "move between panes"
		// gesture lands them on the dashboard instead of a silent
		// no-op.
		m.state = StateDashboard
		return m, nil
	case "1":
		m.activeTab = TabOverview
		return m, nil
	case "2":
		// Sprint 20 TASK-20.4 — Env Diff is now a read-only
		// view of `project.SecretsMeta`. No provider I/O.
		m.activeTab = TabEnvDiff
		return m, nil
	case "3":
		// Sprint 20 TASK-20.4 — Database is now a stack-aware
		// connection cheatsheet. Like Env Diff, it consumes
		// only cached data; live DB queries belong to
		// `webox doctor db creds` (Sprint 21+).
		m.activeTab = TabDatabase
		return m, nil
	case "4":
		return m.enterLiveLogsTab()
	case "h", "l":
		// Vim-style horizontal nav lives in the wizard router,
		// not on the project detail surface. Silently ignore
		// here so muscle memory does not surface stale alerts.
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
