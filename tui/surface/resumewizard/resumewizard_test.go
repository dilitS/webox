package resumewizard_test

import (
	"strings"
	"testing"

	"github.com/dilitS/webox/tui/surface"
	"github.com/dilitS/webox/tui/surface/resumewizard"
	"github.com/dilitS/webox/tui/theme"
	"github.com/dilitS/webox/tui/views"
)

func fixtureContext() surface.Context {
	return surface.Context{
		Screen: views.Screen{
			Width:  100,
			Height: 30,
			Styles: theme.NewStyles(theme.Default()),
			ResumeForm: views.ResumeWizardSnapshot{
				WizardID:     "wiz-2026-05-25",
				ProfileAlias: "main",
				UpdatedAt:    "2026-05-25 01:00:00 UTC",
				StepNames:    []string{"create-subdomain", "issue-ssl"},
			},
		},
	}
}

func TestSurface_BodyShowsSnapshotMetadata(t *testing.T) {
	t.Parallel()

	got := resumewizard.Surface{}.Body(fixtureContext())
	for _, needle := range []string{"[Resume Wizard]", "wiz-2026-05-25", "create-subdomain", "[ r ] Roll back now"} {
		if !strings.Contains(got, needle) {
			t.Errorf("body missing %q\n--- body ---\n%s", needle, got)
		}
	}
}

func TestSurface_BodyMatchesViewRenderer(t *testing.T) {
	t.Parallel()

	ctx := fixtureContext()
	via := resumewizard.Surface{}.Body(ctx)
	want := views.RenderResumeWizard(ctx.Screen)
	if via != want {
		t.Errorf("resume wizard body drift detected")
	}
}

func TestSurface_Crumb(t *testing.T) {
	t.Parallel()

	if got := (resumewizard.Surface{}).Crumb(fixtureContext()); got != "Resume Wizard" {
		t.Errorf("Crumb = %q, want %q", got, "Resume Wizard")
	}
}

func TestSurface_FooterCarriesGlobalHint(t *testing.T) {
	t.Parallel()

	hint := resumewizard.Surface{}.Footer(fixtureContext())
	if hint.ScrollHint {
		t.Error("resume wizard footer must not force scroll hint")
	}
	if !strings.Contains(hint.Text, "[q] quit") {
		t.Errorf("footer missing global hint, got %q", hint.Text)
	}
}

func TestSurface_RejectsScroll(t *testing.T) {
	t.Parallel()

	if (resumewizard.Surface{}).AcceptsScroll(fixtureContext()) {
		t.Error("resume wizard MUST decline scroll: discard phrase prompt consumes tea.KeyRunes")
	}
}

func TestSurface_ContractCompiles(t *testing.T) {
	t.Parallel()

	var _ surface.Surface = resumewizard.Surface{}
}
