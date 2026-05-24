package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/dilitS/webox/internal/version"
	"github.com/dilitS/webox/tui/bento"
	"github.com/dilitS/webox/tui/components"
	"github.com/dilitS/webox/tui/theme"
	"github.com/dilitS/webox/tui/views"
)

// View renders the current model without mutating it.
//
// Every full-screen surface is wrapped with [chromeWrap] so the
// operator sees the same status bar + footer hint strip on the init
// wizard, the dashboard, project detail, and the wizard flows.
// Modals (CI/CD F8, doctor errors) keep their existing overlay
// rendering — they paint on top of the wrapped surface.
func (m Model) View() string {
	screen := m.screen()
	switch m.state {
	case StateInitWizard:
		return m.chromeWrap(screen, "Init Wizard", views.RenderInitWizard(screen))
	case StateDashboard:
		return m.renderDashboard(screen)
	case StateProjectDetail:
		if m.activeTab == TabLogs {
			return m.chromeWrap(screen, "Live Logs", views.RenderLiveLogs(screen))
		}
		return m.chromeWrap(screen, "Project Detail", views.RenderProjectDetail(screen))
	case StateProjectWizard:
		return m.chromeWrap(screen, "Project Wizard", views.RenderProjectWizard(screen))
	case StateResumeWizard:
		return m.chromeWrap(screen, "Resume Wizard", views.RenderResumeWizard(screen))
	case StateImportPreview:
		return m.chromeWrap(screen, "Import Preview", views.RenderImportPreview(screen))
	default:
		return m.styles.Panel.Render(fmt.Sprintf("%s is not enabled", m.state))
	}
}

// chromeWrap paints the cockpit-wide chrome (status bar + footer
// hints) around any non-dashboard view. The dashboard renders its
// own status bar inside the bento engine, so we skip the wrap there.
//
// crumb is the surface name displayed in the status-bar breadcrumb
// cell (e.g. "Init Wizard", "Project Detail"). The wrap is a no-op
// when the terminal is below the Standard threshold so the cockpit
// falls back to the legacy split-pane silhouette.
func (m Model) chromeWrap(screen views.Screen, crumb, body string) string {
	if screen.Width < bentoStandardMinWidth || screen.Height < bentoStandardMinHeight {
		return body
	}
	opts := m.dashboardStatusBar(screen.Width)
	if crumb != "" {
		opts.Sections = append([]string{crumb}, opts.Sections...)
	}
	statusBar := components.RenderStatusBar(opts)
	footer := m.renderFooterHints(screen.Width)
	parts := []string{statusBar, body}
	if footer != "" {
		parts = append(parts, footer)
	}
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// bentoStandardMinWidth/Height mirror [bento.DetectMode]'s Standard
// threshold. Below it the cockpit hides the chrome and renders the
// raw view (Standard fallback owns its own header).
const (
	bentoStandardMinWidth  = 100
	bentoStandardMinHeight = 30
)

// renderFooterHints draws the global keybinding strip below every
// chrome-wrapped surface. We keep the hint set tight (≤80 cols) so
// even Standard Cockpit renders it without wrapping.
func (m Model) renderFooterHints(width int) string {
	tokens := theme.Default()
	hints := "  [q] quit · [?] help · [/] command palette · [Tab] cycle panels"
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color(tokens.TextDim)).
		Width(width)
	return style.Render(hints)
}

func (m Model) renderDashboard(screen views.Screen) string {
	mode := m.BentoMode()
	var base string
	switch mode {
	case bento.ModeStandard:
		base = views.RenderDashboard(screen)
	case bento.ModeTiny:
		base = bento.NewEngine("Webox Cockpit v0.1", nil).
			RenderMode(screen.Width, screen.Height, mode)
	default:
		statusBar := components.RenderStatusBar(m.dashboardStatusBar(screen.Width))
		base = bento.NewEngine("Webox Cockpit v0.1", m.dashboardBentoTiles()).
			WithStatusBar(statusBar).
			RenderMode(screen.Width, screen.Height, mode)
	}
	if !m.cicdModal.Open {
		return base
	}
	overlay := renderCICDLogsModal(m.cicdModal, screen.Width)
	return base + "\n" + overlay
}

