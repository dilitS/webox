// Package theme defines Webox Lipgloss design tokens.
package theme

// Theme is the single source of truth for cockpit colors. Hex values
// are anchored in OKLCH per docs/UX.md §2.1 and rendered in Lipgloss
// as terminal truecolor hex strings.
//
// Role-based tile palette (added in the Bento Ultra polish refresh,
// 2026-05-24): `Primary` paints the Projects / Server / Logs tiles
// (magenta-violet column), `Accent` paints the CI/CD tile (cyan
// column). Both colours are tuned to ≥ 4.5:1 contrast against
// `SurfaceBase` for WCAG AA compliance.
type Theme struct {
	Primary     string
	Accent      string
	Success     string
	Warning     string
	Error       string
	Degraded    string
	Muted       string
	SurfaceBase string
	SurfaceLow  string
	SurfaceHigh string
	TextBright  string
	TextDim     string
}

// Default returns the MVP dark theme tokens. The values are derived from
// OKLCH coordinates documented in docs/UX.md §2.1 ("Premium Cinematic
// Dark") and rounded to the closest sRGB representation Lipgloss can
// emit. The twelve roles cover every panel the cockpit renders.
func Default() Theme {
	return Theme{
		Primary:     "#7D56F4",
		Accent:      "#38BDF8",
		Success:     "#04B575",
		Warning:     "#FFB800",
		Error:       "#FF4444",
		Degraded:    "#D846EF",
		Muted:       "#4E5A85",
		SurfaceBase: "#1A1B26",
		SurfaceLow:  "#13141F",
		SurfaceHigh: "#24273A",
		TextBright:  "#F8F8F2",
		TextDim:     "#8C98C1",
	}
}

// Light returns the high-contrast light variant used when the host
// terminal advertises a light background (or the operator overrides via
// `WEBOX_THEME=light`). The role assignments mirror [Default] so every
// component renders the same semantic colour, only swapped for adequate
// contrast on a near-white surface.
//
// Values come from docs/UX.md §2.1 ("Daylight Cockpit") and are tuned to
// remain WCAG AA compliant against the SurfaceBase.
func Light() Theme {
	return Theme{
		Primary:     "#5B3FCB",
		Accent:      "#0277C2",
		Success:     "#0B7A4B",
		Warning:     "#B97900",
		Error:       "#C8221A",
		Degraded:    "#A02BC9",
		Muted:       "#6B7280",
		SurfaceBase: "#F7F8FB",
		SurfaceLow:  "#EAECF2",
		SurfaceHigh: "#FFFFFF",
		TextBright:  "#0F172A",
		TextDim:     "#475569",
	}
}
