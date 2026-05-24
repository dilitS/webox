package tui

import (
	"github.com/dilitS/webox/tui/surface"
)

// surfaceFor returns the [surface.Surface] implementation responsible
// for the model's current state, or nil when the state has not yet
// been migrated off the legacy `renderRootBody` switch.
//
// The cockpit creates a fresh adapter per render rather than storing a
// pointer in a global registry. That keeps the value-typed [Model]
// semantics intact (every `Update` returns a new value; a long-lived
// pointer would otherwise capture stale state from the previous tick)
// and makes the contract trivially thread-safe: there is no shared
// adapter to mutate. The trade-off is a single 24-byte allocation per
// frame, which is dwarfed by the bento engine's own per-frame work
// (see `BenchmarkRenderUltra` in `tui/bento/engine_bench_test.go`).
//
// Sprint 13 ships the dashboard adapter as the foundation; Sprint 14
// will migrate the remaining states (init wizard, project detail,
// wizards, import preview) and move each adapter into its own
// `tui/surface/<state>/` subpackage.
func (m Model) surfaceFor() surface.Surface {
	if m.state == StateDashboard {
		return dashboardSurface{m: m}
	}
	return nil
}

// dashboardSurface adapts the cockpit's dashboard renderer to the
// [surface.Surface] contract. It captures the current [Model] *by
// value* so the adapter reflects the per-render snapshot and never
// leaks state from earlier frames.
type dashboardSurface struct {
	m Model
}

// Body delegates to the cockpit's legacy dashboard renderer so the
// Sprint 13 surface foundation is byte-identical to the pre-surface
// behaviour (see TestDashboardSurface_BodyMatchesLegacyRenderer).
func (d dashboardSurface) Body(ctx surface.Context) string {
	return d.m.renderDashboardBody(ctx.Screen)
}

// Crumb is empty for the dashboard: the bento engine already brands
// the cockpit with the "WEBOX vX.Y.Z [LIVE]" pill via WithStatusBar,
// so stacking a second crumb would produce a visually noisy header.
func (d dashboardSurface) Crumb(_ surface.Context) string { return "" }

// Footer reuses the cockpit's default keybinding legend. The View
// layer is responsible for appending the `↕ scroll …` indicator when
// the body overflows, so we deliberately leave ScrollHint=false here.
func (d dashboardSurface) Footer(_ surface.Context) surface.FooterHint {
	return surface.FooterHint{
		Text: "  [q] quit · [?] help · [/] command palette · [Tab] cycle panels",
	}
}

// AcceptsScroll returns true so PgUp/PgDn/Home/End and the mouse
// wheel move the bento grid when it overflows. The dashboard rarely
// overflows in Ultra modes thanks to height budgeting, but the
// Standard Cockpit fallback can still spill on 100×30 viewports.
func (d dashboardSurface) AcceptsScroll(_ surface.Context) bool { return true }
