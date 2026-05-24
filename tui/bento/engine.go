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
	// ultraProjectsMinWidth is the floor for the Projects column so
	// the longest demo project name (`Dashboard-Admin`) always fits
	// inside the rounded border with room for the selection pill.
	ultraProjectsMinWidth = 28
)

// Engine renders a slice of [BentoTile] into a single multi-line string.
//
// The engine is stateless: each call to Render rebuilds the layout from
// the latest tile snapshot. Callers should re-create tiles every frame
// rather than mutating in place (matches Bubble Tea's MVU pattern).
type Engine struct {
	title     string
	statusBar string
	tiles     []BentoTile
}

// NewEngine returns an engine pre-loaded with `tiles`. The title appears
// in the gradient header inside the cockpit when no status bar is
// supplied.
func NewEngine(title string, tiles []BentoTile) *Engine {
	return &Engine{
		title: title,
		tiles: append([]BentoTile(nil), tiles...),
	}
}

// WithStatusBar attaches a fully rendered status bar string (produced by
// [components.RenderStatusBar]) that the engine paints above the grid.
// Passing the empty string falls back to the legacy gradient header.
func (e *Engine) WithStatusBar(rendered string) *Engine {
	e.statusBar = rendered
	return e
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

// renderUltraGrid composes the cockpit using the 2026-05-24 design
// refresh layout:
//
//	┌────────── status bar (full width) ───────────┐
//	│                                              │
//	│ ┌──────────┐ ┌──────────────────────────┐    │
//	│ │ Projects │ │      Server tile         │    │
//	│ │  (full   │ ├──────────────────────────┤    │
//	│ │ height)  │ │      CI/CD tile          │    │
//	│ └──────────┘ └──────────────────────────┘    │
//	│ ┌──────────────────────────────────────┐     │
//	│ │           Live Server Logs           │     │
//	│ └──────────────────────────────────────┘     │
//	└──────────────────────────────────────────────┘
//
// UltraPlus appends a deep-dive strip below the logs for the optional
// `≥160×45` tier.
func (e *Engine) renderUltraGrid(width, _ int, mode Mode) string {
	if width <= 0 {
		width = ultraMinWidth
	}

	bySlot := indexTilesBySlot(e.tiles)

	// Column proportions follow the reference cockpit at 1024 px:
	// Projects column ≈ 36%, Server/CICD stack ≈ 64%. Each tile then
	// inherits the column width (minus the 2-char rounded border) via
	// the optional `WithWidth` capability so the right column visibly
	// dominates the cockpit, matching the design brief.
	const (
		projectsRatioNumerator   = 36
		projectsRatioDenominator = 100
		tileBorderOverhead       = 2
	)
	projectsCol := (width * projectsRatioNumerator) / projectsRatioDenominator
	if projectsCol < ultraProjectsMinWidth {
		projectsCol = ultraProjectsMinWidth
	}
	rightCol := width - projectsCol
	if rightCol < ultraProjectsMinWidth {
		rightCol = ultraProjectsMinWidth
	}

	projects := renderTileWithWidth(bySlot[SlotProjects], mode, projectsCol-tileBorderOverhead)
	overview := renderTileWithWidth(bySlot[SlotOverview], mode, rightCol-tileBorderOverhead)
	cicd := renderTileWithWidth(bySlot[SlotCICD], mode, rightCol-tileBorderOverhead)

	rightStack := lipgloss.JoinVertical(lipgloss.Left, overview, cicd)
	mainRow := lipgloss.JoinHorizontal(lipgloss.Top, projects, rightStack)
	logsRow := renderTileWithWidth(bySlot[SlotLogs], mode, width-tileBorderOverhead)

	// Capacity: status bar + main row + logs row + optional topology
	// row (Ultra+ only).
	const maxSections = 4
	sections := make([]string, 0, maxSections)
	if e.statusBar != "" {
		sections = append(sections, e.statusBar)
	} else {
		sections = append(sections, renderHeader(e.title, mode, width))
	}
	sections = append(sections, mainRow, logsRow)

	// 2026-05-24 UX refresh: topology tile renders in both Ultra
	// (`120×35`) and Ultra+ (`160×45`). The MVP scope (Sprint 11)
	// makes the topology a first-class cockpit panel rather than an
	// Ultra+-only deep-dive strip. Ultra+ keeps a thin extras strip
	// below the topology for future "service timeline" widgets.
	if topology := bySlot[SlotTopology]; topology != nil {
		sections = append(sections, renderTileWithWidth(topology, mode, width-tileBorderOverhead))
	}
	if mode == ModeUltraPlus {
		sections = append(sections, renderDeepDiveStrip(width))
	}
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// WidthAware is the optional capability tiles can implement to learn
// the column width the bento engine has allocated for them. Tiles that
// do not implement it fall back to their natural, content-dictated
// width — the legacy behaviour kept for compatibility with tests that
// pre-date the 2026-05-24 design refresh.
type WidthAware interface {
	WithWidth(int) BentoTile
}

// renderTileWithWidth tells the tile what column width it has been
// granted, then renders it. When the tile is nil or does not implement
// [WidthAware] we fall back to the natural-width render.
func renderTileWithWidth(tile BentoTile, mode Mode, width int) string {
	if tile == nil {
		return emptyTilePlaceholder()
	}
	if width > 0 {
		if w, ok := tile.(WidthAware); ok {
			return w.WithWidth(width).Render(mode, false)
		}
	}
	return tile.Render(mode, false)
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

// joinRow was the 3-column horizontal joiner used by the legacy 2x3
// bento grid. The 2026-05-24 design refresh replaced it with a
// Projects-left / Server+CICD-right composition rendered directly in
// [Engine.renderUltraGrid].

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
