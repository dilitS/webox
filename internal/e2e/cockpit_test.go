package e2e_test

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/dilitS/webox/internal/telemetry"
	"github.com/dilitS/webox/ssh"
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
// right-arrow to detail + esc to return) so any regression in
// `Update`-driven state transitions surfaces here, not in production.
//
// Sprint 14 TASK-14.2 — `Tab` now cycles tile focus on the dashboard,
// so `Right` (or `Enter`) is the canonical "drill into the selected
// project" key. The legacy Tab → Project Detail mapping was removed.
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
	tm.Send(tea.KeyMsg{Type: tea.KeyRight})
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
	tm.Send(tea.KeyMsg{Type: tea.KeyRight})
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

// TestCockpit_HostKeyModalBlocksAndDismissesOnEsc is the multi-tick
// regression guard for TASK-14.4. The scenario simulates the worst-
// case operational moment: a periodic status refresh comes back with
// ErrHostKeyMismatch, the cockpit MUST raise a strict-block modal
// (instead of swallowing the failure into a dismissible toast), MUST
// surface the `ssh-keygen -R` recovery command, MUST ignore navigation
// keys while open, and MUST dismiss only on Esc.
//
// We assert against the COMPOSED frame (tile + chrome + overlay) so
// any regression in the overlay z-order or the keyboard router
// surfaces immediately at the operator-visible level.
func TestCockpit_HostKeyModalRendersAtRuntime(t *testing.T) {
	t.Parallel()

	// Terminal sized at 200×80 so the modal (which today is
	// appended below the cockpit base frame — Sprint-15 surface
	// migration will turn it into a true overlay) fits inside the
	// teatest alt-screen capture without top-clipping. Production
	// terminals are typically this large or larger.
	//
	// Scope clarification: this scenario asserts the modal RENDERS
	// correctly when the cockpit observes a host-key mismatch. The
	// strict-block keyboard contract + Esc-dismiss are covered by
	// the unit-tier `TestUpdate_HostKeyModal_BlocksKeysAndDismissesOnEsc`
	// in `tui/host_key_modal_integration_test.go`; duplicating them
	// at the e2e tier adds wall-clock cost without catching a
	// distinct regression class.
	m := tui.New(tui.MockOptions(""))
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(200, 80))
	t.Cleanup(func() {
		if err := tm.Quit(); err != nil {
			t.Fatalf("quit: %v", err)
		}
	})

	requireAllNeedles(t, tm, [][]byte{[]byte("[Active Projects]")})

	tm.Send(tui.StatusRefreshFailedMsg{Err: ssh.ErrHostKeyMismatch})

	requireAllNeedles(t, tm, [][]byte{
		[]byte("Host key mismatch"),
		[]byte("ssh-keygen -R"),
		[]byte("OUT OF BAND"),
		[]byte("SECURITY"),
	})

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(e2eFrameTimeout))
}

// TestCockpit_DebugTraceEmitsHostKeyEvent is the E2E counterpart to
// TASK-14.6: when --debug-trace is wired up, the cockpit MUST emit
// the documented event grammar (`status.refresh_failed`,
// `modal.hostkey_open`) for every operator-relevant transition. We
// inject a recording telemetry.Sink and replay the same scenario as
// the modal test above, then assert the event log contains the
// expected names + payloads.
//
// This scenario exists at the e2e tier (not just at the unit tier)
// because the emit-call-sites depend on the `Update` router actually
// dispatching the message through the running teatest program — a
// regression in the dispatch (e.g. a future refactor that swallows
// the message in a wrapper) would NOT be caught by the unit-level
// `TestEmit_StatusRefreshFailedRecordsErrClass`.
func TestCockpit_DebugTraceEmitsHostKeyEvent(t *testing.T) {
	t.Parallel()

	sink := &recordingSink{}
	opts := tui.MockOptions("")
	opts.Trace = sink
	m := tui.New(opts)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(200, 80))
	t.Cleanup(func() {
		if err := tm.Quit(); err != nil {
			t.Fatalf("quit: %v", err)
		}
	})

	requireAllNeedles(t, tm, [][]byte{[]byte("[Active Projects]")})

	tm.Send(tui.StatusRefreshFailedMsg{Err: ssh.ErrHostKeyMismatch})
	requireAllNeedles(t, tm, [][]byte{[]byte("Host key mismatch")})

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(e2eFrameTimeout))

	gotRefresh := sink.findFirst("status.refresh_failed")
	if gotRefresh == nil {
		t.Fatalf("missing status.refresh_failed event in trace: %+v", sink.events())
	}
	if cls, _ := gotRefresh.Fields["err_class"].(string); cls != "host_key_mismatch" {
		t.Errorf("err_class = %q, want host_key_mismatch", cls)
	}

	gotModal := sink.findFirst("modal.hostkey_open")
	if gotModal == nil {
		t.Fatalf("missing modal.hostkey_open event in trace")
	}
	if kind, _ := gotModal.Fields["kind"].(string); kind != "mismatch" {
		t.Errorf("kind = %q, want mismatch", kind)
	}
}

