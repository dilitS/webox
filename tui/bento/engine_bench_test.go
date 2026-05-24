package bento_test

import (
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/dilitS/webox/tui/bento"
)

// BenchmarkRenderMode exists to keep the cockpit's per-frame work in
// the operator's comfort zone (≤ 16 ms at the largest layout so the
// terminal can still feel 60 fps under bursty `tea.Tick` traffic). The
// benchmark renders the full Bento grid at the three production tiers
// the cockpit ships and reports `ns/op` + `B/op` so CI can spot
// performance regressions before they reach a user.
//
// Why benchmark in CI?
//
//   - The cockpit is keyboard-interactive; a 50 ms regression in
//     `RenderMode` would translate to visible lag on every status
//     refresh (default tick is 5 s but burst refreshes via `Ctrl+R`
//     re-render immediately).
//   - lipgloss is a string builder; subtle changes (e.g. switching a
//     border style or stacking another `Padding` call) can grow
//     allocations 5–10×. The `B/op` counter surfaces that.
//   - Refactors that move bento composition into Surface adapters
//     (Sprint 14) must not regress per-frame cost; this benchmark
//     anchors the baseline.
//
// To run locally:
//
//	go test -bench=. -benchmem ./tui/bento
//
// CI runs `make bench` (added separately) so regressions show up as a
// step failure when allocation counts cross the soft cap.
func BenchmarkRenderMode(b *testing.B) {
	cases := []struct {
		name   string
		width  int
		height int
		mode   bento.Mode
	}{
		{name: "standard-100x30", width: 100, height: 30, mode: bento.ModeStandard},
		{name: "ultra-120x35", width: 120, height: 35, mode: bento.ModeUltra},
		{name: "ultraplus-160x45", width: 160, height: 45, mode: bento.ModeUltraPlus},
	}
	for _, tc := range cases {
		tc := tc
		b.Run(tc.name, func(b *testing.B) {
			tiles := benchmarkTiles()
			engine := bento.NewEngine("Webox Cockpit v0.1", tiles)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = engine.RenderMode(tc.width, tc.height, tc.mode)
			}
		})
	}
}

// benchmarkTiles returns a representative cockpit fixture: five tiles
// of varying body sizes spanning every Slot. The tiles use the same
// thick borders + padding as production so the benchmark stays honest
// — if the lipgloss pipeline grows expensive, the benchmark surfaces
// it.
func benchmarkTiles() []bento.BentoTile {
	return []bento.BentoTile{
		newStubTile("projects", bento.SlotProjects, "📂 [Active Projects]",
			"ShopEase-Web\nAPI-Gateway\nAuth-Service\nDashboard\nDashboard-Admin\nPayment-UI"),
		newStubTile("overview", bento.SlotOverview, "🖥  [SERVER: ShopEase-Web]",
			"Profile: us-east-1\nStack: node-express\nNode.js: v20.11.0\nStatus: ONLINE\nHTTP: 200 OK\nSSL: 114 days remaining\nRepo: dilitS/shopease-web\nLast Deploy: 2h ago"),
		newStubTile("topology", bento.SlotTopology, "🌐 [Live Service Topology]",
			"┌────────────────────────┐\n│ 📦 dilitS/shopease-web │\n└────────────────────────┘\n        │ GHA Deploy\n        ▼ ✓\n┌────────────────────────┐\n│ 🖥 us-east-1           │\n└────────────────────────┘\n        │ Proxy → app\n        ▼ ✓\n┌────────────────────────┐\n│ 🌐 ShopEase-Web        │\n└────────────────────────┘"),
		newStubTile("cicd", bento.SlotCICD, "🚀 [CI/CD PIPELINE: Main Branch]",
			"LIVE  ShopEase-Web · deploy.yml\nBuild #412: SUCCESS ✓ · 1m 42s\nPipeline Steps:\n[1] Git Checkout ✓\n[2] Install Deps ✓\n[3] Code Lint ✓\n[4] Build Artifact ✓\n[5] Unit Tests ✓\n[6] Deploy ✓"),
		newStubTile("logs", bento.SlotLogs, "📜 [Live Server Logs]",
			"[14:32:10] INFO  - API-Gateway: GET /users 200\n[14:32:11] WARN  - Auth-Service: latency 450ms\n[14:32:12] INFO  - ShopEase: /products 88ms\n[14:32:14] DEBUG - Worker: cache hit\n[14:32:15] INFO  - API: Healthcheck OK\n[14:32:18] ERROR - Payment-UI: 502 timeout (redacted)"),
	}
}

// stubTile is a benchmarking-only [bento.BentoTile] that renders a
// thick-bordered panel with a header + body. We avoid pulling the
// production tiles (projectsTile / overviewTile / cicdPipelineTile)
// because they require the full model state; the benchmark stays
// focused on the **engine composition cost**, not the tile content
// pipeline.
type stubTile struct {
	id     string
	slot   bento.Slot
	header string
	body   string
}

func newStubTile(id string, slot bento.Slot, header, body string) *stubTile {
	return &stubTile{id: id, slot: slot, header: header, body: body}
}

func (s *stubTile) ID() string       { return s.id }
func (s *stubTile) Slot() bento.Slot { return s.slot }
func (s *stubTile) Render(_ bento.Mode, _ bool) string {
	frame := lipgloss.NewStyle().
		Border(lipgloss.ThickBorder()).
		Padding(0, 1)
	return frame.Render(s.header + "\n" + s.body)
}

// Compile-time guard so the stub keeps satisfying the BentoTile
// interface after any contract changes. A failing test here is faster
// to diagnose than a confusing bench-time crash.
var _ bento.BentoTile = (*stubTile)(nil)
