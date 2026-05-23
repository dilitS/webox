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
func NewStyles(t Theme) Styles {
	basePanel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(t.Muted)).
		Padding(0, 1)

	return Styles{
		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(t.Primary)).
			Padding(0, 1),
		Panel: basePanel.
			Foreground(lipgloss.Color(t.TextBright)),
		ActivePanel: basePanel.
			BorderForeground(lipgloss.Color(t.Primary)).
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