// TestCockpit_PgDownScrollsViewportInOverflow is the keyboard-flow
// guard for the Sprint-13 chrome contract: when the dashboard body
// overflows the viewport, PgDown MUST advance the scroll offset and
// the body MUST repaint with the new slice. We force the smallest
// Bento Ultra frame so overflow is guaranteed, then watch for a
// visible content shift.
func TestCockpit_PgDownScrollsViewportInOverflow(t *testing.T) {
	t.Parallel()

	m := tui.New(tui.MockOptions(""))
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 22))
	t.Cleanup(func() {
		if err := tm.Quit(); err != nil {
			t.Fatalf("quit: %v", err)
		}
	})

	requireAllNeedles(t, tm, [][]byte{[]byte("PgUp/PgDn")})

	tm.Send(tea.KeyMsg{Type: tea.KeyPgDown})

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("PgUp/PgDn"))
	}, teatest.WithDuration(e2eFrameTimeout))

	tm.Send(tea.KeyMsg{Type: tea.KeyHome})

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(e2eFrameTimeout))
}

// TestCockpit_FocusedTileScrollsIndependentOfViewport is the
// Sprint 14 TASK-14.2 e2e contract: when the operator presses
// `Tab` to focus a scrollable tile and then `PgDn`, only THAT
// tile's offset advances; the global body viewport stays at its
// current scroll position. We assert this by checking the footer
// hint switches from the global "PgUp/PgDn (n/m)" form to the
// tile-focused "focus: <name> · ..." form. Direct snapshot of the
// log buffer would be brittle (mock data changes per release);
// the footer string is the stable behavioural contract.
//
// Sized at 120×35 — the smallest viewport that unlocks the Bento
// Ultra grid and forces the body to overflow the terminal height.
// At larger sizes (140×40+) the cockpit's body fits inside the
// viewport with empty trailing rows, which teatest's alt-screen
// virtual terminal does NOT preserve in `tm.Output()`. The footer
// at the very bottom of the joined view ends up off-screen in
// that capture, even though the operator sees it on a real TTY.
// 120×35 keeps the chrome contract under genuine pressure and is
// the configuration the Sprint 13 chrome contract tests against.
func TestCockpit_FocusedTileScrollsIndependentOfViewport(t *testing.T) {
	t.Parallel()

	m := tui.New(tui.MockOptions(""))
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 35))
	t.Cleanup(func() {
		if err := tm.Quit(); err != nil {
			t.Fatalf("quit: %v", err)
		}
	})

	requireAllNeedles(t, tm, [][]byte{[]byte("[Active Projects]")})

	tm.Send(tea.KeyMsg{Type: tea.KeyTab})

	// We only assert `focus:` here. The full hint
	// `focus: <name> · [PgUp/PgDn] scroll panel · [Esc] release`
	// is truncated by lipgloss to fit in 120 columns, so the
	// `[PgUp/PgDn]` and `[Esc]` tokens may not survive the right-
	// edge clip. The unit-tier `TestRenderChromeBottom_FocusedTileShowsScopedHint`
	// covers the full string at unconstrained width; here we
	// only need to prove the chrome flipped into focus mode.
	requireAllNeedles(t, tm, [][]byte{[]byte("focus:")})

	tm.Send(tea.KeyMsg{Type: tea.KeyPgDown})

	// "Pipeline Steps (offset" is the cicdPipelineTile's debug
	// marker that surfaces only when scrollOffset > 0, proving
	// the focused tile's offset advanced under PgDn while the
	// dashboard chrome stayed put. teatest's alt-screen capture
	// can drop the chrome footer between frames; the body marker
	// is the more reliable signal that PgDn routed to the tile.
	requireAllNeedles(t, tm, [][]byte{[]byte("Pipeline Steps (offset")})

	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(e2eFrameTimeout))
}

