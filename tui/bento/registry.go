package bento

// defaultRegistryCapacity matches the six bento slots the cockpit
// renders today (projects/overview/metrics + cicd/logs/topology).
// Pre-allocating that capacity avoids the first three append() growths
// during the hot path of every dashboard frame.
const defaultRegistryCapacity = 6

// Registry collects [BentoTile] instances in insertion order. It is the
// thin glue between view.go (which knows how to build tiles from the
// model) and the engine (which knows how to compose them).
//
// The registry is intentionally not a map keyed by [Slot]: callers may
// want to register multiple tiles per slot in future sprints (e.g. a
// secondary metrics tile under UltraPlus). The engine handles routing.
type Registry struct {
	tiles []BentoTile
}

// NewRegistry returns an empty registry ready to accept tiles.
func NewRegistry() *Registry {
	return &Registry{tiles: make([]BentoTile, 0, defaultRegistryCapacity)}
}

// Register appends a tile. nil tiles are silently dropped so callers can
// `Register(maybeBuildTile())` without checking for empty values.
func (r *Registry) Register(tile BentoTile) {
	if tile == nil {
		return
	}
	r.tiles = append(r.tiles, tile)
}

// Tiles returns the registered tiles in insertion order. The returned
// slice is a shallow copy; mutating it does not affect the registry.
func (r *Registry) Tiles() []BentoTile {
	out := make([]BentoTile, len(r.tiles))
	copy(out, r.tiles)
	return out
}
