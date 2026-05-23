package bento_test

import (
	"strings"
	"testing"

	"github.com/dilitS/webox/tui/bento"
)

func TestProjectsTileRendersHeaderAndRows(t *testing.T) {
	t.Parallel()

	tile := bento.NewProjectsTile([]string{
		"> alpha.example.com [ONLINE]",
		"  beta.example.com [STALE]",
	})

	out := tile.Render(bento.ModeUltra, true)
	for _, needle := range []string{"[Projects]", "alpha.example.com", "beta.example.com"} {
		if !strings.Contains(out, needle) {
			t.Fatalf("Projects tile missing %q\n--- output ---\n%s", needle, out)
		}
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

func TestOverviewTileRendersDomainAndLines(t *testing.T) {
	t.Parallel()

	tile := bento.NewOverviewTile("app.example.com", []string{
		"Status: ONLINE",
		"HTTP: 200 OK",
	})

	out := tile.Render(bento.ModeUltra, false)
	for _, needle := range []string{"[Overview]", "app.example.com", "Status: ONLINE", "HTTP: 200 OK"} {
		if !strings.Contains(out, needle) {
			t.Fatalf("Overview tile missing %q\n--- output ---\n%s", needle, out)
		}
	}
}

func TestOverviewTileEmptyDomainShowsSelectionHint(t *testing.T) {
	t.Parallel()

	tile := bento.NewOverviewTile("", []string{"Select a project to inspect status."})
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
			header: "[Live Micro-Logs]",
			hints:  []string{"Select a project to start streaming"},
		},
		{
			name:   "cicd placeholder (no GitHub-linked project)",
			tile:   bento.NewCICDPlaceholderTile(),
			header: "[CI/CD Pipeline]",
			hints:  []string{"No GitHub-linked project selected", "[n]"},
		},
		{
			name:   "topology placeholder (live wiring in Sprint 11)",
			tile:   bento.NewTopologyPlaceholderTile(),
			header: "[Topology]",
			hints:  []string{"Sprint 11"},
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
	for _, needle := range []string{"[Header Metrics]", "[LIVE]", "main", "Uptime: 24d 11h", "Load: 0.12", "RAM: 3.4G", "Ping: 18ms"} {
		if !strings.Contains(live, needle) {
			t.Fatalf("live header missing %q\n%s", needle, live)
		}
	}

	stale := bento.NewHeaderMetricsTile(bento.HeaderMetricsSnapshot{ProfileAlias: "main", Stale: true}).
		Render(bento.ModeUltra, true)
	if !strings.Contains(stale, "[STALE]") {
		t.Fatalf("stale tile missing [STALE] marker\n%s", stale)
	}
}

func TestMicroLogsTileShowsTailWithLevelMarkers(t *testing.T) {
	t.Parallel()

	out := bento.NewMicroLogsTile("app.example.com", []bento.MicroLogLine{
		{Level: "INFO", Text: "starting worker pool=4"},
		{Level: "WARN", Text: "queue depth 87%", Redacted: false},
		{Level: "ERROR", Text: "db connect failed", Redacted: true},
	}).Render(bento.ModeUltra, false)

	for _, needle := range []string{"[Live Micro-Logs]", "Stream: app.example.com", "starting worker pool=4", "queue depth", "db connect failed", "(redacted)"} {
		if !strings.Contains(out, needle) {
			t.Fatalf("micro-logs missing %q\n%s", needle, out)
		}
	}
	if !strings.Contains(out, "✗") {
		t.Fatalf("ERROR rows should use ✗ marker\n%s", out)
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
		"[CI/CD Pipeline]",
		"[LIVE]",
		"app.example.com",
		"deploy.yml",
		"Build #412",
		"SUCCESS ✓",
		"14:12 GMT",
		"[1] Git Checkout ✓",
		"[2] Install Deps ✓",
		"[3] Code Lint ✗",
		"[4] Build Artifact ⊘",
		"[5] Unit Tests ⏳",
		"[6] Deploy …",
		"[F8] View logs",
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
	if !strings.Contains(stale, "[STALE]") {
		t.Fatalf("stale tile missing [STALE] marker\n%s", stale)
	}

	limited := bento.NewCICDPipelineTile(bento.CICDPipelineSnapshot{
		ProjectAlias:  "alpha",
		RateLimited:   true,
		RateLimitHint: "Reset in 12min",
	}).Render(bento.ModeUltra, false)
	for _, needle := range []string{"[LIMITED]", "GitHub rate limit reached", "Reset in 12min"} {
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
		bento.NewOverviewTile("", nil),
		bento.NewMetricsPlaceholderTile(),
		bento.NewHeaderMetricsTile(bento.HeaderMetricsSnapshot{}),
		bento.NewCICDPlaceholderTile(),
		bento.NewCICDPipelineTile(bento.CICDPipelineSnapshot{}),
		bento.NewLogsPlaceholderTile(),
		bento.NewMicroLogsTile("", nil),
		bento.NewTopologyPlaceholderTile(),
	}

	// IDs are unique per slot — placeholder + live wiring share the
	// slot's identity by design (the renderer swaps them in place).
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
		// Allow placeholder ↔ live sharing one ID per slot, but
		// reject duplicates with different IDs in the same slot.
		bySlot[slot][id] = true
		if len(bySlot[slot]) > 1 {
			t.Errorf("slot %v has multiple distinct tile IDs: %v", slot, bySlot[slot])
		}
	}
}
