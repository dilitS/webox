package views

import (
	"fmt"
	"strings"
)

const (
	importPreviewMinWidth = 90
	importPreviewMaxWidth = 130
	importPreviewMaxRows  = 14

	importTableLeadingPad   = 2
	importColDomainWidth    = 40
	importColTypeWidth      = 9
	importColNodeWidth      = 7
	truncateMinEllipsisRoom = 3
)

// RenderImportPreview renders the read-only diff between the panel's
// reported subdomains and the projects tracked in `config.json`. It
// is intentionally a single-screen modal: the wizard takes over for
// any actual work — this view only collects an `a` confirmation to
// stub local config entries for the unmanaged rows.
func RenderImportPreview(s Screen) string {
	width := clamp(s.Width, importPreviewMinWidth, importPreviewMaxWidth)
	form := s.ImportForm

	rows := []string{
		"Import Existing Projects",
		"",
		"Read-only scan: Webox compares panel-reported subdomains with config.json.",
		"No server resource is modified by this screen.",
		"",
	}

	switch {
	case form.Loading:
		rows = append(rows, s.Spinner+" scanning provider subdomains...")
	case form.Err != "":
		rows = append(rows, s.Styles.Alert.Render(form.Err))
	default:
		summary := fmt.Sprintf(
			"Found %d subdomain(s): %d already managed, %d new.",
			form.Total, form.Managed, form.Unmanaged,
		)
		rows = append(rows, summary, "", renderImportTable(form))
	}
	if form.Saving {
		rows = append(rows, "", s.Spinner+" writing imported projects...")
	}
	rows = append(
		rows,
		"",
		s.Styles.Muted.Render("[a] Import all unmanaged   [esc] Cancel"),
	)
	return s.Styles.ActivePanel.Width(width).Render(strings.Join(rows, "\n"))
}

func renderImportTable(form ImportPreviewSnapshot) string {
	if len(form.Rows) == 0 {
		return "  (no subdomains reported by any profile)"
	}

	header := fmt.Sprintf("  %-3s  %-40s  %-9s  %-7s  %s",
		"#", "Domain", "Type", "Node", "Profile (status)")
	lines := []string{header, "  " + strings.Repeat("-", len(header)-importTableLeadingPad)}

	limit := len(form.Rows)
	truncated := false
	if limit > importPreviewMaxRows {
		limit = importPreviewMaxRows
		truncated = true
	}
	for i := 0; i < limit; i++ {
		row := form.Rows[i]
		status := "new"
		if row.Managed {
			status = "managed"
		}
		lines = append(lines, fmt.Sprintf(
			"  %-3d  %-40s  %-9s  %-7s  %s (%s)",
			i+1,
			truncateCell(row.Domain, importColDomainWidth),
			truncateCell(fallback(row.Type, "-"), importColTypeWidth),
			truncateCell(fallback(row.NodeVersion, "-"), importColNodeWidth),
			fallback(row.ProfileAlias, "-"),
			status,
		))
	}
	if truncated {
		lines = append(lines, fmt.Sprintf("  ... %d more row(s) hidden", len(form.Rows)-limit))
	}
	return strings.Join(lines, "\n")
}

func truncateCell(s string, width int) string {
	if width <= 0 || len(s) <= width {
		return s
	}
	if width < truncateMinEllipsisRoom {
		return s[:width]
	}
	return s[:width-truncateMinEllipsisRoom] + "..."
}
