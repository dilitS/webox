package bento

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/dilitS/webox/tui/components"
	"github.com/dilitS/webox/tui/theme"
)

// tinyFallbackPaddingY/X size the warning panel rendered when the
// terminal is below the Standard threshold. Named constants document
// the intent and silence golangci-lint's mnd rule.
const (
	tinyFallbackPaddingY = 1
	tinyFallbackPaddingX = 2
	defaultStandardWidth = 100
)

// Engine renders a slice of [BentoTile] into a single multi-line string.
//
// The engine is stateless: each call to Render rebuilds the layout from
// the latest tile snapshot. Callers should re-create tiles every frame
// rather than mutating in place (matches Bubble Tea's MVU pattern).
type Engine struct {
	title string
	tiles []BentoTile
}

// NewEngine returns an engine pre-loaded with `tiles`. The title appears
// in the header bar above the grid.
func NewEngine(title string, tiles []BentoTile) *Engine {
	return &Engine{
		title: title,
		tiles: append([]BentoTile(nil), tiles...),
	}
}

// Render composes the cockpit for the given viewport. The width/height
// arguments come from Bubble Tea's WindowSizeMsg and are passed through
// to [DetectMode]. Tiny terminals get the resize warning; larger ones
// get the Ultra grid (with the UltraPlus variant adding a deep-dive
// strip).
func (e *Engine) Render(width, height int) string {
	return e.RenderMode(width, height, DetectMode(width, height))
}

// RenderMode behaves like [Render] but skips viewport detection and
// renders the explicitly requested mode. Callers use this entrypoint
// when a [Resolve] override pinned the cockpit to a tier that does not
// match the raw viewport (e.g. `WEBOX_LAYOUT=tiny` on a 4K monitor).
func (e *Engine) RenderMode(width, height int, mode Mode) string {
	switch mode {
	case ModeTiny:
		return renderTinyFallback(width, height)
	case ModeStandard:
		return e.renderStandardFallback(width)
	case ModeUltraPlus, ModeUltra:
		return e.renderUltraGrid(width, height, mode)
	default:
		return e.renderUltraGrid(width, height, ModeUltra)
	}
}

// renderTinyFallback emits a single warning panel telling the operator
// the cockpit cannot fit. It deliberately mentions the recommended
// terminal size so the user knows what to aim for.
func renderTinyFallback(width, height int) string {
	tokens := theme.Default()
	lines := []string{
		"Terminal too small for cockpit.",
		"",
		fmt.Sprintf("Current viewport: %dx%d", width, height),
		"Minimum size:    100x30",
		"Bento Ultra:     120x35",
		"Bento Ultra+:    160x45",
		"",
		"Resize the window, then press [r] to redraw.",
		"",
		"[Tiny fallback active]",
	}
	body := strings.Join(lines, "\n")

	return lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(lipgloss.Color(tokens.Warning)).
		Padding(tinyFallbackPaddingY, tinyFallbackPaddingX).
		Foreground(lipgloss.Color(tokens.TextBright)).
		Render(body)
}

// renderStandardFallback is reached only when callers ignore the
// `DetectMode == Standard` short-circuit in view.go. It produces a
// stacked-tile silhouette so the engine never returns an empty string.
func (e *Engine) renderStandardFallback(width int) string {
	if width <= 0 {
		width = defaultStandardWidth
	}

	header := renderHeader(e.title, ModeStandard, width)
	if len(e.tiles) == 0 {
		return header
	}

	rendered := make([]string, 0, len(e.tiles))
	for _, tile := range e.tiles {
		rendered = append(rendered, tile.Render(ModeStandard, false))
	}
	return lipgloss.JoinVertical(lipgloss.Left, append([]string{header}, rendered...)...)
}

// renderUltraGrid arranges the registered tiles into a 2x3 grid. Tiles
// are routed by [Slot]; any tile whose slot is not represented in the
// grid is appended as a deep-dive row underneath. UltraPlus widens the
// header band and emits an extra hint strip.
func (e *Engine) renderUltraGrid(width, _ int, mode Mode) string {
	if width <= 0 {
		width = ultraMinWidth
	}

	header := renderHeader(e.title, mode, width)

	bySlot := indexTilesBySlot(e.tiles)

	topRow := joinRow(
		width, mode,
		bySlot[SlotProjects],
		bySlot[SlotOverview],
		bySlot[SlotMetrics],
	)
	bottomRow := joinRow(
		width, mode,
		bySlot[SlotCICD],
		bySlot[SlotLogs],
		bySlot[SlotTopology],
	)

	sections := []string{header, topRow, bottomRow}
	if mode == ModeUltraPlus {
		sections = append(sections, renderDeepDiveStrip(width))
	}
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// indexTilesBySlot turns the linear tile slice into a slot lookup. When
// two tiles claim the same slot, the last registration wins (matches the
// "last writer" semantics view.go relies on for tile overrides).
func indexTilesBySlot(tiles []BentoTile) map[Slot]BentoTile {
	out := make(map[Slot]BentoTile, len(tiles))
	for _, tile := range tiles {
		out[tile.Slot()] = tile
	}
	return out
}

// joinRow renders up to three tiles side-by-side, distributing the
// available width evenly. nil tiles get replaced by an empty filler so
// the grid keeps its 3-column rhythm even when a slot is unregistered.
//
// totalWidth is accepted (and currently unused) so future sprints can
// implement explicit per-column sizing without changing the call sites.
func joinRow(_ int, mode Mode, tiles ...BentoTile) string {
	if len(tiles) == 0 {
		return ""
	}

	parts := make([]string, 0, len(tiles))
	for _, tile := range tiles {
		if tile == nil {
			parts = append(parts, emptyTilePlaceholder())
			continue
		}
		parts = append(parts, tile.Render(mode, false))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

// emptyTilePlaceholder returns a neutral filler used when a slot has no
// registered tile (e.g. during early sprints before the cell is wired).
func emptyTilePlaceholder() string {
	tokens := theme.Default()
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(tokens.Muted)).
		Padding(0, 1).
		Render(lipgloss.NewStyle().
			Foreground(lipgloss.Color(tokens.TextDim)).
			Render("[empty]"))
}

// renderHeader composes the gradient title bar and the mode badge. We
// delegate to [components.RenderHeaderBar] so every cockpit surface
// shares the same gradient + pill silhouette.
func renderHeader(title string, mode Mode, width int) string {
	label := mode.String()
	if mode == ModeUltraPlus {
		label = "Ultra+"
	}
	return components.RenderHeaderBar(components.HeaderBarOptions{
		Title: title,
		Badge: components.FormatModeBadge(label),
		Width: width,
		Theme: theme.Default(),
	})
}

// renderDeepDiveStrip is the UltraPlus-only footer that hints at the
// extra real estate. Sprint 11 will populate it with the topology
// timeline; for now we render a muted spacer so the silhouette is
// visibly different from plain Ultra.
//
// width is currently unused but kept in the signature so Sprint 11 can
// honour the viewport without churning every caller.
func renderDeepDiveStrip(_ int) string {
	tokens := theme.Default()
	body := "[Deep-dive strip] Reserved for service timelines (Sprint 11+)"
	style := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(lipgloss.Color(tokens.Muted)).
		Foreground(lipgloss.Color(tokens.TextDim)).
		Padding(0, 1)
	return style.Render(body)
}
