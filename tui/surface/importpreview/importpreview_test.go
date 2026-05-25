package importpreview_test

import (
	"strings"
	"testing"

	"github.com/dilitS/webox/tui/surface"
	"github.com/dilitS/webox/tui/surface/importpreview"
	"github.com/dilitS/webox/tui/theme"
	"github.com/dilitS/webox/tui/views"
)

func fixtureContext() surface.Context {
	return surface.Context{
		Screen: views.Screen{
			Width:  100,
			Height: 30,
			Styles: theme.NewStyles(theme.Default()),
			ImportForm: views.ImportPreviewSnapshot{
				Total:     2,
				Managed:   1,
				Unmanaged: 1,
				Rows: []views.ImportRowSnapshot{
					{ProfileAlias: "main", Domain: "demo.smallhost.pl", Type: "node", NodeVersion: "v24", Managed: true},
					{ProfileAlias: "main", Domain: "stage.smallhost.pl", Type: "node", NodeVersion: "v22", Managed: false},
				},
			},
		},
	}
}

func TestSurface_BodyShowsImportSummary(t *testing.T) {
	t.Parallel()

	got := importpreview.Surface{}.Body(fixtureContext())
	for _, needle := range []string{"[Import Existing Projects]", "demo.smallhost.pl", "stage.smallhost.pl", "[a] Import all unmanaged"} {
		if !strings.Contains(got, needle) {
			t.Errorf("body missing %q\n--- body ---\n%s", needle, got)
		}
	}
}

func TestSurface_BodyMatchesViewRenderer(t *testing.T) {
	t.Parallel()

	ctx := fixtureContext()
	via := importpreview.Surface{}.Body(ctx)
	want := views.RenderImportPreview(ctx.Screen)
	if via != want {
		t.Errorf("import preview body drift detected")
	}
}

func TestSurface_Crumb(t *testing.T) {
	t.Parallel()

	if got := (importpreview.Surface{}).Crumb(fixtureContext()); got != "Import Preview" {
		t.Errorf("Crumb = %q, want %q", got, "Import Preview")
	}
}

func TestSurface_FooterPublishesImportKeys(t *testing.T) {
	t.Parallel()

	hint := importpreview.Surface{}.Footer(fixtureContext())
	if hint.ScrollHint {
		t.Error("import preview footer must not force scroll hint")
	}
	for _, needle := range []string{"[q] quit", "[a] import all", "[Esc] back"} {
		if !strings.Contains(hint.Text, needle) {
			t.Errorf("footer missing %q in %q", needle, hint.Text)
		}
	}
	if strings.Contains(hint.Text, "command palette") {
		t.Errorf("import preview footer must not advertise unimplemented command palette: %q", hint.Text)
	}
}

func TestSurface_AcceptsScroll(t *testing.T) {
	t.Parallel()

	if !(importpreview.Surface{}).AcceptsScroll(fixtureContext()) {
		t.Error("import preview must accept scroll: panel-reported subdomain lists can overflow")
	}
}

func TestSurface_ContractCompiles(t *testing.T) {
	t.Parallel()

	var _ surface.Surface = importpreview.Surface{}
}
