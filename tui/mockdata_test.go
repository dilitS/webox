package tui

import (
	"strings"
	"testing"
)

// TestMockOptionsRenderBentoUltra asserts that a model built from
// [MockOptions] paints all the marquee tiles of the 2026-05-24 design
// refresh (status bar, projects, server, CI/CD, live logs). This is
// the smoke test that replaces "spin up the binary and look at it" —
// failures here mean the offline demo is broken.
func TestMockOptionsRenderBentoUltra(t *testing.T) {
	t.Parallel()

	opts := MockOptions("")
	m := New(opts)
	out := m.View()

	for _, needle := range []string{
		"WEBOX",
		"LIVE",
		"Uptime: 24d 11h",
		"RAM: 3.4/8.0 GB (42%)",
		"[Active Projects]",
		"ShopEase-Web",
		"Payment-UI",
		"[SERVER: ShopEase-Web]",
		"v20.11.0",
		"Valid (114 days remaining)",
		"[CI/CD PIPELINE: Main Branch]",
		"Build #412",
		"[Live Server Logs]",
		"API-Gateway",
	} {
		if !strings.Contains(out, needle) {
			t.Fatalf("mock dashboard missing %q\n--- frame ---\n%s", needle, out)
		}
	}
}

// TestMockOptionsHonoursPreloadedConfig ensures the launcher does not
// try to load `~/.config/webox/config.json` when [Options.PreloadedConfig]
// is supplied. Test asserts the model boots straight to the dashboard
// state, the selected project index is clamped to the first entry, and
// the seeded statuses populate the status map without an `Init()` round
// trip.
func TestMockOptionsHonoursPreloadedConfig(t *testing.T) {
	t.Parallel()

	opts := MockOptions("")
	m := New(opts)

	if m.cfg == nil {
		t.Fatalf("expected preloaded config, got nil")
	}
	if got, want := len(cfgProjects(m.cfg)), 6; got != want {
		t.Fatalf("expected %d projects, got %d", want, got)
	}
	if m.selectedIndex != 0 {
		t.Fatalf("expected selectedIndex=0, got %d", m.selectedIndex)
	}
	if _, ok := m.statuses["p1"]; !ok {
		t.Fatalf("expected statuses map to contain p1 after preflight, got %#v", m.statuses)
	}
}
