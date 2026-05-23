package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/dilitS/webox/config"
)

func clamp(value, lower, upper int) int {
	if value < lower {
		return lower
	}
	if value > upper {
		return upper
	}
	return value
}

func selectedProject(s Screen) (config.Project, bool) {
	if s.Config == nil || len(s.Config.Projects) == 0 {
		return config.Project{}, false
	}
	idx := s.SelectedIndex
	if idx < 0 {
		idx = 0
	}
	if idx >= len(s.Config.Projects) {
		idx = len(s.Config.Projects) - 1
	}
	return s.Config.Projects[idx], true
}

func statusFor(s Screen, project config.Project) ProjectStatus {
	if got, ok := s.Statuses[project.ID]; ok {
		return got
	}
	state := "UNKNOWN"
	if project.ImportedAt != nil {
		state = "STALE"
	}
	return ProjectStatus{
		ProjectID:   project.ID,
		HTTPHealth:  "pending",
		SSLDaysLeft: -1,
		NodeVersion: fallback(project.NodeVersion, "unknown"),
		LastDeploy:  "pending Sprint 06",
		State:       state,
	}
}

func renderKV(label, value string) string {
	return fmt.Sprintf("%-14s %s", label+":", value)
}

func fallback(value, def string) string {
	if strings.TrimSpace(value) == "" {
		return def
	}
	return value
}

func fitWidth(width int, parts ...string) string {
	rendered := lipgloss.JoinHorizontal(lipgloss.Top, parts...)
	if width <= 0 {
		return rendered
	}
	return lipgloss.NewStyle().MaxWidth(width).Render(rendered)
}
