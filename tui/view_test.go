package tui

import (
	"bytes"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
)

func TestViewRendersInitWizardDashboardAndProjectDetail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		model   Model
		needles []string
	}{
		{
			name:    "init wizard",
			model:   New(Options{InitialWidth: 80, InitialHeight: 24}).withState(StateInitWizard),
			needles: []string{"Webox - first run setup", "System Pre-requisites", "Default SSH Keypair"},
		},
		{
			name: "dashboard",
			model: New(Options{InitialWidth: 100, InitialHeight: 30}).
				withConfig(fixtureConfig()).
				withStatuses(map[string]ProjectStatus{
					"p1": {ProjectID: "p1", State: ProjectOnline, HTTPHealth: "200 OK", SSLDaysLeft: 27, NodeVersion: "v24.15.0", LastDeploy: "2h ago"},
					"p2": {ProjectID: "p2", State: ProjectStale, HTTPHealth: "stale", SSLDaysLeft: -1, NodeVersion: "v20.12.2", LastDeploy: "unknown"},
				}),
			needles: []string{"Webox Cockpit", "Projects", "sui.demo.smallhost.pl", "STALE", "Overview"},
		},
		{
			name: "project detail overview",
			model: New(Options{InitialWidth: 100, InitialHeight: 30}).
				withConfig(fixtureConfig()).
				withState(StateProjectDetail),
			needles: []string{"Overview", "Env Diff", "unlocked in v0.2", "Restart", "disabled"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out := tt.model.View()
			for _, needle := range tt.needles {
				if !strings.Contains(out, needle) {
					t.Fatalf("view missing %q\n--- view ---\n%s", needle, out)
				}
			}
		})
	}
}

func TestTeatestSmokeDashboardSnapshot(t *testing.T) {
	m := New(Options{InitialWidth: 100, InitialHeight: 30}).withConfig(fixtureConfig())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(100, 30))
	t.Cleanup(func() {
		if err := tm.Quit(); err != nil {
			t.Fatal(err)
		}
	})

	var snapshot []byte
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		if bytes.Contains(out, []byte("Webox Cockpit")) {
			snapshot = append(snapshot[:0], out...)
			return true
		}
		return false
	}, teatest.WithDuration(time.Second), teatest.WithCheckInterval(10*time.Millisecond))
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
	for _, needle := range [][]byte{[]byte("Webox Cockpit"), []byte("Projects")} {
		if !bytes.Contains(snapshot, needle) {
			t.Fatalf("teatest output missing %q\n--- output ---\n%s", string(needle), string(snapshot))
		}
	}
}
