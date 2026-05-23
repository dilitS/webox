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
	default:
		return m.styles.Panel.Render(fmt.Sprintf("%s is not enabled in Sprint 04", m.state))
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
	}
}
