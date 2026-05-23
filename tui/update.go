package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
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
			return m, nil
		}
		m.cfg = msg.Config
		m.state = StateDashboard
		m.selectedIndex = clampIndex(m.selectedIndex, len(cfgProjects(m.cfg)))
		return m, tea.Batch(refreshVisibleProjectsCmd(m), scheduleRefresh(m.refreshInterval))
	case ConfigLoadFailedMsg:
		m.state = StateInitWizard
		m.alert = fmt.Sprintf("config load failed: %v", msg.Err)
		return m, nil
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
	default:
		return m, nil
	}
}

func (m Model) updateInitWizardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.cancel()
		return m, tea.Quit
	case "tab", "shift+tab", "enter":
		m.alert = "first-run actions arrive in Sprint 05"
		return m, nil
	default:
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
	case "right", "tab", "enter":
		if projectCount > 0 {
			m.state = StateProjectDetail
			m.activeTab = TabOverview
		}
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
		m.alert = "action available in Sprint 05+"
	}
	return m, nil
}
