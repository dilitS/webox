package bento_test

import (
	"strings"
	"testing"

	"github.com/dilitS/webox/tui/bento"
)

func TestDetectMode(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		width  int
		height int
		want   bento.Mode
	}{
		{"zero values fall back to standard", 0, 0, bento.ModeStandard},
		{"tiny below 70x22 triggers fallback", 60, 20, bento.ModeTiny},
		{"exact 70x22 lowest viable standard", 70, 22, bento.ModeStandard},
		{"80x24 classic terminal is standard", 80, 24, bento.ModeStandard},
		{"recommended 100x30 is standard", 100, 30, bento.ModeStandard},
		{"119x34 just below Ultra threshold is standard", 119, 34, bento.ModeStandard},
		{"width meets Ultra threshold but height short stays standard", 120, 34, bento.ModeStandard},
		{"height meets Ultra threshold but width short stays standard", 119, 35, bento.ModeStandard},
		{"120x35 unlocks Ultra", 120, 35, bento.ModeUltra},
		{"between Ultra and Plus stays Ultra", 140, 40, bento.ModeUltra},
		{"159x44 just below Plus stays Ultra", 159, 44, bento.ModeUltra},
		{"160x45 unlocks UltraPlus", 160, 45, bento.ModeUltraPlus},
		{"huge terminal stays UltraPlus", 220, 60, bento.ModeUltraPlus},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := bento.DetectMode(tc.width, tc.height)
			if got != tc.want {
				t.Fatalf("DetectMode(%d, %d) = %s, want %s", tc.width, tc.height, got, tc.want)
			}
		})
	}
}

func TestEngineRendersTinyFallback(t *testing.T) {
	t.Parallel()

	engine := bento.NewEngine("Webox Cockpit v0.1", nil)
	out := engine.Render(60, 18)

	needles := []string{"Terminal too small", "Tiny fallback", "100", "30"}
	for _, needle := range needles {
		if !strings.Contains(out, needle) {
			t.Fatalf("tiny fallback missing %q\n--- output ---\n%s", needle, out)
		}
	}
}

func TestEngineRendersUltraSurfacesAllBentoTiles(t *testing.T) {
	t.Parallel()

	registry := bento.NewRegistry()
	registry.Register(bento.NewProjectsTile([]string{"> app.example.com [ONLINE]"}))
	registry.Register(bento.NewOverviewTile("app.example.com", []string{"HTTP: 200", "SSL: 30 days"}))
	registry.Register(bento.NewMetricsPlaceholderTile())
	registry.Register(bento.NewCICDPlaceholderTile())
	registry.Register(bento.NewLogsPlaceholderTile())
	registry.Register(bento.NewTopologyPlaceholderTile())

	engine := bento.NewEngine("Webox Cockpit v0.1", registry.Tiles())
	out := engine.Render(120, 35)

	needles := []string{
		"Webox Cockpit v0.1",
		"[BENTO Ultra]",
		"[Projects]",
		"[Overview]",
		"[Header Metrics]",
		"[CI/CD Pipeline]",
		"[Live Micro-Logs]",
		"[Topology]",
		"app.example.com",
		"> app.example.com [ONLINE]",
		"HTTP: 200",
	}
	for _, needle := range needles {
		if !strings.Contains(out, needle) {
			t.Fatalf("Ultra render missing %q\n--- output ---\n%s", needle, out)
		}
	}
}

func TestEngineRendersUltraPlusGetsExtraSection(t *testing.T) {
	t.Parallel()

	engine := bento.NewEngine("Webox Cockpit v0.1", []bento.BentoTile{
		bento.NewProjectsTile([]string{}),
		bento.NewOverviewTile("", []string{"empty"}),
		bento.NewMetricsPlaceholderTile(),
		bento.NewCICDPlaceholderTile(),
		bento.NewLogsPlaceholderTile(),
		bento.NewTopologyPlaceholderTile(),
	})
	out := engine.Render(160, 45)

	if !strings.Contains(out, "[BENTO Ultra+]") {
		t.Fatalf("UltraPlus mode marker missing\n--- output ---\n%s", out)
	}
}

func TestModeString(t *testing.T) {
	t.Parallel()

	cases := map[bento.Mode]string{
		bento.ModeTiny:      "Tiny",
		bento.ModeStandard:  "Standard",
		bento.ModeUltra:     "Ultra",
		bento.ModeUltraPlus: "UltraPlus",
	}
	for mode, want := range cases {
		if got := mode.String(); got != want {
			t.Errorf("%v.String() = %q, want %q", mode, got, want)
		}
	}
}

func TestRegistryRegistersAndReturnsTiles(t *testing.T) {
	t.Parallel()

	r := bento.NewRegistry()
	if got := r.Tiles(); len(got) != 0 {
		t.Fatalf("empty registry should return empty slice, got %d", len(got))
	}
	r.Register(bento.NewMetricsPlaceholderTile())
	r.Register(bento.NewLogsPlaceholderTile())
	if got := r.Tiles(); len(got) != 2 {
		t.Fatalf("registry size = %d, want 2", len(got))
	}
}
