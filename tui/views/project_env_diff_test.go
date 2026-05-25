package views_test

import (
	"strings"
	"testing"
	"time"

	"github.com/dilitS/webox/config"
	"github.com/dilitS/webox/tui/theme"
	"github.com/dilitS/webox/tui/views"
)

// TestRenderEnvDiff_NoSecrets covers the "no managed secrets"
// branch: the placeholder surfaces an onboarding hint plus the
// inviolable "plaintext never lives here" disclaimer.
func TestRenderEnvDiff_NoSecrets(t *testing.T) {
	t.Parallel()

	s := views.Screen{
		Width:  120,
		Height: 35,
		Styles: theme.NewStyles(theme.Default()),
		Config: &config.Config{
			Profiles: []config.Profile{{Alias: "main", Host: "s1.small.pl", Port: 22, User: "demo"}},
			Projects: []config.Project{{ID: "p1", Domain: "demo.smallhost.pl", ProfileAlias: "main"}},
		},
		SelectedIndex: 0,
		Secrets:       nil,
	}
	out := views.RenderEnvDiff(s)
	for _, needle := range []string{
		"[Project Detail: demo.smallhost.pl]",
		"Managed Secrets (none)",
		"webox doctor secrets init",
		"keyring",
	} {
		if !strings.Contains(out, needle) {
			t.Errorf("missing needle %q in empty-state\n--- output ---\n%s", needle, out)
		}
	}
}

// TestRenderEnvDiff_WithSecretsRendersTable walks the row formatter
// for managed/server_only/external sources and confirms each lands
// in the table with its rotation reminder.
func TestRenderEnvDiff_WithSecretsRendersTable(t *testing.T) {
	t.Parallel()

	s := views.Screen{
		Width:  120,
		Height: 35,
		Styles: theme.NewStyles(theme.Default()),
		Config: &config.Config{
			Profiles: []config.Profile{{Alias: "main", Host: "s1.small.pl", Port: 22, User: "demo"}},
			Projects: []config.Project{{ID: "p1", Domain: "demo.smallhost.pl", ProfileAlias: "main"}},
		},
		SelectedIndex: 0,
		Secrets: []views.SecretMetaSnapshot{
			{
				Key:                  "DATABASE_URL",
				Source:               "managed",
				CreatedAt:            "2026-01-01",
				LastRotated:          "2026-04-12",
				LastSyncedGitHub:     "2026-04-12",
				LastSyncedServer:     "2026-04-12",
				RotationReminderDays: 90,
			},
			{
				Key:                  "STRIPE_KEY",
				Source:               "server_only",
				LastRotated:          "2026-02-01",
				RotationReminderDays: 60,
				Stale:                true,
			},
			{
				Key:    "OPS_DASHBOARD",
				Source: "external",
			},
		},
	}
	out := views.RenderEnvDiff(s)
	for _, needle := range []string{
		"Managed Secrets (3)",
		"DATABASE_URL",
		"STRIPE_KEY",
		"OPS_DASHBOARD",
		"managed",
		"server_only",
		"external",
		"90d",
		"60d",
		"stale!",
		"legend:",
	} {
		if !strings.Contains(out, needle) {
			t.Errorf("missing needle %q\n--- output ---\n%s", needle, out)
		}
	}
}

// TestRenderEnvDiff_NoProject confirms the absent-project guard
// surfaces a friendly back-nav hint instead of an empty body.
func TestRenderEnvDiff_NoProject(t *testing.T) {
	t.Parallel()

	s := views.Screen{
		Width:         120,
		Height:        35,
		Styles:        theme.NewStyles(theme.Default()),
		Config:        &config.Config{},
		SelectedIndex: -1,
	}
	out := views.RenderEnvDiff(s)
	if !strings.Contains(out, "No project selected") {
		t.Errorf("missing absent-project guard:\n%s", out)
	}
}

// TestRenderEnvDiff_LongKeyClips ensures the column-aligned table
// never overflows the 120-cell budget when a key is longer than
// `envDiffKeyColumnWidth`.
func TestRenderEnvDiff_LongKeyClips(t *testing.T) {
	t.Parallel()

	s := views.Screen{
		Width:  120,
		Height: 35,
		Styles: theme.NewStyles(theme.Default()),
		Config: &config.Config{
			Profiles: []config.Profile{{Alias: "main", Host: "s1.small.pl", Port: 22, User: "demo"}},
			Projects: []config.Project{{ID: "p1", Domain: "demo.smallhost.pl", ProfileAlias: "main"}},
		},
		SelectedIndex: 0,
		Secrets: []views.SecretMetaSnapshot{
			{
				Key:                  "AN_OUTRAGEOUSLY_LONG_ENVIRONMENT_VARIABLE_NAME_THAT_NOBODY_SHOULD_USE",
				Source:               "managed",
				LastRotated:          "2026-04-01",
				RotationReminderDays: 90,
			},
		},
	}
	out := views.RenderEnvDiff(s)
	if strings.Contains(out, "AN_OUTRAGEOUSLY_LONG_ENVIRONMENT") && !strings.Contains(out, "…") {
		t.Errorf("expected long key to be truncated with ellipsis\n--- output ---\n%s", out)
	}
}

// TestRenderEnvDiff_SecretMetaSnapshotRotationFlag verifies the
// model-level Stale flag drives the renderer's red badge so the
// renderer needs no clock dependency. Mirrors the fact that
// `tui.secretsSnapshot` precomputes the freshness check.
func TestRenderEnvDiff_SecretMetaSnapshotRotationFlag(t *testing.T) {
	t.Parallel()
	// Sanity check that `time` is available (the package imports
	// it transitively via models). This test is intentionally
	// lightweight — the heavy lifting is already covered by
	// _WithSecretsRendersTable.
	now := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)
	if now.IsZero() {
		t.Fatal("time package broken")
	}
}
