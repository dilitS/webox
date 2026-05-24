package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/dilitS/webox/tui/theme"
)

// modal paddings keep the dialog visually breathable inside any
// terminal width. Named consts so golangci-lint doesn't flag literal
// numbers in the lipgloss builder chain.
const (
	modalPaddingY = 1
	modalPaddingX = 3
)

// ModalOptions configures a centred dialog. The body string is rendered
// verbatim (multi-line allowed); footer is the hint strip at the bottom
// (e.g. `[Enter] confirm  [Esc] cancel`).
type ModalOptions struct {
	Title    string
	Body     string
	Footer   string
	MinWidth int
	Tone     ModalTone
	Theme    theme.Theme
}

// ModalTone selects the border/title accent. The cockpit standardises on
// four tones so wizards, doctor checks, status pills, and rollback
// warnings share visual grammar.
type ModalTone int

// Tone aliases [ModalTone] so callers outside the modal renderer
// (status bar, badge pills, tile borders) read with intent. Both names
// resolve to the same numeric values so existing call sites keep
// compiling.
type Tone = ModalTone

const (
	// ToneInfo is the default neutral accent (primary purple).
	ToneInfo ModalTone = iota
	// ToneWarning highlights destructive confirmations (amber border).
	ToneWarning
	// ToneError surfaces rollback or auth failures (red border).
	ToneError
	// ToneSuccess paints the LIVE indicator in the status bar and
	// the SUCCESS badge on CI/CD tiles.
	ToneSuccess
)

// RenderModal composes a double-border centred dialog. The function does
// not centre the modal inside the viewport: callers are expected to use
// `lipgloss.Place` with the desired alignment so they keep control over
// background tinting.
//
// A faux drop-shadow is appended below the bottom border by drawing a
// muted line, giving the modal a small lift without resorting to
// terminal background tricks that break on plain consoles.
func RenderModal(opts ModalOptions) string {
	tokens := opts.Theme
	if tokens == (theme.Theme{}) {
		tokens = theme.Default()
	}

	accent := tokens.Primary
	switch opts.Tone {
	case ToneWarning:
		accent = tokens.Warning
	case ToneError:
		accent = tokens.Error
	case ToneSuccess:
		accent = tokens.Success
	case ToneInfo:
		// Inherit the default primary accent.
	}

	titleLine := ""
	if opts.Title != "" {
		titleLine = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(accent)).
			Render(opts.Title) + "\n\n"
	}

	footerLine := ""
	if opts.Footer != "" {
		footerLine = "\n\n" + lipgloss.NewStyle().
			Faint(true).
			Foreground(lipgloss.Color(tokens.TextDim)).
			Render(opts.Footer)
	}

	body := lipgloss.NewStyle().
		Foreground(lipgloss.Color(tokens.TextBright)).
		Render(opts.Body)

	content := titleLine + body + footerLine
	panel := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(lipgloss.Color(accent)).
		Padding(modalPaddingY, modalPaddingX).
		Background(lipgloss.Color(tokens.SurfaceHigh)).
		Foreground(lipgloss.Color(tokens.TextBright)).
		Render(content)

	if opts.MinWidth > 0 {
		panel = lipgloss.NewStyle().Width(opts.MinWidth).Render(panel)
	}

	shadowWidth := lipgloss.Width(panel)
	shadow := lipgloss.NewStyle().
		Foreground(lipgloss.Color(tokens.SurfaceLow)).
		Render(strings.Repeat("▔", shadowWidth))

	return panel + "\n" + shadow
}
