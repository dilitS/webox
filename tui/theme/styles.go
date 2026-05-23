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
			"ONLINE": lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(t.Success)),
			"BUILDING": lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(t.Warning)),
			"OFFLINE": lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(t.Error)),
			"STALE": lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(t.Muted)),
			"UNKNOWN": lipgloss.NewStyle().
				Foreground(lipgloss.Color(t.TextDim)),
		},
	}
}

// StatusBadge returns a badge style for a normalized state string.
func (s Styles) StatusBadge(state string) lipgloss.Style {
	if style, ok := s.status[state]; ok {
		return style
	}
	return s.status["UNKNOWN"]
}
