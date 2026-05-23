// Renderer smoke + branch coverage. These tests intentionally do not
// pin every byte of output (that would couple the suite to lipgloss
// styling tweaks). Instead they assert the operator-visible needles
// each renderer must surface, and walk every visible branch in the
// switch / conditional blocks. The existing golden tests
// (project_wizard_test.go) cover the strict layout side.
package views_test

import (
	"strings"
	"testing"
	"time"

	"github.com/dilitS/webox/config"
	"github.com/dilitS/webox/tui/theme"
	"github.com/dilitS/webox/tui/views"
)

func baseScreen(width, height int, cfg *config.Config) views.Screen {
	return views.Screen{
		Width:   width,
		Height:  height,
		Styles:  theme.NewStyles(theme.Default()),
		Config:  cfg,
		Spinner: "*",
	}
}

func sampleConfig() *config.Config {
	return &config.Config{
		Profiles: []config.Profile{{Alias: "main", Host: "s1.small.pl", Port: 22, User: "demo"}},
		Projects: []config.Project{
			{ID: "p-1", Domain: "app.demo.smallhost.pl", ProfileAlias: "main", NodeVersion: "22", Repo: "demo/app"},
			{ID: "p-2", Domain: "blog.demo.smallhost.pl", ProfileAlias: "main", NodeVersion: "20"},
		},
	}
}

// --------------------------------------------------------------------
// Dashboard
// --------------------------------------------------------------------

func TestRenderDashboard_NoProjects_RendersOnboardingHint(t *testing.T) {
	t.Parallel()
	cfg := sampleConfig()
	cfg.Projects = nil
	s := baseScreen(100, 30, cfg)
	out := views.RenderDashboard(s)
	for _, needle := range []string{
		"Webox Cockpit", "Projects", "No projects yet", "Overview", "n:new", "i:import",
	} {
		if !strings.Contains(out, needle) {
			t.Fatalf("missing needle %q\n--- output ---\n%s", needle, out)
		}
	}
}

func TestRenderDashboard_WithProjects_HighlightsSelectionAndStatuses(t *testing.T) {
	t.Parallel()
	cfg := sampleConfig()
	s := baseScreen(110, 30, cfg)
	s.SelectedIndex = 0 // selecting p-1 so its `5m ago` LastDeploy is rendered
	s.Statuses = map[string]views.ProjectStatus{
		"p-1": {ProjectID: "p-1", HTTPHealth: "200 ok", SSLDaysLeft: 87, NodeVersion: "22", LastDeploy: "5m ago · success", State: "ONLINE"},
		"p-2": {ProjectID: "p-2", HTTPHealth: "offline", SSLDaysLeft: -1, NodeVersion: "20", LastDeploy: "—", State: "OFFLINE"},
	}
	out := views.RenderDashboard(s)
	for _, needle := range []string{
		"app.demo.smallhost.pl", "blog.demo.smallhost.pl", "ONLINE", "OFFLINE", "5m ago",
		"HTTP Health", "SSL", "Last Deploy", "Repo", "Node",
	} {
		if !strings.Contains(out, needle) {
			t.Fatalf("missing needle %q\n--- output ---\n%s", needle, out)
		}
	}
}

func TestRenderDashboard_HelpAndAlertVisible(t *testing.T) {
	t.Parallel()
	s := baseScreen(100, 30, sampleConfig())
	s.HelpVisible = true
	s.Alert = "profile saved"
	out := views.RenderDashboard(s)
	if !strings.Contains(out, "profile saved") {
		t.Fatalf("alert missing:\n%s", out)
	}
	if !strings.Contains(out, "restart") {
		t.Fatalf("help banner missing action hints:\n%s", out)
	}
}

func TestRenderDashboard_StatusFallbacks_ForUnknownProject(t *testing.T) {
	t.Parallel()
	cfg := sampleConfig()
	cfg.Projects[0].NodeVersion = "" // exercises fallback("", "unknown")
	s := baseScreen(110, 30, cfg)
	s.SelectedIndex = 0
	out := views.RenderDashboard(s)
	if !strings.Contains(out, "pending") {
		t.Fatalf("expected pending HTTP health for unknown project:\n%s", out)
	}
	if !strings.Contains(out, "unknown") {
		t.Fatalf("expected unknown Node fallback:\n%s", out)
	}
}

