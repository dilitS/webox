package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/dilitS/webox/tui/theme"
)

// RenderEnvDiff renders the Sprint 20 TASK-20.4 read-only Env Diff
// tab on the project detail surface.
//
// Contract:
//
//   - Renders the project's `SecretsMeta` slice (one row per
//     managed env var key) with source / created-at / last-rotated
//     / last-synced columns. Plaintext values NEVER appear here —
//     they live in the keyring or `secrets.enc` (`docs/SECURITY.md
//     §4`).
//   - Calls no provider methods. Every value comes from
//     `views.Screen.Secrets`, which the model populates from
//     `config.Project.SecretsMeta`. That keeps the renderer
//     side-effect free and lets the project detail surface
//     respond to a `2` keypress without an SSH dial.
//   - When the project has no managed secrets the body shows a
//     short onboarding hint pointing to `webox doctor secrets
//     init` (Sprint 21+).
//
// Visual grammar mirrors the Overview tab so operators can hop
// between the two without re-anchoring their eyes:
//
//	🖥 [Project Detail: domain]
//	  [1] Overview  [2] Env Diff  [3] Database  [4] Logs
//
//	🔑 Managed Secrets (N)
//	  KEY                     SOURCE        ROTATED      REMINDER
//	  DATABASE_URL            managed       2026-04-12   90d (stale!)
//	  STRIPE_KEY              managed       2026-05-01   90d
//	  ...
//	  legend: managed = both gh + server, server_only = panel-side, external = operator-managed
//
// The "stale" badge appears when `now - LastRotated >
// RotationReminderDays`; the renderer trusts the precomputed
// `Stale` flag on [SecretMetaSnapshot] (computed by `tui.screen()`
// against the model's clock).
func RenderEnvDiff(s Screen) string {
	project, ok := selectedProject(s)
	if !ok {
		return s.Styles.Panel.
			Width(clamp(s.Width, projectDetailMinWidth, projectDetailMaxWidth)).
			Render("No project selected.\n\nEsc: back")
	}
	width := clamp(s.Width, projectDetailMinWidth, projectDetailMaxWidth)
	tabs := projectDetailTabs(s, "[2] Env Diff")

	header := fmt.Sprintf("🖥 [Project Detail: %s]", project.Domain)

	body := []string{
		header,
		"",
		tabs,
		"",
	}

	if len(s.Secrets) == 0 {
		emptyMessage := []string{
			s.Styles.Muted.Render("🔑 Managed Secrets (none)"),
			"",
			"This project has no Webox-managed secrets yet.",
			"",
			s.Styles.Muted.Render("Sync secrets with: webox doctor secrets init <project> (v0.2)"),
			"",
			s.Styles.Muted.Render("Plaintext values never appear here — they live in the keyring / secrets.enc"),
			s.Styles.HelpHints.Render("[1] overview  [3] database  [4] logs  esc/tab: back"),
		}
		body = append(body, emptyMessage...)
		return s.Styles.ActivePanel.Width(width).Render(strings.Join(body, "\n"))
	}

	tokens := theme.Default()
	heading := []string{
		fmt.Sprintf("🔑 Managed Secrets (%d)", len(s.Secrets)),
		"",
		envDiffHeaderRow(),
	}
	body = append(body, heading...)
	for _, secret := range s.Secrets {
		body = append(body, envDiffRow(secret, tokens))
	}
	footer := []string{
		"",
		s.Styles.Muted.Render("legend: managed = synced with both server + GitHub Actions secrets · server_only = panel-side · external = operator-managed"),
		"",
		s.Styles.Muted.Render("Plaintext values live in the keyring or secrets.enc — never on the cockpit chrome."),
		s.Styles.HelpHints.Render("[1] overview  [3] database  [4] logs  esc/tab: back"),
	}
	body = append(body, footer...)
	return s.Styles.ActivePanel.Width(width).Render(strings.Join(body, "\n"))
}

// envDiffKeyColumnWidth + envDiffSourceColumnWidth +
// envDiffDateColumnWidth keep the column widths in one place so
// header / row formatting cannot drift apart. The values are
// tuned against `100×30` Standard cockpits so the four columns
// fit inside the chrome border with a single space margin.
const (
	envDiffKeyColumnWidth    = 22
	envDiffSourceColumnWidth = 13
	envDiffDateColumnWidth   = 12
)

// envDiffHeaderRow returns the column header line. Width-aligned
// columns keep the table readable on 100×30 cockpits.
func envDiffHeaderRow() string {
	keyCol := pad("KEY", envDiffKeyColumnWidth)
	sourceCol := pad("SOURCE", envDiffSourceColumnWidth)
	dateCol := pad("ROTATED", envDiffDateColumnWidth)
	return "  " + keyCol + "  " + sourceCol + "  " + dateCol + "  REMINDER"
}

// pad left-aligns `s` to `width` cells, truncating with an
// ellipsis when needed. Centralises the width arithmetic so the
// header and body rows stay column-aligned.
func pad(s string, width int) string {
	clipped := clipText(s, width)
	if len(clipped) >= width {
		return clipped
	}
	return clipped + strings.Repeat(" ", width-len(clipped))
}

// envDiffRow formats one snapshot. The Stale flag colours the
// reminder cell red so the operator's eye latches on to it
// without having to scan dates.
func envDiffRow(s SecretMetaSnapshot, tokens theme.Theme) string {
	rotated := s.LastRotated
	if rotated == "" {
		rotated = "—"
	}
	reminder := "—"
	if s.RotationReminderDays > 0 {
		reminder = fmt.Sprintf("%dd", s.RotationReminderDays)
	}
	if s.Stale {
		reminder = lipgloss.NewStyle().
			Foreground(lipgloss.Color(tokens.Error)).
			Render(reminder + " (stale!)")
	}
	source := s.Source
	if source == "" {
		source = "unknown"
	}
	keyCol := pad(s.Key, envDiffKeyColumnWidth)
	sourceCol := pad(source, envDiffSourceColumnWidth)
	dateCol := pad(rotated, envDiffDateColumnWidth)
	return "  " + keyCol + "  " + sourceCol + "  " + dateCol + "  " + reminder
}

// projectDetailTabs renders the four-tab strip used by every
// project detail tab. The active tab is bolded; the rest stay
// dim so the operator can see at a glance which tab they are on
// without having to look at the crumb.
func projectDetailTabs(s Screen, activeLabel string) string {
	labels := []string{"[1] Overview", "[2] Env Diff", "[3] Database", "[4] Logs"}
	rendered := make([]string, 0, len(labels))
	for _, label := range labels {
		if label == activeLabel {
			rendered = append(rendered, lipgloss.NewStyle().Bold(true).Render(label))
			continue
		}
		rendered = append(rendered, s.Styles.Muted.Render(label))
	}
	return strings.Join(rendered, "  ")
}
