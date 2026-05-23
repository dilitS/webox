package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	dashboardMinWidth    = 88
	dashboardMaxWidth    = 120
	dashboardMinHeight   = 24
	dashboardMaxHeight   = 40
	dashboardLeftWidth   = 36
	dashboardPaneGap     = 6
	dashboardFooterLines = 8
)

// RenderDashboard renders the project list and per-project overview tile.
// This is the Standard Cockpit (100x30) fallback used when the Bento Ultra
// layout engine is unavailable or the terminal is below the 120x35 threshold.
func RenderDashboard(s Screen) string {
	width := clamp(s.Width, dashboardMinWidth, dashboardMaxWidth)
	height := clamp(s.Height, dashboardMinHeight, dashboardMaxHeight)
	leftWidth := dashboardLeftWidth
	rightWidth := width - leftWidth - dashboardPaneGap

	header := s.Styles.Header.Width(width).Render("Webox Cockpit v0.1 " + s.Spinner)
	projects := renderProjectList(s, leftWidth, height-dashboardFooterLines)
	overview := renderDashboardOverview(s, rightWidth)
	body := fitWidth(width, projects, "  ", overview)

	footerParts := []string{"q:quit", "up/down:navigate", "right/tab:detail", "n:new", "i:import", "?:help"}
	if s.Alert != "" {
		footerParts = append(footerParts, s.Styles.Alert.Render(s.Alert))
	}
	if s.HelpVisible {
		footerParts = append(footerParts, "[r] restart  [s] ssl renew  [v] tail logs")
	}
	footer := s.Styles.HelpHints.Width(width).Render(strings.Join(footerParts, "  "))

	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

func renderProjectList(s Screen, width, height int) string {
	if s.Config == nil || len(s.Config.Projects) == 0 {
		return s.Styles.Panel.Width(width).Height(height).Render("Projects\n\nNo projects yet.\nPress [n] to start the new-project wizard.")
	}

	rows := []string{"Projects", ""}
	for idx, project := range s.Config.Projects {
		status := statusFor(s, project)
		marker := " "
		style := s.Styles.ProjectRow
		if idx == s.SelectedIndex {
			marker = ">"
			style = s.Styles.SelectedProjectRow
		}
		badge := s.Styles.StatusBadge(status.State).Render(status.State)
		row := fmt.Sprintf("%s %-23s %s %s", marker, project.Domain, badge, fallback(status.NodeVersion, project.NodeVersion))
		rows = append(rows, style.Render(row))
	}

	return s.Styles.ActivePanel.Width(width).Height(height).Render(strings.Join(rows, "\n"))
}

func renderDashboardOverview(s Screen, width int) string {
	project, ok := selectedProject(s)
	if !ok {
		return s.Styles.Panel.Width(width).Render("Overview\n\nSelect a project to inspect status.")
	}
	status := statusFor(s, project)
	ssl := "unknown"
	if status.SSLDaysLeft >= 0 {
		ssl = fmt.Sprintf("%d days remaining", status.SSLDaysLeft)
	}

	lines := []string{
		project.Domain,
		"",
		s.Styles.StatusBadge(status.State).Render(status.State),
		"",
		renderKV("HTTP Health", fallback(status.HTTPHealth, "pending")),
		renderKV("SSL", ssl),
		renderKV("Node", fallback(status.NodeVersion, project.NodeVersion)),
		renderKV("Repo", fallback(project.Repo, "not linked")),
		renderKV("Last Deploy", fallback(status.LastDeploy, "—")),
		"",
		s.Styles.Muted.Render("[r] Restart  [s] SSL Renew  [v] Tail Logs"),
	}

	return s.Styles.Panel.Width(width).Render("Overview\n\n" + strings.Join(lines, "\n"))
}
