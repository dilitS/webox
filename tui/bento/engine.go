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
	ultraLeftMinWidth     = 40
	ultraRightMinWidth    = 46
)

// Engine renders a slice of [BentoTile] into a single multi-line string.
//
// The engine is stateless: each call to Render rebuilds the layout from
// the latest tile snapshot. Callers should re-create tiles every frame
// rather than mutating in place (matches Bubble Tea's MVU pattern).
type Engine struct {
	title         string
	statusBar     string
	tiles         []BentoTile
	focusedSlot   *Slot
	scrollOffsets map[Slot]int
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

// WithFocus marks one slot as the currently focused tile. The engine
// renders that tile with `focused=true` so the renderer can swap the
// thick border for a double-line border (visual indicator).
//
// Sprint 14 TASK-14.2 wires this from the cockpit's Tab / Shift+Tab
// keyboard router so the operator can rotate focus across scrollable
// tiles without leaving the dashboard.
func (e *Engine) WithFocus(slot Slot) *Engine {
	s := slot
	e.focusedSlot = &s
	return e
}

// WithTileScrollOffsets injects the per-slot scroll offsets the
// cockpit owns on the [tui.Model]. The engine forwards each offset
// to the matching [ScrollableTile] before rendering so the visible
// window reflects the operator's PgUp / PgDn input. Slots without an
// offset entry render at offset 0 (newest content).
func (e *Engine) WithTileScrollOffsets(offsets map[Slot]int) *Engine {
	if len(offsets) == 0 {
		e.scrollOffsets = nil
		return e
	}
	cp := make(map[Slot]int, len(offsets))
	for k, v := range offsets {
		cp[k] = v
	}
	e.scrollOffsets = cp
	return e
}

// scrollOffsetAware is implemented by tiles that accept an injected
// scroll offset. Kept package-private because the public surface is
// the [ScrollableTile] interface; this internal helper only exists so
// [Engine.renderTileWithFocus] can apply the model's offset before
// calling Render. Tiles get the capability for free by exposing a
// `WithScrollOffset(offset int) BentoTile` method (mirrors
// [WidthAware]).
type scrollOffsetAware interface {
	BentoTile
	WithScrollOffset(offset int) BentoTile
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
//
// Sprint 20 — the previous "press [r] to redraw" instruction was
// misleading: Bubble Tea auto-emits a `tea.WindowSizeMsg` on every
// resize, so no manual key press is needed. The `r` key was never
// wired up at the global level either. The current copy tells the
// operator the truth: just resize, the cockpit re-renders itself.
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
		"Resize the window — the cockpit re-renders automatically.",
		"Press [q] or [Ctrl+C] to quit.",
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

// renderUltraGrid composes the cockpit using the Sprint 13 responsive
// layout:
//
//	┌────────── status bar (full width) ───────────┐
//	│                                              │
//	│ ┌──────────┐ ┌──────────────────────────┐    │
//	│ │ Projects │ │      Server tile         │    │
//	│ ├──────────┤ ├──────────────────────────┤    │
//	│ │ Topology │ │      CI/CD tile          │    │
//	│ └──────────┘ └──────────────────────────┘    │
//	│ ┌──────────────────────────────────────┐     │
//	│ │           Live Server Logs           │     │
//	│ └──────────────────────────────────────┘     │
//	└──────────────────────────────────────────────┘
//
// The grid is height-aware as of Sprint 13: when the engine knows the
// viewport's height it carves out budgets for each row (status bar →
// top row → second row → logs row → optional UltraPlus strip) and
// asks tiles that exceed their budget to surface a scroll indicator
// instead of pushing every other tile down or padding short siblings
// with empty whitespace. This is what makes the right-hand column
// responsive: the previous "equalize-to-max" strategy left dead space
// under short CI/CD pipelines whenever the topology graph was taller.
//
// UltraPlus appends a deep-dive strip below the logs for the optional
// `≥160×45` tier.
func (e *Engine) renderUltraGrid(width, height int, mode Mode) string {
	if width <= 0 {
		width = ultraMinWidth
	}

	bySlot := indexTilesBySlot(e.tiles)

	const (
		tileBorderOverhead       = 2
		ratioDenominator         = 100
		compactLeftRatio         = 46
		mediumLeftRatio          = 42
		wideLeftRatio            = 38
		mediumViewportBreakpoint = 136
		wideViewportBreakpoint   = 160
	)
	leftRatio := compactLeftRatio
	switch {
	case width >= wideViewportBreakpoint:
		leftRatio = wideLeftRatio
	case width >= mediumViewportBreakpoint:
		leftRatio = mediumLeftRatio
	}
	leftCol := (width * leftRatio) / ratioDenominator
	if leftCol < ultraLeftMinWidth {
		leftCol = ultraLeftMinWidth
	}
	maxLeft := width - ultraRightMinWidth
	if leftCol > maxLeft {
		leftCol = maxLeft
	}
	rightCol := width - leftCol

	budget := planRowBudgets(height, mode)

	projects := e.renderSlot(bySlot[SlotProjects], mode, leftCol-tileBorderOverhead)
	overview := e.renderSlot(bySlot[SlotOverview], mode, rightCol-tileBorderOverhead)
	projects = clipTileBlock(projects, budget.TopRow)
	overview = clipTileBlock(overview, budget.TopRow)
	projects, overview = equalizeBlockHeights(projects, overview)
	topRow := lipgloss.JoinHorizontal(lipgloss.Top, projects, overview)

	topology := e.renderSlot(bySlot[SlotTopology], mode, leftCol-tileBorderOverhead)
	cicd := e.renderSlot(bySlot[SlotCICD], mode, rightCol-tileBorderOverhead)
	topology = clipTileBlock(topology, budget.SecondRow)
	cicd = clipTileBlock(cicd, budget.SecondRow)
	topology, cicd = equalizeBlockHeights(topology, cicd)
	secondRow := lipgloss.JoinHorizontal(lipgloss.Top, topology, cicd)

	logsRow := e.renderSlot(bySlot[SlotLogs], mode, width-tileBorderOverhead)
	logsRow = clipTileBlock(logsRow, budget.Logs)

	// Capacity: status bar + top row + second row + logs row.
	// Sprint 20 — the UltraPlus "deep dive" footer strip used to
	// add a `[Deep-dive strip] Reserved for Sprint 11+` placeholder
	// line. Sprint 11 has long since shipped; the placeholder was
	// dead weight that consumed two precious viewport rows. The
	// extra 4K real estate now flows into the live-log row via
	// [bentoLogsTargetUltraPlus].
	const maxSections = 4
	sections := make([]string, 0, maxSections)
	if e.statusBar != "" {
		sections = append(sections, e.statusBar)
	} else {
		sections = append(sections, renderHeader(e.title, mode, width))
	}
	sections = append(sections, topRow, secondRow, logsRow)
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// rowBudget captures the per-row max-line allowance the engine hands
// to clipTileBlock. A zero value means "no clip" so callers without a
// known viewport height (older tests, fallback paths) preserve the
// pre-Sprint-13 unbounded behaviour.
type rowBudget struct {
	TopRow    int
	SecondRow int
	Logs      int
}

// Height accounting constants kept named so the planner reads top-down.
const (
	bentoStatusBarLines = 1
	// bentoLogsTargetUltra is the preferred number of lines we want
	// the live-log row to occupy in Ultra (status bar already
	// subtracted). It must be large enough to show a meaningful
	// burst of log lines (≥ 6 visible rows + chrome) but small
	// enough that the 2x2 grid above can still breathe.
	bentoLogsTargetUltra     = 10
	bentoLogsTargetUltraPlus = 14
	bentoLogsMinLines        = 7
	// bentoMinRowLines is the smallest sensible per-row budget. Tile
	// chrome itself eats 3 lines (top border + header + bottom
	// border); going below 6 would leave nothing scrollable.
	bentoMinRowLines = 6
	// bentoGridRows reflects the two stacked tile rows in the Ultra
	// grid (top: Projects+Server, second: Topology+CI/CD). We never
	// allocate fewer than this many rows, so it doubles as a divisor
	// when sharing height between them and as a floor on `available`.
	bentoGridRows = 2
)

// planRowBudgets divides the available vertical space between the
// three cockpit rows. When height is unknown (≤ 0) every budget is
// zero, which clipTileBlock interprets as "no clip" — that matches
// every pre-Sprint-13 caller path (tests, narrow legacy fallbacks).
//
// Sprint 20 — the UltraPlus deep-dive strip is gone, so the planner
// no longer reserves two trailing rows for it. The freed lines flow
// into the live-log target ([bentoLogsTargetUltraPlus]), which is
// the most useful place for them on a 160×45+ viewport.
func planRowBudgets(height int, mode Mode) rowBudget {
	if height <= 0 {
		return rowBudget{}
	}

	logs := bentoLogsTargetUltra
	if mode == ModeUltraPlus {
		logs = bentoLogsTargetUltraPlus
	}

	available := height - bentoStatusBarLines - logs
	if available < bentoGridRows*bentoMinRowLines {
		// Shrink the logs row first — the operator can still tail
		// via the Live Logs tab if the bottom row collapses.
		logs = height - bentoStatusBarLines - bentoGridRows*bentoMinRowLines
		if logs < bentoLogsMinLines {
			logs = bentoLogsMinLines
		}
		available = bentoGridRows * bentoMinRowLines
	}

	perRow := available / bentoGridRows
	if perRow < bentoMinRowLines {
		perRow = bentoMinRowLines
	}

	return rowBudget{
		TopRow:    perRow,
		SecondRow: perRow,
		Logs:      logs,
	}
}

// clipTileBlock trims a rendered tile to maxLines while preserving its
// top/bottom borders and the header row. When the tile already fits,
// the function is a no-op. When clipping happens, the penultimate body
// line is replaced with a discreet `… +N more lines` indicator so the
// operator knows the content is truncated — and where to use the
// per-tile scroll affordance (currently the Live Logs tab for tail
// data; topology scroll keys land in Sprint 14).
//
// Sprint 14 TASK-14.7 — the function used to count rows manually
// against magic constants (`borderRows = 2`, `bordersAndHeader = 3`).
// It now parses the rendered string into a structured [TileBlock]
// and defers to [clipBlock]. The magic constants are gone; the
// arithmetic happens in one place against typed lanes.
//
// The wrapper is preserved so existing callers (engine + tests)
// keep working unchanged. Future tile authors SHOULD implement the
// [BlockRenderer] interface directly so the parse step disappears.
func clipTileBlock(rendered string, maxLines int) string {
	if maxLines <= 0 {
		return rendered
	}
	if strings.Count(rendered, "\n") < maxLines {
		return rendered
	}
	block := parseTileBlock(rendered)
	return clipBlock(block, maxLines).Render()
}

// framedIndicatorLine returns a single line that mimics the tile's
// `┃ … ┃` body row: thick border glyphs on both sides, accent-coloured,
// with a dim/faint payload in the middle. The function returns the
// fully-styled string ready to be joined into the output slice.
func framedIndicatorLine(tileWidth int, payload string, tokens theme.Theme) string {
	const (
		sideGlyphs = 2 // left + right `┃`
		minInner   = 2 // a single padded character on each side
	)
	inner := tileWidth - sideGlyphs
	if inner < minInner {
		inner = minInner
	}
	bodyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(tokens.TextDim)).
		Faint(true).
		Width(inner).
		Padding(0, 1)
	borderStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(tokens.Primary))
	return borderStyle.Render("┃") + bodyStyle.Render(payload) + borderStyle.Render("┃")
}

func equalizeBlockHeights(left, right string) (leftPadded, rightPadded string) {
	maxHeight := lipgloss.Height(left)
	if rightHeight := lipgloss.Height(right); rightHeight > maxHeight {
		maxHeight = rightHeight
	}
	pad := func(block string) string {
		return lipgloss.NewStyle().Height(maxHeight).Render(block)
	}
	return pad(left), pad(right)
}

// WidthAware is the optional capability tiles can implement to learn
// the column width the bento engine has allocated for them. Tiles that
// do not implement it fall back to their natural, content-dictated
// width — the legacy behaviour kept for compatibility with tests that
// pre-date the 2026-05-24 design refresh.
type WidthAware interface {
	WithWidth(int) BentoTile
}

// renderSlot is the focus-aware / scroll-aware tile renderer. It
// applies, in order:
//
//  1. The supplied column width via [WidthAware].
//  2. The model-owned scroll offset (if any) via the package-private
//     `scrollOffsetAware` shim — only [ScrollableTile] tiles also
//     expose `WithScrollOffset` so non-scrollable tiles bypass the
//     branch silently.
//  3. The focus flag so the tile picks up the double-line border.
//
// Tiles that implement none of the optional interfaces fall back to
// the natural-size, unfocused render.
func (e *Engine) renderSlot(tile BentoTile, mode Mode, width int) string {
	if tile == nil {
		return emptyTilePlaceholder()
	}
	if width > 0 {
		if w, ok := tile.(WidthAware); ok {
			tile = w.WithWidth(width)
		}
	}
	if e.scrollOffsets != nil {
		if offset, has := e.scrollOffsets[tile.Slot()]; has && offset > 0 {
			if s, ok := tile.(scrollOffsetAware); ok {
				tile = s.WithScrollOffset(offset)
			}
		}
	}
	focused := e.focusedSlot != nil && *e.focusedSlot == tile.Slot()
	return tile.Render(mode, focused)
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

// renderDeepDiveStrip — REMOVED in Sprint 20. The UltraPlus tier
// used to render a "Reserved for Sprint 11+" placeholder strip
// below the live-log row. Sprint 11 has long since shipped (live
// log stream + topology map both live in the main grid), so the
// strip carried no information. The freed two rows now flow into
// the live-log target via [bentoLogsTargetUltraPlus]. If a future
// sprint wants to reintroduce a deep-dive strip it should land
// behind a feature flag plus a fresh ADR; the previous incarnation
// was UX dead weight.
