package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/dilitS/webox/tui/theme"
)

// HeaderBarOptions configures [RenderHeaderBar]. The header is composed
// of a gradient title on the left and an optional pill badge on the
// right (e.g. "[BENTO Ultra+]"). When [HeaderBarOptions.Width] is set,
// the function inserts padding so the badge flushes to the right edge.
type HeaderBarOptions struct {
	Title string
	Badge string
	Width int
	Theme theme.Theme
}

// RenderHeaderBar returns the rendered header line. When width is
// non-positive the function returns title + badge separated by a single
// space (useful for tests).
func RenderHeaderBar(opts HeaderBarOptions) string {
	tokens := opts.Theme
	if tokens == (theme.Theme{}) {
		tokens = theme.Default()
	}

	title := theme.Gradient(opts.Title, tokens.Primary, tokens.Degraded)

	if opts.Badge == "" {
		return title
	}

	badge := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(tokens.SurfaceBase)).
		Background(lipgloss.Color(tokens.Primary)).
		Padding(0, 1).
		Render(opts.Badge)

	if opts.Width <= 0 {
		return title + " " + badge
	}

	gap := opts.Width - lipgloss.Width(title) - lipgloss.Width(badge)
	if gap < 1 {
		gap = 1
	}
	return title + strings.Repeat(" ", gap) + badge
}

// LogoArt returns the multi-line ASCII logo painted with the gradient
// scheme. The logo is kept terminal-safe (no UTF-8 box characters that
// could break in cmd.exe) and intentionally short so it fits inside the
// Standard 100x30 silhouette.
func LogoArt(t theme.Theme) string {
	if t == (theme.Theme{}) {
		t = theme.Default()
	}
	lines := []string{
		"  __    __      _",
		" / / /\\ \\ \\___ | |__   _____  __",
		" \\ \\/  \\/ / _ \\| '_ \\ / _ \\ \\/ /",
		"  \\  /\\  /  __/| |_) | (_) >  < ",
		"   \\/  \\/ \\___||_.__/ \\___/_/\\_\\",
	}
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = theme.Gradient(line, t.Primary, t.Degraded)
	}
	return strings.Join(out, "\n")
}

// FormatModeBadge formats the mode label as the cockpit's signature
// "[BENTO Ultra+]" string. Centralising the formatter keeps every
// surface (engine, view, tests) in sync.
func FormatModeBadge(mode string) string {
	return fmt.Sprintf("[BENTO %s]", mode)
}
