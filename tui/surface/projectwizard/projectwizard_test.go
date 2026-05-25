package projectwizard_test

import (
	"strings"
	"testing"

	"github.com/dilitS/webox/config"
	"github.com/dilitS/webox/tui/surface"
	"github.com/dilitS/webox/tui/surface/projectwizard"
	"github.com/dilitS/webox/tui/theme"
	"github.com/dilitS/webox/tui/views"
)

// fixtureContext returns a wizard context anchored at the supplied
// step. The wizard renderer needs at least one profile in the
// snapshot to render the picker without surfacing the "No profile
// configured" alert, so we seed one.
func fixtureContext(step int) surface.Context {
	return surface.Context{
		Screen: views.Screen{
			Width:  100,
			Height: 30,
			Styles: theme.NewStyles(theme.Default()),
			Config: &config.Config{
				Profiles: []config.Profile{{
					Alias: "main", Host: "s1.small.pl", Port: 22, User: "demo",
				}},
			},
			ProjectForm: views.ProjectWizardSnapshot{
				Step:         step,
				ProfileAlias: "main",
				Stack:        "node-express",
				NodeVersion:  "v24.15.0",
				Domain:       "demo.smallhost.pl",
			},
		},
	}
}

func TestSurface_BodyShowsWizardHeader(t *testing.T) {
	t.Parallel()

	got := projectwizard.Surface{}.Body(fixtureContext(0))
	if !strings.Contains(got, "[New Project Wizard]") {
		t.Errorf("body missing wizard header\n--- body ---\n%s", got)
	}
}

func TestSurface_BodyMatchesViewRenderer(t *testing.T) {
	t.Parallel()

	ctx := fixtureContext(4) // domain step
	via := projectwizard.Surface{}.Body(ctx)
	want := views.RenderProjectWizard(ctx.Screen)
	if via != want {
		t.Errorf("project wizard body drift\n--- via surface ---\n%s\n--- via views ---\n%s", via, want)
	}
}

func TestSurface_Crumb(t *testing.T) {
	t.Parallel()

	if got := (projectwizard.Surface{}).Crumb(fixtureContext(0)); got != "Project Wizard" {
		t.Errorf("Crumb = %q, want %q", got, "Project Wizard")
	}
}

func TestSurface_FooterCarriesGlobalHint(t *testing.T) {
	t.Parallel()

	hint := projectwizard.Surface{}.Footer(fixtureContext(0))
	if hint.ScrollHint {
		t.Error("project wizard footer must not force scroll hint")
	}
	if !strings.Contains(hint.Text, "[q] quit") {
		t.Errorf("footer missing global hint, got %q", hint.Text)
	}
}

func TestSurface_RejectsScroll(t *testing.T) {
	t.Parallel()

	if (projectwizard.Surface{}).AcceptsScroll(fixtureContext(0)) {
		t.Error("project wizard MUST decline scroll: input steps consume tea.KeyRunes verbatim")
	}
}

func TestSurface_ContractCompiles(t *testing.T) {
	t.Parallel()

	var _ surface.Surface = projectwizard.Surface{}
}
