package projectdetail_test

import (
	"strings"
	"testing"

	"github.com/dilitS/webox/config"
	"github.com/dilitS/webox/tui/surface"
	"github.com/dilitS/webox/tui/surface/projectdetail"
	"github.com/dilitS/webox/tui/theme"
	"github.com/dilitS/webox/tui/views"
)

// fixtureContext seeds a snapshot with one project so the renderers
// have something concrete to draw. Both tabs need a `selectedProject`
// hit to render anything beyond the "no project selected" hint.
func fixtureContext(activeTab string) surface.Context {
	return surface.Context{
		Screen: views.Screen{
			Width:         100,
			Height:        30,
			SelectedIndex: 0,
			ActiveTab:     activeTab,
			Styles:        theme.NewStyles(theme.Default()),
			Config: &config.Config{
				Projects: []config.Project{{
					ID: "p1", Domain: "demo.smallhost.pl", ProfileAlias: "main",
					NodeVersion: "v24.15.0", Stack: "node-express",
				}},
			},
			Statuses: map[string]views.ProjectStatus{
				"p1": {ProjectID: "p1", State: "ONLINE", HTTPHealth: "200 OK", SSLDaysLeft: 27},
			},
			LiveLogs: views.LiveLogsSnapshot{
				Domain:     "demo.smallhost.pl",
				Connected:  true,
				AutoScroll: true,
				BufferUsed: 0,
				BufferCap:  1000,
			},
		},
	}
}

func TestSurface_OverviewBodyContainsTabsAndStatus(t *testing.T) {
	t.Parallel()

	got := projectdetail.Surface{}.Body(fixtureContext("Overview"))
	for _, needle := range []string{"[1] Overview", "[4] Logs", "demo.smallhost.pl"} {
		if !strings.Contains(got, needle) {
			t.Errorf("overview body missing %q\n--- body ---\n%s", needle, got)
		}
	}
}

func TestSurface_LogsBodyMatchesLiveLogsRenderer(t *testing.T) {
	t.Parallel()

	ctx := fixtureContext("Logs")
	via := projectdetail.Surface{}.Body(ctx)
	want := views.RenderLiveLogs(ctx.Screen)
	if via != want {
		t.Errorf("logs surface body drift\n--- via surface ---\n%s\n--- via views ---\n%s", via, want)
	}
}

// TestSurface_EnvDiffBodyDispatchesToRenderer guards the Sprint 20
// TASK-20.4 dispatch contract: pressing `2` switches activeTab to
// "Env Diff" which the surface routes to RenderEnvDiff.
func TestSurface_EnvDiffBodyDispatchesToRenderer(t *testing.T) {
	t.Parallel()

	ctx := fixtureContext("Env Diff")
	via := projectdetail.Surface{}.Body(ctx)
	want := views.RenderEnvDiff(ctx.Screen)
	if via != want {
		t.Errorf("env diff surface body drift\n--- via surface ---\n%s\n--- via views ---\n%s", via, want)
	}
}

// TestSurface_DatabaseBodyDispatchesToRenderer mirrors the Env Diff
// dispatch test for the Database tab (key `3`).
func TestSurface_DatabaseBodyDispatchesToRenderer(t *testing.T) {
	t.Parallel()

	ctx := fixtureContext("Database")
	via := projectdetail.Surface{}.Body(ctx)
	want := views.RenderDatabase(ctx.Screen)
	if via != want {
		t.Errorf("database surface body drift\n--- via surface ---\n%s\n--- via views ---\n%s", via, want)
	}
}

// TestSurface_EnvDiffAndDatabaseCrumbs checks the crumb labels the
// View pins above the body.
func TestSurface_EnvDiffAndDatabaseCrumbs(t *testing.T) {
	t.Parallel()

	if got := (projectdetail.Surface{}).Crumb(fixtureContext("Env Diff")); got != "Env Diff" {
		t.Errorf("Crumb(Env Diff) = %q, want %q", got, "Env Diff")
	}
	if got := (projectdetail.Surface{}).Crumb(fixtureContext("Database")); got != "Database" {
		t.Errorf("Crumb(Database) = %q, want %q", got, "Database")
	}
}

func TestSurface_OverviewFallbackForUnknownTab(t *testing.T) {
	t.Parallel()

	via := projectdetail.Surface{}.Body(fixtureContext("Garbage"))
	want := views.RenderProjectDetail(fixtureContext("Garbage").Screen)
	if via != want {
		t.Errorf("unknown tab MUST fall back to Overview renderer; drift detected")
	}
}

func TestSurface_CrumbSwitchesPerTab(t *testing.T) {
	t.Parallel()

	cases := []struct {
		tab  string
		want string
	}{
		{"Overview", "Project Detail"},
		{"Logs", "Live Logs"},
		{"", "Project Detail"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.tab, func(t *testing.T) {
			t.Parallel()
			if got := (projectdetail.Surface{}).Crumb(fixtureContext(tc.tab)); got != tc.want {
				t.Errorf("Crumb(%q) = %q, want %q", tc.tab, got, tc.want)
			}
		})
	}
}

func TestSurface_FooterPublishesProjectDetailKeys(t *testing.T) {
	t.Parallel()

	hint := projectdetail.Surface{}.Footer(fixtureContext("Overview"))
	if hint.ScrollHint {
		t.Error("project detail footer must let View inject the scroll hint dynamically")
	}
	for _, needle := range []string{"[q] quit", "[Esc/Tab] back", "[1] overview", "[2] env", "[3] db", "[4] logs", "[r] restart", "[s] ssl", "[v] tail"} {
		if !strings.Contains(hint.Text, needle) {
			t.Errorf("footer missing %q in %q", needle, hint.Text)
		}
	}
	if strings.Contains(hint.Text, "command palette") {
		t.Errorf("project detail footer must not advertise unimplemented command palette: %q", hint.Text)
	}
}

func TestSurface_AcceptsScroll(t *testing.T) {
	t.Parallel()

	if !(projectdetail.Surface{}).AcceptsScroll(fixtureContext("Overview")) {
		t.Error("project detail must accept scroll: action-output panel can overflow on Standard cockpit")
	}
}

func TestSurface_ContractCompiles(t *testing.T) {
	t.Parallel()

	var _ surface.Surface = projectdetail.Surface{}
}