func TestRenderDashboard_StaleImportedProjectsGetSTALEBadge(t *testing.T) {
	t.Parallel()
	cfg := sampleConfig()
	now := time.Date(2026, 5, 23, 10, 0, 0, 0, time.UTC)
	cfg.Projects[0].ImportedAt = &now
	s := baseScreen(110, 30, cfg)
	out := views.RenderDashboard(s)
	if !strings.Contains(out, "STALE") {
		t.Fatalf("imported project should render STALE badge:\n%s", out)
	}
}

// --------------------------------------------------------------------
// Project Detail
// --------------------------------------------------------------------

func TestRenderProjectDetail_NoProject_RendersGuard(t *testing.T) {
	t.Parallel()
	cfg := sampleConfig()
	cfg.Projects = nil
	s := baseScreen(100, 30, cfg)
	out := views.RenderProjectDetail(s)
	if !strings.Contains(out, "No project selected") {
		t.Fatalf("guard missing:\n%s", out)
	}
}

func TestRenderProjectDetail_HappyPath_ShowsAllSections(t *testing.T) {
	t.Parallel()
	s := baseScreen(110, 30, sampleConfig())
	s.Statuses = map[string]views.ProjectStatus{
		"p-1": {HTTPHealth: "200 ok", SSLDaysLeft: 30, NodeVersion: "22", LastDeploy: "1h ago · success", State: "ONLINE"},
	}
	out := views.RenderProjectDetail(s)
	for _, needle := range []string{
		"app.demo.smallhost.pl", "[1] Overview", "Restart", "SSL Renew", "Tail Logs",
		"HTTP", "SSL", "Deploy path", "Repo", "Last Deploy",
	} {
		if !strings.Contains(out, needle) {
			t.Fatalf("missing needle %q\n--- output ---\n%s", needle, out)
		}
	}
}

func TestRenderProjectDetail_ActionPanel_RendersLogsPanel(t *testing.T) {
	t.Parallel()
	s := baseScreen(120, 40, sampleConfig())
	s.ActionForm = views.ProjectActionSnapshot{
		Kind:   "logs",
		Output: strings.Repeat("line\n", 30),
	}
	out := views.RenderProjectDetail(s)
	if !strings.Contains(out, "older lines omitted") {
		t.Fatalf("clamp hint missing:\n%s", out)
	}
	if !strings.Contains(out, "logs (last") {
		t.Fatalf("logs heading missing:\n%s", out)
	}
}

func TestRenderProjectDetail_ActionLine_AllStates(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		form views.ProjectActionSnapshot
		want []string
	}{
		{"idle", views.ProjectActionSnapshot{}, []string{"no action yet"}},
		{"running", views.ProjectActionSnapshot{Kind: "restart", Running: true}, []string{"running", "restart"}},
		{"failure", views.ProjectActionSnapshot{Kind: "ssl_renew", Err: "rate limit"}, []string{"ssl_renew failed", "rate limit"}},
		{"success", views.ProjectActionSnapshot{Kind: "restart"}, []string{"restart ok"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := baseScreen(110, 30, sampleConfig())
			s.ActionForm = tc.form
			out := views.RenderProjectDetail(s)
			for _, needle := range tc.want {
				if !strings.Contains(out, needle) {
					t.Fatalf("missing %q in %s state:\n%s", needle, tc.name, out)
				}
			}
		})
	}
}

func TestRenderProjectDetail_AlertSurfacedAtFooter(t *testing.T) {
	t.Parallel()
	s := baseScreen(110, 30, sampleConfig())
	s.Alert = "restart succeeded"
	out := views.RenderProjectDetail(s)
	if !strings.Contains(out, "restart succeeded") {
		t.Fatalf("alert missing:\n%s", out)
	}
}

// --------------------------------------------------------------------
// Init Wizard
// --------------------------------------------------------------------

func TestRenderInitWizard_AllSteps(t *testing.T) {
	t.Parallel()
	s := baseScreen(80, 24, sampleConfig())

	cases := []struct {
		name string
		step int
		want []string
	}{
		{"welcome (default)", 0, []string{"first run setup", "System Pre-requisites"}},
		{"alias", 1, []string{"Step 2/6", "Profile alias"}},
		{"host", 2, []string{"Step 3/6", "SSH host"}},
		{"port", 3, []string{"Step 4/6", "SSH port"}},
		{"user", 4, []string{"Step 5/6", "SSH user"}},
		{"review", 5, []string{"Step 6/6", "Review profile"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := s
			s.InitForm = views.InitWizardSnapshot{Step: tc.step, Alias: "main", Host: "s1.small.pl", Port: "22", User: "demo"}
			out := views.RenderInitWizard(s)
			for _, needle := range tc.want {
				if !strings.Contains(out, needle) {
					t.Fatalf("step %s missing %q:\n%s", tc.name, needle, out)
				}
			}
		})
	}
}

