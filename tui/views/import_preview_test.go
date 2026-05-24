package views_test

import (
	"strings"
	"testing"

	"github.com/dilitS/webox/tui/theme"
	"github.com/dilitS/webox/tui/views"
)

func TestRenderImportPreview_Loading(t *testing.T) {
	t.Parallel()
	s := views.Screen{
		Width:  100,
		Height: 30,
		Styles: theme.NewStyles(theme.Default()),
		ImportForm: views.ImportPreviewSnapshot{
			Loading: true,
		},
		Spinner: "...",
	}
	out := views.RenderImportPreview(s)
	if !strings.Contains(out, "Import Existing Projects") {
		t.Fatalf("missing header:\n%s", out)
	}
	if !strings.Contains(out, "scanning") {
		t.Fatalf("missing scanning hint:\n%s", out)
	}
}

func TestRenderImportPreview_RowsTable(t *testing.T) {
	t.Parallel()
	s := views.Screen{
		Width:  120,
		Height: 30,
		Styles: theme.NewStyles(theme.Default()),
		ImportForm: views.ImportPreviewSnapshot{
			Total:     2,
			Managed:   1,
			Unmanaged: 1,
			Rows: []views.ImportRowSnapshot{
				{Domain: "managed.demo.smallhost.pl", Type: "nodejs", NodeVersion: "20", ProfileAlias: "main", Managed: true},
				{Domain: "ghost.demo.smallhost.pl", Type: "nodejs", NodeVersion: "22", ProfileAlias: "main"},
			},
		},
	}
	out := views.RenderImportPreview(s)
	for _, needle := range []string{"managed.demo", "ghost.demo", "managed)", "new)"} {
		if !strings.Contains(out, needle) {
			t.Fatalf("expected %q in render:\n%s", needle, out)
		}
	}
}

func TestRenderImportPreview_Error(t *testing.T) {
	t.Parallel()
	s := views.Screen{
		Width:  100,
		Height: 30,
		Styles: theme.NewStyles(theme.Default()),
		ImportForm: views.ImportPreviewSnapshot{
			Err: "ssh unavailable",
		},
	}
	out := views.RenderImportPreview(s)
	if !strings.Contains(out, "ssh unavailable") {
		t.Fatalf("expected error in render:\n%s", out)
	}
}

func TestRenderImportPreview_Truncates(t *testing.T) {
	t.Parallel()
	rows := make([]views.ImportRowSnapshot, 20)
	for i := range rows {
		rows[i] = views.ImportRowSnapshot{Domain: "row.demo.smallhost.pl", Type: "nodejs"}
	}
	s := views.Screen{
		Width:  120,
		Height: 30,
		Styles: theme.NewStyles(theme.Default()),
		ImportForm: views.ImportPreviewSnapshot{
			Total: len(rows),
			Rows:  rows,
		},
	}
	out := views.RenderImportPreview(s)
	if !strings.Contains(out, "more row(s) hidden") {
		t.Fatalf("expected truncation hint:\n%s", out)
	}
}
