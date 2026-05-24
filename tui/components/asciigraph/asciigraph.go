// Package asciigraph renders a small, hand-laid-out service dependency
// graph as a box-drawing string suitable for embedding inside a Bubble
// Tea / Lipgloss bento tile.
//
// The renderer is intentionally narrow in scope — it only handles the
// 3-level topology Webox cares about in v0.1 (`GitHub → Server →
// {Subdomain, DB}`). It is **not** a general-purpose DAG layout
// engine: Sprint 11 (the topology map sprint) declared general
// layouts out of scope, and the implementation here mirrors that
// decision.
//
// Design constraints:
//   - Pure renderer. No I/O, no goroutines, no clocks. Producers feed
//     a fully resolved [Graph] snapshot per frame; pulsation lives on
//     the calling tile.
//   - State-aware glyphs. Every edge carries an [EdgeState] that maps
//     to a glyph + colour pair via [EdgeGlyphs] / theme tokens. Tests
//     assert the glyph contract so the cockpit cannot regress to a
//     uniform `---`.
//   - Deterministic. Two calls with the same [Graph] produce
//     byte-identical output. Snapshot tests rely on this.
//
// The package lives under `tui/components/` because it is a leaf
// renderer reused by exactly one consumer today (the topology tile),
// but the contract is consciously stable — Sprint 12 plans to share
// it with the deep-dive strip rendered in Bento Ultra+.
package asciigraph

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/dilitS/webox/tui/theme"
)

// NodeState mirrors [EdgeState] for the boxed service node itself.
// We keep the type separate so a degraded edge does not implicitly
// repaint its endpoint nodes (e.g. SSL warning on the proxy edge
// keeps the Subdomain node green when the upstream still responds).
type NodeState int

// Node states. UNKNOWN is the zero value so callers do not have to
// pre-fill the struct when the upstream has not reported yet.
const (
	NodeUnknown NodeState = iota
	NodeOnline
	NodeBuilding
	NodeDegraded
	NodeOffline
)

// EdgeState selects the glyph + colour the renderer applies to the
// connector between two nodes. Mapping table lives in [EdgeGlyphs].
type EdgeState int

// Edge states. ONLINE is the default for established connections;
// BUILDING is for in-flight deploys (animated by the tile, not the
// renderer); DEGRADED is for SSL-near-expiry / 4xx; OFFLINE is for
// 5xx, timeout, or SSH probe failure.
const (
	EdgeUnknown EdgeState = iota
	EdgeOnline
	EdgeBuilding
	EdgeDegraded
	EdgeOffline
)

// Node is one service in the topology graph (e.g. `gh-repo`,
// `server`, `subdomain`, `db`). Label is the human-readable string
// rendered inside the box. Icon is an optional single-glyph prefix
// (typically an emoji) painted before the label.
type Node struct {
	ID    string
	Label string
	Icon  string
	State NodeState
}

// Edge connects two [Node.ID]s. Label is rendered above the
// connector (e.g. `GHA Deploy`, `Proxy`, `MySQL Tunnel`). Pulse, when
// true, lets the calling tile know the renderer should be re-invoked
// with [EdgeGlyphs] step-shifted; the renderer itself is stateless.
type Edge struct {
	From  string
	To    string
	Label string
	State EdgeState
	Pulse bool
}

// Graph is the snapshot the renderer consumes. The producer is
// `tui/bento/tiles/topology.go::BuildGraph`; tests build them inline
// (see `asciigraph_test.go`).
type Graph struct {
	Repo      Node
	Server    Node
	Subdomain Node
	DB        *Node

	RepoToServer      Edge
	ServerToSubdomain Edge
	ServerToDB        *Edge
}

