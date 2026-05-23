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

	tabs := strings.Join([]string{
		"[1] Overview",
		s.Styles.Muted.Render("[2] Env Diff - unlocked in v0.2"),
		s.Styles.Muted.Render("[3] Database - unlocked in v0.2"),
		s.Styles.Muted.Render("[4] Logs - unlocked in v0.2"),
	}, "  ")

	ssl := "unknown"
	if status.SSLDaysLeft >= 0 {
		ssl = fmt.Sprintf("%d days remaining", status.SSLDaysLeft)
	}

	body := []string{
		project.Domain,
		tabs,
		"",
		s.Styles.StatusBadge(status.State).Render(status.State),
		"",
		renderKV("HTTP", fallback(status.HTTPHealth, "pending")),
		renderKV("Node", fallback(status.NodeVersion, project.NodeVersion)),
		renderKV("SSL", ssl),
		renderKV("Deploy path", fmt.Sprintf("~/domains/%s/public_html", project.Domain)),
		renderKV("Repo", fallback(project.Repo, "not linked")),
		renderKV("Last Deploy", fallback(status.LastDeploy, "pending Sprint 06")),
		"",
		s.Styles.Muted.Render("[r] Restart disabled  [s] SSL Renew disabled  [v] Logs disabled"),
		s.Styles.HelpHints.Render("left/esc:back  q:quit"),
	}
	if s.Alert != "" {
		body = append(body, "", s.Styles.Alert.Render(s.Alert))
	}

	return s.Styles.ActivePanel.Width(width).Render(strings.Join(body, "\n"))
}
