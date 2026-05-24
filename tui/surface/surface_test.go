package surface_test

import (
	"testing"

	"github.com/dilitS/webox/tui/surface"
	"github.com/dilitS/webox/tui/views"
)

type stubSurface struct {
	body   string
	crumb  string
	footer surface.FooterHint
	scroll bool
}

func (s stubSurface) Body(_ surface.Context) string               { return s.body }
func (s stubSurface) Crumb(_ surface.Context) string              { return s.crumb }
func (s stubSurface) Footer(_ surface.Context) surface.FooterHint { return s.footer }
func (s stubSurface) AcceptsScroll(_ surface.Context) bool        { return s.scroll }

func TestRegistry_RegisterAndLookup(t *testing.T) {
	t.Parallel()

	r := surface.NewRegistry()
	want := stubSurface{body: "hello"}
	r.Register("dashboard", want)

	got := r.Lookup("dashboard")
	if got == nil {
		t.Fatalf("expected surface registered for 'dashboard', got nil")
	}
	if got.Body(surface.Context{}) != "hello" {
		t.Fatalf("registered surface returned wrong body: %q", got.Body(surface.Context{}))
	}
}

func TestRegistry_LookupUnknownReturnsNil(t *testing.T) {
	t.Parallel()

	r := surface.NewRegistry()
	if got := r.Lookup("missing"); got != nil {
		t.Fatalf("expected nil for unregistered key, got %+v", got)
	}
}

func TestRegistry_ResetClearsAllSurfaces(t *testing.T) {
	t.Parallel()

	r := surface.NewRegistry()
	r.Register("a", stubSurface{})
	r.Register("b", stubSurface{})
	r.Reset()

	if r.Lookup("a") != nil || r.Lookup("b") != nil {
		t.Fatal("Reset did not clear surfaces")
	}
}

func TestFooterHint_Empty(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		hint surface.FooterHint
		want bool
	}{
		{"zero value", surface.FooterHint{}, true},
		{"only scroll hint", surface.FooterHint{ScrollHint: true}, false},
		{"only text", surface.FooterHint{Text: "x"}, false},
		{"both", surface.FooterHint{Text: "x", ScrollHint: true}, false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.hint.Empty(); got != tc.want {
				t.Errorf("Empty() = %t, want %t", got, tc.want)
			}
		})
	}
}

// TestSurfaceContractAcceptsContext is a compile-time guard ensuring
// the [surface.Surface] interface stays narrow enough to drive every
// rendering decision from a single read-only [surface.Context].
func TestSurfaceContractAcceptsContext(t *testing.T) {
	t.Parallel()

	var _ surface.Surface = stubSurface{}
	ctx := surface.Context{Screen: views.Screen{Width: 120, Height: 35}}
	got := stubSurface{body: "body", crumb: "Crumb", footer: surface.FooterHint{Text: "footer"}, scroll: true}
	if got.Body(ctx) != "body" || got.Crumb(ctx) != "Crumb" || got.Footer(ctx).Text != "footer" || !got.AcceptsScroll(ctx) {
		t.Fatal("contract drift: stub surface no longer matches interface")
	}
}
