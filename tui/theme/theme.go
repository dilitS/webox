// Package theme defines Webox Lipgloss design tokens.
package theme

// Theme is the single source of truth for cockpit colors. Hex values
// are anchored in OKLCH per docs/UX.md §2.1 and rendered in Lipgloss
// as terminal truecolor hex strings.
type Theme struct {
	Primary     string
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

// Default returns the MVP dark theme tokens.
func Default() Theme {
	return Theme{
		Primary:     "#7D56F4",
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
