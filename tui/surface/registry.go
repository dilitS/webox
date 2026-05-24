package surface

import "sync"

// Registry is a process-wide lookup table mapping opaque state keys to
// surface implementations. It exists so `tui.View` can transparently
// delegate to the new Surface contract for migrated states while the
// rest of the legacy switch keeps working unchanged.
//
// The registry is safe for concurrent use; in practice the TUI is
// single-threaded so the mutex only guards init-time registration vs.
// rendering. Registration happens in `tui.init()` (or per-surface
// init() in their own packages) before the Bubble Tea program starts.
type Registry struct {
	mu       sync.RWMutex
	surfaces map[string]Surface
}

// NewRegistry returns an empty registry. Most callers want the
// process-wide [Default] registry instead.
func NewRegistry() *Registry {
	return &Registry{surfaces: make(map[string]Surface)}
}

// Register associates a state key with a Surface implementation. The
// state key is the string form of `tui.StateType` (we cannot import
// `tui` here without a cycle, so the caller is responsible for
// stringifying — see `tui.registerSurfaces`). Overwriting an existing
// registration is allowed for tests; production code should panic if
// a state is registered twice, but we keep the policy at the call
// site instead of baking it into the registry.
func (r *Registry) Register(stateKey string, s Surface) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.surfaces[stateKey] = s
}

// Lookup returns the surface registered for the state key, or nil if
// no surface has been registered yet. `tui.View` interprets nil as
// "fall back to the legacy renderRootBody switch" so partial
// migrations are safe.
func (r *Registry) Lookup(stateKey string) Surface {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.surfaces[stateKey]
}

// Reset clears every registration. Test-only helper — production code
// must never call this because surfaces are typically registered in
// `init()` and resetting them mid-flight breaks live cockpits.
func (r *Registry) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.surfaces = make(map[string]Surface)
}
