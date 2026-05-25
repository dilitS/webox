package bento

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/dilitS/webox/tui/theme"
)

// TileBlock is the structured representation of a rendered tile,
// introduced in Sprint 14 TASK-14.7 to replace the string-level
// `clipTileBlock` heuristic with a typed contract.
//
// A TileBlock decomposes the visual frame into the four layers the
// cockpit cares about:
//
//   - TopBorder is the first line of the bordered frame
//     (e.g. `┏━━━━┓`). Always exactly one row.
//   - Header is the title strip immediately under the top border.
//     Typically one row but allowed to span multiple when a tile
//     packs a status pill + workflow name onto separate lines.
//   - Body is the variable-height content area. Engine clips here
//     when the row budget is exceeded — both Header and the
//     borders are preserved.
//   - BottomBorder mirrors TopBorder. Always exactly one row.
//
// Every layer is the FULL bordered line including side glyphs, ANSI
// escape sequences, and lipgloss padding — the engine joins them
// verbatim so colour information survives clipping. Tiles that
// produce non-bordered output (e.g. the resize warning) leave
// TopBorder / BottomBorder empty; the clipper treats those as a
// degenerate case and falls back to plain line slicing.
type TileBlock struct {
	TopBorder    string
	Header       []string
	Body         []string
	BottomBorder string
	// AccentRGB is the optional accent colour the indicator line
	// uses when the engine has to inject a `… +N more lines` row.
	// Empty string falls back to [theme.Default()].Primary.
	AccentRGB string
}

// Lines returns the TileBlock as a flat slice ready for
// `strings.Join("\n", ...)`. Empty layers are skipped so degenerate
// (border-less) tiles render identically to their original
// `Render()` output.
func (b TileBlock) Lines() []string {
	out := make([]string, 0, 2+len(b.Header)+len(b.Body))
	if b.TopBorder != "" {
		out = append(out, b.TopBorder)
	}
	out = append(out, b.Header...)
	out = append(out, b.Body...)
	if b.BottomBorder != "" {
		out = append(out, b.BottomBorder)
	}
	return out
}

// Render returns the TileBlock as a single newline-joined string.
// Convenience wrapper used by the engine when injecting the result
// into a `lipgloss.JoinVertical` call.
func (b TileBlock) Render() string {
	return strings.Join(b.Lines(), "\n")
}

// LineCount returns the total number of rendered rows. Used by the
// engine to pre-size its `available` budget before clipping.
func (b TileBlock) LineCount() int {
	count := len(b.Header) + len(b.Body)
	if b.TopBorder != "" {
		count++
	}
	if b.BottomBorder != "" {
		count++
	}
	return count
}

// BlockRenderer is the optional capability a tile implements when it
// can hand the engine a structured [TileBlock] instead of a raw
// string. The engine prefers `RenderBlock` over `Render` when both
// are available so the clip logic can operate on typed lanes.
//
// Existing tiles do not implement this interface yet; the engine
// falls back to [parseTileBlock] which parses a flat rendered
// string into a TileBlock heuristically. New tiles SHOULD implement
// `BlockRenderer` directly so the parse-step can be retired in a
// future sprint.
type BlockRenderer interface {
	BentoTile
	RenderBlock(mode Mode, focused bool) TileBlock
}

// parseTileBlock decomposes a rendered tile string into a
// [TileBlock]. Used as the legacy adapter for tiles that have not
// yet migrated to [BlockRenderer]. The heuristic:
//
//  1. The first line is treated as the top border.
//  2. The second line is treated as the header (the title row that
//     all current tiles render under the top border).
//  3. The last line is treated as the bottom border.
//  4. Everything in between is the body.
//
// Tiles that render fewer than three lines (degenerate/empty) are
// returned with the entire string in `Body` and empty borders —
// the engine recognises this case and falls back to naïve line
// slicing during clip.
func parseTileBlock(rendered string) TileBlock {
	lines := strings.Split(rendered, "\n")
	const minBorderedLines = 3
	if len(lines) < minBorderedLines {
		return TileBlock{Body: lines}
	}
	return TileBlock{
		TopBorder:    lines[0],
		Header:       []string{lines[1]},
		Body:         lines[2 : len(lines)-1],
		BottomBorder: lines[len(lines)-1],
	}
}

// clipBlock trims a [TileBlock] so the rendered output fits within
// `maxLines` total rows. Returns a NEW TileBlock — the input is not
// mutated. When the block already fits, it is returned unchanged.
//
// Replaces the magic-constant heuristic in [clipTileBlock] with a
// straightforward arithmetic step: borders + header are reserved,
// the body is sliced, and an indicator row is appended so the
// operator can tell content was hidden.
//
// A degenerate block (no borders) is sliced naïvely — preserving
// the legacy behaviour for tiles that do not produce a frame.
func clipBlock(block TileBlock, maxLines int) TileBlock {
	if maxLines <= 0 {
		return block
	}
	if block.LineCount() <= maxLines {
		return block
	}
	if block.TopBorder == "" || block.BottomBorder == "" {
		flat := block.Lines()
		if len(flat) > maxLines {
			flat = flat[:maxLines]
		}
		return TileBlock{Body: flat}
	}

	// Reserved rows = top border + bottom border + every header
	// line + the indicator we are about to inject. The body slot
	// gets whatever is left.
	const (
		indicatorRow = 1
		// frameBorders accounts for the top + bottom border of a
		// well-formed [TileBlock]. Named so the magic-number
		// linter does not flag the arithmetic below.
		frameBorders = 2
	)
	reserved := frameBorders + len(block.Header) + indicatorRow
	if reserved >= maxLines {
		// Cannot keep the frame intact; fall back to a naïve
		// slice that at least preserves the top border so the
		// cockpit silhouette stays recognisable.
		flat := block.Lines()
		if len(flat) > maxLines {
			flat = flat[:maxLines]
		}
		return TileBlock{Body: flat}
	}

	visibleBody := maxLines - reserved
	if visibleBody < 0 {
		visibleBody = 0
	}
	hidden := len(block.Body) - visibleBody
	if hidden < 1 {
		hidden = 1
	}

	tileWidth := lipgloss.Width(block.TopBorder)
	tokens := theme.Default()
	indicator := framedIndicatorLine(
		tileWidth,
		"… +"+intString(hidden)+" more lines · scroll inside tab/modal",
		tokens,
	)

	clipped := TileBlock{
		TopBorder:    block.TopBorder,
		Header:       block.Header,
		BottomBorder: block.BottomBorder,
		AccentRGB:    block.AccentRGB,
	}
	clipped.Body = make([]string, 0, visibleBody+1)
	if visibleBody > 0 {
		clipped.Body = append(clipped.Body, block.Body[:visibleBody]...)
	}
	clipped.Body = append(clipped.Body, indicator)
	return clipped
}
