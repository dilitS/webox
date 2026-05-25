package presets

import (
	"fmt"
	"sort"
	"sync"
)

// Registry is the read-only runtime API to the preset catalog. It
// exposes deterministic ordering, id lookup, provider-type and
// region filtering, and per-load error metadata for `webox doctor
// preset` reporting.
//
// Registry is safe for concurrent use after construction. It does
// not provide a mutation API: in v0.2 baseline the catalog is
// build-time material, ratified by ADR-0008. Future filesystem
// presets (post-RFC) would extend Registry with a refresh path.
type Registry struct {
	mu       sync.RWMutex
	presets  []*Preset
	byID     map[string]*Preset
	loadErrs map[string]error
}

// NewRegistryFromResult constructs a Registry from a LoadResult.
// Callers obtain LoadResult via [LoadEmbedded] (production) or
// [LoadFrom] (tests / future filesystem path). Construction is
// trivial and never returns an error — schema-level failures live
// inside res.Errors.
func NewRegistryFromResult(res LoadResult) *Registry {
	r := &Registry{
		presets:  make([]*Preset, len(res.Presets)),
		byID:     make(map[string]*Preset, len(res.Presets)),
		loadErrs: make(map[string]error, len(res.Errors)),
	}
	copy(r.presets, res.Presets)
	for _, p := range res.Presets {
		r.byID[p.ID] = p
	}
	for k, v := range res.Errors {
		r.loadErrs[k] = v
	}
	sort.Slice(r.presets, func(i, j int) bool {
		return r.presets[i].ID < r.presets[j].ID
	})
	return r
}

// List returns a copy of all loaded presets, sorted by id. The
// returned slice is independent: callers may sort, filter, or
// truncate it without affecting the registry.
func (r *Registry) List() []*Preset {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Preset, len(r.presets))
	copy(out, r.presets)
	return out
}

// Get returns the preset registered under id. It returns
// [ErrPresetNotFound] (wrapped via fmt.Errorf so callers preserve
// the id context) when no such preset exists.
func (r *Registry) Get(id string) (*Preset, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.byID[id]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrPresetNotFound, id)
	}
	return p, nil
}

// ByProviderType returns the subset of presets whose provider_type
// matches providerType. The result is sorted: verified presets
// first, then candidate, then research, then community, with
// deprecated last; ties broken by id ASC. Callers in Provider
// Catalog rely on this ordering for predictable rendering.
func (r *Registry) ByProviderType(providerType string) []*Preset {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var matches []*Preset
	for _, p := range r.presets {
		if p.ProviderType == providerType {
			matches = append(matches, p)
		}
	}
	sortByStatusThenID(matches)
	return matches
}

// ByRegion returns the subset of presets whose Region() matches
// region (one of presets.RegionPoland / RegionEurope / RegionGlobal
// / RegionAdvanced). Sorting matches ByProviderType (status, then
// id).
func (r *Registry) ByRegion(region string) []*Preset {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var matches []*Preset
	for _, p := range r.presets {
		if p.Region() == region {
			matches = append(matches, p)
		}
	}
	sortByStatusThenID(matches)
	return matches
}

// Regions returns the set of region tags present in the registry,
// sorted in display order: Poland → Europe → Global → Advanced.
// Empty regions (no presets) are omitted so the Provider Catalog
// never shows an empty group header.
func (r *Registry) Regions() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	seen := map[string]struct{}{}
	for _, p := range r.presets {
		seen[p.Region()] = struct{}{}
	}
	order := []string{RegionPoland, RegionEurope, RegionGlobal, RegionAdvanced}
	out := make([]string, 0, len(order))
	for _, region := range order {
		if _, ok := seen[region]; ok {
			out = append(out, region)
		}
	}
	return out
}

// LoadErrors returns a copy of the per-file errors recorded when
// the registry was constructed. Used by `webox doctor preset` to
// surface schema drift in CI logs without aborting the registry
// load.
func (r *Registry) LoadErrors() map[string]error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]error, len(r.loadErrs))
	for k, v := range r.loadErrs {
		out[k] = v
	}
	return out
}

// Count returns the number of valid presets in the registry.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.presets)
}

// statusRank assigns a stable ordering for status values so
// Provider Catalog renders verified presets first. Lower rank
// renders earlier.
const (
	rankVerified   = 0
	rankCandidate  = 1
	rankResearch   = 2
	rankCommunity  = 3
	rankDeprecated = 4
	rankUnknown    = 5
)

func statusRank(s Status) int {
	switch s {
	case StatusVerified:
		return rankVerified
	case StatusCandidate:
		return rankCandidate
	case StatusResearch:
		return rankResearch
	case StatusCommunity:
		return rankCommunity
	case StatusDeprecated:
		return rankDeprecated
	default:
		return rankUnknown
	}
}

func sortByStatusThenID(in []*Preset) {
	sort.Slice(in, func(i, j int) bool {
		if statusRank(in[i].Status) != statusRank(in[j].Status) {
			return statusRank(in[i].Status) < statusRank(in[j].Status)
		}
		return in[i].ID < in[j].ID
	})
}

// Default returns the process-wide registry, lazily initialised
// from the embedded preset catalog on first call. Subsequent
// calls return the same *Registry instance. The lazy
// initialisation is wrapped in sync.Once so concurrent first
// callers cooperate cleanly.
//
// Default returns the registry pointer and a non-nil error when
// the embedded catalog is unreadable (catastrophic — should not
// happen post-build). Per-file schema drift is reported via
// Registry.LoadErrors() and does not propagate as an error here.
func Default() (*Registry, error) {
	defaultOnce.Do(func() {
		res, err := LoadEmbedded()
		if err != nil {
			defaultErr = err
			return
		}
		defaultReg = NewRegistryFromResult(res)
	})
	return defaultReg, defaultErr
}

// MustDefault is the shortcut for call sites that treat a missing
// registry as a panic-worthy programming error (TUI surface
// bootstrap, CLI subcommand init). Returns the registry value of
// [Default] or panics on error.
func MustDefault() *Registry {
	r, err := Default()
	if err != nil {
		panic(fmt.Sprintf("presets: Default() failed: %v", err))
	}
	return r
}

var (
	defaultOnce sync.Once
	defaultReg  *Registry
	defaultErr  error
)
