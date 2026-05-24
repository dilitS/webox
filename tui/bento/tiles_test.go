package bento_test

import (
	"strings"
	"testing"

	"github.com/dilitS/webox/tui/bento"
)

func TestProjectsTileRendersHeaderAndRows(t *testing.T) {
	t.Parallel()

	tile := bento.NewProjectsTile([]bento.ProjectRowSnapshot{
		{Name: "alpha.example.com", State: "ONLINE", Selected: true},
		{Name: "beta.example.com", State: "STALE"},
	})

	out := tile.Render(bento.ModeUltra, true)
	for _, needle := range []string{"[Active Projects]", "alpha.example.com", "beta.example.com"} {
		if !strings.Contains(out, needle) {
			t.Fatalf("Projects tile missing %q\n--- output ---\n%s", needle, out)
		}
	}
	if !strings.Contains(out, "●") {
		t.Fatalf("Projects tile should render colored dot indicators\n%s", out)
	}
}

func TestProjectsTileEmptyShowsHint(t *testing.T) {
	t.Parallel()

	tile := bento.NewProjectsTile(nil)
	out := tile.Render(bento.ModeUltra, true)
	if !strings.Contains(out, "No projects yet") {
		t.Fatalf("expected empty hint, got:\n%s", out)
	}
	if !strings.Contains(out, "[n]") {
		t.Fatalf("expected 'press [n]' hint, got:\n%s", out)
	}
}

func TestOverviewTileRendersServerLines(t *testing.T) {
	t.Parallel()

	tile := bento.NewOverviewTile(bento.ServerOverviewSnapshot{
		ProjectAlias: "app.example.com",
		Lines: []bento.ServerOverviewLine{
			{Icon: "✓", Label: "Status", Value: "ONLINE", Status: "ONLINE"},
			{Icon: "⇄", Label: "HTTP", Value: "200 OK"},
			{Icon: "⚿", Label: "SSL", Value: "Valid (30d)", Status: "ONLINE"},
		},
	})

	out := tile.Render(bento.ModeUltra, false)
	for _, needle := range []string{"[SERVER: app.example.com]", "Status:", "ONLINE", "HTTP:", "200 OK", "SSL:", "Valid (30d)"} {
		if !strings.Contains(out, needle) {
			t.Fatalf("Server tile missing %q\n--- output ---\n%s", needle, out)
		}
	}
}

func TestOverviewTileEmptyShowsSelectionHint(t *testing.T) {
	t.Parallel()

	tile := bento.NewOverviewTile(bento.ServerOverviewSnapshot{})
	out := tile.Render(bento.ModeUltra, false)
	if !strings.Contains(out, "Select a project") {
		t.Fatalf("expected selection hint, got:\n%s", out)
	}
}

func TestPlaceholderTilesShowMeaningfulFallbackCopy(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		tile   bento.BentoTile
		header string
		hints  []string
	}{
		{
			name:   "metrics placeholder (pre first poll)",
			tile:   bento.NewMetricsPlaceholderTile(),
			header: "[Header Metrics]",
			hints:  []string{"CPU / RAM / Disk pulse", "Awaiting first SSH poll"},
		},
		{
			name:   "logs placeholder (no project selected)",
			tile:   bento.NewLogsPlaceholderTile(),
			header: "[Live Server Logs]",
			hints:  []string{"Select a project to start streaming"},
		},
		{
			name:   "cicd placeholder (no GitHub-linked project)",
			tile:   bento.NewCICDPlaceholderTile(),
			header: "[CI/CD PIPELINE: Main Branch]",
			hints:  []string{"No GitHub-linked project selected", "[n]"},
		},
		{
			name:   "topology placeholder (no project selected)",
			tile:   bento.NewTopologyPlaceholderTile(),
			header: "[Live Service Topology]",
			hints:  []string{"Service dependency graph"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out := tc.tile.Render(bento.ModeUltra, false)
			if !strings.Contains(out, tc.header) {
				t.Fatalf("placeholder missing header %q\n%s", tc.header, out)
			}
			for _, hint := range tc.hints {
				if !strings.Contains(out, hint) {
					t.Fatalf("placeholder missing hint %q\n%s", hint, out)
				}
			}
		})
	}
}

func TestHeaderMetricsTileRendersLiveAndStaleIndicator(t *testing.T) {
	t.Parallel()

	live := bento.NewHeaderMetricsTile(bento.HeaderMetricsSnapshot{
		ProfileAlias: "main",
		UptimeLabel:  "24d 11h",
		LoadLabel:    "0.12, 0.28, 0.31",
		RAMLabel:     "3.4G/8.0G (42%)",
		RTTLabel:     "18ms",
	}).Render(bento.ModeUltra, true)
	for _, needle := range []string{"[Header Metrics]", "LIVE", "main", "Uptime: 24d 11h", "Load:", "0.12", "RAM:", "3.4G", "Ping:", "18ms"} {
		if !strings.Contains(live, needle) {
			t.Fatalf("live header missing %q\n%s", needle, live)
		}
	}

	stale := bento.NewHeaderMetricsTile(bento.HeaderMetricsSnapshot{ProfileAlias: "main", Stale: true}).
		Render(bento.ModeUltra, true)
	if !strings.Contains(stale, "STALE") {
		t.Fatalf("stale tile missing STALE marker\n%s", stale)
	}
}