// dashboardStatusBar composes the StatusBar snapshot rendered above the
// Bento grid. The snapshot pulls from the active profile + the latest
// SSH metrics cache; missing fields collapse to "—" so the bar never
// shows blank cells.
func (m Model) dashboardStatusBar(width int) components.StatusBarOptions {
	now := m.nowFn()
	tone := components.ToneSuccess
	live := "LIVE"
	stale := m.metricsAreStale()
	if stale {
		tone = components.ToneWarning
		live = "STALE"
	}
	if !m.metricsHaveAnyData() {
		tone = components.ToneInfo
		live = "PENDING"
	}

	sections := []string{now.Format("15:04:05")}
	if profile := m.activeProfileAlias(); profile != "" {
		sections = append(sections, profile)
	}
	if m.headerMetrics.UptimeLabel != "" {
		sections = append(sections, "Uptime: "+m.headerMetrics.UptimeLabel)
	}
	if m.headerMetrics.LoadLabel != "" {
		sections = append(sections, "Load: "+m.headerMetrics.LoadLabel)
	}
	if m.headerMetrics.RAMLabel != "" {
		sections = append(sections, "RAM: "+m.headerMetrics.RAMLabel)
	}
	if m.headerMetrics.RTTLabel != "" {
		sections = append(sections, "Ping: "+m.headerMetrics.RTTLabel)
	}

	return components.StatusBarOptions{
		Brand:     "WEBOX " + version.Short(),
		LiveLabel: live,
		Tone:      tone,
		Sections:  sections,
		Width:     width,
	}
}

// renderCICDLogsModal builds the F8 logs viewer. The modal uses the
// double-border component from Sprint 08 and inherits the FAILED ✗
// red border when the run conclusion was a failure (Sprint 10 plan
// §TASK-10.3 acceptance criteria).
func renderCICDLogsModal(modal cicdLogsModalForm, screenWidth int) string {
	if !modal.Open {
		return ""
	}
	tokens := theme.Default()
	tone := components.ToneInfo
	if modal.RunStatus == bento.CICDStatusFailure {
		tone = components.ToneError
	}

	header := fmt.Sprintf("Workflow Run #%d · %s", modal.RunNumber, cicdModalStatusVerb(modal.RunStatus))
	if modal.ProjectAlias != "" {
		header += " · " + modal.ProjectAlias
	}

	var body strings.Builder
	switch {
	case modal.Loading:
		body.WriteString("Fetching workflow logs (gh run view --log)…")
	case modal.Err != "":
		body.WriteString("Error: ")
		body.WriteString(modal.Err)
	case len(modal.Lines) == 0:
		body.WriteString("No log output yet.")
	default:
		const maxRows = 20
		start := modal.ScrollOffset
		if start < 0 {
			start = 0
		}
		end := start + maxRows
		if end > len(modal.Lines) {
			end = len(modal.Lines)
		}
		for i := start; i < end; i++ {
			line := modal.Lines[i]
			prefix := ""
			if line.StepName != "" {
				prefix = "[" + line.StepName + "] "
			}
			body.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color(tokens.TextBright)).
				Render(prefix + line.Text))
			body.WriteString("\n")
		}
		if len(modal.Lines) > maxRows {
			body.WriteString("\n")
			fmt.Fprintf(&body, "(showing %d–%d of %d lines · ↑/↓ scroll)", start+1, end, len(modal.Lines))
		}
	}

	const (
		modalSidePadding = 4
		modalMinWidth    = 60
	)
	minWidth := screenWidth - modalSidePadding
	if minWidth < modalMinWidth {
		minWidth = modalMinWidth
	}

	return components.RenderModal(components.ModalOptions{
		Title:    header,
		Body:     body.String(),
		Footer:   "↑/↓ scroll · Esc/F8 close",
		MinWidth: minWidth,
		Tone:     tone,
		Theme:    tokens,
	})
}

