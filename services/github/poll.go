//nolint:revive // Polling API names mirror GitHub Actions terminology.
package github

import (
	"context"
	"fmt"
	"time"
)

const defaultPollInterval = 5 * time.Second

type RunGetter interface {
	GetLatestRun(ctx context.Context, repo RepoRef, req LatestRunRequest) (*WorkflowRun, error)
}

type PollOptions struct {
	Interval time.Duration
	Timeout  time.Duration
	Sleep    func(context.Context, time.Duration) error
}

func WaitForWorkflowCompletion(ctx context.Context, getter RunGetter, repo RepoRef, req LatestRunRequest, opts PollOptions) (*WorkflowRun, error) {
	if getter == nil {
		return nil, ErrRunGetterNil
	}
	if opts.Interval <= 0 {
		opts.Interval = defaultPollInterval
	}
	if opts.Sleep == nil {
		opts.Sleep = sleepContext
	}
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}
	for {
		run, err := getter.GetLatestRun(ctx, repo, req)
		if err != nil {
			return nil, err
		}
		if run != nil && run.Status == "completed" {
			if run.Conclusion == "success" {
				return run, nil
			}
			return run, fmt.Errorf("%w: run conclusion %q", ErrWorkflowDispatchFailed, run.Conclusion)
		}
		if err := opts.Sleep(ctx, opts.Interval); err != nil {
			return nil, err
		}
	}
}
