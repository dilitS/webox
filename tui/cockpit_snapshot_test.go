package tui

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestCockpitSnapshots produces deterministic Bento cockpit renders for
// every supported layout tier. The test runs as a normal `go test` step
// (so the View() call paths stay covered) but only writes the rendered
// strings to docs/screenshots/ when the operator opts in via the
// `WEBOX_SNAPSHOT=1` environment variable.
//
// Rationale:
//   - CI keeps the test in the happy-path: rendering must succeed and the
//     needle assertions guard against regressions.
//   - The opt-in side effect prevents accidental git churn when a
//     developer runs `go test ./...` locally.
//
// To refresh the in-repo screenshots run:
//
//	WEBOX_SNAPSHOT=1 go test ./tui -run TestCockpitSnapshots
func TestCockpitSnapshots(t *testing.T) {
	cases := []struct {
		name    string
		width   int
		height  int
		needles []string
	}{
		{
			name:    "standard-100x30",
			width:   100,
			height:  30,
			needles: []string{"Webox Cockpit", "Projects", "Overview"},
		},
		{
			name:    "bento-ultra-120x35",
			width:   120,
			height:  35,
			needles: []string{"[BENTO Ultra]", "[Projects]", "[Overview]", "[CI/CD Pipeline]"},
		},
		{
			name:    "bento-ultraplus-160x45",
			width:   160,
			height:  45,
			needles: []string{"[BENTO Ultra+]", "Deep-dive strip"},
		},
		{
			name:    "tiny-fallback-60x18",
			width:   60,
			height:  18,
			needles: []string{"Terminal too small", "Tiny fallback"},
		},
	}

	write := os.Getenv("WEBOX_SNAPSHOT") == "1"
	outDir := filepath.Join("..", "docs", "screenshots")
	if write {
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			t.Fatalf("create snapshot dir: %v", err)
		}
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			m := New(Options{InitialWidth: tc.width, InitialHeight: tc.height}).
				withConfig(fixtureConfig()).
				withStatuses(map[string]ProjectStatus{
					"p1": {
						ProjectID:   "p1",
						State:       ProjectOnline,
						HTTPHealth:  "200 OK",
						SSLDaysLeft: 27,
						NodeVersion: "v24.15.0",
						LastDeploy:  "2h ago",
					},
					"p2": {
						ProjectID:   "p2",
						State:       ProjectStale,
						HTTPHealth:  "stale",
						SSLDaysLeft: -1,
						NodeVersion: "v20.12.2",
						LastDeploy:  "unknown",
					},
				})

			out := m.View()
			for _, needle := range tc.needles {
				if !strings.Contains(out, needle) {
					t.Fatalf("snapshot %q missing %q\n--- view ---\n%s",
						tc.name, needle, out)
				}
			}

			if !write {
				return
			}

			if err := os.WriteFile(
				filepath.Join(outDir, "sprint-08-"+tc.name+".txt"),
				[]byte(stripANSI(out)),
				0o600,
			); err != nil {
				t.Fatalf("write plain snapshot: %v", err)
			}
		})
	}
}

var ansiSequence = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

// stripANSI removes SGR escapes so the snapshot files render in any
// text viewer without colour bleed. lipgloss auto-disables colours
// when stdout is not a TTY (i.e. during `go test`), but we strip
// defensively in case a future CI sets `LIPGLOSS_FORCE_COLOR=1`.
func stripANSI(s string) string {
	return ansiSequence.ReplaceAllString(s, "")
}
