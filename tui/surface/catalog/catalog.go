// Package catalog hosts the [surface.Surface] for the Sprint 20
// TASK-20.2 read-only Provider Catalog screen.
//
// The screen lives behind the dashboard `p` keybinding and lets
// operators browse the embedded preset registry without leaving
// the cockpit. It is intentionally a leaf surface: no provider
// I/O, no clipboard side-effects (the model owns those), no
// persisted state.
package catalog

import (
	"github.com/dilitS/webox/tui/surface"
	"github.com/dilitS/webox/tui/views"
)

// Surface is the value-typed [surface.Surface] for the Provider
// Catalog screen.
type Surface struct{}

// Body delegates to [views.RenderProviderCatalog].
func (Surface) Body(ctx surface.Context) string {
	return views.RenderProviderCatalog(ctx.Screen)
}

// Crumb labels the surface as "Provider Catalog" so the Sprint
// 13 status-bar crumb is unambiguous.
func (Surface) Crumb(_ surface.Context) string { return "Provider Catalog" }

// Footer surfaces the keys that drive the catalog. The View
// layer appends the global `↕ scroll` indicator dynamically when
// the catalog body overflows.
func (Surface) Footer(_ surface.Context) surface.FooterHint {
	return surface.FooterHint{Text: defaultFooterHint}
}

// AcceptsScroll is true: the catalog can overflow on smaller
// cockpits when the registry has many presets per region.
func (Surface) AcceptsScroll(_ surface.Context) bool { return true }

const defaultFooterHint = "  [q] quit · [?] help · [↑/↓] select · [Enter] toggle detail · [c] copy briefing · [Esc] back"
