package tui_test

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dilitS/webox/config"
	ghsvc "github.com/dilitS/webox/services/github"
	"github.com/dilitS/webox/status"
	"github.com/dilitS/webox/tui"
)

func TestFetchProjectStatuses_LastDeploy_NoRepoYieldsPlaceholder(t *testing.T) {
	t.Parallel()

	projects := []config.Project{{ID: "p1", Domain: "skip.example.com"}}
	fetcher := func(context.Context, ghsvc.RepoRef, string) (*ghsvc.WorkflowRun, error) {
		t.Fatal("fetcher must not be called when project has no Repo")
		return nil, nil
	}
	statuses, err := tui.FetchProjectStatusesWithGitHub(
		context.Background(),
		projects,
		status.NewCache(status.Options{}),
		fetcher,
	)
	if err != nil {
		t.Fatalf("FetchProjectStatusesWithGitHub: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if statuses[0].LastDeploy == "" {
		t.Fatalf("LastDeploy should be a non-empty placeholder, got %q", statuses[0].LastDeploy)
	}
}

func TestFetchProjectStatuses_LastDeploy_RendersSuccessAsRelativeTime(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	twoMinAgo := now.Add(-2 * time.Minute)
	projects := []config.Project{
		{ID: "p1", Domain: "app.example.com", Repo: "demo/app"},
	}
	var calls atomic.Int32
	fetcher := func(_ context.Context, ref ghsvc.RepoRef, workflow string) (*ghsvc.WorkflowRun, error) {
		calls.Add(1)
		if ref.Owner != "demo" || ref.Name != "app" {
			t.Fatalf("ref = %+v, want demo/app", ref)
		}
		if workflow != "deploy.yml" {
			t.Fatalf("workflow = %q, want deploy.yml", workflow)
		}
		return &ghsvc.WorkflowRun{
			ID:         101,
			Status:     "completed",
			Conclusion: "success",
			UpdatedAt:  twoMinAgo,
		}, nil
	}
	cache := status.NewCache(status.Options{})
	statuses, err := tui.FetchProjectStatusesWithGitHub(context.Background(), projects, cache, fetcher)
	if err != nil {
		t.Fatalf("FetchProjectStatusesWithGitHub: %v", err)
	}
	if got := statuses[0].LastDeploy; !strings.Contains(got, "success") {
		t.Fatalf("LastDeploy=%q, want a success indicator", got)
	}
	if calls.Load() != 1 {
		t.Fatalf("fetcher called %d times, want 1", calls.Load())
	}
}

func TestFetchProjectStatuses_LastDeploy_CacheHitsAvoidSecondFetch(t *testing.T) {
	t.Parallel()

	projects := []config.Project{{ID: "p1", Domain: "app.example.com", Repo: "demo/app"}}
	var calls atomic.Int32
	fetcher := func(context.Context, ghsvc.RepoRef, string) (*ghsvc.WorkflowRun, error) {
		calls.Add(1)
		return &ghsvc.WorkflowRun{Status: "completed", Conclusion: "success", UpdatedAt: time.Now().Add(-time.Minute)}, nil
	}
	cache := status.NewCache(status.Options{})
	for i := 0; i < 3; i++ {
		if _, err := tui.FetchProjectStatusesWithGitHub(context.Background(), projects, cache, fetcher); err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("fetcher hit %d times across 3 refreshes; cache TTL not respected", got)
	}
}

func TestFetchProjectStatuses_LastDeploy_FailureDegradesGracefully(t *testing.T) {
	t.Parallel()

	projects := []config.Project{{ID: "p1", Domain: "app.example.com", Repo: "demo/app"}}
	fetcher := func(context.Context, ghsvc.RepoRef, string) (*ghsvc.WorkflowRun, error) {
		return nil, errors.New("rate limited")
	}
	statuses, err := tui.FetchProjectStatusesWithGitHub(
		context.Background(),
		projects,
		status.NewCache(status.Options{}),
		fetcher,
	)
	if err != nil {
		t.Fatalf("FetchProjectStatusesWithGitHub should not surface fetch errors, got %v", err)
	}
	if statuses[0].LastDeploy == "" {
		t.Fatal("LastDeploy must be populated with a degraded placeholder")
	}
}

func TestFetchProjectStatuses_LastDeploy_NilFetcherBehavesLikeNoOp(t *testing.T) {
	t.Parallel()

	projects := []config.Project{{ID: "p1", Domain: "app.example.com", Repo: "demo/app"}}
	statuses, err := tui.FetchProjectStatusesWithGitHub(context.Background(), projects, status.NewCache(status.Options{}), nil)
	if err != nil {
		t.Fatalf("FetchProjectStatusesWithGitHub: %v", err)
	}
	if statuses[0].LastDeploy == "" {
		t.Fatal("LastDeploy should never be empty (nil fetcher should yield a placeholder)")
	}
}
