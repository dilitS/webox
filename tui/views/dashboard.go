package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/dilitS/webox/tui/theme"
)

const (
	dashboardMinWidth         = 88
	dashboardMaxWidth         = 120
	dashboardMinHeight        = 24
	dashboardMaxHeight        = 40
	dashboardPaneGap          = 2
	dashboardFooterLines      = 4
	dashboardLeftMinWidth     = 34
	dashboardRightMinWidth    = 46
	dashboardCompactLeftRatio = 42
	dashboardWideLeftRatio    = 38
	dashboardRatioDenominator = 100
	dashboardWideBreakpoint   = 108
)

// RenderDashboard renders the project list and per-project overview tile.
//
// This is the Standard Cockpit (`100×30 ≤ width < 120×35`) fallback used
// when the Bento Ultra layout engine is unavailable. Sprint 13 aligned
// its visual grammar with Bento Ultra: bracketed emoji titles
// (`📂 [Active Projects]`, `🖥 [SERVER: …]`), rounded selection pills
// painted in the cockpit's primary tone, and thick-bordered panels
// that match the cockpit tiles end-to-end. The caller (`tui.View`)
// pins the cockpit status bar + footer hints around it, so this
// function intentionally returns the body only (no banner line).
func RenderDashboard(s Screen) string {
	width := clamp(s.Width, dashboardMinWidth, dashboardMaxWidth)
	height := clamp(s.Height, dashboardMinHeight, dashboardMaxHeight)
	leftWidth := responsiveDashboardLeftWidth(width)
	rightWidth := width - leftWidth - dashboardPaneGap

	projects := renderProjectList(s, leftWidth, height-dashboardFooterLines)
	overview := renderDashboardOverview(s, rightWidth)
	return fitWidth(width, projects, "  ", overview)
}

func responsiveDashboardLeftWidth(width int) int {
	ratio := dashboardCompactLeftRatio
	if width >= dashboardWideBreakpoint {
		ratio = dashboardWideLeftRatio
	}
	left := (width * ratio) / dashboardRatioDenominator
	if left < dashboardLeftMinWidth {
		left = dashboardLeftMinWidth
	}
	maxLeft := width - dashboardPaneGap - dashboardRightMinWidth
	if left > maxLeft {
		left = maxLeft
	}
	return left
}

// dashboardHeader renders the bracketed emoji title used by every
// Standard cockpit panel so the surface reads with the same grammar as
// Bento Ultra tiles. The accent argument is the role colour (Primary
// for Projects / Server / Logs, Accent for CI/CD-flavoured panels).
func dashboardHeader(title, accent string) string {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(accent)).
		Render(title)
}

func renderProjectList(s Screen, width, height int) string {
	tokens := theme.Default()
	header := dashboardHeader("📂 [Active Projects]", tokens.Primary)

	if s.Config == nil || len(s.Config.Projects) == 0 {
		body := header + "\n\nNo projects yet.\nPress [n] to start the new-project wizard."
		return s.Styles.Panel.Width(width).Height(height).Render(body)
	}

	rows := []string{header, ""}
	for idx, project := range s.Config.Projects {
		status := statusFor(s, project)
		dot := lipgloss.NewStyle().
			Foreground(lipgloss.Color(dashboardStateColor(status.State, tokens))).
			Render("●")
		name := project.Domain
		if idx == s.SelectedIndex {
			pill := lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(tokens.TextBright)).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color(tokens.Primary)).
				Padding(0, 1).
				Render(name)
			rows = append(rows, lipgloss.JoinHorizontal(
				lipgloss.Top,
				pill,
				lipgloss.NewStyle().PaddingLeft(1).PaddingTop(1).Render(dot),
			))
			continue
		}
		nameStyled := lipgloss.NewStyle().Foreground(lipgloss.Color(tokens.TextBright)).Render(name)
		rows = append(rows, "  "+nameStyled+" "+dot)
	}

	return s.Styles.ActivePanel.Width(width).Height(height).Render(strings.Join(rows, "\n"))
}

// dashboardStateColor mirrors `tui/bento.stateColor` (kept inlined to
// preserve the views package boundary — adding a `tui/bento` import
// from `tui/views` would create a cycle).
func dashboardStateColor(state string, tokens theme.Theme) string {
	switch strings.ToUpper(state) {
	case "ONLINE":
		return tokens.Success
	case "BUILDING":
		return tokens.Warning
	case "OFFLINE":
		return tokens.Error
	case "STALE":
		return tokens.Muted
	case "DEGRADED":
		return tokens.Degraded
	default:
		return tokens.TextDim
	}
}

func renderDashboardOverview(s Screen, width int) string {
	tokens := theme.Default()
	project, ok := selectedProject(s)
	if !ok {
		header := dashboardHeader("🖥  [SERVER: (none)]", tokens.Primary)
		return s.Styles.Panel.Width(width).Render(header + "\n\nSelect a project to inspect status.")
	}
	status := statusFor(s, project)
	ssl := "unknown"
	if status.SSLDaysLeft >= 0 {
		ssl = fmt.Sprintf("%d days remaining", status.SSLDaysLeft)
	}

	header := dashboardHeader("🖥  [SERVER: "+project.Domain+"]", tokens.Primary)
	lines := []string{
		header,
		"",
		s.Styles.StatusBadge(status.State).Render(status.State),
		"",
		renderKV("HTTP Health", fallback(status.HTTPHealth, "pending")),
		renderKV("SSL", ssl),
		renderKV("Node", fallback(status.NodeVersion, project.NodeVersion)),
		renderKV("Repo", fallback(project.Repo, "not linked")),
		renderKV("Last Deploy", fallback(status.LastDeploy, "—")),
	}
	if len(s.Connections) > 0 {
		lines = append(lines, "", lipgloss.NewStyle().Foreground(lipgloss.Color(tokens.TextDim)).Render("Connections:"))
		lines = append(lines, s.Connections...)
	}
	lines = append(lines, "", s.Styles.Muted.Render("[r] Restart  [s] SSL Renew  [v] Tail Logs"))

	return s.Styles.Panel.Width(width).Render(strings.Join(lines, "\n"))
}