func cicdModalStatusVerb(s bento.CICDStatus) string {
	switch s {
	case bento.CICDStatusSuccess:
		return "SUCCESS ✓"
	case bento.CICDStatusFailure:
		return "FAILED ✗"
	case bento.CICDStatusInProgress:
		return "IN_PROGRESS ⏳"
	case bento.CICDStatusQueued:
		return "QUEUED …"
	case bento.CICDStatusCancelled:
		return "CANCELLED ⊗"
	case bento.CICDStatusSkipped:
		return "SKIPPED ⊘"
	case bento.CICDStatusUnknown:
		return "UNKNOWN ?"
	default:
		return "UNKNOWN ?"
	}
}

func (m Model) dashboardBentoTiles() []bento.BentoTile {
	registry := bento.NewRegistry()
	registry.Register(bento.NewProjectsTile(m.dashboardProjectRows()))
	registry.Register(bento.NewOverviewTile(m.dashboardServerSnapshot()))

	if snap, ok := buildCICDPipelineSnapshot(m); ok {
		registry.Register(bento.NewCICDPipelineTile(snap))
	} else {
		registry.Register(bento.NewCICDPlaceholderTile())
	}
	registry.Register(m.dashboardLiveLogsTile())
	registry.Register(m.dashboardTopologyTile())

	return registry.Tiles()
}

// dashboardTopologyTile builds the live service-topology tile for the
// currently selected project. When the operator has not picked one
// (or there are no projects yet) we fall back to the cyan placeholder
// so the cockpit silhouette never collapses.
func (m Model) dashboardTopologyTile() bento.BentoTile {
	projects := cfgProjects(m.cfg)
	if len(projects) == 0 || m.selectedIndex < 0 || m.selectedIndex >= len(projects) {
		return bento.NewTopologyPlaceholderTile()
	}
	project := projects[m.selectedIndex]
	status, hasStatus := m.statuses[project.ID]
	ci, hasCI := m.cicdSnapshots[project.ID]
	// Pulse is driven by the model clock: even seconds → off, odd
	// seconds → on. The bento engine repaints the cockpit on every
	// `RefreshDashboardMsg`, so this naturally shimmers BUILDING /
	// OFFLINE edges without an extra ticker.
	const pulseModulus = 2
	pulse := m.nowFn().Second()%pulseModulus == 1
	snap := buildTopologySnapshot(project, status, hasStatus, ci, hasCI, pulse)
	return bento.NewTopologyTile(snap)
}

func (m Model) dashboardProjectRows() []bento.ProjectRowSnapshot {
	projects := cfgProjects(m.cfg)
	if len(projects) == 0 {
		return nil
	}
	rows := make([]bento.ProjectRowSnapshot, 0, len(projects))
	for idx, project := range projects {
		state := ProjectUnknown
		if status, ok := m.statuses[project.ID]; ok {
			state = status.State
		}
		rows = append(rows, bento.ProjectRowSnapshot{
			Name:     project.Domain,
			State:    string(state),
			Selected: idx == m.selectedIndex,
		})
	}
	return rows
}

