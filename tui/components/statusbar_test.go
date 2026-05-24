package components_test

import (
	"strings"
	"testing"

	"github.com/dilitS/webox/tui/components"
)

func TestRenderStatusBar_BrandAndPill(t *testing.T) {
	t.Parallel()

	out := components.RenderStatusBar(components.StatusBarOptions{
		Brand:     "WEBOX v1.4.2",
		LiveLabel: "LIVE",
		Tone:      components.ToneSuccess,
		Sections:  []string{"Uptime: 24d 11h", "RAM: 3.4/8.0 GB (42%)", "Ping: 18ms"},
		Width:     120,
	})

	for _, needle := range []string{
		"WEBOX v1.4.2",
		"LIVE",
		"Uptime: 24d 11h",
		"RAM: 3.4/8.0 GB (42%)",
		"Ping: 18ms",
	} {
		if !strings.Contains(out, needle) {
			t.Fatalf("status bar missing %q\n--- output ---\n%s", needle, out)
		}
	}
}

func TestRenderStatusBar_EmptyBrandFallsBackToWEBOX(t *testing.T) {
	t.Parallel()

	out := components.RenderStatusBar(components.StatusBarOptions{
		Brand: "",
		Width: 80,
	})
	if !strings.Contains(out, "WEBOX") {
		t.Fatalf("expected default WEBOX brand, got:\n%s", out)
	}
}

func TestRenderStatusBar_SkipsEmptySections(t *testing.T) {
	t.Parallel()

	out := components.RenderStatusBar(components.StatusBarOptions{
		Brand:    "WEBOX v0.1",
		Sections: []string{"", "Uptime: 1h", " "},
		Width:    60,
	})

	if !strings.Contains(out, "Uptime: 1h") {
		t.Fatalf("expected Uptime cell, got:\n%s", out)
	}
	if strings.Count(out, "│") > 1 {
		t.Fatalf("expected ≤1 separator after skipping empties, got %d:\n%s", strings.Count(out, "│"), out)
	}
}

func TestRenderStatusBar_NoWidth_NoPadding(t *testing.T) {
	t.Parallel()

	out := components.RenderStatusBar(components.StatusBarOptions{
		Brand:    "WEBOX",
		Sections: []string{"X: Y"},
	})

	if strings.Contains(out, strings.Repeat(" ", 20)) {
		t.Fatalf("expected compact bar when width=0, got:\n%s", out)
	}
}
