package tui

import (
	"strings"

	"github.com/dilitS/webox/config"
	"github.com/dilitS/webox/tui/bento"
	"github.com/dilitS/webox/tui/components/asciigraph"
)

// buildTopologySnapshot composes the per-frame topology projection
// from the active project + cached status + CI/CD entry. Pure
// function; no I/O. Producers feed the latest model snapshot.
//
// Layout: GitHub Repo → Server → Subdomain (+ optional MySQL/Postgres
// leaf when [config.Project.DBName] is set). Edge states fold the
// underlying signals into the asciigraph enum:
//
//   - RepoToServer: BUILDING when the last CI run is in_progress;
//     ONLINE when last conclusion=success; OFFLINE on failure;
//     UNKNOWN when no run is recorded yet.
//   - ServerToSubdomain: ONLINE on HTTP 2xx + SSL>14d; DEGRADED when
//     SSL <14d or HTTP 4xx; OFFLINE on 5xx / timeout / project
//     reported OFFLINE.
//
// The builder is deliberately tolerant of nil inputs — `status` may
// be missing on first frame and the renderer still has to paint a
// reasonable placeholder topology so the operator sees the shape.
func buildTopologySnapshot(project config.Project, status ProjectStatus, hasStatus bool, ci cicdSnapshotEntry, hasCI, pulse bool) bento.TopologySnapshot {
	repo := asciigraph.Node{
		ID:    "gh",
		Icon:  "📦",
		Label: nonEmptyTopo(project.Repo, "(no repo)"),
		State: asciigraph.NodeOnline,
	}
	if project.Repo == "" {
		repo.State = asciigraph.NodeUnknown
	}

	server := asciigraph.Node{
		ID:    "srv",
		Icon:  "🖥",
		Label: nonEmptyTopo(project.ProfileAlias, "(no profile)"),
		State: nodeFromState(status.State, hasStatus),
	}

	subdomain := asciigraph.Node{
		ID:    "sub",
		Icon:  "🌐",
		Label: nonEmptyTopo(project.Domain, "(no domain)"),
		State: nodeFromState(status.State, hasStatus),
	}

	repoEdge := asciigraph.Edge{
		From:  "gh",
		To:    "srv",
		Label: "GHA Deploy",
		State: edgeFromCICD(ci, hasCI),
		Pulse: pulse,
	}

	proxyEdge := asciigraph.Edge{
		From:  "srv",
		To:    "sub",
		Label: "Proxy → " + nonEmptyTopo(stack(project), "node"),
		State: edgeFromProjectStatus(status, hasStatus),
		Pulse: pulse,
	}

	graph := asciigraph.Graph{
		Repo:              repo,
		Server:            server,
		Subdomain:         subdomain,
		RepoToServer:      repoEdge,
		ServerToSubdomain: proxyEdge,
	}

	// DB leaf is intentionally omitted in v0.1: `config.Project` has
	// no DB metadata yet (DB management is STRETCH v0.2+). The
	// renderer already supports the optional leaf so wiring it up
	// post-MVP is a one-line `graph.DB = &db` add.
	return bento.TopologySnapshot{
		Graph:    graph,
		Pulse:    pulse,
		HelpHint: "↻ live · " + projectStateLabel(status.State, hasStatus),
	}
}

// nodeFromState maps the textual [ProjectState] into the asciigraph
// enum. Unknown / missing statuses fall back to NodeUnknown so the
// renderer paints a muted box rather than an aggressive red.
func nodeFromState(s ProjectState, has bool) asciigraph.NodeState {
	if !has {
		return asciigraph.NodeUnknown
	}
	switch s {
	case ProjectOnline:
		return asciigraph.NodeOnline
	case ProjectBuilding:
		return asciigraph.NodeBuilding
	case ProjectOffline:
		return asciigraph.NodeOffline
	case ProjectStale:
		return asciigraph.NodeDegraded
	default:
		return asciigraph.NodeUnknown
	}
}

// edgeFromProjectStatus folds HTTP + SSL signals into a single edge
// colour. The function is deliberately conservative: a DEGRADED hint
// (SSL <14d, HTTP 4xx) does not get promoted to OFFLINE unless the
// upstream actually reports the project as offline.
func edgeFromProjectStatus(s ProjectStatus, has bool) asciigraph.EdgeState {
	if !has {
		return asciigraph.EdgeUnknown
	}
	switch s.State {
	case ProjectOnline:
		if s.SSLDaysLeft >= 0 && s.SSLDaysLeft < 14 {
			return asciigraph.EdgeDegraded
		}
		return asciigraph.EdgeOnline
	case ProjectBuilding:
		return asciigraph.EdgeBuilding
	case ProjectOffline:
		return asciigraph.EdgeOffline
	case ProjectStale:
		return asciigraph.EdgeDegraded
	default:
		return asciigraph.EdgeUnknown
	}
}

// edgeFromCICD reads the cached CI/CD entry (if any) and maps the
// run's conclusion into the asciigraph edge enum. Missing entries
// render as UNKNOWN so the operator does not see a misleading green
// arrow before the first poll completes.
func edgeFromCICD(ci cicdSnapshotEntry, has bool) asciigraph.EdgeState {
	if !has || ci.Run == nil {
		return asciigraph.EdgeUnknown
	}
	switch strings.ToLower(ci.Run.Status) {
	case "in_progress", "queued", "pending":
		return asciigraph.EdgeBuilding
	case "completed":
		switch strings.ToLower(ci.Run.Conclusion) {
		case "success":
			return asciigraph.EdgeOnline
		case "failure", "timed_out":
			return asciigraph.EdgeOffline
		case "cancelled", "skipped":
			return asciigraph.EdgeDegraded
		}
	}
	return asciigraph.EdgeUnknown
}

func projectStateLabel(s ProjectState, has bool) string {
	if !has {
		return "Awaiting first probe…"
	}
	switch s {
	case ProjectOnline:
		return "All systems nominal"
	case ProjectBuilding:
		return "Deploy in flight"
	case ProjectOffline:
		return "Connection lost — see CI/CD tile"
	case ProjectStale:
		return "Status stale (probe overdue)"
	default:
		return string(s)
	}
}

func stack(p config.Project) string {
	if p.Stack != "" {
		return p.Stack
	}
	if p.NodeVersion != "" {
		return "node " + p.NodeVersion
	}
	return ""
}

func nonEmptyTopo(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
