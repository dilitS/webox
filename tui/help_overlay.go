package tui

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/dilitS/webox/tui/surface"
	"github.com/dilitS/webox/tui/theme"
	"github.com/dilitS/webox/tui/views"
)

// renderHelpOverlay returns the Sprint 20 TASK-20.5 help overlay
// for the surface that owns the current state.
//
// The overlay carries two columns:
//
//  1. Surface-specific keys, parsed from `surface.Footer().Text`.
//     Footers use a stable "[key] description …" grammar enforced
//     by [tui.surface.FooterHint] tests, so we lean on a single
//     regex extraction rather than mirroring every binding here.
//  2. Global keys (`?` toggle, `q` quit, `Esc` dismiss) — these
//     never make it to the per-surface footers because they are
//     trivially discoverable, but the help screen is the right
//     place to enumerate them.
//
// The overlay is intentionally narrow (40-column width) so it
// stays readable on Tiny mode (`ModeTiny`) too. The caller is
// responsible for rendering it on top of `base` — we return the
// rendered string only.
func renderHelpOverlay(m Model, screen views.Screen) string {
	tokens := theme.Default()
	width := overlayWidth(screen.Width)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(tokens.Primary))
	mutedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(tokens.TextDim))
	keyStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(tokens.Accent))
	const overlayPaddingX = 2
	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(tokens.Primary)).
		Padding(0, overlayPaddingX).
		Width(width)

	surfaceLabel := helpSurfaceLabel(m, screen)
	rows := []string{
		titleStyle.Render("? Help"),
		mutedStyle.Render("Surface: " + surfaceLabel),
		"",
		titleStyle.Render("Surface keys"),
	}
	bindings := helpExtractBindings(m, screen)
	if len(bindings) == 0 {
		rows = append(rows, mutedStyle.Render("(no surface-specific keys)"))
	} else {
		for _, b := range bindings {
			rows = append(rows, helpRow(b, keyStyle, mutedStyle, width))
		}
	}
	globals := []string{
		"",
		titleStyle.Render("Global keys"),
		helpRow(helpBinding{key: "[?]", desc: "toggle this help overlay"}, keyStyle, mutedStyle, width),
		helpRow(helpBinding{key: "[Esc]", desc: "dismiss overlay / back"}, keyStyle, mutedStyle, width),
		helpRow(helpBinding{key: "[Enter]", desc: "confirm / dismiss overlay"}, keyStyle, mutedStyle, width),
		helpRow(helpBinding{key: "[q]", desc: "quit cockpit"}, keyStyle, mutedStyle, width),
		helpRow(helpBinding{key: "[Ctrl+C]", desc: "force quit cockpit"}, keyStyle, mutedStyle, width),
		"",
		mutedStyle.Render("Press ? or Esc to close."),
	}
	rows = append(rows, globals...)
	return border.Render(strings.Join(rows, "\n"))
}

// helpSurfaceLabel returns a human-readable name for the active
// surface. Falls back to "Cockpit" when no surface adapter is
// registered for the state — that should never happen in
// production but the help overlay should never panic on first
// touch either.
func helpSurfaceLabel(m Model, screen views.Screen) string {
	s := m.surfaceFor()
	if s == nil {
		return "Cockpit"
	}
	ctx := surface.Context{Screen: screen}
	if c := s.Crumb(ctx); c != "" {
		return c
	}
	return "Cockpit"
}

// helpExtractBindings parses `Surface.Footer().Text` for the
// active surface. The footer grammar is stable across surfaces:
// space-separated `[key] description` segments split by `·` /
// `|`. We split on those separators first, then extract `[key]`
// + the trailing description.
func helpExtractBindings(m Model, screen views.Screen) []helpBinding {
	s := m.surfaceFor()
	if s == nil {
		return nil
	}
	hint := s.Footer(surface.Context{Screen: screen}).Text
	hint = strings.TrimSpace(hint)
	if hint == "" {
		return nil
	}
	separator := regexp.MustCompile(`\s*[·|]\s*`)
	parts := separator.Split(hint, -1)
	bindings := make([]helpBinding, 0, len(parts))
	for _, raw := range parts {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if b, ok := helpParseBinding(raw); ok {
			bindings = append(bindings, b)
		}
	}
	return bindings
}

