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

func TestPlaceholderTilesAdvertiseSprintNumbers(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		tile        bento.BentoTile
		header      string
		sprintHints []string
	}{
		{
			name:        "metrics placeholder",
			tile:        bento.NewMetricsPlaceholderTile(),
			header:      "[Header Metrics]",
			sprintHints: []string{"Sprint 09"},
		},
		{
			name:        "logs placeholder",
			tile:        bento.NewLogsPlaceholderTile(),
			header:      "[Live Micro-Logs]",
			sprintHints: []string{"Sprint 09"},
		},
		{
			name:        "cicd placeholder",
			tile:        bento.NewCICDPlaceholderTile(),
			header:      "[CI/CD Pipeline]",
			sprintHints: []string{"Sprint 10"},
		},
		{
			name:        "topology placeholder",
			tile:        bento.NewTopologyPlaceholderTile(),
			header:      "[Topology]",
			sprintHints: []string{"Sprint 11"},
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
			for _, hint := range tc.sprintHints {
				if !strings.Contains(out, hint) {
					t.Fatalf("placeholder missing sprint hint %q\n%s", hint, out)
				}
			}
		})
	}
}

func TestTileIDsAreStableAndUnique(t *testing.T) {
	t.Parallel()

	tiles := []bento.BentoTile{
		bento.NewProjectsTile(nil),
		bento.NewOverviewTile("", nil),
		bento.NewMetricsPlaceholderTile(),
		bento.NewCICDPlaceholderTile(),
		bento.NewLogsPlaceholderTile(),
		bento.NewTopologyPlaceholderTile(),
	}

	seen := map[string]bool{}
	for _, tile := range tiles {
		id := tile.ID()
		if id == "" {
			t.Errorf("tile %T has empty ID", tile)
		}
		if seen[id] {
			t.Errorf("duplicate tile ID %q", id)
		}
		seen[id] = true
	}
}
