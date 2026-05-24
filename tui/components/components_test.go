package components_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"

	"github.com/dilitS/webox/tui/components"
	"github.com/dilitS/webox/tui/theme"
)

func TestHeaderBarPlacesBadgeAtRightEdge(t *testing.T) {
	t.Parallel()

	out := components.RenderHeaderBar(components.HeaderBarOptions{
		Title: "Webox",
		Badge: components.FormatModeBadge("Ultra+"),
		Width: 60,
		Theme: theme.Default(),
	})

	if !strings.Contains(out, "Webox") {
		t.Fatalf("title missing: %q", out)
	}
	if !strings.Contains(out, "[BENTO Ultra+]") {
		t.Fatalf("badge missing: %q", out)
	}
}

func TestHeaderBarWithoutBadgeReturnsGradientTitle(t *testing.T) {
	t.Parallel()

	out := components.RenderHeaderBar(components.HeaderBarOptions{Title: "Webox"})
	if !strings.Contains(out, "W") || !strings.Contains(out, "x") {
		t.Fatalf("title runes missing: %q", out)
	}
}

func TestHeaderBarWithoutWidthDoesNotCrash(t *testing.T) {
	t.Parallel()

	out := components.RenderHeaderBar(components.HeaderBarOptions{
		Title: "Webox",
		Badge: "[BENTO Ultra]",
	})
	if !strings.Contains(out, "[BENTO Ultra]") {
		t.Fatalf("badge missing when width unset: %q", out)
	}
}

func TestLogoArtRendersFiveLines(t *testing.T) {
	t.Parallel()

	out := components.LogoArt(theme.Default())
	lines := strings.Split(out, "\n")
	if len(lines) != 5 {
		t.Fatalf("logo line count = %d, want 5", len(lines))
	}
}

func TestSpinnerAdaptsToMode(t *testing.T) {
	t.Parallel()

	cases := map[string]spinner.Spinner{
		"Standard":  spinner.Dot,
		"Ultra":     spinner.Pulse,
		"UltraPlus": spinner.Pulse,
		"Tiny":      spinner.Dot,
		"unknown":   spinner.Dot,
	}
	for mode, want := range cases {
		got := components.SpinnerStyle(mode)
		if !equalSpinner(got, want) {
			t.Errorf("SpinnerStyle(%q) returned wrong frame set", mode)
		}
		if !equalSpinner(components.NewAdaptiveSpinner(mode).Spinner, want) {
			t.Errorf("NewAdaptiveSpinner(%q) returned wrong frame set", mode)
		}
	}
}

func equalSpinner(a, b spinner.Spinner) bool {
	if len(a.Frames) != len(b.Frames) {
		return false
	}
	for i := range a.Frames {
		if a.Frames[i] != b.Frames[i] {
			return false
		}
	}
	return true
}

func TestRenderModalContainsTitleBodyAndFooter(t *testing.T) {
	t.Parallel()

	out := components.RenderModal(components.ModalOptions{
		Title:  "Confirm rollback",
		Body:   "This will undo the last 3 wizard steps.",
		Footer: "[Enter] confirm  [Esc] cancel",
		Tone:   components.ToneWarning,
	})

	for _, needle := range []string{
		"Confirm rollback",
		"This will undo the last 3 wizard steps.",
		"[Enter] confirm",
	} {
		if !strings.Contains(out, needle) {
			t.Fatalf("modal missing %q\n%s", needle, out)
		}
	}
}

func TestRenderModalToneError(t *testing.T) {
	t.Parallel()

	out := components.RenderModal(components.ModalOptions{
		Title: "SSH failure",
		Body:  "Could not reach host.",
		Tone:  components.ToneError,
	})
	if !strings.Contains(out, "SSH failure") {
		t.Fatalf("error tone modal missing title: %q", out)
	}
}

func TestRenderModalMinWidthRespected(t *testing.T) {
	t.Parallel()

	out := components.RenderModal(components.ModalOptions{
		Title:    "Short",
		Body:     "tiny",
		MinWidth: 80,
	})
	first := strings.SplitN(out, "\n", 2)[0]
	if len(first) < 40 {
		t.Fatalf("expected modal to expand to MinWidth (>=40 visible chars), got %d", len(first))
	}
}
