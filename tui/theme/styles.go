package theme

import "github.com/charmbracelet/lipgloss"

// Styles groups reusable Lipgloss styles so views do not hardcode ANSI.
type Styles struct {
	Header             lipgloss.Style
	Panel              lipgloss.Style
	ActivePanel        lipgloss.Style
	ProjectRow         lipgloss.Style
	SelectedProjectRow lipgloss.Style
	HelpHints          lipgloss.Style
	Muted              lipgloss.Style
	Alert              lipgloss.Style
	Value              lipgloss.Style

	status map[string]lipgloss.Style
}

// NewStyles builds component styles for a theme.
//
// As of the 2026-05-24 UX refresh, panels render with
// [lipgloss.ThickBorder] (┏━━━┓) while active panels upgrade to
// [lipgloss.DoubleBorder] (╔═══╗). This matches the bento cockpit
// tiles so wizards / detail screens / dashboard share the same
// frame language end-to-end.
func NewStyles(t Theme) Styles {
	basePanel := lipgloss.NewStyle().
		Border(lipgloss.ThickBorder()).
		BorderForeground(lipgloss.Color(t.Muted)).
		Padding(0, 1)

	activePanelBorder := lipgloss.DoubleBorder()

	return Styles{
		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(t.Primary)).
			Padding(0, 1),
		Panel: basePanel.
			Foreground(lipgloss.Color(t.TextBright)),
		ActivePanel: lipgloss.NewStyle().
			Border(activePanelBorder).
			BorderForeground(lipgloss.Color(t.Primary)).
			Padding(0, 1).
			Foreground(lipgloss.Color(t.TextBright)),
		ProjectRow: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.TextBright)),
		SelectedProjectRow: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(t.Primary)),
		HelpHints: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.TextDim)),
		Muted: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Muted)),
		Alert: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Warning)),
		Value: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.TextBright)).
			Bold(true),
		status: map[string]lipgloss.Style{
			"ONLINE":   newPremiumBadge(t.Success, t.SurfaceBase),
			"BUILDING": newPremiumBadge(t.Warning, t.SurfaceBase),
			"OFFLINE":  newPremiumBadge(t.Error, t.TextBright),
			"STALE":    newPremiumBadge(t.Muted, t.TextBright),
			"DEGRADED": newPremiumBadge(t.Degraded, t.SurfaceBase),
			"UNKNOWN": lipgloss.NewStyle().
				Foreground(lipgloss.Color(t.TextDim)).
				Padding(0, 1),
		},
	}
}

// newPremiumBadge composes the cockpit's pill-shaped status badges.
// They are filled with the role colour and bolded so the operator's
// eye lands on them first in the project list.
func newPremiumBadge(bg, fg string) lipgloss.Style {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(fg)).
		Background(lipgloss.Color(bg)).
		Padding(0, 1)
}

// StatusBadge returns a badge style for a normalized state string.
func (s Styles) StatusBadge(state string) lipgloss.Style {
	if style, ok := s.status[state]; ok {
		return style
	}
	return s.status["UNKNOWN"]
}
