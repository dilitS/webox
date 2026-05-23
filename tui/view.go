package tui

import (
	"fmt"

	"github.com/dilitS/webox/tui/views"
)

// View renders the current model without mutating it.
func (m Model) View() string {
	screen := m.screen()
	switch m.state {
	case StateInitWizard:
		return views.RenderInitWizard(screen)
	case StateDashboard:
		return views.RenderDashboard(screen)
	case StateProjectDetail:
		return views.RenderProjectDetail(screen)
	case StateProjectWizard:
		return views.RenderProjectWizard(screen)
	default:
		return m.styles.Panel.Render(fmt.Sprintf("%s is not enabled in Sprint 05", m.state))
	}
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
	}
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
