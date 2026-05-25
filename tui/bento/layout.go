package bento

// Rect is a logical, in-cell screen rectangle for a bento slot.
//
// Coordinates are zero-indexed cells from the top-left of the bento
// engine's rendered output (NOT the full terminal — `view.go` adds
// chrome on top of the bento body). The rectangle is half-open on
// both axes:
//
//	contains-set = [X, X+Width) × [Y, Y+Height)
//
// so two adjacent slots never both claim the same border cell. That
// matches how Bubble Tea reports `tea.MouseMsg{X, Y}` (cells, not
// pixels) and how `lipgloss` measures rendered widths.
type Rect struct {
	X, Y, Width, Height int
}

// Empty reports whether the rectangle is degenerate (zero or
// negative size). Hit-testing skips empty rects so a slot that is
// not present in the current viewport tier never wrongly claims
// a click.
func (r Rect) Empty() bool {
	return r.Width <= 0 || r.Height <= 0
}

// Contains reports whether (x, y) falls inside the half-open
// rectangle.
func (r Rect) Contains(x, y int) bool {
	if r.Empty() {
		return false
	}
	return x >= r.X && x < r.X+r.Width && y >= r.Y && y < r.Y+r.Height
}

// LayoutMap reports where each registered slot lives in the
// rendered cockpit so the cockpit's mouse router can resolve a
// click coordinate into a [Slot] without parsing the rendered
// string.
//
// The map is intentionally a flat snapshot rather than a full
// scene graph: the bento engine plans the grid deterministically
// from `width`, `height`, and the active [Mode] (see
// [planRowBudgets] + the column-ratio helpers in [Engine]), so the
// model can recompute it on every mouse press without re-parsing
// the rendered output. Empty rects mean the slot is not laid out
// in the current tier (e.g. Topology / CICD / Logs are not
// rendered in Tiny / Standard fallbacks).
type LayoutMap struct {
	// StatusBar is the one-line header strip painted above the
	// grid (composed by [components.RenderStatusBar]). Empty when
	// the active surface manages its own chrome (Tiny fallback).
	StatusBar Rect
	// Slots maps each registered bento [Slot] to the rectangle it
	// occupies. Absent or empty entries indicate the slot is not
	// rendered in the current viewport tier.
	Slots map[Slot]Rect
}

// SlotAt returns the slot whose rectangle contains (x, y), if any.
//
// The status bar reports as `(0, false)` (no Slot value); callers
// that need to detect status-bar clicks should check
// [LayoutMap.StatusBar] directly. The status bar uses Y == 0 so a
// click on row 0 always means "status bar".
//
// The lookup is O(slots) — fine for the six-slot bento today, and
// fine even if a future tier adds a handful more. We deliberately
// do not maintain a sorted index: the slot count is bounded by
// [defaultRegistryCapacity] and the map is rebuilt every frame.
func (m LayoutMap) SlotAt(x, y int) (Slot, bool) {
	for slot, rect := range m.Slots {
		if rect.Contains(x, y) {
			return slot, true
		}
	}
	return 0, false
}

// ComputeLayout returns the [LayoutMap] that mirrors the slot
// rectangles [Engine.RenderMode] would produce for the same
// (width, height, mode) triple. It is deterministic, side-effect
// free, and does NOT actually render any tile string — the cockpit
// can call it from `Update` (the pure-MVU side) to resolve a mouse
// click into a slot without invoking the View pipeline.
//
// The function reuses the column-ratio constants and
// [planRowBudgets] helper from [Engine.renderUltraGrid] so the
// returned rectangles track the rendered cockpit on a 1:1 basis.
// Tests probe every interior cell to guard against drift.
func (e *Engine) ComputeLayout(width, height int, mode Mode) LayoutMap {
	switch mode {
	case ModeTiny:
		return LayoutMap{}
	case ModeStandard:
		return computeStandardLayout(width, height)
	case ModeUltraPlus, ModeUltra:
		return computeUltraLayout(width, height, mode)
	default:
		return computeUltraLayout(width, height, ModeUltra)
	}
}

