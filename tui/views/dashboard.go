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
	dashboardLeftMinWidth     = 34
	dashboardRightMinWidth    = 46
	dashboardCompactLeftRatio = 42
	dashboardWideLeftRatio    = 38
	dashboardRatioDenominator = 100
	dashboardWideBreakpoint   = 108

	// Mini-bento ribbon budgets — Sprint 20 TASK-20.3. The two
	// strips appear below the projects/server row so the Standard
	// cockpit reads as a multi-tile bento. Sizing was picked by
	// adding up `body chrome (3 = top/bottom border + tile header)
	// + payload rows (1-3)` against the 100×30 floor. See
	// `standard_dashboard_test.go::TestRenderDashboard_Standard_FitsBudget`.
	dashboardCICDStripLines = 4
	dashboardLogsStripLines = 5
	// dashboardMainRowReserve is how many lines we keep available
	// for the projects+server top row after the mini-strips have
	// been allocated. Total body ≤ `dashboardMainRowReserve +
	// dashboardCICDStripLines + dashboardLogsStripLines` so the
	// View-level footer (1 line) + status bar (1 line) still fit
	// within the screen height.
	dashboardMainRowReserve = 17
)

// RenderDashboard renders the Standard Cockpit body — projects +
// server tile on top, mini CI/CD ribbon and mini live-log ribbon
// below. This is the `100×30 ≤ width < 120×35` fallback used when
// the Bento Ultra layout engine is unavailable.
//
// As of Sprint 20 TASK-20.3 the renderer composes a true
// mini-bento grid (matching the Ultra cockpit's grammar) instead
// of leaving 12 rows of dead space below the server pane:
//
//	┌── projects / server (main row) ──────────────┐
//	│ ┌──────────┐ ┌──────────────────────────┐    │
//	│ │ Projects │ │      Server overview     │    │
//	│ └──────────┘ └──────────────────────────┘    │
//	├──────────────────────────────────────────────┤
//	│ ⚡ [CI/CD] · #N · STATUS · Nm ago             │ (4 rows)
//	├──────────────────────────────────────────────┤
//	│ 📜 [Live Logs] · domain                       │ (5 rows)
//	│ ! ERROR ...                                   │
//	│ * INFO ...                                    │
//	└──────────────────────────────────────────────┘
//
// `tui.View` pins the cockpit status bar + footer hints around
// the body, so this function intentionally returns the body
// payload only (no banner line, no chrome).
func RenderDashboard(s Screen) string {
	width := clamp(s.Width, dashboardMinWidth, dashboardMaxWidth)
	height := clamp(s.Height, dashboardMinHeight, dashboardMaxHeight)

	mainRowHeight := height - dashboardCICDStripLines - dashboardLogsStripLines
	if mainRowHeight > dashboardMainRowReserve {
		mainRowHeight = dashboardMainRowReserve
	}
	if mainRowHeight < dashboardMinMainRowLines {
		mainRowHeight = dashboardMinMainRowLines
	}

	leftWidth := responsiveDashboardLeftWidth(width)
	rightWidth := width - leftWidth - dashboardPaneGap

	projects := renderProjectList(s, leftWidth, mainRowHeight)
	overview := renderDashboardOverview(s, rightWidth, mainRowHeight)
	mainRow := fitWidth(width, projects, "  ", overview)

	cicdStrip := renderMiniCICDStrip(s, width)
	logsStrip := renderMiniLogsStrip(s, width)

	return strings.Join([]string{mainRow, cicdStrip, logsStrip}, "\n")
}

// dashboardMinMainRowLines is the absolute floor for the
// projects+server row so the Standard cockpit still surfaces a
// project name + selection pill on tiny terminals (90×26-ish
// near the Tiny→Standard threshold).
const dashboardMinMainRowLines = 9

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

