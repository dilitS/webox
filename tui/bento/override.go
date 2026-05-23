package bento

import "strings"

// ParseLayoutOverride normalises a `WEBOX_LAYOUT` env value into a
// concrete [Mode]. The second return value reports whether the input
// was a recognised override. An empty string, "auto", or unknown value
// returns (ModeStandard, false) so callers can fall back to viewport
// detection without special-casing nil.
//
// Accepted values (case-insensitive):
//
//   - "tiny"            -> ModeTiny
//   - "standard"        -> ModeStandard
//   - "ultra"           -> ModeUltra
//   - "ultraplus", "ultra+", "plus" -> ModeUltraPlus
//   - "" or "auto"      -> no override (false)
func ParseLayoutOverride(raw string) (Mode, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "auto":
		return ModeStandard, false
	case "tiny":
		return ModeTiny, true
	case "standard":
		return ModeStandard, true
	case "ultra":
		return ModeUltra, true
	case "ultraplus", "ultra+", "plus":
		return ModeUltraPlus, true
	default:
		return ModeStandard, false
	}
}

// Resolve combines [DetectMode] with an optional override returned by
// [ParseLayoutOverride]. The override always wins so power users can
// force-render the Ultra cockpit on smaller terminals (and get warned
// by the resulting wrapping).
func Resolve(width, height int, overrideRaw string) Mode {
	if mode, ok := ParseLayoutOverride(overrideRaw); ok {
		return mode
	}
	return DetectMode(width, height)
}
