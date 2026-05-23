package github

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWaitForWorkflowCompletionPollsUntilCompleted(t *testing.T) {
	t.Parallel()

	runs := []*WorkflowRun{
		{ID: 1, Status: "queued", HTMLURL: "https://example.test/1"},
		{ID: 1, Status: "in_progress", HTMLURL: "https://example.test/1"},
		{ID: 1, Status: "completed", Conclusion: "success", HTMLURL: "https://example.test/1"},
	}
	client := &Client{primary: transportFunc{
		getLatestRun: func(context.Context, RepoRef, LatestRunRequest) (*WorkflowRun, error) {
			got := runs[0]
			runs = runs[1:]
			return got, nil
		},
	}}
	got, err := WaitForWorkflowCompletion(context.Background(), client, RepoRef{Owner: "dilitS", Name: "demo"}, LatestRunRequest{Branch: "main"}, PollOptions{
		Interval: time.Nanosecond,
		Sleep:    func(context.Context, time.Duration) error { return nil },
	})
	if err != nil {
		t.Fatalf("WaitForWorkflowCompletion: %v", err)
	}
	if got.Conclusion != "success" {
		t.Fatalf("Conclusion = %q, want success", got.Conclusion)
	}
}

func TestWaitForWorkflowCompletionReportsFailureConclusion(t *testing.T) {
	t.Parallel()

	client := &Client{primary: transportFunc{
		getLatestRun: func(context.Context, RepoRef, LatestRunRequest) (*WorkflowRun, error) {
			return &WorkflowRun{ID: 1, Status: "completed", Conclusion: "failure"}, nil
		},
	}}
	_, err := WaitForWorkflowCompletion(context.Background(), client, RepoRef{Owner: "dilitS", Name: "demo"}, LatestRunRequest{}, PollOptions{
		Sleep: func(context.Context, time.Duration) error { return nil },
	})
	if !errors.Is(err, ErrWorkflowDispatchFailed) {
		t.Fatalf("err = %v, want ErrWorkflowDispatchFailed", err)
	}
}
