package e2e_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/dilitS/webox/tui"
)

// e2eFrameTimeout caps how long any single WaitFor block can block.
// 2s is enough for the slowest tick we expect (mock-mode cockpit boot
// touches the status cache + status bar timer) but small enough that
// CI surfaces hangs quickly.
const e2eFrameTimeout = 2 * time.Second

// TestCockpit_MockBootShowsAllSurfaces is the smoke-level scenario:
// boot the offline `webox --mock` cockpit and assert every Bento
// Ultra slot rendered at least once. Catches regressions that hide
// behind successful unit tests but break the **composed** frame
// (e.g. wrong sequencing of status-bar / tile / footer assembly).
func TestCockpit_MockBootShowsAllSurfaces(t *testing.T) {
	t.Parallel()

	m := tui.New(tui.MockOptions(""))
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(140, 40))
	t.Cleanup(func() {
		if err := tm.Quit(); err != nil {
			t.Fatalf("quit: %v", err)
		}
	})

	requireAllNeedles(t, tm, [][]byte{
		[]byte("WEBOX"),
		[]byte("[Active Projects]"),
		[]byte("[SERVER:"),
		[]byte("[Live Service Topology]"),
		[]byte("[CI/CD PIPELINE: Main Branch]"),
		[]byte("[Live Server Logs]"),
	})

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(e2eFrameTimeout))
}

// TestCockpit_TabIntoProjectDetailAndBack walks the operator from the
// dashboard into the per-project Overview tab and back via `esc`. The
// scenario hits the most travelled keyboard path (dashboard up/down +
// tab to detail + esc to return) so any regression in
// `Update`-driven state transitions surfaces here, not in production.
func TestCockpit_TabIntoProjectDetailAndBack(t *testing.T) {
	t.Parallel()

	m := tui.New(tui.MockOptions(""))
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(140, 40))
	t.Cleanup(func() {
		if err := tm.Quit(); err != nil {
			t.Fatalf("quit: %v", err)
		}
	})

	requireAllNeedles(t, tm, [][]byte{[]byte("[Active Projects]")})
	tm.Send(tea.KeyMsg{Type: tea.KeyTab})
	requireAllNeedles(t, tm, [][]byte{[]byte("Project Detail")})
	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
	requireAllNeedles(t, tm, [][]byte{[]byte("[Active Projects]")})

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(e2eFrameTimeout))
}

// TestCockpit_OpenLiveLogsTab presses `4` from project detail to land
// in the live-log surface. The Sprint 09 tail flow is one of the
// busiest production paths and most likely to drift when the ring
// buffer / redactor are touched.
func TestCockpit_OpenLiveLogsTab(t *testing.T) {
	t.Parallel()

	m := tui.New(tui.MockOptions(""))
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(140, 40))
	t.Cleanup(func() {
		if err := tm.Quit(); err != nil {
			t.Fatalf("quit: %v", err)
		}
	})

	requireAllNeedles(t, tm, [][]byte{[]byte("[Active Projects]")})
	tm.Send(tea.KeyMsg{Type: tea.KeyTab})
	requireAllNeedles(t, tm, [][]byte{[]byte("Project Detail")})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("4")})
	requireAllNeedles(t, tm, [][]byte{[]byte("[4] Logs"), []byte("Tail -f:")})

	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(e2eFrameTimeout))
}

// TestCockpit_TinyFallbackShowsResizeWarning hits the `< 70×22`
// threshold and confirms the cockpit refuses to render the bento grid
// — instead surfacing the "Terminal too small" warning. The warning is
// the only thing keeping users from a broken-looking frame on tiny
// terminals; a regression here would manifest as silently truncated
// output, which is much harder to spot than a broken test.
func TestCockpit_TinyFallbackShowsResizeWarning(t *testing.T) {
	t.Parallel()

	m := tui.New(tui.MockOptions(""))
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(60, 18))
	t.Cleanup(func() {
		if err := tm.Quit(); err != nil {
			t.Fatalf("quit: %v", err)
		}
	})

	requireAllNeedles(t, tm, [][]byte{[]byte("Terminal too small")})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(e2eFrameTimeout))
}

// TestCockpit_ScrollHintAppearsOnOverflow forces a minimal-height
// Bento Ultra frame so the dashboard body cannot fit in the viewport,
// then asserts the bottom chrome surfaces the `↕ scroll: …`
// indicator. Catches the regression class where the chrome contract
// gets reordered and the hint silently disappears.
//
// 120×22 sits on the Bento Ultra width threshold but at the minimum
// terminal height (22) — the Ultra grid will render its 2×2 + logs
// layout (~24 lines) and overflow by a few rows, which is exactly
// what the scroll hint exists for.
func TestCockpit_ScrollHintAppearsOnOverflow(t *testing.T) {
	t.Parallel()

	m := tui.New(tui.MockOptions(""))
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 22))
	t.Cleanup(func() {
		if err := tm.Quit(); err != nil {
			t.Fatalf("quit: %v", err)
		}
	})

	requireAllNeedles(t, tm, [][]byte{[]byte("PgUp/PgDn")})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(e2eFrameTimeout))
}

// requireAllNeedles polls the terminal output until every needle is
// present, failing the test if `e2eFrameTimeout` elapses first.
func requireAllNeedles(t *testing.T, tm *teatest.TestModel, needles [][]byte) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		for _, needle := range needles {
			if !bytes.Contains(out, needle) {
				return false
			}
		}
		return true
	}, teatest.WithDuration(e2eFrameTimeout), teatest.WithCheckInterval(10*time.Millisecond))
}

// shorthand: a trivial regression sanity check that the
// `internal/e2e` package itself wires up properly. Without it `go
// test ./internal/e2e/...` would silently report "[no test files]"
// when the scenarios are skipped by a build tag, masking import
// breakage.
func TestE2EPackageLoads(t *testing.T) {
	t.Parallel()
	if strings.TrimSpace("e2e") == "" {
		t.Fatal("unreachable")
	}
}
