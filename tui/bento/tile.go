package bento

// Slot enumerates the logical positions a tile can occupy in the Ultra
// grid. The engine routes tiles by Slot rather than by index so callers
// can register them in any order.
type Slot int

const (
	// SlotProjects is the top-left projects list.
	SlotProjects Slot = iota
	// SlotOverview is the top-right project overview pane.
	SlotOverview
	// SlotMetrics is the top-right header metrics tile (CPU/RAM/Disk).
	SlotMetrics
	// SlotCICD is the second-row right CI/CD pipeline tile.
	SlotCICD
	// SlotLogs is the full-width logs row below the 2x2 grid.
	SlotLogs
	// SlotTopology is the second-row left service topology tile.
	SlotTopology
)

// BentoTile is the contract every cell in the cockpit must implement.
//
// Implementations MUST be pure functions of their constructor inputs:
// the engine calls Render every frame and must not observe ordering
// effects. A tile that needs live data should snapshot it during
// construction (see how view.go builds tiles each frame from the model).
type BentoTile interface {
	// ID returns a stable identifier used for snapshot diffs and
	// registry deduplication. Tile IDs should be lowercase kebab-case
	// (e.g. "projects", "cicd-pipeline").
	ID() string
	// Slot reports where the tile prefers to live in the Ultra grid.
	// The engine may reposition tiles in Standard/Tiny modes.
	Slot() Slot
	// Render returns the tile body for the given mode. The `focused`
	// flag lets the tile highlight its border or content when the
	// operator has the panel selected.
	Render(mode Mode, focused bool) string
}

// ScrollableTile is the optional capability tiles implement to
// participate in the Sprint 14 TASK-14.2 per-tile scroll routing.
//
// The cockpit lets the operator press `Tab` / `Shift+Tab` to cycle
// focus across scrollable tiles; while one is focused, `PgUp` /
// `PgDn` / `Home` / `End` and the mouse wheel scroll its body
// instead of the global viewport. Non-scrollable tiles (Server
// overview, Topology, Header metrics) deliberately do not implement
// the interface so the focus cycle skips them.
//
// Lifecycle contract:
//
//  1. The cockpit constructs a fresh tile each frame from snapshot
//     data. The tile's offset is captured at construction time
//     via [Engine.WithTileScrollOffsets].
//  2. The engine reads the offset via `ScrollOffset()` to render
//     the visible window.
//  3. When the operator presses a scroll key, the cockpit's Update
//     loop calls `Scroll(delta)` on the freshly built tile and
//     persists the resulting offset on the model so it survives
//     into the next frame's construction.
//
// Implementations MUST clamp the offset to `[0, max]` where `max`
// is the largest offset that still keeps at least one body row
// visible. `Scroll(0)` is a clamp probe used by tests / debug
// tooling and MUST be a no-op for an in-range offset.
type ScrollableTile interface {
	BentoTile
	// Scroll returns a copy of the tile with the offset adjusted
	// by `delta` (positive = scroll forward / down, negative =
	// scroll backward / up). The receiver is NOT mutated; callers
	// inspect the returned tile via `ScrollOffset()` to learn the
	// clamped value.
	Scroll(delta int) ScrollableTile
	// ScrollOffset returns the current offset (0 = top / oldest).
	ScrollOffset() int
}