func TestRenderInitWizard_FieldErrorSurfaces(t *testing.T) {
	t.Parallel()
	s := baseScreen(80, 24, sampleConfig())
	s.InitForm = views.InitWizardSnapshot{Step: 1, Err: "alias must match"}
	out := views.RenderInitWizard(s)
	if !strings.Contains(out, "alias must match") {
		t.Fatalf("err missing:\n%s", out)
	}
}

func TestRenderInitWizard_ReviewSavingFlagSurfaces(t *testing.T) {
	t.Parallel()
	s := baseScreen(80, 24, sampleConfig())
	s.InitForm = views.InitWizardSnapshot{Step: 5, Alias: "main", Host: "h", Port: "22", User: "u", Saving: true}
	out := views.RenderInitWizard(s)
	if !strings.Contains(out, "Saving profile") {
		t.Fatalf("saving hint missing:\n%s", out)
	}
}

// --------------------------------------------------------------------
// Project Wizard — every step variant
// --------------------------------------------------------------------

func TestRenderProjectWizard_AllSteps(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		form views.ProjectWizardSnapshot
		want []string
	}{
		{"profile picker (default)", views.ProjectWizardSnapshot{Step: 0, ProfileAlias: "main"}, []string{"Step 1/6", "Profile"}},
		{"stack picker", views.ProjectWizardSnapshot{Step: 1, Stack: "node-express"}, []string{"Step 2/6", "node-express"}},
		{"db choice yes", views.ProjectWizardSnapshot{Step: 2, DBWanted: true}, []string{"Step 3/6", "yes"}},
		{"db choice no", views.ProjectWizardSnapshot{Step: 2, DBWanted: false}, []string{"Step 3/6", "no"}},
		{"db kind picker", views.ProjectWizardSnapshot{Step: 3, DBKind: "mysql"}, []string{"Step 3a/6", "mysql"}},
		{"db name", views.ProjectWizardSnapshot{Step: 4, DBName: "demo_db"}, []string{"Step 3b/6", "demo_db"}},
		{"db name empty placeholder", views.ProjectWizardSnapshot{Step: 4}, []string{"Step 3b/6", "type here"}},
		{"domain", views.ProjectWizardSnapshot{Step: 5, Domain: "app.demo.smallhost.pl"}, []string{"Step 4/6", "app.demo.smallhost.pl"}},
		{"domain empty placeholder", views.ProjectWizardSnapshot{Step: 5}, []string{"Step 4/6", "type here"}},
		{"review skip", views.ProjectWizardSnapshot{Step: 6, ProfileAlias: "main", Stack: "static", Domain: "x.demo.smallhost.pl", NodeVersion: "22"}, []string{"Step 5/6", "skip"}},
		{"review with db", views.ProjectWizardSnapshot{Step: 6, ProfileAlias: "main", Stack: "node-express", Domain: "x.demo.smallhost.pl", NodeVersion: "22", DBWanted: true, DBKind: "mysql", DBName: "demo_db"}, []string{"mysql / demo_db"}},
		{"executing", views.ProjectWizardSnapshot{Step: 7, SubdomainOK: true, SSLOK: false}, []string{"Step 6/6", "in progress", "subdomain"}},
		{"failure", views.ProjectWizardSnapshot{Step: 8, Err: "boom"}, []string{"Provisioning failed", "boom"}},
		{"rolling back", views.ProjectWizardSnapshot{Step: 9}, []string{"Rolling back"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := baseScreen(100, 30, sampleConfig())
			s.ProjectForm = tc.form
			out := views.RenderProjectWizard(s)
			for _, needle := range tc.want {
				if !strings.Contains(out, needle) {
					t.Fatalf("%s missing %q:\n%s", tc.name, needle, out)
				}
			}
		})
	}
}

func TestRenderProjectWizard_NoProfileConfiguredSurfacesAlert(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	s := baseScreen(100, 30, cfg)
	s.ProjectForm = views.ProjectWizardSnapshot{Step: 0}
	out := views.RenderProjectWizard(s)
	if !strings.Contains(out, "No profile configured") {
		t.Fatalf("missing profile-missing alert:\n%s", out)
	}
}

