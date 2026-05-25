package catalog

import (
	"strings"
	"testing"

	"github.com/dilitS/webox/tui/surface"
	"github.com/dilitS/webox/tui/theme"
	"github.com/dilitS/webox/tui/views"
)

// TestSurface_BodyRendersHeader smoke-tests the surface adapter
// by handing it a hand-rolled snapshot and asserting the View
// layer paints the catalog header.
func TestSurface_BodyRendersHeader(t *testing.T) {
	t.Parallel()

	ctx := surface.Context{
		Screen: views.Screen{
			Width:  120,
			Height: 35,
			Styles: theme.NewStyles(theme.Default()),
			Catalog: views.ProviderCatalogSnapshot{
				Groups: []views.ProviderCatalogGroup{
					{
						Region: "Poland",
						Rows: []views.ProviderCatalogRow{
							{ID: "smallhost-devil", DisplayName: "smallhost.pl", Status: "verified"},
						},
					},
				},
				SelectedID: "smallhost-devil",
			},
		},
	}
	out := Surface{}.Body(ctx)
	if !strings.Contains(out, "Provider Catalog") {
		t.Errorf("body missing screen header:\n%s", out)
	}
	if !strings.Contains(out, "smallhost-devil") {
		t.Errorf("body missing catalog row:\n%s", out)
	}
}

// TestSurface_CrumbAndFooterAdvertiseKeys is the chrome
// contract: the operator should always see the keys for the
// active surface inside the chrome footer.
func TestSurface_CrumbAndFooterAdvertiseKeys(t *testing.T) {
	t.Parallel()

	ctx := surface.Context{Screen: views.Screen{Styles: theme.NewStyles(theme.Default())}}
	s := Surface{}
	if got := s.Crumb(ctx); got != "Provider Catalog" {
		t.Errorf("crumb = %q, want %q", got, "Provider Catalog")
	}
	hint := s.Footer(ctx).Text
	for _, needle := range []string{"[c] copy briefing", "[Enter] toggle detail", "[Esc] back"} {
		if !strings.Contains(hint, needle) {
			t.Errorf("footer hint missing %q\n--- hint ---\n%s", needle, hint)
		}
	}
}
