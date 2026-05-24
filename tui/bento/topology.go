package bento

import (
	"github.com/dilitS/webox/tui/components/asciigraph"
)

// TopologySnapshot is the view-layer projection used by the topology
// tile. Producers (currently `tui/view.go`) compose it from the
// active project config + the latest `status.Cache` snapshot before
// every frame. Pulse is toggled by the tile clock so BUILDING and
// OFFLINE edges shimmer; the renderer itself stays pure.
type TopologySnapshot struct {
	Graph asciigraph.Graph
	// Pulse propagates the bento engine's animation tick into the
	// renderer (BUILDING dashes drift; OFFLINE bolts pulse). Tile
	// authors are encouraged to mirror it into [asciigraph.Edge.Pulse]
	// before calling Render.
	Pulse bool
	// HelpHint is the dim footer rendered under the graph (e.g.
	// "Updated 2s ago · ↻ refresh"). Empty hides the line.
	HelpHint string
}

// topologyTile renders the [Live Service Topology] tile. Slot is
// [SlotTopology]; the engine places it in the left column second row
// of the responsive Ultra / Ultra+ grid.
type topologyTile struct {
	snap  TopologySnapshot
	width int
}

// NewTopologyTile builds the live topology tile from a snapshot.
// The snapshot is produced in the view layer so the bento package
// stays free of `status.Cache` and `config.Project` imports.
func NewTopologyTile(snap TopologySnapshot) BentoTile {
	return &topologyTile{snap: snap}
}

// ID satisfies [BentoTile].
func (t *topologyTile) ID() string { return "topology" }

// Slot satisfies [BentoTile].
func (t *topologyTile) Slot() Slot { return SlotTopology }

// WithWidth satisfies [WidthAware] so the engine can give the tile the
// exact column width allocated to the left-hand topology cell.
func (t *topologyTile) WithWidth(w int) BentoTile {
	clone := *t
	clone.width = w
	return &clone
}

// Render satisfies [BentoTile].
func (t *topologyTile) Render(mode Mode, focused bool) string {
	width := t.width
	if width <= 0 {
		width = 60
	}
	body := asciigraph.Render(t.snap.Graph, width)

	hint := t.snap.HelpHint
	if hint == "" {
		hint = "↻ refresh · [Tab] cycle nodes · [Esc] back"
	}

	return renderTilePanel(tilePanelOptions{
		Header:   "🌐 [Live Service Topology]",
		Body:     body + "\n\n" + hint,
		Mode:     mode,
		Focused:  focused,
		Accent:   AccentCyan,
		MinWidth: t.width,
	})
}
