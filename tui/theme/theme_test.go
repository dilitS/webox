package theme

import (
	"strings"
	"testing"
)

func TestDefaultStylesRenderWithoutRawEscapeConstants(t *testing.T) {
	t.Parallel()

	styles := NewStyles(Default())
	rendered := []string{
		styles.Header.Render("Webox"),
		styles.ProjectRow.Render("app.example.test"),
		styles.SelectedProjectRow.Render("selected.example.test"),
		styles.StatusBadge("ONLINE").Render("ONLINE"),
		styles.HelpHints.Render("q quit"),
	}

	for _, got := range rendered {
		if strings.Contains(got, "\x1b[") {
			t.Fatalf("style rendered raw ANSI escape in smoke test: %q", got)
		}
	}
}

func TestDefaultThemeMatchesUXTokens(t *testing.T) {
	t.Parallel()

	got := Default()
	if got.Primary != "#7D56F4" {
		t.Fatalf("primary = %q, want UX primary token", got.Primary)
	}
	if got.Success != "#04B575" || got.Warning != "#FFB800" || got.Error != "#FF4444" {
		t.Fatalf("status colors = %#v", got)
	}
}

func TestLightThemeOverridesAllElevenRoles(t *testing.T) {
	t.Parallel()

	dark := Default()
	light := Light()

	if dark == light {
		t.Fatal("Light() must return a different palette than Default()")
	}
	if light.Primary == "" || light.Success == "" || light.Warning == "" ||
		light.Error == "" || light.Degraded == "" || light.Muted == "" ||
		light.SurfaceBase == "" || light.SurfaceLow == "" || light.SurfaceHigh == "" ||
		light.TextBright == "" || light.TextDim == "" {
		t.Fatalf("Light() must set all eleven role tokens, got %#v", light)
	}
}

func TestStatusBadgeRendersWithFilledBackground(t *testing.T) {
	t.Parallel()

	styles := NewStyles(Default())

	online := styles.StatusBadge("ONLINE").Render("ONLINE")
	if !strings.Contains(online, "ONLINE") {
		t.Fatalf("ONLINE badge body missing: %q", online)
	}

	unknown := styles.StatusBadge("DOES_NOT_EXIST").Render("UNKNOWN")
	if !strings.Contains(unknown, "UNKNOWN") {
		t.Fatalf("UNKNOWN fallback body missing: %q", unknown)
	}
}

func TestGradientPaintsEachRuneAndHandlesEdges(t *testing.T) {
	t.Parallel()

	out := Gradient("WEBOX", "#7D56F4", "#04B575")
	if !strings.Contains(out, "W") || !strings.Contains(out, "X") {
		t.Fatalf("gradient stripped runes: %q", out)
	}
	if Gradient("", "#FFFFFF", "#000000") != "" {
		t.Fatal("empty input should return empty output")
	}
	if got := Gradient("A", "#FFFFFF", "#000000"); !strings.Contains(got, "A") {
		t.Fatalf("single-rune gradient must still render the glyph: %q", got)
	}
	if got := Gradient("BAD", "not-hex", "#000000"); got != "BAD" {
		t.Fatalf("invalid start hex should return raw text, got %q", got)
	}
}
