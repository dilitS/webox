package tui

import (
	"fmt"

	"github.com/dilitS/webox/tui/bento"
	"github.com/dilitS/webox/tui/views"
)

// View renders the current model without mutating it.
func (m Model) View() string {
	screen := m.screen()
	switch m.state {
	case StateInitWizard:
		return views.RenderInitWizard(screen)
	case StateDashboard:
		return m.renderDashboard(screen)
	case StateProjectDetail:
		if m.activeTab == TabLogs {
			return views.RenderLiveLogs(screen)
		}
		return views.RenderProjectDetail(screen)
	case StateProjectWizard:
		return views.RenderProjectWizard(screen)
	case StateResumeWizard:
		return views.RenderResumeWizard(screen)
	case StateImportPreview:
		return views.RenderImportPreview(screen)
	default:
		return m.styles.Panel.Render(fmt.Sprintf("%s is not enabled", m.state))
	}
}

func (m Model) renderDashboard(screen views.Screen) string {
	mode := m.BentoMode()
	switch mode {
	case bento.ModeStandard:
		return views.RenderDashboard(screen)
	case bento.ModeTiny:
		return bento.NewEngine("Webox Cockpit v0.1", nil).
			RenderMode(screen.Width, screen.Height, mode)
	default:
		return bento.NewEngine("Webox Cockpit v0.1", m.dashboardBentoTiles()).
			RenderMode(screen.Width, screen.Height, mode)
	}
}

func (m Model) dashboardBentoTiles() []bento.BentoTile {
	registry := bento.NewRegistry()
	registry.Register(bento.NewProjectsTile(m.dashboardProjectRows()))

	domain, overview := m.dashboardOverviewSnapshot()
	registry.Register(bento.NewOverviewTile(domain, overview))

	registry.Register(bento.NewMetricsPlaceholderTile())
	registry.Register(bento.NewCICDPlaceholderTile())
	registry.Register(bento.NewLogsPlaceholderTile())
	registry.Register(bento.NewTopologyPlaceholderTile())

	return registry.Tiles()
}

func (m Model) dashboardProjectRows() []string {
	projects := cfgProjects(m.cfg)
	if len(projects) == 0 {
		return nil
	}
	rows := make([]string, 0, len(projects))
	for idx, project := range projects {
		marker := " "
		if idx == m.selectedIndex {
			marker = ">"
		}
		state := ProjectUnknown
		if status, ok := m.statuses[project.ID]; ok {
			state = status.State
		}
		rows = append(rows, fmt.Sprintf("%s %s [%s]", marker, project.Domain, state))
	}
	return rows
}

func (m Model) dashboardOverviewSnapshot() (domain string, lines []string) {
	projects := cfgProjects(m.cfg)
	if len(projects) == 0 || m.selectedIndex < 0 || m.selectedIndex >= len(projects) {
		return "", []string{"Select a project to inspect status."}
	}

	project := projects[m.selectedIndex]
	status, ok := m.statuses[project.ID]
	if !ok {
		return project.Domain, []string{
			"HTTP: pending",
			"SSL: unknown",
			"Node: " + fallbackString(project.NodeVersion, "unknown"),
			"Repo: " + fallbackString(project.Repo, "not linked"),
			"Last deploy: pending",
		}
	}

	ssl := "unknown"
	if status.SSLDaysLeft >= 0 {
		ssl = fmt.Sprintf("%d days remaining", status.SSLDaysLeft)
	}

	return project.Domain, []string{
		"Status: " + string(status.State),
		"HTTP: " + fallbackString(status.HTTPHealth, "pending"),
		"SSL: " + ssl,
		"Node: " + fallbackString(status.NodeVersion, fallbackString(project.NodeVersion, "unknown")),
		"Repo: " + fallbackString(project.Repo, "not linked"),
		"Last deploy: " + fallbackString(status.LastDeploy, "—"),
	}
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
