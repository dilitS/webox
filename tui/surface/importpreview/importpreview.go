// Package importpreview hosts the [surface.Surface] for the
// read-only "import existing projects" diff (PRD F9).
//
// The surface compares the panel-reported subdomains to the local
// `config.json` and asks the operator to accept the unmanaged rows.
// It can overflow on `100×30` Standard cockpits when the panel
// reports more than `importPreviewMaxRows` (14) subdomains, so the
// surface accepts body scrolling.
package importpreview

import (
	"github.com/dilitS/webox/tui/surface"
	"github.com/dilitS/webox/tui/views"
)

// Surface is the [surface.Surface] for the import-preview screen.
// Stateless — every datum (loading flag, scan result rows, save
// progress, error text) flows through the snapshot embedded in
// [surface.Context].
type Surface struct{}

// Body delegates to [views.RenderImportPreview]. The renderer surfaces
// the diff summary (`Found N subdomain(s): M managed, K new`),
// the truncatable table, and the save progress / error states.
func (Surface) Body(ctx surface.Context) string {
	return views.RenderImportPreview(ctx.Screen)
}

// Crumb labels the surface as "Import Preview" so the cockpit
// status bar makes it obvious that no provisioning is happening
// (the screen is read-only until the operator presses `a`).
func (Surface) Crumb(_ surface.Context) string { return "Import Preview" }

// Footer returns the global cockpit legend. The screen's per-row
// hints (`[a] Import all unmanaged   [esc] Cancel`) stay inside
// the body so they sit next to the table.
func (Surface) Footer(_ surface.Context) surface.FooterHint {
	return surface.FooterHint{Text: defaultFooterHint}
}

// AcceptsScroll is true. The import-preview table truncates at 14
// rows in the body itself (per `views.importPreviewMaxRows`), so
// the chrome-level scroll only kicks in when the cockpit's other
// content (status bar, summary line, save spinner) pushes the body
// past the available height. Sprint 14 TASK-14.2 will layer
// per-tile scroll on top once the table evolves into a focusable
// tile of its own.
func (Surface) AcceptsScroll(_ surface.Context) bool { return true }

// defaultFooterHint surfaces the keys that actually do something on
// the import-preview surface: bulk-import all, or back out to the
// dashboard. The dishonest `[/] command palette` reference is gone
// (no palette ships in v0.1).
const defaultFooterHint = "  [q] quit · [?] help · [a] import all · [Esc] back"
