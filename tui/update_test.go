package tui

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dilitS/webox/config"
	"github.com/dilitS/webox/status"
)

func TestUpdateConfigLoadedRoutesByFirstRunState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		msg  ConfigLoadedMsg
		want State
	}{
		{
			name: "missing config opens init wizard",
			msg:  ConfigLoadedMsg{Missing: true},
			want: StateInitWizard,
		},
		{
			name: "existing config opens dashboard",
			msg: ConfigLoadedMsg{Config: &config.Config{
				SchemaVersion: config.Current,
				Language:      "en",
				Profiles:      []config.Profile{{Alias: "main", Type: "smallhost", Host: "s1.small.pl", User: "demo"}},
				Projects:      []config.Project{{ID: "p1", Domain: "app.example.test", ProfileAlias: "main"}},
			}},
			want: StateDashboard,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotModel, _ := New(Options{}).Update(tt.msg)
			got := gotModel.(Model)
			if got.State() != tt.want {
				t.Fatalf("state = %s, want %s", got.State(), tt.want)
			}
		})
	}
}

func TestUpdateDashboardNavigationAndDetailReturn(t *testing.T) {
	t.Parallel()

	m := New(Options{})
	m, _ = applyMsg(t, m, ConfigLoadedMsg{Config: fixtureConfig()})

	m, _ = applyMsg(t, m, key(tea.KeyDown, ""))
	if m.SelectedIndex() != 1 {
		t.Fatalf("selected index after down = %d, want 1", m.SelectedIndex())
	}

	m, _ = applyMsg(t, m, key(tea.KeyRight, ""))
	if m.State() != StateProjectDetail {
		t.Fatalf("state after right = %s, want %s", m.State(), StateProjectDetail)
	}
	if m.ActiveTab() != TabOverview {
		t.Fatalf("active tab = %s, want %s", m.ActiveTab(), TabOverview)
	}

	m, _ = applyMsg(t, m, key(tea.KeyEsc, ""))
	if m.State() != StateDashboard {
		t.Fatalf("state after esc = %s, want %s", m.State(), StateDashboard)
	}
}

func TestUpdateDisabledProjectDetailActionsSetAlert(t *testing.T) {
	t.Parallel()

	m := New(Options{})
	m, _ = applyMsg(t, m, ConfigLoadedMsg{Config: fixtureConfig()})
	m, _ = applyMsg(t, m, key(tea.KeyRight, ""))

	for _, pressed := range []string{"r", "s", "v", "2", "3", "4"} {
		pressed := pressed
		t.Run(pressed, func(t *testing.T) {
			got, _ := applyMsg(t, m, key(tea.KeyRunes, pressed))
			if got.Alert() == "" {
				t.Fatalf("key %q should set disabled-action alert", pressed)
			}
			if got.State() != StateProjectDetail {
				t.Fatalf("key %q changed state to %s", pressed, got.State())
			}
		})
	}
}

func TestUpdateStatusRefreshedMergesProjectStatuses(t *testing.T) {
	t.Parallel()

	m := New(Options{})
	m, _ = applyMsg(t, m, ConfigLoadedMsg{Config: fixtureConfig()})

	m, _ = applyMsg(t, m, StatusRefreshedMsg{
		Statuses: []ProjectStatus{
			{
				ProjectID:   "p1",
				HTTPHealth:  "200 OK",
				SSLDaysLeft: 27,
				NodeVersion: "v24.15.0",
				LastDeploy:  "2h ago",
				State:       ProjectOnline,
			},
		},
	})

	got, ok := m.ProjectStatus("p1")
	if !ok {
		t.Fatal("status for p1 missing")
	}
	if got.State != ProjectOnline || got.HTTPHealth != "200 OK" || got.SSLDaysLeft != 27 {
		t.Fatalf("status = %+v, want online 200 OK with 27 SSL days", got)
	}
}

func TestUpdateQuitCancelsContext(t *testing.T) {
	t.Parallel()

	var cancelled atomic.Bool
	m := New(Options{
		NewContext: func() (context.Context, context.CancelFunc) {
			return context.Background(), func() { cancelled.Store(true) }
		},
	})

	_, cmd := m.Update(key(tea.KeyRunes, "q"))
	if !cancelled.Load() {
		t.Fatal("quit should call the model cancel func")
	}
	if cmd == nil {
		t.Fatal("quit should return tea.Quit command")
	}
}

func TestRefreshVisibleProjectsCmdUsesStatusCache(t *testing.T) {
	t.Parallel()

	calls := atomic.Int32{}
	m := New(Options{
		Cache: status.NewCache(status.Options{}),
		FetchStatuses: func(ctx context.Context, projects []config.Project, cache *status.Cache) ([]ProjectStatus, error) {
			calls.Add(1)
			if len(projects) != 2 {
				t.Fatalf("projects passed to refresh = %d, want 2", len(projects))
			}
			return []ProjectStatus{{ProjectID: "p1", State: ProjectOnline}}, nil
		},
	})
	m, _ = applyMsg(t, m, ConfigLoadedMsg{Config: fixtureConfig()})

	msg := refreshVisibleProjectsCmd(m)()
	refreshed, ok := msg.(StatusRefreshedMsg)
	if !ok {
		t.Fatalf("refresh cmd returned %T, want StatusRefreshedMsg", msg)
	}
	if calls.Load() != 1 {
		t.Fatalf("fetch calls = %d, want 1", calls.Load())
	}
	if len(refreshed.Statuses) != 1 || refreshed.Statuses[0].ProjectID != "p1" {
		t.Fatalf("statuses = %+v", refreshed.Statuses)
	}
}

func TestRefreshTickSchedulesNextRefresh(t *testing.T) {
	t.Parallel()

	m := New(Options{RefreshInterval: time.Millisecond})
	m, _ = applyMsg(t, m, ConfigLoadedMsg{Config: fixtureConfig()})

	_, cmd := m.Update(RefreshTickMsg(time.Now()))
	if cmd == nil {
		t.Fatal("refresh tick should schedule status refresh")
	}
}

func applyMsg(t *testing.T, m Model, msg tea.Msg) (Model, tea.Cmd) {
	t.Helper()

	got, cmd := m.Update(msg)
	typed, ok := got.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want tui.Model", got)
	}
	return typed, cmd
}

func key(typ tea.KeyType, value string) tea.KeyMsg {
	if typ == tea.KeyRunes {
		return tea.KeyMsg{Type: typ, Runes: []rune(value)}
	}
	return tea.KeyMsg{Type: typ}
}

func fixtureConfig() *config.Config {
	return &config.Config{
		SchemaVersion: config.Current,
		Language:      "en",
		Profiles: []config.Profile{
			{Alias: "main", Type: "smallhost", Host: "s1.small.pl", User: "demo"},
		},
		Projects: []config.Project{
			{ID: "p1", Domain: "sui.demo.smallhost.pl", ProfileAlias: "main", Repo: "dilitS/sui", Stack: "vite-react", NodeVersion: "v24.15.0"},
			{ID: "p2", Domain: "legacy.demo.smallhost.pl", ProfileAlias: "main", Repo: "dilitS/legacy", Stack: "node-express", NodeVersion: "v20.12.2"},
		},
	}
}
