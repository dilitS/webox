package tui

import (
	"github.com/dilitS/webox/tui/surface"
	"github.com/dilitS/webox/tui/surface/catalog"
	"github.com/dilitS/webox/tui/surface/importpreview"
	"github.com/dilitS/webox/tui/surface/initwizard"
	"github.com/dilitS/webox/tui/surface/projectdetail"
	"github.com/dilitS/webox/tui/surface/projectwizard"
	"github.com/dilitS/webox/tui/surface/resumewizard"
)

// surfaceFor returns the [surface.Surface] implementation responsible
// for the model's current state. After Sprint 14 TASK-14.1 every
// production state has an adapter — the legacy
// `tui/view.go::renderRootBody` switch keeps only a defensive default
// branch so a future state added without a surface fails loudly
// instead of silently returning the empty body.
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
// The dashboard adapter intentionally lives in this file (rather than
// a `tui/surface/dashboard/` subpackage like the other states) because
// it is tightly coupled to the cockpit's bento engine, header
// metrics, host-key modal overlay, and CI/CD modal overlay — moving
// it would require either re-exporting all of those internals or
// duplicating them. Sprint 15 will revisit the layering once the
// modal overlay system is generalised.
func (m Model) surfaceFor() surface.Surface {
	switch m.state {
	case StateDashboard:
		return dashboardSurface{m: m}
	case StateInitWizard:
		return initwizard.Surface{}
	case StateProjectDetail:
		return projectdetail.Surface{}
	case StateProjectWizard:
		return projectwizard.Surface{}
	case StateResumeWizard:
		return resumewizard.Surface{}
	case StateImportPreview:
		return importpreview.Surface{}
	case StateProviderCatalog:
		return catalog.Surface{}
	default:
		return nil
	}
}

// dashboardSurface adapts the cockpit's dashboard renderer to the
// [surface.Surface] contract. It captures the current [Model] *by
// value* so the adapter reflects the per-render snapshot and never
// leaks state from earlier frames.
type dashboardSurface struct {
	m Model
}

// Body delegates to the cockpit's dashboard renderer.
func (d dashboardSurface) Body(ctx surface.Context) string {
	return d.m.renderDashboardBody(ctx.Screen)
}

// Crumb is empty for the dashboard: the bento engine already brands
// the cockpit with the "WEBOX vX.Y.Z [LIVE]" pill via WithStatusBar,
// so stacking a second crumb would produce a visually noisy header.
func (d dashboardSurface) Crumb(_ surface.Context) string { return "" }

// Footer surfaces the keys that actually do something on the
// dashboard: cycle scrollable panels, drill into the selected
// project, plus the cockpit-wide quit / help affordances. Sprint 20
// — the previous `[/] command palette` mention referred to a feature
// that does not ship in v0.1 / v0.2; including it lied to the
// operator and was the most-reported "feels unfinished" cue. The
// View layer appends the `↕ scroll …` and focus suffixes on top
// when applicable, so we leave ScrollHint=false here.
func (d dashboardSurface) Footer(_ surface.Context) surface.FooterHint {
	return surface.FooterHint{
		Text: "  [q] quit · [?] help · [Tab] cycle panels · [Right/Enter] open · [n] new · [i] import · [p] catalog",
	}
}

// AcceptsScroll returns true so PgUp/PgDn/Home/End and the mouse
// wheel move the bento grid when it overflows. The dashboard rarely
// overflows in Ultra modes thanks to height budgeting, but the
// Standard Cockpit fallback can still spill on 100×30 viewports.
func (d dashboardSurface) AcceptsScroll(_ surface.Context) bool { return true }
