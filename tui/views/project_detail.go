package views

import (
	"fmt"
	"strings"
)

const (
	projectDetailMinWidth = 80
	projectDetailMaxWidth = 120
)

// RenderProjectDetail renders the Overview tab and dimmed roadmap tabs.
func RenderProjectDetail(s Screen) string {
	project, ok := selectedProject(s)
	if !ok {
		return s.Styles.Panel.Width(clamp(s.Width, projectDetailMinWidth, projectDetailMaxWidth)).Render("No project selected.\n\nEsc: back")
	}
	status := statusFor(s, project)
	width := clamp(s.Width, projectDetailMinWidth, projectDetailMaxWidth)

	tabs := projectDetailTabs(s, "[1] Overview")

	ssl := "unknown"
	if status.SSLDaysLeft >= 0 {
		ssl = fmt.Sprintf("%d days remaining", status.SSLDaysLeft)
	}

	body := []string{
		"🖥 [Project Detail: " + project.Domain + "]",
		"",
		tabs,
		"",
		s.Styles.StatusBadge(status.State).Render(status.State),
		"",
		renderKV("HTTP", fallback(status.HTTPHealth, "pending")),
		renderKV("Node", fallback(status.NodeVersion, project.NodeVersion)),
		renderKV("SSL", ssl),
		renderKV("Deploy path", fmt.Sprintf("~/domains/%s/public_html", project.Domain)),
		renderKV("Repo", fallback(project.Repo, "not linked")),
		renderKV("Last Deploy", fallback(status.LastDeploy, "no run yet")),
		"",
		actionLine(s),
		"",
		s.Styles.Muted.Render("[r] Restart  [s] SSL Renew  [v] Tail Logs"),
		s.Styles.HelpHints.Render("left/esc:back  q:quit"),
	}
	if action := renderProjectActionPanel(s, width); action != "" {
		body = append(body, "", action)
	}
	if s.Alert != "" {
		body = append(body, "", s.Styles.Alert.Render(s.Alert))
	}

	return s.Styles.ActivePanel.Width(width).Render(strings.Join(body, "\n"))
}

func actionLine(s Screen) string {
	switch {
	case s.ActionForm.Running && s.ActionForm.Kind != "":
		return s.Styles.Muted.Render(s.Spinner + " running " + s.ActionForm.Kind)
	case s.ActionForm.Err != "":
		return s.Styles.Alert.Render(s.ActionForm.Kind + " failed: " + s.ActionForm.Err)
	case s.ActionForm.Kind != "":
		return s.Styles.Muted.Render(s.ActionForm.Kind + " ok")
	default:
		return s.Styles.Muted.Render("no action yet")
	}
}

// actionPanelInsetWidth is subtracted from the parent panel width so
// the nested log panel does not overflow the rounded border of the
// outer ActivePanel container.
const actionPanelInsetWidth = 4

func renderProjectActionPanel(s Screen, panelWidth int) string {
	if s.ActionForm.Kind != "logs" || s.ActionForm.Output == "" {
		return ""
	}
	lines := strings.Split(strings.TrimRight(s.ActionForm.Output, "\n"), "\n")
	const maxRendered = 12
	if len(lines) > maxRendered {
		lines = append([]string{fmt.Sprintf("... (%d older lines omitted)", len(lines)-maxRendered)}, lines[len(lines)-maxRendered:]...)
	}
	rendered := strings.Join(lines, "\n")
	heading := fmt.Sprintf("logs (last %d lines)\n", len(lines))
	return s.Styles.Panel.Width(panelWidth - actionPanelInsetWidth).Render(heading + rendered)
}