func (m Model) dashboardServerSnapshot() bento.ServerOverviewSnapshot {
	projects := cfgProjects(m.cfg)
	if len(projects) == 0 || m.selectedIndex < 0 || m.selectedIndex >= len(projects) {
		return bento.ServerOverviewSnapshot{ProjectAlias: ""}
	}
	project := projects[m.selectedIndex]
	status, hasStatus := m.statuses[project.ID]

	state := "UNKNOWN"
	if hasStatus {
		state = string(status.State)
	}

	ssl := "unknown"
	sslState := ""
	if hasStatus && status.SSLDaysLeft >= 0 {
		ssl = fmt.Sprintf("Valid (%d days remaining)", status.SSLDaysLeft)
		sslState = "ONLINE"
	} else if hasStatus {
		sslState = "STALE"
	}

	httpHealth := "pending"
	if hasStatus {
		httpHealth = fallbackString(status.HTTPHealth, "pending")
	}
	nodeVer := fallbackString(
		valueIfStatus(hasStatus, status.NodeVersion),
		fallbackString(project.NodeVersion, "unknown"),
	)

	// Icon column intentionally uses 1-cell glyphs (not emoji) so the
	// value column stays vertically aligned. The 2026-05-24 UX
	// refresh routes emoji to the tile *headers* where they sit on
	// their own line; data rows keep the geometric icon set.
	lines := []bento.ServerOverviewLine{
		{Icon: "▣", Label: "Profile", Value: fallbackString(project.ProfileAlias, "(unbound)")},
		{Icon: "◆", Label: "Stack", Value: fallbackString(project.Stack, "—")},
		{Icon: "◉", Label: "Node.js", Value: nodeVer, Status: state},
		{Icon: "✓", Label: "Status", Value: state, Status: state},
		{Icon: "↔", Label: "HTTP", Value: httpHealth},
		{Icon: "⚿", Label: "SSL", Value: ssl, Status: sslState},
		{Icon: "⎇", Label: "Repo", Value: fallbackString(project.Repo, "not linked")},
		{Icon: "⏲", Label: "Last Deploy", Value: fallbackString(valueIfStatus(hasStatus, status.LastDeploy), "—")},
	}
	return bento.ServerOverviewSnapshot{
		ProjectAlias: project.Domain,
		Lines:        lines,
	}
}

// valueIfStatus returns value when hasStatus is true, otherwise the
// empty string. Helper to keep the snapshot builder readable.
func valueIfStatus(hasStatus bool, value string) string {
	if !hasStatus {
		return ""
	}
	return value
}

// dashboardLiveLogsTile renders the bottom-row Live Server Logs tile.
// When no live stream is active (no project selected, or producer not
// yet running), the placeholder fills the slot so the cockpit silhouette
// never collapses.
func (m Model) dashboardLiveLogsTile() bento.BentoTile {
	mode := m.BentoMode()
	tailCap := liveLogsTailCap(mode)
	if m.liveLogs.Buffer == nil || m.liveLogs.Buffer.Len() == 0 {
		return bento.NewLogsPlaceholderTile()
	}
	raw := m.liveLogs.Buffer.Tail(tailCap)
	lines := make([]bento.MicroLogLine, 0, len(raw))
	for _, line := range raw {
		lines = append(lines, bento.MicroLogLine{
			Timestamp: extractTimestamp(line.Text),
			Level:     line.Level,
			Source:    extractSource(line.Text),
			Text:      stripParsedPrefix(line.Text),
			Redacted:  line.Redacted,
		})
	}
	// Subtract the two side gutters reserved by [bento.Engine] when
	// it composes the cockpit grid.
	const sideGutters = 2
	return bento.NewMicroLogsTileWithWidth(m.liveLogs.Domain, lines, m.width-sideGutters)
}

// timestamp / source parsing keeps the cockpit's log tile resilient to
// the mixed log formats SSH `tail -f` emits in the wild. When the
// optional prefix is not present we leave the cells empty so the
// renderer falls back to a level-only row.
//
// The parser is intentionally conservative: missing prefixes never
// raise an error, and the original text is preserved so the operator
// can scroll the raw payload if needed.
func extractTimestamp(text string) string {
	if len(text) >= 10 && text[0] == '[' && text[9] == ']' {
		return text[1:9]
	}
	return ""
}