// computeUltraLayout mirrors [Engine.renderUltraGrid]'s width / height
// arithmetic so the returned rectangles describe the same cells the
// renderer paints. The layout is composed top-down so debugging is
// straightforward:
//
//	┌──────── status bar (y=0, h=1) ────────────────┐
//	│ ┌────────┐ ┌─────────────────────┐  TopRow    │
//	│ │Projects│ │      Overview       │            │
//	│ └────────┘ └─────────────────────┘            │
//	│ ┌────────┐ ┌─────────────────────┐  SecondRow │
//	│ │Topology│ │       CI/CD         │            │
//	│ └────────┘ └─────────────────────┘            │
//	│ ┌──────────────────────────────────┐ Logs row │
//	│ │           Live Logs              │          │
//	│ └──────────────────────────────────┘          │
//	└────────────────────────────────────────────────┘
func computeUltraLayout(width, height int, mode Mode) LayoutMap {
	if width <= 0 {
		width = ultraMinWidth
	}

	const (
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
	if budget.TopRow == 0 || budget.SecondRow == 0 || budget.Logs == 0 {
		// Height not provided / too small to plan budgets — fall
		// back to a coarse split so the layout is still useful for
		// hit-testing the projects column.
		budget = rowBudget{
			TopRow:    bentoMinRowLines,
			SecondRow: bentoMinRowLines,
			Logs:      bentoLogsMinLines,
		}
	}

	statusY := 0
	const statusHeight = 1
	topY := statusY + statusHeight
	secondY := topY + budget.TopRow
	logsY := secondY + budget.SecondRow

	slots := map[Slot]Rect{
		SlotProjects: {X: 0, Y: topY, Width: leftCol, Height: budget.TopRow},
		SlotOverview: {X: leftCol, Y: topY, Width: rightCol, Height: budget.TopRow},
		SlotTopology: {X: 0, Y: secondY, Width: leftCol, Height: budget.SecondRow},
		SlotCICD:     {X: leftCol, Y: secondY, Width: rightCol, Height: budget.SecondRow},
		SlotLogs:     {X: 0, Y: logsY, Width: width, Height: budget.Logs},
	}

	return LayoutMap{
		StatusBar: Rect{X: 0, Y: statusY, Width: width, Height: statusHeight},
		Slots:     slots,
	}
}

// computeStandardLayout describes the 100×30 fallback. The Standard
// fallback used to be a stacked-tile silhouette; Sprint 20 TASK-20.3
// will replace it with a proper mini-bento, but the layout map can
// already publish coarse rectangles so a click on the projects band
// drills the operator into the project detail. We allocate two
// horizontal bands (projects on top, overview on the bottom) under
// the status bar; richer slots (CI/CD + logs) collapse into the
// overview band until the mini-bento lands.
func computeStandardLayout(width, height int) LayoutMap {
	if width <= 0 {
		width = defaultStandardWidth
	}
	if height <= 0 {
		height = defaultStandardHeight
	}
	const (
		statusHeight  = 1
		minBandRows   = 2
		standardBands = 2 // projects band + overview band
	)
	available := height - statusHeight
	if available < minBandRows {
		available = minBandRows
	}
	projectsHeight := available / standardBands
	if projectsHeight < 1 {
		projectsHeight = 1
	}
	overviewHeight := available - projectsHeight
	slots := map[Slot]Rect{
		SlotProjects: {X: 0, Y: statusHeight, Width: width, Height: projectsHeight},
		SlotOverview: {X: 0, Y: statusHeight + projectsHeight, Width: width, Height: overviewHeight},
	}
	return LayoutMap{
		StatusBar: Rect{X: 0, Y: 0, Width: width, Height: statusHeight},
		Slots:     slots,
	}
}

// defaultStandardHeight matches the lower bound of the Standard
// tier. We use it as a safety floor when callers pass a zero
// height (older tests, fallback paths).
const defaultStandardHeight = 30
