// Standard cockpit (100×30) mini-bento regression suite. Covers
// Sprint 20 TASK-20.3: the Standard fallback used to be a plain
// 2-pane projects/server split; the new layout adds compact CI/CD
// and live-log strips below the main row so the cockpit reads as
// a proper mini-bento at the 100×30 floor.
package views_test

import (
	"strings"
	"testing"

	"github.com/dilitS/webox/config"
	"github.com/dilitS/webox/tui/theme"
	"github.com/dilitS/webox/tui/views"
)

// TestRenderDashboard_Standard_FitsBudget guards the chrome
// budget: at the cockpit floor 100×30, the body MUST fit in 29
// lines so the View layer's footer line never spills off-screen.
// Tests bloat the buffer with a CI snapshot and 5 log lines so
// the mini-strips are non-empty.
func TestRenderDashboard_Standard_FitsBudget(t *testing.T) {
	t.Parallel()

	cfg := standardConfigWithProjects()
	s := baseScreen(100, 30, cfg)
	s.SelectedIndex = 0
	s.Statuses = map[string]views.ProjectStatus{
		"p-1": {ProjectID: "p-1", State: "ONLINE", HTTPHealth: "200 OK", SSLDaysLeft: 87, NodeVersion: "22", LastDeploy: "5m ago · success"},
	}
	s.CICDMini = views.CICDMiniSnapshot{
		HasRun:    true,
		Status:    "SUCCESS",
		JobName:   "Build & Test",
		RunNumber: 42,
		UpdatedAt: "2m ago",
	}
	s.LiveLogs = views.LiveLogsSnapshot{
		Domain:     "app.demo.smallhost.pl",
		Connected:  true,
		BufferUsed: 12,
		BufferCap:  256,
		Lines: []views.LiveLogLineSnapshot{
			{Level: "INFO", Text: "[13:30:00] INFO - app: started"},
			{Level: "INFO", Text: "[13:30:01] INFO - http: listening on :3000"},
			{Level: "INFO", Text: "[13:30:02] INFO - db: connected to pg-default"},
			{Level: "WARN", Text: "[13:30:03] WARN - cache: redis miss for key=foo"},
			{Level: "INFO", Text: "[13:30:04] INFO - request GET /api/users 200"},
		},
	}
	out := views.RenderDashboard(s)
	lineCount := strings.Count(out, "\n") + 1

	// Footer is composed by tui/view.go on top of the body, so the
	// budget here is the screen height minus the chrome (1 status
	// bar painted ABOVE this body, 1 footer painted BELOW).
	const standardBodyBudget = 29
	if lineCount > standardBodyBudget {
		t.Errorf("body line count = %d, want ≤ %d (Standard 100×30 fallback budget)\n--- output ---\n%s",
			lineCount, standardBodyBudget, out)
	}
}

// TestRenderDashboard_Standard_RendersMiniStrips asserts the
// new mini-bento ribbons surface their data needles (CI/CD job
// name + run number + log line text). The golden file approach
// would be too brittle; we walk the visible payload instead.
func TestRenderDashboard_Standard_RendersMiniStrips(t *testing.T) {
	t.Parallel()

	cfg := standardConfigWithProjects()
	s := baseScreen(100, 30, cfg)
	s.SelectedIndex = 0
	s.Statuses = map[string]views.ProjectStatus{
		"p-1": {ProjectID: "p-1", State: "ONLINE"},
	}
	s.CICDMini = views.CICDMiniSnapshot{
		HasRun:     true,
		Status:     "FAILED",
		JobName:    "Deploy",
		RunNumber:  17,
		UpdatedAt:  "1h ago",
		FailedStep: "build:web",
	}
	s.LiveLogs = views.LiveLogsSnapshot{
		Domain:     "app.demo.smallhost.pl",
		Connected:  true,
		BufferUsed: 4,
		BufferCap:  256,
		Lines: []views.LiveLogLineSnapshot{
			{Level: "INFO", Text: "[13:30:00] INFO - app: started"},
			{Level: "ERROR", Text: "[13:30:01] ERROR - app: panic deferred"},
			{Level: "WARN", Text: "[13:30:02] WARN - app: shutdown requested"},
			{Level: "INFO", Text: "[13:30:03] INFO - app: bye"},
		},
	}

	out := views.RenderDashboard(s)
	for _, needle := range []string{
		"[CI/CD",
		"#17",
		"FAILED",
		"build:web",
		"1h ago",
		"[Live Logs]",
		"shutdown requested",
		"bye",
	} {
		if !strings.Contains(out, needle) {
			t.Errorf("missing needle %q in Standard mini-bento\n--- output ---\n%s", needle, out)
		}
	}
}

