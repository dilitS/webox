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
	case StateResumeWizard:
		return views.RenderResumeWizard(screen)
	case StateImportPreview:
		return views.RenderImportPreview(screen)
	default:
		return m.styles.Panel.Render(fmt.Sprintf("%s is not enabled", m.state))
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
		ResumeForm:    resumeFormSnapshot(m.resumeForm),
		ActionForm:    actionFormSnapshot(m.actionForm),
		ImportForm:    importFormSnapshot(m),
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
