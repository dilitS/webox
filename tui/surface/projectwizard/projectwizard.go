// Package projectwizard hosts the [surface.Surface] for the
// new-project provisioning wizard (PRD F3).
//
// The wizard is modal — it captures the operator's intent across
// multiple steps (profile → stack → optional DB → subdomain →
// review → execute → optional rollback) and any background scroll
// would steal runes from the embedded text inputs. The surface
// therefore declines body scrolling. Sprint 14 TASK-14.1 migrated
// the wizard onto the surface contract so the legacy
// `tui/view.go::renderRootBody` switch stays empty.
package projectwizard

import (
	"github.com/dilitS/webox/tui/surface"
	"github.com/dilitS/webox/tui/views"
)

// Surface is the [surface.Surface] for the project wizard. It is
// stateless: every per-step datum (selected profile, captured
// subdomain, in-flight provisioning status, rollback results) flows
// through the snapshot embedded in [surface.Context].
type Surface struct{}

// Body delegates to [views.RenderProjectWizard]. The renderer
// switches on the snapshot's `Step` field so the wizard's seven
// sub-screens (profile picker, stack picker, DB choice / kind /
// name, domain capture, review, executing progress, failure menu,
// rollback progress) all reach the operator through the same
// pure-function entry point.
func (Surface) Body(ctx surface.Context) string {
	return views.RenderProjectWizard(ctx.Screen)
}

// Crumb labels the surface in the cockpit status bar.
func (Surface) Crumb(_ surface.Context) string { return "Project Wizard" }

// Footer returns the global cockpit legend. The wizard's per-step
// hints (`[ Enter ] Next  [ Shift+Tab ] Back  [ Esc ] Cancel`,
// `[ y ] Rollback`, etc.) stay inside the body so they sit next to
// the related action.
func (Surface) Footer(_ surface.Context) surface.FooterHint {
	return surface.FooterHint{Text: defaultFooterHint}
}

// AcceptsScroll is false: every wizard step is sized to fit on a
// single Standard-cockpit viewport tier (`100×30`); allowing PgUp /
// PgDn to scroll the body would also let those keys travel through
// the keyboard router into [tui.Model.updateProjectWizardKey], which
// accepts arbitrary `tea.KeyRunes` for the input steps and would
// silently consume them as text.
func (Surface) AcceptsScroll(_ surface.Context) bool { return false }

const defaultFooterHint = "  [q] quit · [?] help · [/] command palette · [Tab] cycle panels"
