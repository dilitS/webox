package components

import "github.com/charmbracelet/bubbles/spinner"

// SpinnerStyle picks the right Bubbles spinner frame set for a given
// terminal mode. Standard cockpits get the compact `Dot` spinner that
// fits in 1-character gutters; Ultra and UltraPlus cockpits get the
// `Pulse` variant which feels more cinematic in the wider header band.
//
// The function returns a [spinner.Spinner] rather than a configured
// [spinner.Model] so callers can attach their own styles/colours per
// view (see how `tui/model.go` wires the colour to the active theme).
func SpinnerStyle(mode string) spinner.Spinner {
	switch mode {
	case "Ultra", "UltraPlus":
		return spinner.Pulse
	default:
		return spinner.Dot
	}
}

// NewAdaptiveSpinner constructs a fully-configured [spinner.Model] that
// already uses the right frame set for `mode`. Tests rely on the
// returned model being usable straight away (no extra Setup() step).
func NewAdaptiveSpinner(mode string) spinner.Model {
	s := spinner.New()
	s.Spinner = SpinnerStyle(mode)
	return s
}
