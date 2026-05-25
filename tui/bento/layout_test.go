package bento

import (
	"fmt"
	"testing"
)

// TestRect_Empty checks the zero-size sentinel.
func TestRect_Empty(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		rect Rect
		want bool
	}{
		{"zero value", Rect{}, true},
		{"zero width", Rect{X: 0, Y: 0, Width: 0, Height: 5}, true},
		{"zero height", Rect{X: 0, Y: 0, Width: 10, Height: 0}, true},
		{"negative width", Rect{X: 0, Y: 0, Width: -1, Height: 5}, true},
		{"negative height", Rect{X: 0, Y: 0, Width: 10, Height: -1}, true},
		{"normal", Rect{X: 1, Y: 2, Width: 10, Height: 5}, false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.rect.Empty(); got != tc.want {
				t.Errorf("Empty() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestRect_Contains validates the half-open hit-test contract:
// the right and bottom edges are exclusive so two adjacent slots
// never both claim the same cell.
func TestRect_Contains(t *testing.T) {
	t.Parallel()

	r := Rect{X: 5, Y: 10, Width: 20, Height: 8}
	cases := []struct {
		name string
		x, y int
		want bool
	}{
		{"top-left corner", 5, 10, true},
		{"top-right interior", 24, 10, true},
		{"bottom-right interior", 24, 17, true},
		{"right edge exclusive", 25, 10, false},
		{"bottom edge exclusive", 5, 18, false},
		{"left of rect", 4, 10, false},
		{"above rect", 5, 9, false},
		{"far away", 100, 100, false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := r.Contains(tc.x, tc.y); got != tc.want {
				t.Errorf("Contains(%d,%d) = %v, want %v", tc.x, tc.y, got, tc.want)
			}
		})
	}
}

// TestRect_Contains_Empty asserts an empty rect never claims any
// cell — even (0,0). Hit-testing on absent slots must be silent.
func TestRect_Contains_Empty(t *testing.T) {
	t.Parallel()
	r := Rect{}
	if r.Contains(0, 0) {
		t.Fatalf("empty rect claimed (0,0)")
	}
}

// TestLayoutMap_SlotAt walks a synthetic 3-slot layout and checks
// every interior cell maps to the right slot, with non-slot cells
// reporting (_, false).
func TestLayoutMap_SlotAt(t *testing.T) {
	t.Parallel()

	layout := LayoutMap{
		Slots: map[Slot]Rect{
			SlotProjects: {X: 0, Y: 1, Width: 30, Height: 10},
			SlotOverview: {X: 30, Y: 1, Width: 50, Height: 10},
			SlotLogs:     {X: 0, Y: 11, Width: 80, Height: 8},
		},
	}

	cases := []struct {
		name     string
		x, y     int
		wantSlot Slot
		wantOK   bool
	}{
		{"projects center", 15, 5, SlotProjects, true},
		{"overview center", 50, 5, SlotOverview, true},
		{"logs center", 40, 14, SlotLogs, true},
		{"between projects/overview (right edge of projects exclusive)", 30, 5, SlotOverview, true},
		{"status bar y=0", 10, 0, 0, false},
		{"below logs", 10, 19, 0, false},
		{"right of logs", 80, 14, 0, false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotSlot, gotOK := layout.SlotAt(tc.x, tc.y)
			if gotOK != tc.wantOK {
				t.Fatalf("SlotAt(%d,%d) ok=%v, want %v", tc.x, tc.y, gotOK, tc.wantOK)
			}
			if gotOK && gotSlot != tc.wantSlot {
				t.Errorf("SlotAt(%d,%d) slot=%v, want %v", tc.x, tc.y, gotSlot, tc.wantSlot)
			}
		})
	}
}

// TestEngine_ComputeLayout_Tiny — the Tiny fallback is a single
// resize warning panel, no clickable slots. Layout must be empty.
func TestEngine_ComputeLayout_Tiny(t *testing.T) {
	t.Parallel()

	engine := NewEngine("test", nil)
	layout := engine.ComputeLayout(60, 18, ModeTiny)
	if len(layout.Slots) != 0 {
		t.Fatalf("Tiny layout has %d slots, want 0: %+v", len(layout.Slots), layout.Slots)
	}
}

// TestEngine_ComputeLayout_Ultra checks every slot in the Ultra
// 120×35 grid has a non-empty rectangle and that the rectangles
// cover the viewport without overlapping (the engine never
// double-claims a cell).
func TestEngine_ComputeLayout_Ultra(t *testing.T) {
	t.Parallel()

	const (
		width  = 120
		height = 35
	)

	engine := NewEngine("test", nil)
	layout := engine.ComputeLayout(width, height, ModeUltra)

	// Sanity: every Ultra slot must be present.
	expectedSlots := []Slot{SlotProjects, SlotOverview, SlotTopology, SlotCICD, SlotLogs}
	for _, slot := range expectedSlots {
		rect, ok := layout.Slots[slot]
		if !ok {
			t.Errorf("slot %v missing from Ultra layout", slot)
			continue
		}
		if rect.Empty() {
			t.Errorf("slot %v has empty rect: %+v", slot, rect)
		}
	}

	// Status bar must be one row tall, full width.
	if got := layout.StatusBar; got.X != 0 || got.Y != 0 || got.Width != width || got.Height != 1 {
		t.Errorf("status bar = %+v, want {0,0,%d,1}", got, width)
	}

	// Slot rectangles must not overlap. We probe every interior
	// cell once and check exactly one slot claims it.
	for y := 1; y < height; y++ {
		for x := 0; x < width; x++ {
			claims := 0
			for _, slot := range expectedSlots {
				if layout.Slots[slot].Contains(x, y) {
					claims++
				}
			}
			if claims > 1 {
				// First failure is enough — bail out early.
				t.Fatalf("cell (%d,%d) claimed by %d slots", x, y, claims)
			}
		}
	}
}

// TestEngine_ComputeLayout_UltraPlus mirrors the Ultra check at
// 160×45 so the layout planner adapts to the larger viewport.
func TestEngine_ComputeLayout_UltraPlus(t *testing.T) {
	t.Parallel()

	const (
		width  = 160
		height = 45
	)
	engine := NewEngine("test", nil)
	layout := engine.ComputeLayout(width, height, ModeUltraPlus)

	for _, slot := range []Slot{SlotProjects, SlotOverview, SlotTopology, SlotCICD, SlotLogs} {
		if r, ok := layout.Slots[slot]; !ok || r.Empty() {
			t.Errorf("slot %v missing or empty in UltraPlus: ok=%v rect=%+v", slot, ok, r)
		}
	}

	// UltraPlus must give the logs row more cells than Ultra (the
	// freed deep-dive strip flows there).
	ultra := NewEngine("test", nil).ComputeLayout(width, height, ModeUltra)
	if got, want := layout.Slots[SlotLogs].Height, ultra.Slots[SlotLogs].Height; got <= want {
		t.Errorf("UltraPlus logs height = %d, must exceed Ultra logs height = %d", got, want)
	}
}

// TestEngine_ComputeLayout_DimensionsTable spans three Ultra
// viewports and checks no slot leaks past the right edge or the
// status-bar row.
func TestEngine_ComputeLayout_DimensionsTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		w, h int
		mode Mode
	}{
		{120, 35, ModeUltra},
		{140, 40, ModeUltra},
		{160, 45, ModeUltraPlus},
		{170, 50, ModeUltraPlus},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(fmt.Sprintf("%dx%d-%v", tc.w, tc.h, tc.mode), func(t *testing.T) {
			t.Parallel()
			engine := NewEngine("test", nil)
			layout := engine.ComputeLayout(tc.w, tc.h, tc.mode)
			for slot, rect := range layout.Slots {
				if rect.X < 0 || rect.Y < 1 {
					t.Errorf("slot %v starts inside chrome: %+v", slot, rect)
				}
				if rect.X+rect.Width > tc.w {
					t.Errorf("slot %v overflows right edge (x=%d w=%d viewport=%d)",
						slot, rect.X, rect.Width, tc.w)
				}
				if rect.Y+rect.Height > tc.h {
					t.Errorf("slot %v overflows bottom (y=%d h=%d viewport=%d)",
						slot, rect.Y, rect.Height, tc.h)
				}
			}
		})
	}
}