func renderDashboardOverview(s Screen, width, height int) string {
	tokens := theme.Default()
	project, ok := selectedProject(s)
	if !ok {
		header := dashboardHeader("🖥  [SERVER: (none)]", tokens.Primary)
		return s.Styles.Panel.Width(width).Height(height).Render(header + "\n\nSelect a project to inspect status.")
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

	return s.Styles.Panel.Width(width).Height(height).Render(strings.Join(lines, "\n"))
}

// renderMiniCICDStrip draws the compact CI/CD ribbon below the
// main projects/server row. The strip is a 4-line tile with a
// rounded thin border so it visually nests with the upper bento
// row. When no CI run has been observed for the selected project
// the strip falls back to a `(no run yet)` placeholder.
func renderMiniCICDStrip(s Screen, width int) string {
	tokens := theme.Default()
	header := dashboardHeader("⚡ [CI/CD]", tokens.Accent)

	if !s.CICDMini.HasRun {
		body := s.Styles.Muted.Render("(no run yet — connect a GitHub Actions workflow to populate this strip)")
		return renderMiniRibbon(s, width, dashboardCICDStripLines, header, body)
	}

	statusColour := cicdStatusColour(s.CICDMini.Status, tokens)
	statusBadge := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(statusColour)).
		Render(s.CICDMini.Status)

	headline := []string{
		fmt.Sprintf("Run #%d", s.CICDMini.RunNumber),
		statusBadge,
	}
	if s.CICDMini.JobName != "" {
		headline = append(headline, clipText(s.CICDMini.JobName, miniStripJobNameLimit))
	}
	if s.CICDMini.UpdatedAt != "" {
		headline = append(headline, s.Styles.Muted.Render(s.CICDMini.UpdatedAt))
	}

	rows := []string{strings.Join(headline, " · ")}
	if s.CICDMini.FailedStep != "" {
		rows = append(rows, lipgloss.NewStyle().
			Foreground(lipgloss.Color(tokens.Error)).
			Render("↪ failed: "+s.CICDMini.FailedStep))
	}

	return renderMiniRibbon(s, width, dashboardCICDStripLines, header, strings.Join(rows, "\n"))
}

// renderMiniLogsStrip draws the bottom mini live-log ribbon. We
// render the most recent two log lines with their level marker so
// the operator can confirm the project is alive without leaving
// the dashboard. Empty buffers fall back to a friendly `Waiting`
// hint so the strip never goes blank.
func renderMiniLogsStrip(s Screen, width int) string {
	tokens := theme.Default()

	headerLabel := "📜 [Live Logs]"
	if s.LiveLogs.Domain != "" {
		headerLabel = "📜 [Live Logs] · " + clipText(s.LiveLogs.Domain, miniStripDomainLimit)
	}
	header := dashboardHeader(headerLabel, tokens.Primary)

	if len(s.LiveLogs.Lines) == 0 {
		body := s.Styles.Muted.Render("Waiting for the first log line… (no logs streamed yet)")
		return renderMiniRibbon(s, width, dashboardLogsStripLines, header, body)
	}

	const tail = 3
	start := len(s.LiveLogs.Lines) - tail
	if start < 0 {
		start = 0
	}
	rows := make([]string, 0, tail)
	for _, line := range s.LiveLogs.Lines[start:] {
		marker := "·"
		colour := tokens.TextDim
		switch line.Level {
		case "ERROR":
			marker, colour = "✗", tokens.Error
		case "WARN":
			marker, colour = "!", tokens.Warning
		case "INFO":
			marker, colour = "·", tokens.TextBright
		}
		text := clipText(line.Text, width-miniStripBorderOverhead)
		rows = append(rows, lipgloss.NewStyle().
			Foreground(lipgloss.Color(colour)).
			Render(marker+" "+text))
	}
	return renderMiniRibbon(s, width, dashboardLogsStripLines, header, strings.Join(rows, "\n"))
}

const (
	// miniStripJobNameLimit + miniStripDomainLimit keep the
	// mini-bento ribbons under the 100-cell budget at the cockpit
	// floor. Both are tuned against
	// `TestRenderDashboard_Standard_LongJobNameClips`.
	miniStripJobNameLimit   = 28
	miniStripDomainLimit    = 36
	miniStripBorderOverhead = 6 // thin border + padding eats ~6 cells
)

// renderMiniRibbon is the shared chrome for the CI/CD and Live
// Logs strips: rounded border + tile header on row 0 + body
// payload below. The caller hands us a height budget so the strip
// always slots into the same vertical band regardless of payload
// length (overflow is silently clipped — operators see the full
// data on the project detail surface).
func renderMiniRibbon(_ Screen, width, height int, header, body string) string {
	tokens := theme.Default()
	const borderOverhead = 2 // 1 cell border on each axis
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(tokens.Muted)).
		Padding(0, 1).
		Width(width - borderOverhead).
		Height(height - borderOverhead)

	content := header
	if body != "" {
		content = header + "\n" + body
	}
	return style.Render(content)
}

// cicdStatusColour maps the verb to a theme tone so the mini
// ribbon stands out at a glance.
func cicdStatusColour(status string, tokens theme.Theme) string {
	switch status {
	case "SUCCESS":
		return tokens.Success
	case "FAILED":
		return tokens.Error
	case "IN_PROGRESS":
		return tokens.Warning
	case "QUEUED":
		return tokens.Accent
	default:
		return tokens.TextDim
	}
}

// clipText shortens `s` to fit `limit` cells with an ellipsis
// suffix so visible width never blows past the strip budget.
func clipText(s string, limit int) string {
	if limit <= 0 || len(s) <= limit {
		return s
	}
	if limit <= 1 {
		return "…"
	}
	return s[:limit-1] + "…"
}