// EdgeGlyphs returns the glyph pair (connector + endpoint arrow) for
// each edge state. Exposed so the tile can render the same legend on
// a side panel if it ever needs one — and so unit tests can assert
// the contract without poking the renderer internals.
func EdgeGlyphs(state EdgeState, pulse bool) (connector, arrow string) {
	switch state {
	case EdgeBuilding:
		if pulse {
			return " ╌╌ ╌╌ ╌╌ ", "▶"
		}
		return "╌╌ ╌╌ ╌╌ ╌", "▶"
	case EdgeDegraded:
		return "━━━━━━━━━━", "⚠"
	case EdgeOffline:
		if pulse {
			return "⚡ ⚡ ⚡ ⚡ ⚡", "✗"
		}
		return "⚡   ⚡   ⚡", "✗"
	case EdgeOnline:
		return "──────────", "✓"
	default:
		return "··········", "?"
	}
}

// nodeGlyph returns the small marker rendered on the side of a box
// (the "·●·" suffix). We keep the renderer monochrome here and let
// the colour layer do the visual work — the dot is only an
// accessibility nicety for non-colour terminals.
func nodeGlyph(state NodeState) string {
	switch state {
	case NodeOnline:
		return "●"
	case NodeBuilding:
		return "◐"
	case NodeDegraded:
		return "◑"
	case NodeOffline:
		return "○"
	default:
		return "·"
	}
}

// Render lays out g as a single multi-line string. The width hint is
// advisory: the renderer keeps every box short enough to fit inside
// `width`, but never wraps a label — narrow viewports get truncated
// labels suffixed with `…` instead of a malformed graph.
//
// Layout (boxes drawn with light box-drawing characters):
//
//	┌─────────────────────┐
//	│ 📦 GitHub Repo       │  ● ONLINE
//	│ owner/repo           │
//	└─────────┬───────────┘
//	          │ GHA Deploy
//	          ▼ ──── ✓
//	┌─────────────────────┐
//	│ 🖥️  Production server│  ● ONLINE
//	│ host.example.com     │
//	└─────────┬───────────┘
//	          │ Proxy / DB
//	          ▼
//	┌───────────┐ ┌───────────┐
//	│ 🌐 Subdomain│ │ 🗄️ MySQL  │
//	│ app.io     │ │ user@db   │
//	└───────────┘ └───────────┘
//
// The two leaf boxes (subdomain + db) sit side by side; when DB is
// nil only the subdomain is rendered and centred under the server.
func Render(g Graph, width int) string {
	tokens := theme.Default()

	boxWidth := chooseBoxWidth(width)

	repoBox := renderNode(g.Repo, boxWidth, tokens)
	serverBox := renderNode(g.Server, boxWidth, tokens)
	subBox := renderNode(g.Subdomain, leafBoxWidth(width, g.DB != nil), tokens)

	leafRow := subBox
	if g.DB != nil {
		dbBox := renderNode(*g.DB, leafBoxWidth(width, true), tokens)
		leafRow = lipgloss.JoinHorizontal(lipgloss.Top, subBox, "  ", dbBox)
	}

	repoEdge := renderEdge(g.RepoToServer, tokens)
	serverEdge := renderEdge(g.ServerToSubdomain, tokens)

	var dbEdgeLines string
	if g.ServerToDB != nil {
		dbEdgeLines = renderEdge(*g.ServerToDB, tokens)
	}

	sections := []string{
		repoBox,
		repoEdge,
		serverBox,
		serverEdge,
	}
	if dbEdgeLines != "" && g.DB != nil {
		sections = append(sections, "  "+lipgloss.NewStyle().
			Foreground(lipgloss.Color(tokens.TextDim)).
			Render("(DB tunnel: "+g.ServerToDB.Label+")"))
	}
	sections = append(sections, leafRow)

	return strings.Join(sections, "\n")
}

const (
	defaultBoxWidth = 32
	minBoxWidth     = 18
	maxBoxWidth     = 44
	defaultLeafBox  = 22
	minLeafBox      = 14
	maxLeafBox      = 28
	edgeIndent      = 8

	// boxBorderPadding accounts for the thick border (1 cell on
	// each side) plus the 1-cell padding inside the box, so the
	// content width sits flush with the tile's effective inner
	// width.
	boxBorderPadding = 4
	// twoLeafColumns is the number of side-by-side leaf boxes
	// (subdomain + DB) when a project owns a DB; used to halve the
	// available width across both columns.
	twoLeafColumns = 2
	// leafGutter is the inter-leaf spacing rendered between the
	// subdomain box and the DB box (split column layout).
	leafGutter = 3
	// labelTruncatePad reserves space for the truncation ellipsis.
	labelTruncatePad = 2
)

