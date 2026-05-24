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
