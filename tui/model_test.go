package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dilitS/webox/tui/bento"
)

func TestBentoModeRespectsLayoutOverride(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		opts Options
		want bento.Mode
	}{
		{
			name: "auto on standard viewport",
			opts: Options{InitialWidth: 100, InitialHeight: 30},
			want: bento.ModeStandard,
		},
		{
			name: "auto on ultra viewport",
			opts: Options{InitialWidth: 120, InitialHeight: 35},
			want: bento.ModeUltra,
		},
		{
			name: "override forces tiny on large viewport",
			opts: Options{InitialWidth: 160, InitialHeight: 45, LayoutOverride: "tiny"},
			want: bento.ModeTiny,
		},
		{
			name: "override forces ultraplus on small viewport",
			opts: Options{InitialWidth: 80, InitialHeight: 24, LayoutOverride: "ultraplus"},
			want: bento.ModeUltraPlus,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := New(tc.opts)
			if got := m.BentoMode(); got != tc.want {
				t.Fatalf("BentoMode = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestWindowSizeMsgRecomputesBentoModeAndSpinner(t *testing.T) {
	t.Parallel()

	m := New(Options{InitialWidth: 100, InitialHeight: 30})
	if m.BentoMode() != bento.ModeStandard {
		t.Fatalf("starting mode = %s, want Standard", m.BentoMode())
	}

	resized, _ := m.Update(tea.WindowSizeMsg{Width: 160, Height: 45})
	m2 := resized.(Model)
	if m2.BentoMode() != bento.ModeUltraPlus {
		t.Fatalf("after resize mode = %s, want UltraPlus", m2.BentoMode())
	}
	if len(m2.spinner.Spinner.Frames) == 0 {
		t.Fatal("spinner frames empty after resize")
	}
}

func TestViewRoutesToTinyOverrideEvenOnLargeViewport(t *testing.T) {
	t.Parallel()

	m := New(Options{InitialWidth: 160, InitialHeight: 45, LayoutOverride: "tiny"}).
		withConfig(fixtureConfig())
	out := m.View()
	if !strings.Contains(out, "Terminal too small") {
		t.Fatalf("tiny override view missing fallback message:\n%s", out)
	}
}