func chooseBoxWidth(width int) int {
	if width <= 0 {
		return defaultBoxWidth
	}
	w := width - boxBorderPadding
	if w < minBoxWidth {
		return minBoxWidth
	}
	if w > maxBoxWidth {
		return maxBoxWidth
	}
	return w
}

func leafBoxWidth(width int, twoLeaves bool) int {
	if !twoLeaves {
		return chooseBoxWidth(width)
	}
	if width <= 0 {
		return defaultLeafBox
	}
	w := (width / twoLeafColumns) - leafGutter
	if w < minLeafBox {
		return minLeafBox
	}
	if w > maxLeafBox {
		return maxLeafBox
	}
	return w
}

func nodeColor(state NodeState, tokens theme.Theme) string {
	switch state {
	case NodeOnline:
		return tokens.Success
	case NodeBuilding:
		return tokens.Warning
	case NodeDegraded:
		return tokens.Degraded
	case NodeOffline:
		return tokens.Error
	default:
		return tokens.Muted
	}
}

func edgeColor(state EdgeState, tokens theme.Theme) string {
	switch state {
	case EdgeOnline:
		return tokens.Success
	case EdgeBuilding:
		return tokens.Warning
	case EdgeDegraded:
		return tokens.Degraded
	case EdgeOffline:
		return tokens.Error
	default:
		return tokens.Muted
	}
}

// renderNode paints a single boxed node. The box uses heavy
// box-drawing characters (┏━┓┗━┛) so the cockpit immediately reads
// as "infrastructure component" instead of "tile chrome" (which uses
// the lighter rounded border).
func renderNode(n Node, width int, tokens theme.Theme) string {
	if width < minBoxWidth {
		width = minBoxWidth
	}

	color := nodeColor(n.State, tokens)
	label := n.Label
	if label == "" {
		label = "(unnamed)"
	}
	headerLine := label
	if n.Icon != "" {
		headerLine = n.Icon + " " + label
	}

	headerLine = truncate(headerLine, width-labelTruncatePad)
	dot := lipgloss.NewStyle().
		Foreground(lipgloss.Color(color)).
		Render(nodeGlyph(n.State))

	body := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(tokens.TextBright)).
		Render(headerLine) + "  " + dot

	box := lipgloss.NewStyle().
		Border(lipgloss.ThickBorder()).
		BorderForeground(lipgloss.Color(color)).
		Padding(0, 1).
		Width(width).
		Render(body)
	return box
}

// renderEdge paints a vertical connector with a side label. The
// vertical glyph follows the edge colour so a red `⚡` cascade reads
// unmistakably as "this connection is down" even before the operator
// reads the label.
func renderEdge(e Edge, tokens theme.Theme) string {
	color := edgeColor(e.State, tokens)
	_, arrow := EdgeGlyphs(e.State, e.Pulse)

	verticalGlyph := "│"
	switch e.State {
	case EdgeOffline:
		verticalGlyph = "║"
	case EdgeBuilding:
		verticalGlyph = "╎"
	}

	indent := strings.Repeat(" ", edgeIndent)
	vertical := lipgloss.NewStyle().
		Foreground(lipgloss.Color(color)).
		Render(verticalGlyph)
	arrowGlyph := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(color)).
		Render("▼ " + arrow)

	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(tokens.TextDim))
	labelLine := indent + vertical + " " + labelStyle.Render(e.Label)
	bodyLine := indent + arrowGlyph
	return labelLine + "\n" + indent + vertical + "\n" + bodyLine
}

func truncate(s string, limit int) string {
	if limit <= 1 {
		return s
	}
	if lipgloss.Width(s) <= limit {
		return s
	}
	runes := []rune(s)
	cut := limit - 1
	if cut < 1 {
		cut = 1
	}
	if cut > len(runes) {
		cut = len(runes)
	}
	return string(runes[:cut]) + "…"
}