func extractSource(text string) string {
	idx := strings.Index(text, " - ")
	if idx < 0 {
		return ""
	}
	colon := strings.Index(text[idx+3:], ":")
	if colon < 0 {
		return ""
	}
	return text[idx+3 : idx+3+colon]
}

// stripParsedPrefix returns the message body after `[timestamp] LEVEL - source: `.
// When no prefix is present the original text is returned untouched.
func stripParsedPrefix(text string) string {
	idx := strings.Index(text, ": ")
	if extractTimestamp(text) == "" {
		return text
	}
	if idx < 0 {
		return text
	}
	return text[idx+2:]
}

func fallbackString(value, def string) string {
	if value == "" {
		return def
	}
	return value
}

func (m Model) screen() views.Screen {
	statuses := make(map[string]views.ProjectStatus, len(m.statuses))
	for key, value := range m.statuses {
		statuses[key] = views.ProjectStatus{
			ProjectID:   value.ProjectID,
			HTTPHealth:  value.HTTPHealth,
			SSLDaysLeft: value.SSLDaysLeft,
			NodeVersion: value.NodeVersion,
			LastDeploy:  value.LastDeploy,
			State:       string(value.State),
			Stale:       value.Stale,
		}
	}
	width := m.width
	if width == 0 {
		width = 100
	}
	height := m.height
	if height == 0 {
		height = 30
	}
	return views.Screen{
		Width:         width,
		Height:        height,
		SelectedIndex: m.selectedIndex,
		ActiveTab:     m.activeTab.String(),
		Alert:         m.alert,
		HelpVisible:   m.helpVisible,
		Spinner:       m.spinner.View(),
		Config:        m.cfg,
		Statuses:      statuses,
		Styles:        m.styles,
		InitForm:      initFormSnapshot(m.initForm),
		ProjectForm:   projectFormSnapshot(m.projectForm),
		ResumeForm:    resumeFormSnapshot(m.resumeForm),
		ActionForm:    actionFormSnapshot(m.actionForm),
		ImportForm:    importFormSnapshot(m),
		LiveLogs:      liveLogsSnapshot(m),
	}
}

// liveLogsSnapshot is the pure view-layer projection of [liveLogsForm].
// The buffer is read via Snapshot() so consumers cannot mutate the
// streamer's underlying ring while rendering.
func liveLogsSnapshot(m Model) views.LiveLogsSnapshot {
	snap := views.LiveLogsSnapshot{
		Domain:     m.liveLogs.Domain,
		LogPath:    m.liveLogs.LogPath,
		AutoScroll: m.liveLogs.AutoScroll,
		Connected:  m.liveLogs.Connected,
		Err:        m.liveLogs.StreamErr,
	}
	if m.liveLogs.Buffer != nil {
		snap.BufferCap = m.liveLogs.Buffer.Cap()
		snap.BufferUsed = m.liveLogs.Buffer.Len()
		raw := m.liveLogs.Buffer.Tail(liveLogsTailCap(m.BentoMode()))
		for _, line := range raw {
			snap.Lines = append(snap.Lines, views.LiveLogLineSnapshot{
				Level:    line.Level,
				Text:     line.Text,
				Redacted: line.Redacted,
			})
		}
	}
	return snap
}

// Live-log tail-cap thresholds. Numbers come from `docs/UX.md §4.3`
// (Live Log Stream) — Ultra+ shows more history, Standard fits 12 rows
// without scrolling on an 80x24 terminal.
const (
	liveLogsTailCapUltraPlus = 24
	liveLogsTailCapUltra     = 18
	liveLogsTailCapStandard  = 12
)

// liveLogsTailCap caps the number of rendered rows so the live-log
// panel never overflows the cockpit.
func liveLogsTailCap(mode bento.Mode) int {
	switch mode {
	case bento.ModeUltraPlus:
		return liveLogsTailCapUltraPlus
	case bento.ModeUltra:
		return liveLogsTailCapUltra
	default:
		return liveLogsTailCapStandard
	}
}

