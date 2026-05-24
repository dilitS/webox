package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/dilitS/webox/tui/theme"
)

// StatusBarOptions configures [RenderStatusBar]. The status bar is the
// full-width chrome at the very top of the cockpit. It packs the
// branding badge (Webox + version + LIVE pill) on the left with a
// pipe-delimited stream of server metrics on the right.
//
// Each section is independent: empty strings are skipped so the bar
// gracefully degrades when SSH metrics, profile metadata, or the GitHub
// pipeline payload are not yet available.
type StatusBarOptions struct {
	// Brand is the leading "Webox vX.Y.Z" segment. When empty the
	// bar falls back to "WEBOX".
	Brand string
	// LiveLabel is the inverted pill rendered right after the brand
	// (typically "LIVE" / "STALE" / "OFFLINE"). The colour follows the
	// Tone field; an empty label hides the pill entirely.
	LiveLabel string
	// Tone routes LiveLabel to the matching theme accent. ToneInfo is
	// the magenta default, ToneSuccess paints LIVE green, ToneWarning
	// surfaces STALE in amber, ToneError flags OFFLINE in red.
	Tone Tone
	// Sections is the right-aligned stream of "key: value" cells
	// (e.g. "Uptime: 24d 11h", "RAM: 3.4/8.0 GB (42%)"). Empty
	// entries are stripped before rendering.
	Sections []string
	// Width is the total bar width; when ≤ 0 the function returns the
	// brand-only header so unit tests do not have to fake terminal
	// sizing.
	Width int
	// Theme exposes the active palette. Zero value falls back to
	// [theme.Default].
	Theme theme.Theme
}

// statusBarSeparator is the muted-pipe glyph that splits the right-hand
// metrics list. We keep it as a constant so unit tests can grep for it.
const statusBarSeparator = " │ "

// RenderStatusBar composes the cockpit-wide status bar. The renderer is
// pure: no I/O, no time lookups — callers feed a fully formatted
// snapshot.
func RenderStatusBar(opts StatusBarOptions) string {
	tokens := opts.Theme
	if tokens == (theme.Theme{}) {
		tokens = theme.Default()
	}

	brand := strings.TrimSpace(opts.Brand)
	if brand == "" {
		brand = "WEBOX"
	}
	brandStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(tokens.TextBright))
	left := brandStyle.Render(brand)

	if opts.LiveLabel != "" {
		pill := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(tokens.SurfaceBase)).
			Background(lipgloss.Color(toneColor(opts.Tone, tokens))).
			Padding(0, 1).
			Render(opts.LiveLabel)
		left = left + " " + pill
	}

	right := renderStatusBarSections(opts.Sections, tokens)

	if opts.Width <= 0 {
		if right == "" {
			return left
		}
		return left + statusBarSeparator + right
	}

	gap := opts.Width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func renderStatusBarSections(sections []string, tokens theme.Theme) string {
	cells := make([]string, 0, len(sections))
	cellStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(tokens.TextBright))
	sepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(tokens.Muted))
	for _, raw := range sections {
		cell := strings.TrimSpace(raw)
		if cell == "" {
			continue
		}
		cells = append(cells, cellStyle.Render(cell))
	}
	if len(cells) == 0 {
		return ""
	}
	return strings.Join(cells, sepStyle.Render(statusBarSeparator))
}

func toneColor(t Tone, tokens theme.Theme) string {
	switch t {
	case ToneSuccess:
		return tokens.Success
	case ToneWarning:
		return tokens.Warning
	case ToneError:
		return tokens.Error
	default:
		return tokens.Primary
	}
}