// helpBinding is a single keybinding row inside the help
// overlay — `[key]` plus a short description. We keep the
// brackets because the operator's muscle memory is wired to
// "the thing in square brackets is the keystroke".
type helpBinding struct {
	key  string
	desc string
}

// bindingPattern matches a leading `[token]` followed by an
// optional descriptive remainder.
var bindingPattern = regexp.MustCompile(`^(\[[^\]]+\])\s*(.*)$`)

// helpParseBinding extracts `[key]` + description from a single
// footer segment.
func helpParseBinding(raw string) (helpBinding, bool) {
	matches := bindingPattern.FindStringSubmatch(raw)
	if matches == nil {
		return helpBinding{}, false
	}
	desc := strings.TrimSpace(matches[2])
	return helpBinding{key: matches[1], desc: desc}, true
}

// helpRow renders one keybinding row. Keys are right-padded to
// a fixed column so the description column lines up regardless
// of binding length.
func helpRow(b helpBinding, keyStyle, descStyle lipgloss.Style, width int) string {
	const keyColumn = 12
	keyCell := b.key
	if len(keyCell) < keyColumn {
		keyCell += strings.Repeat(" ", keyColumn-len(keyCell))
	}
	desc := b.desc
	if desc == "" {
		desc = "(no description)"
	}
	// Truncate at width minus borders / padding so we never
	// overflow the modal box. `chromeOverhead` covers
	// `Padding(0,2)` (4 cols) plus the rounded border (2 cols)
	// plus the spacer between key and description (1 col) plus
	// a 1-col safety margin for renderers that drop a trailing
	// glyph on narrow terminals.
	const chromeOverhead = 8
	maxDesc := width - keyColumn - chromeOverhead
	if maxDesc < 1 {
		maxDesc = 1
	}
	if len(desc) > maxDesc {
		desc = desc[:maxDesc-1] + "…"
	}
	return keyStyle.Render(keyCell) + " " + descStyle.Render(desc)
}

// overlayWidth clamps the overlay box to a sensible reading
// width. We never grow past 70 columns because the help text is
// short and a wider box looks empty; we never shrink below the
// minimum the help title fits in.
func overlayWidth(screenWidth int) int {
	const (
		minWidth = 36
		maxWidth = 70
	)
	if screenWidth <= 0 {
		return minWidth
	}
	if screenWidth < minWidth {
		return minWidth
	}
	if screenWidth > maxWidth {
		return maxWidth
	}
	return screenWidth
}

// helpOverlayFullscreen renders the help panel centered on a
// blank canvas of `screen.Width × screen.Height`. We never
// composite over the dashboard body because terminal frames
// (especially the bento bento engine on Ultra+) already paint
// borders + status bar — overlapping with another modal would
// produce illegible double-border artifacts. Instead we replace
// the View() output entirely; the operator dismisses with `?`
// or `Esc` to return to the underlying state, which never
// changed.
func helpOverlayFullscreen(m Model, screen views.Screen) string {
	overlay := renderHelpOverlay(m, screen)
	height := screen.Height
	if height < 1 {
		height = 1
	}
	width := screen.Width
	if width < 1 {
		width = 1
	}
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, overlay)
}

// helpDebugString is exposed only to tests so they can assert
// the surface label and binding contents without colour codes.
// It produces the same content as `renderHelpOverlay` minus the
// styling and frame.
func helpDebugString(m Model, screen views.Screen) string {
	bindings := helpExtractBindings(m, screen)
	rows := make([]string, 0, 1+len(bindings))
	rows = append(rows, "surface="+helpSurfaceLabel(m, screen))
	for _, b := range bindings {
		rows = append(rows, fmt.Sprintf("%s %s", b.key, b.desc))
	}
	return strings.Join(rows, "\n")
}
