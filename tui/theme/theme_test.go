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