func TestMicroLogsTileShowsTimestampedLevels(t *testing.T) {
	t.Parallel()

	out := bento.NewMicroLogsTile("app.example.com", []bento.MicroLogLine{
		{Timestamp: "14:32:10", Level: "INFO", Source: "API-Gateway", Text: "GET /users 200"},
		{Timestamp: "14:32:11", Level: "WARN", Source: "Auth-Service", Text: "High latency 450ms"},
		{Timestamp: "14:32:14", Level: "DEBUG", Source: "Worker", Text: "cache hit"},
		{Timestamp: "14:32:15", Level: "ERROR", Source: "DB", Text: "connect failed", Redacted: true},
	}).Render(bento.ModeUltra, false)

	for _, needle := range []string{
		"[Live Server Logs]",
		"14:32:10",
		"INFO",
		"API-Gateway",
		"GET /users 200",
		"WARN",
		"High latency",
		"DEBUG",
		"ERROR",
		"connect failed",
		"(redacted)",
	} {
		if !strings.Contains(out, needle) {
			t.Fatalf("micro-logs missing %q\n%s", needle, out)
		}
	}
}

func TestCICDPipelineTileRendersHeaderAndSteps(t *testing.T) {
	t.Parallel()

	snap := bento.CICDPipelineSnapshot{
		ProjectAlias: "app.example.com",
		WorkflowName: "deploy.yml",
		RunNumber:    412,
		RunStatus:    bento.CICDStatusSuccess,
		HeaderTime:   "14:12 GMT",
		Duration:     "1m 42s",
		Steps: []bento.CICDStepSnapshot{
			{Number: 1, Name: "Git Checkout", Status: bento.CICDStatusSuccess, Duration: "2s"},
			{Number: 2, Name: "Install Deps", Status: bento.CICDStatusSuccess, Duration: "12s"},
			{Number: 3, Name: "Code Lint", Status: bento.CICDStatusFailure, Duration: "5s"},
			{Number: 4, Name: "Build Artifact", Status: bento.CICDStatusSkipped},
			{Number: 5, Name: "Unit Tests", Status: bento.CICDStatusInProgress, Duration: "00:14"},
			{Number: 6, Name: "Deploy", Status: bento.CICDStatusQueued},
		},
	}

	out := bento.NewCICDPipelineTile(snap).Render(bento.ModeUltra, true)

	for _, needle := range []string{
		"[CI/CD PIPELINE: Main Branch]",
		"LIVE",
		"app.example.com",
		"deploy.yml",
		"Build #412",
		"SUCCESS ✓",
		"14:12 GMT",
		"[1]",
		"Git Checkout",
		"[3]",
		"Code Lint",
		"[6]",
		"Deploy",
		"View Details (F8)",
	} {
		if !strings.Contains(out, needle) {
			t.Fatalf("CI/CD tile missing %q\n--- output ---\n%s", needle, out)
		}
	}
}

func TestCICDPipelineTileRendersStaleAndRateLimited(t *testing.T) {
	t.Parallel()

	stale := bento.NewCICDPipelineTile(bento.CICDPipelineSnapshot{
		ProjectAlias: "alpha",
		WorkflowName: "ci.yml",
		Stale:        true,
		Steps: []bento.CICDStepSnapshot{
			{Number: 1, Name: "Lint", Status: bento.CICDStatusSuccess},
		},
	}).Render(bento.ModeUltra, false)
	if !strings.Contains(stale, "STALE") {
		t.Fatalf("stale tile missing STALE marker\n%s", stale)
	}

	limited := bento.NewCICDPipelineTile(bento.CICDPipelineSnapshot{
		ProjectAlias:  "alpha",
		RateLimited:   true,
		RateLimitHint: "Reset in 12min",
	}).Render(bento.ModeUltra, false)
	for _, needle := range []string{"LIMITED", "GitHub rate limit reached", "Reset in 12min"} {
		if !strings.Contains(limited, needle) {
			t.Fatalf("rate-limited tile missing %q\n%s", needle, limited)
		}
	}
}

func TestCICDPipelineTileNoRunPlaceholder(t *testing.T) {
	t.Parallel()

	out := bento.NewCICDPipelineTile(bento.CICDPipelineSnapshot{
		ProjectAlias: "alpha",
	}).Render(bento.ModeUltra, false)
	if !strings.Contains(out, "No workflow run yet") {
		t.Fatalf("expected 'No workflow run yet' hint, got\n%s", out)
	}
}

func TestMicroLogsTileEmptyShowsWaitingHint(t *testing.T) {
	t.Parallel()

	out := bento.NewMicroLogsTile("demo", nil).Render(bento.ModeUltra, false)
	if !strings.Contains(out, "waiting for first line") {
		t.Fatalf("empty micro-logs should advertise waiting state\n%s", out)
	}
}

func TestTileIDsAreStableAndUniquePerSlot(t *testing.T) {
	t.Parallel()

	tiles := []bento.BentoTile{
		bento.NewProjectsTile(nil),
		bento.NewOverviewTile(bento.ServerOverviewSnapshot{}),
		bento.NewMetricsPlaceholderTile(),
		bento.NewHeaderMetricsTile(bento.HeaderMetricsSnapshot{}),
		bento.NewCICDPlaceholderTile(),
		bento.NewCICDPipelineTile(bento.CICDPipelineSnapshot{}),
		bento.NewLogsPlaceholderTile(),
		bento.NewMicroLogsTile("", nil),
		bento.NewTopologyPlaceholderTile(),
	}

	bySlot := map[bento.Slot]map[string]bool{}
	for _, tile := range tiles {
		id := tile.ID()
		if id == "" {
			t.Errorf("tile %T has empty ID", tile)
		}
		slot := tile.Slot()
		if bySlot[slot] == nil {
			bySlot[slot] = map[string]bool{}
		}
		bySlot[slot][id] = true
		if len(bySlot[slot]) > 1 {
			t.Errorf("slot %v has multiple distinct tile IDs: %v", slot, bySlot[slot])
		}
	}
}