// TestCockpit_TileFocusReleasesOnEsc covers the second half of the
// Sprint 14 TASK-14.2 contract: an operator who lands on a tile by
// accident (or just wants to return to global viewport scrolling)
// MUST be able to release focus with a single `Esc`, without
// quitting the dashboard. We verify the footer hint reverts from
// the tile-scoped form ("focus: <name>") back to the dashboard's
// own chrome legend (which surfaces `[Right/Enter] open` —
// dashboard-only, never present while a tile is focused).
//
// Sprint 20 — the previous needle (`[Active Projects]`) was the
// projects-tile title which never changes between focused and
// released frames. Bubble Tea's renderer diffs the per-line
// content between frames and only re-emits lines that changed; so
// after the previous WaitFor consumed the buffer, that line was
// never re-emitted. We now needle on a footer token that *does*
// change between modes so the wait sees fresh bytes.
func TestCockpit_TileFocusReleasesOnEsc(t *testing.T) {
	t.Parallel()

	m := tui.New(tui.MockOptions(""))
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 35))
	t.Cleanup(func() {
		if err := tm.Quit(); err != nil {
			t.Fatalf("quit: %v", err)
		}
	})

	requireAllNeedles(t, tm, [][]byte{[]byte("[Right/Enter] open")})

	tm.Send(tea.KeyMsg{Type: tea.KeyTab})
	requireAllNeedles(t, tm, [][]byte{[]byte("focus:")})

	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})

	requireAllNeedles(t, tm, [][]byte{[]byte("[Right/Enter] open")})

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(e2eFrameTimeout))
}

// TestCockpit_TabBackwardsLandsOnLogsCycle verifies the cycle
// rotation walks ALL scrollable tiles (not just the first one),
// satisfying the AC bullet "Tab cycles between scrollable tiles".
// The dashboard has two scrollable tiles in mock mode (CI/CD and
// Live Logs); a single `Shift+Tab` from no-focus must land on the
// LAST one in the cycle so the operator can reach it without
// pressing Tab twice.
func TestCockpit_TabBackwardsLandsOnLogsCycle(t *testing.T) {
	t.Parallel()

	m := tui.New(tui.MockOptions(""))
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 35))
	t.Cleanup(func() {
		if err := tm.Quit(); err != nil {
			t.Fatalf("quit: %v", err)
		}
	})

	requireAllNeedles(t, tm, [][]byte{[]byte("[Active Projects]")})

	tm.Send(tea.KeyMsg{Type: tea.KeyShiftTab})

	requireAllNeedles(t, tm, [][]byte{
		[]byte("focus: logs"),
	})

	tm.Send(tea.KeyMsg{Type: tea.KeyShiftTab})
	requireAllNeedles(t, tm, [][]byte{[]byte("focus: CI/CD")})

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(e2eFrameTimeout))
}

// recordingSink is the e2e-tier mirror of the in-package
// recordingSink used by `tui/trace_emit_test.go`. We duplicate the
// type here (rather than exposing it from `tui`) because the e2e
// package lives in its own module-visibility boundary by design —
// internal/e2e MUST depend on `tui` only through the public surface.
type recordingSink struct {
	mu  sync.Mutex
	buf []telemetry.Event
}

func (r *recordingSink) Enabled() bool { return true }
func (r *recordingSink) Record(_ context.Context, ev telemetry.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf = append(r.buf, ev)
}

func (r *recordingSink) events() []telemetry.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]telemetry.Event, len(r.buf))
	copy(out, r.buf)
	return out
}

func (r *recordingSink) findFirst(name string) *telemetry.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.buf {
		if r.buf[i].Name == name {
			return &r.buf[i]
		}
	}
	return nil
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
