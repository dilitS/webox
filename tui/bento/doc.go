// Package bento implements the adaptive Bento Ultra layout engine for the
// Webox cockpit. It owns:
//
//   - Mode detection (Tiny / Standard / Ultra / UltraPlus) based on the
//     terminal viewport, with `WEBOX_LAYOUT` env-var overrides handled by
//     the caller (see [DetectMode]).
//   - The [BentoTile] contract that every dashboard cell implements,
//     plus a [Registry] for ordered composition.
//   - A pure rendering [Engine] that arranges tiles into a 12-column grid
//     using lipgloss primitives, falling back to a single-pane warning
//     panel when the terminal cannot host the Standard layout.
//
// The package is deliberately framework-light: it knows nothing about
// status fetching, SSH, or wizard state. View-layer adapters in
// `tui/view.go` build the tile slice each frame from the [tui.Model] and
// hand it to the engine.
//
// Sprint scope:
//   - Sprint 08 (current) ships the engine, registry, projects+overview
//     tiles, and placeholder tiles for metrics/CI/CD/logs/topology so the
//     Ultra silhouette is visible end-to-end even before live data flows.
//   - Sprint 09 swaps the metrics and logs placeholders for live tiles
//     fed by the SSH log streamer.
//   - Sprint 10 wires the CI/CD tile to the GitHub Actions client.
//   - Sprint 11 wires the topology tile to the live graph builder.
//
// Per AGENTS.md §2.2 the engine never imports networking primitives and
// every method is a pure function of its inputs.
package bento
