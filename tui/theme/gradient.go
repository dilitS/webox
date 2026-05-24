package theme

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// gradient internals. Constants are named so golangci-lint's mnd rule
// stays quiet and so future tweaks have a single source of truth.
const (
	// gradientRuneOverhead is the per-rune byte budget we reserve when
	// pre-allocating the gradient output builder (one rune + ANSI SGR
	// sequence with truecolor escape).
	gradientRuneOverhead = 12
	// hexColorLength is the length of a 6-digit RGB hex string after
	// stripping the optional '#'.
	hexColorLength = 6
	// hexBitMaskBits is the number of bits per byte channel in a
	// packed 0xRRGGBB integer.
	hexBitMaskBits = 8
	// roundingNudge is the +0.5 offset used to round-to-nearest when
	// converting a float interpolation to a uint8 channel value.
	roundingNudge = 0.5
)

// Gradient renders `text` with a horizontal foreground gradient between
// the two stop colours. The blend is performed in sRGB space which is
// good enough for the cockpit's header band; perceptual OKLCH blending
// would require a dependency on a colour-science library and is not
// worth it for ≤80-column titles.
//
// If `text` is empty or `width` is non-positive the function returns the
// text unchanged. The implementation walks the rune slice (not the byte
// slice) so multi-byte glyphs do not corrupt the gradient.
func Gradient(text, startHex, endHex string) string {
	if text == "" {
		return ""
	}
	runes := []rune(text)
	if len(runes) == 1 {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color(startHex)).
			Render(string(runes))
	}

	sr, sg, sb, ok := parseHexColor(startHex)
	if !ok {
		return text
	}
	er, eg, eb, ok := parseHexColor(endHex)
	if !ok {
		return text
	}

	var b strings.Builder
	b.Grow(len(text) * gradientRuneOverhead)

	last := float64(len(runes) - 1)
	for i, r := range runes {
		t := float64(i) / last
		color := blendHex(sr, sg, sb, er, eg, eb, t)
		b.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color(color)).
			Render(string(r)))
	}
	return b.String()
}

// parseHexColor accepts either #RRGGBB or RRGGBB.
func parseHexColor(s string) (r, g, b uint8, ok bool) {
	s = strings.TrimPrefix(s, "#")
	if len(s) != hexColorLength {
		return 0, 0, 0, false
	}
	var v uint32
	if _, err := fmt.Sscanf(s, "%06x", &v); err != nil {
		return 0, 0, 0, false
	}
	const redShift = 16
	return uint8(v >> redShift), uint8(v >> hexBitMaskBits), uint8(v), true
}

// blendHex linearly interpolates two sRGB triples and returns the
// #RRGGBB hex string lipgloss expects.
func blendHex(sr, sg, sb, er, eg, eb uint8, t float64) string {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	mix := func(a, b uint8) uint8 {
		return uint8(float64(a) + (float64(b)-float64(a))*t + roundingNudge)
	}
	return fmt.Sprintf("#%02X%02X%02X", mix(sr, er), mix(sg, eg), mix(sb, eb))
}