// TestRenderDashboard_Standard_NoCIRunPlaceholder confirms the
// CI/CD strip degrades gracefully when no run has been observed
// yet — a project may be brand new or unconnected to GitHub.
func TestRenderDashboard_Standard_NoCIRunPlaceholder(t *testing.T) {
	t.Parallel()

	cfg := standardConfigWithProjects()
	s := baseScreen(100, 30, cfg)
	s.SelectedIndex = 0
	s.CICDMini = views.CICDMiniSnapshot{}
	out := views.RenderDashboard(s)

	for _, needle := range []string{"[CI/CD", "no run yet"} {
		if !strings.Contains(out, needle) {
			t.Errorf("missing needle %q in CI/CD placeholder\n--- output ---\n%s", needle, out)
		}
	}
}

// TestRenderDashboard_Standard_NoLogsPlaceholder verifies the
// live-log strip falls back to a friendly "waiting" placeholder
// when the buffer is empty.
func TestRenderDashboard_Standard_NoLogsPlaceholder(t *testing.T) {
	t.Parallel()

	cfg := standardConfigWithProjects()
	s := baseScreen(100, 30, cfg)
	s.LiveLogs = views.LiveLogsSnapshot{}
	out := views.RenderDashboard(s)
	if !strings.Contains(out, "[Live Logs]") {
		t.Errorf("missing [Live Logs] header in placeholder branch\n--- output ---\n%s", out)
	}
	if !strings.Contains(out, "Waiting") && !strings.Contains(out, "no log") {
		t.Errorf("missing waiting/no-log placeholder text\n--- output ---\n%s", out)
	}
}

// TestRenderDashboard_Standard_LongJobNameClips ensures the CI/CD
// strip never spills horizontally — a 60-char job name on a
// 100-wide cockpit must collapse to a truncated form so the
// status pill stays visible.
func TestRenderDashboard_Standard_LongJobNameClips(t *testing.T) {
	t.Parallel()

	cfg := standardConfigWithProjects()
	s := baseScreen(100, 30, cfg)
	s.SelectedIndex = 0
	s.CICDMini = views.CICDMiniSnapshot{
		HasRun:    true,
		Status:    "SUCCESS",
		JobName:   "this-is-a-deliberately-overlong-job-name-that-must-be-clipped",
		RunNumber: 99,
		UpdatedAt: "3m ago",
	}
	out := views.RenderDashboard(s)
	for _, line := range strings.Split(out, "\n") {
		if visualWidth(line) > 100 {
			t.Errorf("body line exceeds 100 cells: width=%d  line=%q", visualWidth(line), line)
		}
	}
}

func standardConfigWithProjects() *config.Config {
	return &config.Config{
		Profiles: []config.Profile{{Alias: "main", Host: "s1.small.pl", Port: 22, User: "demo"}},
		Projects: []config.Project{
			{ID: "p-1", Domain: "app.demo.smallhost.pl", ProfileAlias: "main", NodeVersion: "22", Repo: "demo/app"},
			{ID: "p-2", Domain: "blog.demo.smallhost.pl", ProfileAlias: "main", NodeVersion: "20"},
		},
	}
}

// visualWidth approximates lipgloss.Width without importing it
// (this test file already lives in a black-box package). ANSI
// escape sequences are stripped naively so display width is
// counted in cells.
func visualWidth(s string) int {
	out := 0
	skip := 0
	for i, r := range s {
		if skip > 0 {
			skip--
			continue
		}
		if r == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			// Find the closing letter.
			for j := i + 2; j < len(s); j++ {
				if s[j] >= 0x40 && s[j] <= 0x7e {
					skip = j - i
					break
				}
			}
			continue
		}
		out++
	}
	return out
}

// silence unused warnings when the file is built without the
// adjacent renderers_coverage_test.go: keep the theme import
// referenced for go vet's sake.
var _ = theme.Default
