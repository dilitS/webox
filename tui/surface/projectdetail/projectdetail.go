// Package projectdetail hosts the [surface.Surface] for the
// per-project detail screen (Overview tab + Sprint 09 live-log tab).
//
// The surface dispatches between the two tabs via the snapshot field
// `Screen.ActiveTab` ("Overview" / "Logs"); the dispatch lives here
// rather than in the dashboard's adapter because both tabs share the
// same chrome contract (left-aligned title, dimmed roadmap tabs,
// Sprint-13 status bar above) but render different bodies.
package projectdetail

import (
	"github.com/dilitS/webox/tui/surface"
	"github.com/dilitS/webox/tui/views"
)

// Surface is the value-typed [surface.Surface] for project detail.
// It carries no fields — the active tab is read from
// `Screen.ActiveTab` so the same instance handles both Overview and
// Logs without per-render mutation.
type Surface struct{}

// Body picks the renderer based on the active tab.
//
//   - `Overview` → [views.RenderProjectDetail].
//   - `Env Diff` → [views.RenderEnvDiff] (read-only secrets list).
//   - `Database` → [views.RenderDatabase] (stack-aware cheatsheet).
//   - `Logs`     → [views.RenderLiveLogs] (Sprint 09 live tail).
//
// Any other value falls back to the Overview renderer; the model
// guarantees `ActiveTab` is one of the four enabled tabs so the
// fallback only triggers on a future contract regression and keeps
// the surface from rendering an empty body.
func (Surface) Body(ctx surface.Context) string {
	switch ctx.Screen.ActiveTab {
	case tabLogs:
		return views.RenderLiveLogs(ctx.Screen)
	case tabEnvDiff:
		return views.RenderEnvDiff(ctx.Screen)
	case tabDatabase:
		return views.RenderDatabase(ctx.Screen)
	default:
		return views.RenderProjectDetail(ctx.Screen)
	}
}

// Crumb labels the surface based on the active tab. Sprint 13
// chrome contract — `docs/UX.md §4.2`.
func (Surface) Crumb(ctx surface.Context) string {
	switch ctx.Screen.ActiveTab {
	case tabLogs:
		return "Live Logs"
	case tabEnvDiff:
		return "Env Diff"
	case tabDatabase:
		return "Database"
	default:
		return "Project Detail"
	}
}

// Footer returns the global cockpit legend. Per-tab hints
// (`[r] Restart`, `[f] toggle auto-scroll`, etc.) are embedded inside
// the body by the renderers so they sit next to the related action;
// the chrome footer keeps the cockpit-wide affordances visible.
//
// `ScrollHint` is left false: the View layer injects the
// `↕ scroll: PgUp/PgDn` indicator dynamically when the body
// overflows, regardless of what each surface declares.
func (Surface) Footer(_ surface.Context) surface.FooterHint {
	return surface.FooterHint{Text: defaultFooterHint}
}

// AcceptsScroll is true for both tabs:
//
//   - Overview can overflow on `100×30` Standard cockpits when the
//     selected project has a long action-output panel.
//   - Logs has its own `↑/↓` shortcuts but PgUp/PgDn still need to
//     move the chrome viewport when the live-log buffer plus the
//     status strip overflow the terminal.
//
// Sprint 14 TASK-14.2 layers per-tile focus on top of this: when a
// scrollable tile (live-log buffer, future CI/CD modal) takes focus
// the same PgUp/PgDn keys move that tile's offset instead. Until
// 14.2 lands, the surface-level scroll keeps the operator in
// control of the chrome viewport.
func (Surface) AcceptsScroll(_ surface.Context) bool { return true }

const (
	// tab* string constants mirror `tui.DetailTab.String()`. We
	// compare against the string so this leaf package does not
	// import `tui` (would be a cycle: `tui` imports the surface
	// package).
	tabLogs     = "Logs"
	tabEnvDiff  = "Env Diff"
	tabDatabase = "Database"

	// defaultFooterHint surfaces the keys that actually do something
	// on the project-detail surface: back to dashboard, the four
	// tabs (1 / 2 / 3 / 4), and the three project actions
	// (restart / SSL renew / log tail).
	defaultFooterHint = "  [q] quit · [?] help · [Esc/Tab] back · [1] overview · [2] env · [3] db · [4] logs · [r] restart · [s] ssl · [v] tail"
)
