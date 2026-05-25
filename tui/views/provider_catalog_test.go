package views

import (
	"strings"
	"testing"

	"github.com/dilitS/webox/tui/theme"
)

// TestRenderProviderCatalog_EmptyShowsOnboardingHint guarantees
// the renderer never panics on an empty registry and surfaces a
// remediation hint pointing operators at `webox doctor preset`.
func TestRenderProviderCatalog_EmptyShowsOnboardingHint(t *testing.T) {
	t.Parallel()

	out := RenderProviderCatalog(Screen{
		Width:  120,
		Height: 35,
		Styles: theme.NewStyles(theme.Default()),
	})
	if !strings.Contains(out, "Catalog is empty") {
		t.Errorf("empty catalog body missing onboarding hint:\n%s", out)
	}
	if !strings.Contains(out, "webox doctor preset") {
		t.Errorf("empty catalog body missing remediation command:\n%s", out)
	}
}

// TestRenderProviderCatalog_RendersGroupsAndStatusPills makes
// sure each row displays its preset id, status pill, and
// capability badges.
func TestRenderProviderCatalog_RendersGroupsAndStatusPills(t *testing.T) {
	t.Parallel()

	snap := ProviderCatalogSnapshot{
		Groups: []ProviderCatalogGroup{
			{
				Region: "Poland",
				Rows: []ProviderCatalogRow{
					{ID: "smallhost-devil", DisplayName: "smallhost.pl", Status: "verified", Badges: []string{"http", "ssh"}},
				},
			},
			{
				Region: "Europe",
				Rows: []ProviderCatalogRow{
					{ID: "ovh-cloud", DisplayName: "OVH", Status: "research", Badges: []string{"http"}},
				},
			},
		},
		SelectedID: "smallhost-devil",
	}
	out := RenderProviderCatalog(Screen{
		Width:   120,
		Height:  35,
		Catalog: snap,
		Styles:  theme.NewStyles(theme.Default()),
	})
	for _, needle := range []string{"smallhost-devil", "VERIFIED", "RESEARCH", "Poland", "Europe", "http"} {
		if !strings.Contains(out, needle) {
			t.Errorf("catalog body missing %q\n--- body ---\n%s", needle, out)
		}
	}
}

// TestRenderProviderCatalog_DetailBlockShowsWhenIDSet renders
// the deep-dive block when the snapshot carries a Detail with
// a non-empty ID.
func TestRenderProviderCatalog_DetailBlockShowsWhenIDSet(t *testing.T) {
	t.Parallel()

	snap := ProviderCatalogSnapshot{
		Groups: []ProviderCatalogGroup{
			{
				Region: "Poland",
				Rows: []ProviderCatalogRow{
					{ID: "smallhost-devil", DisplayName: "smallhost.pl", Status: "verified"},
				},
			},
		},
		SelectedID: "smallhost-devil",
		Detail: ProviderCatalogDetail{
			ID:           "smallhost-devil",
			DisplayName:  "smallhost.pl Devil",
			Status:       "verified",
			Region:       "Poland",
			PanelName:    "Devil",
			PanelAPI:     "ssh",
			PanelAPIPort: 22,
			DeployPath:   "/home/{user}/domains/{domain}/public_nodejs",
			LogPath:      "/home/{user}/domains/{domain}/logs/error.log",
		},
	}
	out := RenderProviderCatalog(Screen{
		Width:   140,
		Height:  40,
		Catalog: snap,
		Styles:  theme.NewStyles(theme.Default()),
	})
	for _, needle := range []string{
		"Detail",
		"smallhost.pl Devil",
		"deploy path",
		"public_nodejs",
		"ssh (:22)",
	} {
		if !strings.Contains(out, needle) {
			t.Errorf("detail body missing %q\n--- body ---\n%s", needle, out)
		}
	}
}

// TestRenderProviderCatalog_CopyHintRendered verifies the green
// ack appears when the model surfaces a CopyHint.
func TestRenderProviderCatalog_CopyHintRendered(t *testing.T) {
	t.Parallel()

	snap := ProviderCatalogSnapshot{
		Groups: []ProviderCatalogGroup{
			{
				Region: "Poland",
				Rows: []ProviderCatalogRow{
					{ID: "smallhost-devil", DisplayName: "smallhost.pl", Status: "verified"},
				},
			},
		},
		SelectedID: "smallhost-devil",
		CopyHint:   "briefing copied to clipboard",
	}
	out := RenderProviderCatalog(Screen{
		Width:   120,
		Height:  35,
		Catalog: snap,
		Styles:  theme.NewStyles(theme.Default()),
	})
	if !strings.Contains(out, "briefing copied to clipboard") {
		t.Errorf("body missing copy hint:\n%s", out)
	}
}

// TestRenderProviderCatalog_LoadErrorsRendered ensures registry
// load failures are surfaced inline rather than swallowed.
func TestRenderProviderCatalog_LoadErrorsRendered(t *testing.T) {
	t.Parallel()

	snap := ProviderCatalogSnapshot{
		Groups: []ProviderCatalogGroup{
			{
				Region: "Poland",
				Rows: []ProviderCatalogRow{
					{ID: "smallhost-devil", Status: "verified"},
				},
			},
		},
		SelectedID: "smallhost-devil",
		LoadErrors: []string{"missing-file: invalid JSON at line 4"},
	}
	out := RenderProviderCatalog(Screen{
		Width:   120,
		Height:  35,
		Catalog: snap,
		Styles:  theme.NewStyles(theme.Default()),
	})
	if !strings.Contains(out, "Load errors") || !strings.Contains(out, "missing-file") {
		t.Errorf("load error not surfaced:\n%s", out)
	}
}
