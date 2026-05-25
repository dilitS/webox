package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dilitS/webox/config"
)

func modelWithProject(t *testing.T) Model {
	t.Helper()
	m := New(Options{InitialWidth: 100, InitialHeight: 30})
	m = m.withConfig(&config.Config{
		Profiles: []config.Profile{{Alias: "main", Host: "demo.example", User: "deploy"}},
		Projects: []config.Project{{ID: "p1", Domain: "app.demo.example", ProfileAlias: "main"}},
	})
	m.state = StateProjectDetail
	return m
}

func TestEnterLiveLogsTabAllocatesBufferLazily(t *testing.T) {
	t.Parallel()

	m := modelWithProject(t)
	if m.liveLogs.Buffer != nil {
		t.Fatalf("buffer should not be allocated before entering tab")
	}

	got, _ := m.updateProjectDetailKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("4")})
	updated := got.(Model)
	if updated.activeTab != TabLogs {
		t.Fatalf("activeTab = %s, want TabLogs", updated.activeTab)
	}
	if updated.liveLogs.Buffer == nil {
		t.Fatal("buffer should be allocated after entering tab")
	}
	if !updated.liveLogs.AutoScroll {
		t.Fatal("AutoScroll should default to true")
	}
	if updated.liveLogs.ProjectID != "p1" {
		t.Fatalf("ProjectID = %q, want p1", updated.liveLogs.ProjectID)
	}
}

func TestLiveLogsToggleAutoScroll(t *testing.T) {
	t.Parallel()

	m := modelWithProject(t)
	mAny, _ := m.updateProjectDetailKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("4")})
	m = mAny.(Model)

	mAny, _ = m.updateProjectDetailKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	m = mAny.(Model)
	if m.liveLogs.AutoScroll {
		t.Fatal("first 'f' should turn auto-scroll off")
	}

	mAny, _ = m.updateProjectDetailKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	m = mAny.(Model)
	if !m.liveLogs.AutoScroll {
		t.Fatal("second 'f' should turn auto-scroll back on")
	}
}

func TestLiveLogsClearBuffer(t *testing.T) {
	t.Parallel()

	m := modelWithProject(t)
	mAny, _ := m.updateProjectDetailKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("4")})
	m = mAny.(Model)

	for i := 0; i < 5; i++ {
		m.liveLogs.Buffer.Push(LiveLogLine{Level: "INFO", Text: "line"})
	}
	if got := m.liveLogs.Buffer.Len(); got != 5 {
		t.Fatalf("buffer length = %d, want 5 before clear", got)
	}

	mAny, _ = m.updateProjectDetailKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	m = mAny.(Model)
	if got := m.liveLogs.Buffer.Len(); got != 0 {
		t.Fatalf("buffer length = %d, want 0 after clear", got)
	}
}

func TestLiveLogsEscReturnsToOverview(t *testing.T) {
	t.Parallel()

	m := modelWithProject(t)
	mAny, _ := m.updateProjectDetailKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("4")})
	m = mAny.(Model)

	mAny, _ = m.updateProjectDetailKey(tea.KeyMsg{Type: tea.KeyEsc})
	m = mAny.(Model)
	if m.activeTab != TabOverview {
		t.Fatalf("activeTab = %s, want TabOverview after esc", m.activeTab)
	}
	if m.state != StateProjectDetail {
		t.Fatalf("state = %s, want StateProjectDetail (still on detail page)", m.state)
	}
}

func TestLiveLogsViewRendersTabAndBuffer(t *testing.T) {
	t.Parallel()

	m := modelWithProject(t)
	mAny, _ := m.updateProjectDetailKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("4")})
	m = mAny.(Model)
	m.liveLogs.LogPath = "logs/node.log"
	m.liveLogs.Connected = true
	m.liveLogs.Buffer.Push(LiveLogLine{Level: "INFO", Text: "starting worker"})
	m.liveLogs.Buffer.Push(LiveLogLine{Level: "ERROR", Text: "db down", Redacted: true})

	out := m.View()
	for _, needle := range []string{
		"app.demo.example",
		"[4] Logs",
		"Stream:",
		"Tail -f: On",
		"Active File: logs/node.log",
		"starting worker",
		"db down",
		"(redacted)",
		"[Esc] back",
	} {
		if !strings.Contains(out, needle) {
			t.Fatalf("live logs view missing %q\n--- output ---\n%s", needle, out)
		}
	}
}

// TestProjectDetailKeyHandlerSilentlyIgnoresDimmedTabs replaces the
// pre-Sprint-20 contract that raised a redundant "tab available in
// v0.2" alert. The tab label itself ("[2] Env Diff - unlocked in
// v0.2") already carries that information; the alert was noise.
// New contract: pressing a dimmed tab is a complete no-op.
func TestProjectDetailKeyHandlerSilentlyIgnoresDimmedTabs(t *testing.T) {
	t.Parallel()

	m := modelWithProject(t)
	mAny, _ := m.updateProjectDetailKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	m = mAny.(Model)
	if m.alert != "" {
		t.Fatalf("pressing '2' raised alert %q, want silent ignore", m.alert)
	}
	if m.activeTab != TabOverview {
		t.Fatalf("activeTab = %s, want still TabOverview", m.activeTab)
	}
}
