//nolint:revive // Exported client methods mirror the documented GitHub service contract.
package github

import (
	"context"
	"errors"
)

// Options wires the high-level client. Supplying both transports gives
// the MVP behavior: try gh CLI first, then REST+PAT only when gh is not
// available.
type Options struct {
	Primary  Transport
	Fallback Transport
}

type Client struct {
	primary  Transport
	fallback Transport
}

func NewClient(opts Options) *Client {
	if opts.Primary == nil {
		opts.Primary = NewCLITransport(nil)
	}
	return &Client{primary: opts.Primary, fallback: opts.Fallback}
}

func (c *Client) CreateRepo(ctx context.Context, req CreateRepoRequest) (*Repository, error) {
	return withFallback(ctx, c.primary, c.fallback, func(ctx context.Context, transport Transport) (*Repository, error) {
		return transport.CreateRepo(ctx, req)
	})
}

func (c *Client) AddDeployKey(ctx context.Context, repo RepoRef, req DeployKeyRequest) (*DeployKey, error) {
	return withFallback(ctx, c.primary, c.fallback, func(ctx context.Context, transport Transport) (*DeployKey, error) {
		return transport.AddDeployKey(ctx, repo, req)
	})
}

func (c *Client) SetActionsSecret(ctx context.Context, repo RepoRef, name string, value []byte) error {
	_, err := withFallback(ctx, c.primary, c.fallback, func(ctx context.Context, transport Transport) (struct{}, error) {
		return struct{}{}, transport.SetActionsSecret(ctx, repo, name, value)
	})
	return err
}

func (c *Client) DispatchWorkflow(ctx context.Context, repo RepoRef, req DispatchWorkflowRequest) (*WorkflowDispatch, error) {
	return withFallback(ctx, c.primary, c.fallback, func(ctx context.Context, transport Transport) (*WorkflowDispatch, error) {
		return transport.DispatchWorkflow(ctx, repo, req)
	})
}

func (c *Client) GetLatestRun(ctx context.Context, repo RepoRef, req LatestRunRequest) (*WorkflowRun, error) {
	return withFallback(ctx, c.primary, c.fallback, func(ctx context.Context, transport Transport) (*WorkflowRun, error) {
		return transport.GetLatestRun(ctx, repo, req)
	})
}

func withFallback[T any](
	ctx context.Context,
	primary Transport,
	fallback Transport,
	call func(context.Context, Transport) (T, error),
) (T, error) {
	value, err := call(ctx, primary)
	if err == nil {
		return value, nil
	}
	if fallback == nil || !errors.Is(err, ErrGHUnavailable) {
		var zero T
		return zero, err
	}
	return call(ctx, fallback)
}
