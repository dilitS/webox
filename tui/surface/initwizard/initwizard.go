// Package initwizard hosts the [surface.Surface] implementation for
// the first-run init wizard. The wizard captures profile metadata
// (alias / host / port / user) before the cockpit can render the
// dashboard for the first time, so the surface is intentionally
// modal: it declines body scrolling so PgUp/PgDn keys cannot steal
// runes from the embedded text inputs.
//
// The package exists to walk `tui/` off its god-package shape (see
// Sprint 14 TASK-14.1). Body delegation routes through the existing
// `tui/views` renderer so the migration is byte-identical to the
// pre-surface output — the regression guard in
// `tui/surface_adapters_test.go` enforces that contract on every
// surface tier (Standard / Ultra / Ultra+).
package initwizard

import (
	"github.com/dilitS/webox/tui/surface"
	"github.com/dilitS/webox/tui/views"
)

// Surface is the value-typed [surface.Surface] for the init wizard.
// It carries no state — every per-frame datum (current step, captured
// alias / host, validation errors, save spinner) flows through the
// snapshot embedded in [surface.Context]. The zero value is usable
// directly so the cockpit can construct a fresh Surface per render
// without allocations beyond the interface conversion.
type Surface struct{}

// Body delegates to [views.RenderInitWizard]. Keeping the renderer in
// `tui/views` instead of inlining it here avoids a duplicate code
// path during the gradual surface migration: tests, benchmarks and
// the legacy fallback in `tui/view.go` all share the same string
// builder.
func (Surface) Body(ctx surface.Context) string {
	return views.RenderInitWizard(ctx.Screen)
}

// Crumb labels the cockpit status bar so the operator never wonders
// which surface owns the current frame. The string matches the
// Sprint 13 chrome contract documented in `docs/UX.md §4.2`.
func (Surface) Crumb(_ surface.Context) string { return "Init Wizard" }

// Footer returns the global keybinding hint. The wizard's per-step
// hints (`[ Enter ] Next  [ Shift+Tab ] Back  [ Esc ] Back/Quit`)
// stay embedded inside the body so they appear next to the input
// field, while the chrome footer mirrors the cockpit-wide legend.
//
// `ScrollHint` is left false because the wizard never overflows: its
// silhouette is sized to fit inside the Standard cockpit's `100×30`
// minimum (per `docs/UX.md §11.1`).
func (Surface) Footer(_ surface.Context) surface.FooterHint {
	return surface.FooterHint{Text: defaultFooterHint}
}

// AcceptsScroll declares the wizard as non-scrollable. The body is
// designed to fit on a single viewport tier; allowing PgUp/PgDn to
// move the body would also let the same keys travel through the
// `tea.KeyMsg` chain into [tui.Model.updateInitWizardKey], which
// would silently consume them as input runes (the wizard accepts
// any `tea.KeyRunes`).
func (Surface) AcceptsScroll(_ surface.Context) bool { return false }

// defaultFooterHint surfaces the keys that actually advance the
// init wizard. Sprint 20 split the per-surface hints (closing the
// long-standing TODO from Sprint 14 TASK-14.1) so we no longer
// advertise the unimplemented `[/] command palette` or the
// dashboard-only `[Tab] cycle panels` here.
const defaultFooterHint = "  [q] quit · [?] help · [Tab/Enter] next · [Esc] back"
