package initwizard_test

import (
	"strings"
	"testing"

	"github.com/dilitS/webox/tui/surface"
	"github.com/dilitS/webox/tui/surface/initwizard"
	"github.com/dilitS/webox/tui/theme"
	"github.com/dilitS/webox/tui/views"
)

// fixtureContext is a minimal renderable context — enough for the
// embedded `tui/views` renderer to produce a non-empty frame. The
// helper keeps each test body short and the magic terminal size
// hidden in one place.
func fixtureContext(step int) surface.Context {
	return surface.Context{
		Screen: views.Screen{
			Width:    100,
			Height:   30,
			Styles:   theme.NewStyles(theme.Default()),
			InitForm: views.InitWizardSnapshot{Step: step},
		},
	}
}

func TestSurface_BodyContainsWelcomeBranding(t *testing.T) {
	t.Parallel()

	got := initwizard.Surface{}.Body(fixtureContext(0))
	for _, needle := range []string{"first run setup", "[ Enter ] Continue"} {
		if !strings.Contains(got, needle) {
			t.Errorf("body missing %q\n--- body ---\n%s", needle, got)
		}
	}
}

func TestSurface_BodyDelegatesToViewRenderer(t *testing.T) {
	t.Parallel()

	ctx := fixtureContext(1)
	via := initwizard.Surface{}.Body(ctx)
	want := views.RenderInitWizard(ctx.Screen)

	if via != want {
		t.Errorf("surface body drift\n--- via surface ---\n%s\n--- via views ---\n%s", via, want)
	}
}

func TestSurface_Crumb(t *testing.T) {
	t.Parallel()

	if got := (initwizard.Surface{}).Crumb(fixtureContext(0)); got != "Init Wizard" {
		t.Errorf("Crumb = %q, want %q", got, "Init Wizard")
	}
}

func TestSurface_FooterCarriesGlobalHint(t *testing.T) {
	t.Parallel()

	hint := initwizard.Surface{}.Footer(fixtureContext(0))
	if hint.ScrollHint {
		t.Error("init wizard footer should not force a scroll hint")
	}
	for _, needle := range []string{"[q] quit", "[?] help", "[/] command palette", "[Tab] cycle panels"} {
		if !strings.Contains(hint.Text, needle) {
			t.Errorf("footer missing %q in %q", needle, hint.Text)
		}
	}
}

func TestSurface_RejectsScroll(t *testing.T) {
	t.Parallel()

	if (initwizard.Surface{}).AcceptsScroll(fixtureContext(0)) {
		t.Error("init wizard MUST NOT accept body scroll: PgUp/PgDn would steal runes from text inputs")
	}
}

// TestSurface_ContractCompiles is a compile-time guard: the package
// MUST satisfy [surface.Surface] without method-receiver shenanigans.
// A future refactor that turns the value receivers into pointers
// breaks this guard immediately.
func TestSurface_ContractCompiles(t *testing.T) {
	t.Parallel()

	var _ surface.Surface = initwizard.Surface{}
}
