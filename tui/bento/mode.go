package bento

// Mode represents the responsive layout tier the engine should render.
//
// The thresholds match docs/UX.md §4.2 and ADR-0007. They are deliberately
// inclusive on their lower bound so that an exactly-sized terminal unlocks
// the higher tier without ambiguity.
type Mode int

const (
	// ModeTiny is the legacy/SSH-on-mobile fallback. The engine renders a
	// single warning panel asking the operator to resize the terminal.
	ModeTiny Mode = iota
	// ModeStandard is the 100x30 cockpit silhouette inherited from
	// Sprint 04. It uses the classic two-pane projects+overview layout.
	ModeStandard
	// ModeUltra is the 120x35 bento grid: six tiles arranged across a
	// 12-column grid (projects/overview/metrics + logs/cicd/topology).
	ModeUltra
	// ModeUltraPlus is the 160x45 generous layout reserved for full-screen
	// 4K terminals. Every Ultra tile gets more breathing room and a
	// dedicated "deep dive" strip below the primary grid.
	ModeUltraPlus
)

const (
	standardMinWidth   = 70
	standardMinHeight  = 22
	ultraMinWidth      = 120
	ultraMinHeight     = 35
	ultraPlusMinWidth  = 160
	ultraPlusMinHeight = 45
)

// String returns the canonical mode name used in logs, env overrides, and
// header badges (e.g. "[BENTO Ultra]").
func (m Mode) String() string {
	switch m {
	case ModeTiny:
		return "Tiny"
	case ModeStandard:
		return "Standard"
	case ModeUltra:
		return "Ultra"
	case ModeUltraPlus:
		return "UltraPlus"
	default:
		return "Unknown"
	}
}

// DetectMode picks the highest tier whose minimum width *and* height are
// satisfied. A zero/negative dimension is treated as "unknown viewport"
// and falls back to Standard, matching Bubble Tea's initial render before
// the first WindowSizeMsg arrives.
//
// The function is intentionally pure: env-var overrides (WEBOX_LAYOUT)
// are resolved by the caller so tests do not need to manipulate the
// process environment.
func DetectMode(width, height int) Mode {
	if width <= 0 || height <= 0 {
		return ModeStandard
	}
	if width < standardMinWidth || height < standardMinHeight {
		return ModeTiny
	}
	if width >= ultraPlusMinWidth && height >= ultraPlusMinHeight {
		return ModeUltraPlus
	}
	if width >= ultraMinWidth && height >= ultraMinHeight {
		return ModeUltra
	}
	return ModeStandard
}
