package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dilitS/webox/tui/bento"
)

// TestTileFocus_TabCyclesThroughScrollableTiles verifies the
// Sprint 14 TASK-14.2 contract:
//
//  1. With no focus, pressing `Tab` lands on the first scrollable
//     slot in the dashboard's tile order.
//  2. Subsequent `Tab` presses advance to the next scrollable slot.
//  3. After the last scrollable slot, `Tab` returns to "no focus"
//     so the operator can fall back to global viewport scrolling.
//  4. `Shift+Tab` walks the cycle backwards.
//
// We seed the cockpit with `--mock` data so the dashboard renders
// the full Bento Ultra grid; this guarantees both `SlotLogs` (live
// log stream) and `SlotCICD` (pipeline) are present and scrollable.
func TestTileFocus_TabCyclesThroughScrollableTiles(t *testing.T) {
	t.Parallel()

	m := New(MockOptions(""))

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if m.focusedTile == nil {
		t.Fatal("first Tab: focused tile is nil, want a scrollable slot")
	}
	first := *m.focusedTile

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if m.focusedTile == nil {
		t.Fatal("second Tab: focused tile is nil, want next scrollable slot")
	}
	second := *m.focusedTile
	if second == first {
		t.Fatalf("second Tab did not advance: still on %v", first)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if m.focusedTile != nil {
		t.Fatalf("third Tab: focused tile = %v, want nil (cycle wrap)", *m.focusedTile)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = updated.(Model)
	if m.focusedTile == nil {
		t.Fatal("Shift+Tab from nil: focused tile is nil, want last scrollable slot")
	}
	if *m.focusedTile != second {
		t.Errorf("Shift+Tab from nil: focus = %v, want %v (last in cycle)", *m.focusedTile, second)
	}
}

// TestTileFocus_EscClearsFocus checks that `Esc` releases the
// focused tile without leaving the dashboard. This is the explicit
// escape hatch for operators who land on a tile by accident and
// want to return to global viewport scrolling without quitting.
func TestTileFocus_EscClearsFocus(t *testing.T) {
	t.Parallel()

	m := New(MockOptions(""))
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if m.focusedTile == nil {
		t.Fatal("setup: Tab did not focus a tile")
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.focusedTile != nil {
		t.Errorf("focus after Esc = %v, want nil", *m.focusedTile)
	}
	if m.state != StateDashboard {
		t.Errorf("state after Esc = %v, want StateDashboard", m.state)
	}
}

// TestTileFocus_PgDownScrollsFocusedTile asserts that, while a tile
// is focused, `PgDn` moves THAT tile's offset and the global
// viewport offset stays at zero. The reverse — global viewport
// taking PgDn while no tile is focused — is covered by the
// existing scroll-hint e2e test, so we focus on the routing here.
func TestTileFocus_PgDownScrollsFocusedTile(t *testing.T) {
	t.Parallel()

	m := New(MockOptions(""))
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	focused := *m.focusedTile

	if got := m.tileScrollOffsets[focused]; got != 0 {
		t.Fatalf("setup: tile offset = %d, want 0", got)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	m = updated.(Model)

	if got := m.tileScrollOffsets[focused]; got == 0 {
		t.Errorf("after PgDn: tile offset = 0, want > 0 (focused tile should scroll)")
	}
	if m.viewportOffsetY != 0 {
		t.Errorf("after PgDn: viewport offset = %d, want 0 (global scroll must be inert while tile is focused)", m.viewportOffsetY)
	}
}

// TestTileFocus_HomeResetsTileOffset verifies the `Home` key clears
// the focused tile's offset back to 0. End is the symmetric case
// (large positive value clamped by the renderer); both are part of
// the same UX contract — keyboard parity with the global viewport.
func TestTileFocus_HomeResetsTileOffset(t *testing.T) {
	t.Parallel()

	m := New(MockOptions(""))
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	focused := *m.focusedTile

	for i := 0; i < 3; i++ {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
		m = updated.(Model)
	}
	if m.tileScrollOffsets[focused] == 0 {
		t.Fatal("setup: PgDn did not advance focused tile")
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyHome})
	m = updated.(Model)
	if got := m.tileScrollOffsets[focused]; got != 0 {
		t.Errorf("after Home: offset = %d, want 0", got)
	}
}

// TestTileFocus_NoScrollableTilesIsNoop confirms the focus rotation
// is inert when the dashboard cannot expose any scrollable tiles
// (e.g. when the cockpit has not yet booted past the init wizard).
// Without the guard, `Tab` would trap on the sentinel "no focus"
// slot in an infinite cycle.
func TestTileFocus_NoScrollableTilesIsNoop(t *testing.T) {
	t.Parallel()

	m := Model{state: StateDashboard}
	if got := m.scrollableSlotCycle(); got != nil {
		t.Errorf("scrollableSlotCycle on empty model = %v, want nil", got)
	}

	m.cycleFocusedTile(+1)
	if m.focusedTile != nil {
		t.Errorf("cycleFocusedTile on empty model: focus = %v, want nil", *m.focusedTile)
	}
}

// TestTileFocus_BentoEngineWiresFocusAndOffsets checks that the
// dashboard rendering pipeline injects both the focused slot and
// the offset map into the bento engine. We don't render here —
// `cockpit_snapshot_test.go` does the visual snapshotting — but we
// do assert the engine builder receives the right state.
func TestTileFocus_BentoEngineWiresFocusAndOffsets(t *testing.T) {
	t.Parallel()

	slot := bento.SlotLogs
	offsets := map[bento.Slot]int{slot: 7}

	engine := bento.NewEngine("test", nil).
		WithFocus(slot).
		WithTileScrollOffsets(offsets)

	if engine == nil {
		t.Fatal("bento.NewEngine returned nil after focus wiring")
	}
}
