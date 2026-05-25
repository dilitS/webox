package views_test

import (
	"strings"
	"testing"

	"github.com/dilitS/webox/config"
	"github.com/dilitS/webox/tui/theme"
	"github.com/dilitS/webox/tui/views"
)

// TestRenderDatabase_StandardCheatsheet confirms the static
// cheatsheet surfaces the conventional DB name + the documented
// connection commands. The renderer never reaches into a provider
// (read-only contract); every value comes from the project
// metadata or static defaults.
func TestRenderDatabase_StandardCheatsheet(t *testing.T) {
	t.Parallel()

	s := views.Screen{
		Width:  120,
		Height: 35,
		Styles: theme.NewStyles(theme.Default()),
		Config: &config.Config{
			Profiles: []config.Profile{{Alias: "main", Host: "s1.small.pl", Port: 22, User: "demo"}},
			Projects: []config.Project{{
				ID:           "p1",
				Domain:       "shop.demo.smallhost.pl",
				ProfileAlias: "main",
				Stack:        "node-express",
			}},
		},
		SelectedIndex: 0,
	}
	out := views.RenderDatabase(s)
	for _, needle := range []string{
		"[Project Detail: shop.demo.smallhost.pl]",
		"💾 [Database]",
		"Stack: node-express",
		"<devil_user>_shop", // conventional name derived from subdomain
		"mysql -h s1.small.pl",
		"psql -h s1.small.pl",
		"webox doctor db creds",
		"keyring",
	} {
		if !strings.Contains(out, needle) {
			t.Errorf("missing needle %q\n--- output ---\n%s", needle, out)
		}
	}
}

// TestRenderDatabase_HyphenatedDomainSlug exercises the slug
// normalization (- → _) so the rendered name matches the Devil
// panel's `<user>_<slug>` convention.
func TestRenderDatabase_HyphenatedDomainSlug(t *testing.T) {
	t.Parallel()

	s := views.Screen{
		Width:  120,
		Height: 35,
		Styles: theme.NewStyles(theme.Default()),
		Config: &config.Config{
			Profiles: []config.Profile{{Alias: "main", Host: "s1.small.pl", Port: 22, User: "demo"}},
			Projects: []config.Project{{
				ID:           "p1",
				Domain:       "auth-service.demo.smallhost.pl",
				ProfileAlias: "main",
			}},
		},
		SelectedIndex: 0,
	}
	out := views.RenderDatabase(s)
	if !strings.Contains(out, "<devil_user>_auth_service") {
		t.Errorf("expected slug `auth_service`, hyphen → underscore\n--- output ---\n%s", out)
	}
}

// TestRenderDatabase_MissingStackFallback confirms an empty stack
// renders the placeholder, not an empty cell.
func TestRenderDatabase_MissingStackFallback(t *testing.T) {
	t.Parallel()

	s := views.Screen{
		Width:  120,
		Height: 35,
		Styles: theme.NewStyles(theme.Default()),
		Config: &config.Config{
			Profiles: []config.Profile{{Alias: "main", Host: "s1.small.pl", Port: 22, User: "demo"}},
			Projects: []config.Project{{ID: "p1", Domain: "blog.demo.smallhost.pl", ProfileAlias: "main"}},
		},
		SelectedIndex: 0,
	}
	out := views.RenderDatabase(s)
	if !strings.Contains(out, "(no stack set)") {
		t.Errorf("missing stack fallback:\n%s", out)
	}
}

// TestRenderDatabase_NoProject covers the absent-project guard.
func TestRenderDatabase_NoProject(t *testing.T) {
	t.Parallel()

	s := views.Screen{
		Width:         120,
		Height:        35,
		Styles:        theme.NewStyles(theme.Default()),
		Config:        &config.Config{},
		SelectedIndex: -1,
	}
	out := views.RenderDatabase(s)
	if !strings.Contains(out, "No project selected") {
		t.Errorf("missing absent-project guard:\n%s", out)
	}
}
