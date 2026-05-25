package tui

import (
	"strings"
	"testing"

	"github.com/dilitS/webox/tui/surface"
)

// TestDashboardSurface_BodyMatchesLegacyRenderer is a regression guard
// for the Sprint 13 surface foundation: the dashboard adapter MUST
// return the exact same body as the legacy `renderDashboardBody` path
// so the gradual migration cannot accidentally change cockpit output.
//
// Sprint 14 will replace the legacy call with the surface and delete
// this guard; until then it keeps both code paths in lock-step.
func TestDashboardSurface_BodyMatchesLegacyRenderer(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		width  int
		height int
	}{
		{"standard-100x30", 100, 30},
		{"ultra-120x35", 120, 35},
		{"ultraplus-160x45", 160, 45},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := New(Options{InitialWidth: tc.width, InitialHeight: tc.height}).
				withConfig(fixtureConfig()).
				withStatuses(map[string]ProjectStatus{
					"p1": {ProjectID: "p1", State: ProjectOnline, HTTPHealth: "200 OK", SSLDaysLeft: 27, NodeVersion: "v24.15.0", LastDeploy: "2h ago"},
				})

			screen := m.screen()
			legacy := m.renderDashboardBody(screen)
			via := m.surfaceFor().Body(surface.Context{Screen: screen})

			if legacy != via {
				t.Fatalf("dashboard surface drift\n--- legacy ---\n%s\n--- via surface ---\n%s",
					legacy, via)
			}
		})
	}
}

// TestDashboardSurface_FooterAndCrumb pins the cockpit footer / crumb
// contract: the dashboard surface intentionally exposes an empty crumb
// (the bento status bar already brands the cockpit) and the full
// keybinding legend with scroll-hint disabled (View injects the
// indicator dynamically when the body overflows).
func TestDashboardSurface_FooterAndCrumb(t *testing.T) {
	t.Parallel()

	m := New(Options{InitialWidth: 120, InitialHeight: 35}).
		withConfig(fixtureConfig())
	s := m.surfaceFor()
	if s == nil {
		t.Fatal("expected dashboard surface to be registered")
	}
	ctx := surface.Context{Screen: m.screen()}

	if crumb := s.Crumb(ctx); crumb != "" {
		t.Errorf("dashboard crumb = %q, want empty", crumb)
	}
	footer := s.Footer(ctx)
	if footer.ScrollHint {
		t.Error("dashboard footer should not force scroll hint; View adds it dynamically")
	}
	for _, needle := range []string{"[q] quit", "[?] help", "[/] command palette", "[Tab] cycle panels"} {
		if !strings.Contains(footer.Text, needle) {
			t.Errorf("footer missing %q\n--- footer ---\n%s", needle, footer.Text)
		}
	}
	if !s.AcceptsScroll(ctx) {
		t.Error("dashboard surface should accept scroll (PgUp/PgDn/Mouse)")
	}
}

// TestSurfaceFor_AllProductionStatesMigrated is the post-migration
// inversion of the Sprint 13 fallback guard: after Sprint 14
// TASK-14.1 every production state MUST have a registered surface
// so the legacy `renderRootBody` switch can be reduced to its
// defensive default. A regression that drops a `case` from
// `surfaceFor()` surfaces here immediately rather than as a silent
// blank body in production.
func TestSurfaceFor_AllProductionStatesMigrated(t *testing.T) {
	t.Parallel()

	for _, s := range []State{
		StateDashboard,
		StateInitWizard,
		StateProjectDetail,
		StateProjectWizard,
		StateResumeWizard,
		StateImportPreview,
	} {
		s := s
		t.Run(s.String(), func(t *testing.T) {
			t.Parallel()
			m := New(Options{InitialWidth: 120, InitialHeight: 35}).withConfig(fixtureConfig())
			m.state = s
			if got := m.surfaceFor(); got == nil {
				t.Fatalf("state %s has no surface adapter — Sprint 14 TASK-14.1 requires every production state to be migrated", s)
			}
		})
	}
}

// TestSurfaceFor_UnknownStateReturnsNil pins the defensive default
// branch in `surfaceFor`: a future state constant that ships
// without a registered surface MUST surface as nil so the chrome
// renders the "X is not enabled" placeholder, not a silently empty
// body.
func TestSurfaceFor_UnknownStateReturnsNil(t *testing.T) {
	t.Parallel()

	const sentinelUnknownState State = 999
	m := New(Options{InitialWidth: 120, InitialHeight: 35}).withConfig(fixtureConfig())
	m.state = sentinelUnknownState
	if got := m.surfaceFor(); got != nil {
		t.Fatalf("unknown state %d returned %T — defensive default broken", sentinelUnknownState, got)
	}
}
