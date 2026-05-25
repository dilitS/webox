// Package resumewizard hosts the [surface.Surface] for the resume-on-
// launch flow. When `pending_cleanups.json` exists at startup the
// cockpit boots into this surface so the operator can choose between
// rolling back the orphaned wizard run, keeping resources for manual
// cleanup, or discarding the snapshot after typing a confirmation
// phrase (DESIGN §10.3).
//
// The surface is modal: it consumes arbitrary `tea.KeyRunes` while
// the discard-confirmation prompt is open, so it declines body
// scrolling. Sprint 14 TASK-14.1 migrated the resume wizard onto
// the surface contract so the legacy `tui/view.go` switch no longer
// has to special-case it.
package resumewizard

import (
	"github.com/dilitS/webox/tui/surface"
	"github.com/dilitS/webox/tui/views"
)

// Surface is the [surface.Surface] for the resume wizard. Stateless
// by design — every datum (snapshot id, remaining steps, rollback
// progress, discard phrase) flows through the snapshot embedded in
// [surface.Context].
type Surface struct{}

// Body delegates to [views.RenderResumeWizard]. The renderer surfaces
// the snapshot metadata, remaining cleanup steps, rollback progress,
// and the discard-confirmation prompt depending on the form's
// in-flight flags.
func (Surface) Body(ctx surface.Context) string {
	return views.RenderResumeWizard(ctx.Screen)
}

// Crumb labels the surface as "Resume Wizard" so the cockpit status
// bar makes it obvious that an interrupted wizard run is being
// resumed (rather than the operator having opened a fresh wizard).
func (Surface) Crumb(_ surface.Context) string { return "Resume Wizard" }

// Footer returns the global cockpit legend. The resume wizard's
// per-step hints (`[ r ] Roll back now   [ k ] Keep and exit   [ d ]
// Discard snapshot`) live inside the body so they sit next to the
// snapshot summary.
func (Surface) Footer(_ surface.Context) surface.FooterHint {
	return surface.FooterHint{Text: defaultFooterHint}
}

// AcceptsScroll is false. Two reasons:
//
//  1. The discard prompt accepts arbitrary `tea.KeyRunes` to type
//     the confirmation phrase. PgUp / PgDn would silently land in
//     [tui.Model.updateResumeWizardKey] and corrupt the input.
//
//  2. The body fits on `100×30` Standard cockpits by design — the
//     remaining-steps list is always shorter than the LIFO rollback
//     stack capacity (≤ 16 entries; DESIGN §10.2).
func (Surface) AcceptsScroll(_ surface.Context) bool { return false }

const defaultFooterHint = "  [q] quit · [?] help · [/] command palette · [Tab] cycle panels"
