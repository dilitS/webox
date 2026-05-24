package bento_test

import (
	"testing"

	"github.com/dilitS/webox/tui/bento"
)

func TestParseLayoutOverride(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input    string
		wantMode bento.Mode
		wantOk   bool
	}{
		{"", bento.ModeStandard, false},
		{"auto", bento.ModeStandard, false},
		{"AUTO", bento.ModeStandard, false},
		{" tiny ", bento.ModeTiny, true},
		{"standard", bento.ModeStandard, true},
		{"Standard", bento.ModeStandard, true},
		{"ultra", bento.ModeUltra, true},
		{"UltraPlus", bento.ModeUltraPlus, true},
		{"ultra+", bento.ModeUltraPlus, true},
		{"plus", bento.ModeUltraPlus, true},
		{"banana", bento.ModeStandard, false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			mode, ok := bento.ParseLayoutOverride(tc.input)
			if mode != tc.wantMode || ok != tc.wantOk {
				t.Fatalf("ParseLayoutOverride(%q) = (%s, %v), want (%s, %v)",
					tc.input, mode, ok, tc.wantMode, tc.wantOk)
			}
		})
	}
}

func TestResolveOverridesViewportDetection(t *testing.T) {
	t.Parallel()

	if got := bento.Resolve(120, 35, "tiny"); got != bento.ModeTiny {
		t.Errorf("override should force Tiny, got %s", got)
	}
	if got := bento.Resolve(60, 18, "ultra"); got != bento.ModeUltra {
		t.Errorf("override should force Ultra on small viewport, got %s", got)
	}
	if got := bento.Resolve(100, 30, ""); got != bento.ModeStandard {
		t.Errorf("no override should use detection, got %s", got)
	}
	if got := bento.Resolve(160, 45, "auto"); got != bento.ModeUltraPlus {
		t.Errorf("auto should detect UltraPlus, got %s", got)
	}
}
