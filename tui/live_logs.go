package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/dilitS/webox/tui/components"
)

// enterLiveLogsTab activates the Sprint 09 live-log tab for the
// currently selected project. The ring buffer is created lazily so
// projects that never trigger streaming pay no allocation cost.
//
// Production wiring (cmd/webox) will hand the model a SSH executor +
// streamer at startup and dispatch [LiveLogStreamCmd]; until then the
// tab renders the "Waiting for the first log line…" hint so the
// operator can verify the tab navigates correctly without an SSH
// dependency.
func (m Model) enterLiveLogsTab() (tea.Model, tea.Cmd) {
	projects := cfgProjects(m.cfg)
	if len(projects) == 0 || m.selectedIndex < 0 || m.selectedIndex >= len(projects) {
		m.alert = "no project selected"
		return m, nil
	}
	project := projects[m.selectedIndex]

	if m.liveLogs.Buffer == nil || m.liveLogs.ProjectID != project.ID {
		m.liveLogs = liveLogsForm{
			ProjectID:  project.ID,
			Domain:     project.Domain,
			AutoScroll: true,
			Buffer:     components.NewRingBuffer[LiveLogLine](liveLogsCapacity),
		}
	}
	m.activeTab = TabLogs
	return m, nil
}

// updateLiveLogsKey routes keys while the operator is on Tab [4]. The
// surface is intentionally narrow: only the documented Sprint 09
// shortcuts are honoured so the tab feels predictable.
func (m Model) updateLiveLogsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "left", "1":
		m.activeTab = TabOverview
		return m, nil
	case "4":
		return m, nil
	case "f":
		m.liveLogs.AutoScroll = !m.liveLogs.AutoScroll
		if m.liveLogs.AutoScroll {
			m.alert = "auto-scroll on"
		} else {
			m.alert = "auto-scroll paused"
		}
		return m, nil
	case "c":
		if m.liveLogs.Buffer != nil {
			m.liveLogs.Buffer = components.NewRingBuffer[LiveLogLine](liveLogsCapacity)
		}
		m.alert = "log buffer cleared"
		return m, nil
	case "up", "k":
		if m.liveLogs.AutoScroll {
			m.liveLogs.AutoScroll = false
			m.alert = "auto-scroll paused"
		}
		m.liveLogs.ScrollOffset++
		return m, nil
	case "down", "j":
		if m.liveLogs.ScrollOffset > 0 {
			m.liveLogs.ScrollOffset--
		}
		return m, nil
	}
	return m, nil
}
