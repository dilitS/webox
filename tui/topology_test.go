package tui

import (
	"strings"
	"testing"

	"github.com/dilitS/webox/config"
	ghsvc "github.com/dilitS/webox/services/github"
	"github.com/dilitS/webox/tui/components/asciigraph"
)

func TestBuildTopologySnapshotHealthyProject(t *testing.T) {
	t.Parallel()

	project := config.Project{
		ID:           "p1",
		Domain:       "shop-ease.io",
		ProfileAlias: "us-east-1",
		Repo:         "dilitS/shopease",
		Stack:        "node-express",
	}
	status := ProjectStatus{ProjectID: "p1", State: ProjectOnline, HTTPHealth: "200 OK", SSLDaysLeft: 114}
	ci := cicdSnapshotEntry{
		Run: &cicdRunSummary{Status: "completed", Conclusion: "success"},
	}

	snap := buildTopologySnapshot(project, status, true, ci, true, false)

	if snap.Graph.Repo.State != asciigraph.NodeOnline {
		t.Errorf("repo node should be ONLINE, got %v", snap.Graph.Repo.State)
	}
	if snap.Graph.Server.State != asciigraph.NodeOnline {
		t.Errorf("server node should be ONLINE, got %v", snap.Graph.Server.State)
	}
	if snap.Graph.Subdomain.State != asciigraph.NodeOnline {
		t.Errorf("subdomain node should be ONLINE, got %v", snap.Graph.Subdomain.State)
	}
	if snap.Graph.RepoToServer.State != asciigraph.EdgeOnline {
		t.Errorf("CI edge should be ONLINE, got %v", snap.Graph.RepoToServer.State)
	}
	if snap.Graph.ServerToSubdomain.State != asciigraph.EdgeOnline {
		t.Errorf("proxy edge should be ONLINE, got %v", snap.Graph.ServerToSubdomain.State)
	}
}

func TestBuildTopologySnapshotSSLDegraded(t *testing.T) {
	t.Parallel()
	project := config.Project{ID: "p", Domain: "x.io", ProfileAlias: "a"}
	status := ProjectStatus{ProjectID: "p", State: ProjectOnline, SSLDaysLeft: 3}

	snap := buildTopologySnapshot(project, status, true, cicdSnapshotEntry{}, false, false)

	if snap.Graph.ServerToSubdomain.State != asciigraph.EdgeDegraded {
		t.Fatalf("expected proxy edge DEGRADED (SSL<14d), got %v", snap.Graph.ServerToSubdomain.State)
	}
	if snap.Graph.RepoToServer.State != asciigraph.EdgeUnknown {
		t.Fatalf("no CI snapshot → CI edge UNKNOWN, got %v", snap.Graph.RepoToServer.State)
	}
}

func TestBuildTopologySnapshotOfflineCascade(t *testing.T) {
	t.Parallel()
	project := config.Project{ID: "p", Domain: "x.io", ProfileAlias: "a"}
	status := ProjectStatus{ProjectID: "p", State: ProjectOffline}
	ci := cicdSnapshotEntry{
		Run: &cicdRunSummary{Status: "completed", Conclusion: "failure"},
	}

	snap := buildTopologySnapshot(project, status, true, ci, true, true)

	if snap.Graph.RepoToServer.State != asciigraph.EdgeOffline {
		t.Fatalf("CI edge should be OFFLINE on failed run, got %v", snap.Graph.RepoToServer.State)
	}
	if snap.Graph.ServerToSubdomain.State != asciigraph.EdgeOffline {
		t.Fatalf("proxy edge should be OFFLINE when project is offline, got %v", snap.Graph.ServerToSubdomain.State)
	}
	if !snap.Pulse {
		t.Fatalf("pulse should propagate into snapshot")
	}
}

func TestBuildTopologySnapshotBuildingInProgress(t *testing.T) {
	t.Parallel()
	project := config.Project{ID: "p", Domain: "x.io", ProfileAlias: "a"}
	status := ProjectStatus{ProjectID: "p", State: ProjectBuilding}
	ci := cicdSnapshotEntry{Run: &cicdRunSummary{Status: "in_progress"}}

	snap := buildTopologySnapshot(project, status, true, ci, true, false)

	if snap.Graph.RepoToServer.State != asciigraph.EdgeBuilding {
		t.Fatalf("CI edge should be BUILDING on in_progress run, got %v", snap.Graph.RepoToServer.State)
	}
	if snap.Graph.ServerToSubdomain.State != asciigraph.EdgeBuilding {
		t.Fatalf("proxy edge should be BUILDING during deploy, got %v", snap.Graph.ServerToSubdomain.State)
	}
}

func TestBuildTopologySnapshotMissingStatusFallsBackToUnknown(t *testing.T) {
	t.Parallel()
	project := config.Project{ID: "p", Domain: "x.io", ProfileAlias: "a"}

	snap := buildTopologySnapshot(project, ProjectStatus{}, false, cicdSnapshotEntry{}, false, false)

	if snap.Graph.Server.State != asciigraph.NodeUnknown {
		t.Fatalf("missing status → server NodeUnknown, got %v", snap.Graph.Server.State)
	}
	if snap.Graph.RepoToServer.State != asciigraph.EdgeUnknown {
		t.Fatalf("missing CI → edge UNKNOWN, got %v", snap.Graph.RepoToServer.State)
	}
	if !strings.Contains(snap.HelpHint, "Awaiting") {
		t.Fatalf("missing status → hint should mention awaiting probe, got %q", snap.HelpHint)
	}
}

// Belt-and-braces: ensure ghsvc.WorkflowRun's status field plays well
// with edgeFromCICD's lowercasing.
func TestEdgeFromCICDIsCaseInsensitive(t *testing.T) {
	t.Parallel()
	ci := cicdSnapshotEntry{Run: &cicdRunSummary{Status: "COMPLETED", Conclusion: "SUCCESS"}}
	if edgeFromCICD(ci, true) != asciigraph.EdgeOnline {
		t.Fatalf("uppercase status should still resolve to EdgeOnline")
	}
	_ = ghsvc.WorkflowRun{}
}
