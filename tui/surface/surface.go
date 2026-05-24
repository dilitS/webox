// Package surface formalises the per-state TUI rendering contract that
// the Sprint 13 chrome refactor exposed: every surface is now a body
// renderer surrounded by a uniform top/bottom chrome composed by
// `tui.View`. This package defines the small interface a state must
// implement to participate in that contract.
//
// The interface is deliberately *additive* to the legacy
// `Model.renderRootBody` switch: a state can opt-in by registering an
// implementation here and `tui.View` will prefer the surface; states
// not yet migrated keep working through the existing switch with no
// behavioural change. This unblocks the gradual TUI package
// decomposition planned for Sprint 14 without a big-bang refactor of
// the cockpit on a release-hardening branch.
//
// Why a separate package?
//
//   - **Couplig discipline.** `tui/` is already a god-package; the
//     surface contract lives in a dedicated leaf package so each
//     migrated state can also move into its own subpackage without
//     fighting import cycles with `tui`.
//   - **Testability.** Each surface can be unit-tested with a
//     focused [Context] struct rather than constructing the full
//     `tui.Model` — see `tui/surface/dashboard_test.go` for the
//     first reference.
//   - **Reviewability.** A small interface forces every new state to
//     answer four well-defined questions (body, footer hint, scroll
//     accept, crumb) instead of letting them sprout ad-hoc helpers
//     inside the giant `Model` struct.
package surface

import "github.com/dilitS/webox/tui/views"

// Surface is the contract every TUI state implements once it has been
// migrated off the legacy `Model.renderRootBody` switch. Implementations
// are pure: they take a snapshot [Context] and return the strings
// `tui.View` will frame with chrome. They must not mutate the supplied
// context.
//
// All methods are called once per render. Implementations should be
// cheap to call (target ≤ 1 ms at the 160×45 cockpit tier — see the
// per-frame benchmark in `tui/bento/engine_bench_test.go`).
type Surface interface {
	// Body returns the scrollable content the chrome will frame. It
	// MUST NOT include the cockpit status bar or footer hint — those
	// are appended by `tui.View` so every surface gets a consistent
	// chrome contract.
	Body(ctx Context) string

	// Crumb is the breadcrumb shown on the right-hand side of the
	// cockpit status bar. Returning "" hides the crumb (the dashboard
	// uses this because its own bento status bar already brands the
	// cockpit).
	Crumb(ctx Context) string

	// Footer is the keybinding hint rendered as the bottom chrome.
	// Implementations should return the legend without a leading
	// newline; `tui.View` is responsible for placement. Returning ""
	// hides the hint (Tiny mode does this).
	Footer(ctx Context) FooterHint

	// AcceptsScroll declares whether `PgUp` / `PgDn` / `Home` / `End`
	// keys and mouse wheel events should move the surface's body
	// inside the viewport. Modal states (init wizard, command
	// palette) typically return false so scroll keys are not
	// hijacked from form controls.
	AcceptsScroll(ctx Context) bool
}

// Context is the read-only snapshot a surface receives at render time.
// It is intentionally narrow — the dashboard does not need to know how
// the topology graph is built, only what screen geometry the operator
// has. Migrated surfaces extend their constructor to capture richer
// dependencies (config, statuses, fixtures) so this struct stays small.
type Context struct {
	// Screen is the populated views.Screen for the current frame.
	// It already carries the operator's terminal width / height,
	// the active config, project statuses, alert message, etc.
	Screen views.Screen
}

// FooterHint is the structured footer payload. The text is the
// keybinding legend; ScrollHint is true when the chrome should also
// surface the `↕ scroll: PgUp/PgDn …` indicator (the View layer adds
// it when the body overflows, regardless, but a Surface can force-show
// the hint for surfaces that always overflow such as Live Logs).
type FooterHint struct {
	Text       string
	ScrollHint bool
}

// Empty reports whether the footer payload is a no-op. Useful for
// Tiny-mode surfaces that intentionally suppress the bottom chrome.
func (h FooterHint) Empty() bool {
	return h.Text == "" && !h.ScrollHint
}