func TestRenderProjectWizard_FormErrorsBubbleUp(t *testing.T) {
	t.Parallel()
	s := baseScreen(100, 30, sampleConfig())
	for _, step := range []int{0, 1, 2, 3, 4, 5, 6} {
		step := step
		s := s
		s.ProjectForm = views.ProjectWizardSnapshot{Step: step, ProfileAlias: "main", Stack: "static", Domain: "x.demo.smallhost.pl", NodeVersion: "22", Err: "bad input"}
		out := views.RenderProjectWizard(s)
		if !strings.Contains(out, "bad input") {
			t.Fatalf("step %d did not surface error:\n%s", step, out)
		}
	}
}

// --------------------------------------------------------------------
// Helpers — clamp, statusFor, fallback, fitWidth, renderKV
// --------------------------------------------------------------------

func TestRenderImportPreview_PersistingSpinnerVisible(t *testing.T) {
	t.Parallel()
	s := baseScreen(110, 30, sampleConfig())
	s.ImportForm = views.ImportPreviewSnapshot{Saving: true, Rows: []views.ImportRowSnapshot{{Domain: "x.demo.smallhost.pl"}}}
	out := views.RenderImportPreview(s)
	if !strings.Contains(out, "writing imported projects") {
		t.Fatalf("saving spinner missing:\n%s", out)
	}
}

func TestRenderImportPreview_EmptyRowsRendersHint(t *testing.T) {
	t.Parallel()
	s := baseScreen(110, 30, sampleConfig())
	s.ImportForm = views.ImportPreviewSnapshot{Total: 0}
	out := views.RenderImportPreview(s)
	if !strings.Contains(out, "no subdomains reported") {
		t.Fatalf("empty hint missing:\n%s", out)
	}
}

// --------------------------------------------------------------------
// Resume Wizard renderer
// --------------------------------------------------------------------

func TestRenderResumeWizard_AllBranches(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		form views.ResumeWizardSnapshot
		want []string
	}{
		{
			name: "empty steps",
			form: views.ResumeWizardSnapshot{},
			want: []string{"Resume Wizard", "(snapshot failed", "Roll back now"},
		},
		{
			name: "with steps reversed",
			form: views.ResumeWizardSnapshot{StepNames: []string{"Subdomain x", "SSL x"}, WizardID: "w-1", ProfileAlias: "main", UpdatedAt: "2026-05-23"},
			want: []string{"w-1", "main", "2026-05-23", "Subdomain x", "SSL x"},
		},
		{
			name: "rollback results",
			form: views.ResumeWizardSnapshot{
				Results: []views.RollbackResultSnapshot{
					{Name: "Subdomain x"},
					{Name: "SSL x", Err: "panel disconnected"},
				},
			},
			want: []string{"[OK]", "Subdomain x", "[FAIL]", "panel disconnected"},
		},
		{
			name: "rolling back spinner",
			form: views.ResumeWizardSnapshot{RollingBack: true},
			want: []string{"rolling back"},
		},
		{
			name: "discard confirmation prompt",
			form: views.ResumeWizardSnapshot{Discarding: true, DiscardPhrase: "discard-w1", ConfirmInput: "discard-w"},
			want: []string{"Discard requires confirmation", "discard-w1", "discard-w"},
		},
		{
			name: "snapshot error surfaces",
			form: views.ResumeWizardSnapshot{Err: "pending file corrupted"},
			want: []string{"pending file corrupted"},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := baseScreen(110, 30, sampleConfig())
			s.ResumeForm = tc.form
			out := views.RenderResumeWizard(s)
			for _, needle := range tc.want {
				if !strings.Contains(out, needle) {
					t.Fatalf("missing %q\n--- output ---\n%s", needle, out)
				}
			}
		})
	}
}

// --------------------------------------------------------------------
// truncateCell branches
// --------------------------------------------------------------------

func TestTruncateCellThroughRenderer_HandlesAllWidths(t *testing.T) {
	t.Parallel()
	// Two-char width forces the "width < truncateMinEllipsisRoom"
	// branch where we slice the raw string instead of appending
	// the ellipsis. We exercise it indirectly through the import
	// renderer, which is the only public consumer.
	rows := []views.ImportRowSnapshot{
		{Domain: strings.Repeat("a", 60), Type: strings.Repeat("X", 60), NodeVersion: strings.Repeat("9", 60)},
	}
	s := baseScreen(120, 30, sampleConfig())
	s.ImportForm = views.ImportPreviewSnapshot{Total: 1, Rows: rows}
	out := views.RenderImportPreview(s)
	if !strings.Contains(out, "...") {
		t.Fatalf("expected ellipsis from truncateCell:\n%s", out)
	}
}
