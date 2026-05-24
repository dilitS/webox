package asciigraph

import (
	"strings"
	"testing"
)

func TestEdgeGlyphsContract(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		state    EdgeState
		pulse    bool
		wantConn string
		wantArr  string
	}{
		{"online", EdgeOnline, false, "──────────", "✓"},
		{"degraded", EdgeDegraded, false, "━━━━━━━━━━", "⚠"},
		{"building_a", EdgeBuilding, false, "╌╌ ╌╌ ╌╌ ╌", "▶"},
		{"building_b", EdgeBuilding, true, " ╌╌ ╌╌ ╌╌ ", "▶"},
		{"offline_a", EdgeOffline, false, "⚡   ⚡   ⚡", "✗"},
		{"offline_b", EdgeOffline, true, "⚡ ⚡ ⚡ ⚡ ⚡", "✗"},
		{"unknown", EdgeUnknown, false, "··········", "?"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			conn, arr := EdgeGlyphs(tc.state, tc.pulse)
			if conn != tc.wantConn {
				t.Fatalf("connector mismatch: want %q got %q", tc.wantConn, conn)
			}
			if arr != tc.wantArr {
				t.Fatalf("arrow mismatch: want %q got %q", tc.wantArr, arr)
			}
		})
	}
}

func TestRenderHappyPathOnlineNoDB(t *testing.T) {
	t.Parallel()

	g := Graph{
		Repo:              Node{ID: "gh", Label: "owner/shop-ease", Icon: "📦", State: NodeOnline},
		Server:            Node{ID: "srv", Label: "demo.smallhost.pl", Icon: "🖥️", State: NodeOnline},
		Subdomain:         Node{ID: "sub", Label: "shop-ease.io", Icon: "🌐", State: NodeOnline},
		RepoToServer:      Edge{From: "gh", To: "srv", Label: "GHA Deploy", State: EdgeOnline},
		ServerToSubdomain: Edge{From: "srv", To: "sub", Label: "Proxy → :3000", State: EdgeOnline},
	}

	out := Render(g, 60)
	for _, need := range []string{
		"owner/shop-ease",
		"demo.smallhost.pl",
		"shop-ease.io",
		"GHA Deploy",
		"Proxy",
		// Light borders signal "infrastructure box inside a tile":
		// the tile chrome is the heavy frame (┏━), the boxes
		// inside it stay lighter (┌─) so the hierarchy reads as
		// "grid > tile > nodes" instead of two competing frame
		// weights. See renderNode docstring.
		"┌",
		"┘",
		// Online arrow is ✓.
		"▼ ✓",
	} {
		if !strings.Contains(out, need) {
			t.Fatalf("render missing %q\n--- output ---\n%s", need, out)
		}
	}
}

func TestRenderOfflineCascadePulsesGlyph(t *testing.T) {
	t.Parallel()

	g := Graph{
		Repo:              Node{ID: "gh", Label: "owner/repo", State: NodeOnline},
		Server:            Node{ID: "srv", Label: "host", State: NodeOnline},
		Subdomain:         Node{ID: "sub", Label: "app.io", State: NodeOffline},
		RepoToServer:      Edge{State: EdgeOnline, Label: "deploy"},
		ServerToSubdomain: Edge{State: EdgeOffline, Label: "Proxy", Pulse: true},
	}

	out := Render(g, 60)
	if !strings.Contains(out, "▼ ✗") {
		t.Fatalf("offline edge missing ✗ arrow\n%s", out)
	}
	if !strings.Contains(out, "║") {
		t.Fatalf("offline edge missing heavy vertical ║ glyph\n%s", out)
	}
	if !strings.Contains(out, "○") {
		t.Fatalf("offline node missing hollow circle marker\n%s", out)
	}
}

func TestRenderWithDBAddsLeafRow(t *testing.T) {
	t.Parallel()

	db := Node{ID: "db", Label: "mysql:webox", Icon: "🗄️", State: NodeOnline}
	dbEdge := Edge{From: "srv", To: "db", Label: "MySQL Tunnel", State: EdgeOnline}
	g := Graph{
		Repo:              Node{ID: "gh", Label: "owner/repo", State: NodeOnline},
		Server:            Node{ID: "srv", Label: "host", State: NodeOnline},
		Subdomain:         Node{ID: "sub", Label: "app.io", State: NodeOnline},
		RepoToServer:      Edge{State: EdgeOnline, Label: "deploy"},
		ServerToSubdomain: Edge{State: EdgeOnline, Label: "Proxy"},
		DB:                &db,
		ServerToDB:        &dbEdge,
	}

	out := Render(g, 80)
	if !strings.Contains(out, "mysql:webox") {
		t.Fatalf("db label not rendered\n%s", out)
	}
	if !strings.Contains(out, "MySQL Tunnel") {
		t.Fatalf("db edge label missing\n%s", out)
	}
}

func TestRenderTruncatesLongLabels(t *testing.T) {
	t.Parallel()

	g := Graph{
		Repo:      Node{Label: strings.Repeat("a", 100), State: NodeOnline},
		Server:    Node{Label: "srv", State: NodeOnline},
		Subdomain: Node{Label: "sub", State: NodeOnline},
	}
	out := Render(g, 30)
	if !strings.Contains(out, "…") {
		t.Fatalf("expected truncation ellipsis on narrow width:\n%s", out)
	}
}

func TestRenderIsDeterministic(t *testing.T) {
	t.Parallel()
	g := Graph{
		Repo:      Node{Label: "repo", State: NodeOnline},
		Server:    Node{Label: "server", State: NodeOnline},
		Subdomain: Node{Label: "sub", State: NodeOnline},
	}
	a := Render(g, 60)
	b := Render(g, 60)
	if a != b {
		t.Fatalf("render not deterministic\n--- a ---\n%s\n--- b ---\n%s", a, b)
	}
}