func importFormSnapshot(m Model) views.ImportPreviewSnapshot {
	snap := views.ImportPreviewSnapshot{
		Loading: m.importForm.Loading,
		Saving:  m.importForm.Saving,
		Err:     m.importForm.Err,
	}
	for _, row := range m.importForm.Rows {
		snap.Rows = append(snap.Rows, views.ImportRowSnapshot{
			ProfileAlias: row.ProfileAlias,
			Domain:       row.Domain,
			Type:         row.Type,
			NodeVersion:  row.NodeVersion,
			Managed:      row.Managed,
		})
		if row.Managed {
			snap.Managed++
		} else {
			snap.Unmanaged++
		}
	}
	snap.Total = len(snap.Rows)
	return snap
}

func actionFormSnapshot(f projectActionForm) views.ProjectActionSnapshot {
	snap := views.ProjectActionSnapshot{
		Kind:      string(f.Kind),
		ProjectID: f.ProjectID,
		Running:   f.Running,
	}
	if f.Output != nil {
		snap.Output = string(f.Output)
	}
	if f.Err != nil {
		snap.Err = f.Err.Error()
	}
	return snap
}

func initFormSnapshot(f initWizardForm) views.InitWizardSnapshot {
	return views.InitWizardSnapshot{
		Step:   int(f.step),
		Alias:  f.alias,
		Host:   f.host,
		Port:   f.port,
		User:   f.user,
		Err:    f.err,
		Saving: f.saving,
	}
}

func projectFormSnapshot(f projectWizardForm) views.ProjectWizardSnapshot {
	snap := views.ProjectWizardSnapshot{
		Step:         int(f.step),
		ProfileAlias: f.profileAlias,
		Stack:        f.stack,
		Domain:       f.domain,
		NodeVersion:  f.nodeVersion,
		DBWanted:     f.dbWanted,
		DBKind:       f.dbKind,
		DBName:       f.dbName,
		Err:          f.err,
		Executing:    f.executing,
		RolledBack:   f.rolledBack,
		RollbackErr:  errString(f.rollbackErr),
	}
	if f.report != nil {
		snap.SubdomainOK = f.report.Subdomain.OK
		snap.SSLOK = f.report.SSL.OK
		snap.DatabaseOK = f.report.Database.OK
		if f.report.Subdomain.Err != nil {
			snap.SubdomainErr = f.report.Subdomain.Err.Error()
		}
		if f.report.SSL.Err != nil {
			snap.SSLErr = f.report.SSL.Err.Error()
		}
		if f.report.Database.Err != nil {
			snap.DatabaseErr = f.report.Database.Err.Error()
		}
	}
	for _, r := range f.rollbackResults {
		snap.RollbackResults = append(snap.RollbackResults, views.RollbackResultSnapshot{
			Name: r.Step.Name,
			Err:  errString(r.Err),
		})
	}
	return snap
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func resumeFormSnapshot(f resumeWizardForm) views.ResumeWizardSnapshot {
	snap := views.ResumeWizardSnapshot{
		Err:           f.err,
		Discarding:    f.discarding,
		DiscardPhrase: f.discardPhrase(),
		ConfirmInput:  f.confirmInput,
		RollingBack:   f.rollingBack,
	}
	if f.snapshot != nil {
		snap.WizardID = f.snapshot.WizardID
		snap.ProfileAlias = f.snapshot.ProfileAlias
		if !f.snapshot.UpdatedAt.IsZero() {
			snap.UpdatedAt = f.snapshot.UpdatedAt.Format("2006-01-02 15:04:05 UTC")
		}
		for _, step := range f.snapshot.Steps {
			snap.StepNames = append(snap.StepNames, step.Name)
		}
	}
	for _, result := range f.results {
		snap.Results = append(snap.Results, views.RollbackResultSnapshot{
			Name: result.Step.Name,
			Err:  errString(result.Err),
		})
	}
	return snap
}
