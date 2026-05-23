//nolint:revive // Cleanup methods implement wizard rollback's public metadata-only interface.
package github

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

func (c *Client) RemoveGitHubRepo(ctx context.Context, owner, repo string) error {
	_, err := withFallback(ctx, c.primary, c.fallback, func(ctx context.Context, transport Transport) (struct{}, error) {
		cleanup, ok := transport.(CleanupTransport)
		if !ok {
			return struct{}{}, ErrGHUnavailable
		}
		return struct{}{}, cleanup.RemoveGitHubRepo(ctx, owner, repo)
	})
	return err
}

func (c *Client) RemoveGitHubDeployKey(ctx context.Context, owner, repo string, keyID int64) error {
	_, err := withFallback(ctx, c.primary, c.fallback, func(ctx context.Context, transport Transport) (struct{}, error) {
		cleanup, ok := transport.(CleanupTransport)
		if !ok {
			return struct{}{}, ErrGHUnavailable
		}
		return struct{}{}, cleanup.RemoveGitHubDeployKey(ctx, owner, repo, keyID)
	})
	return err
}

func (c *Client) RemoveGitHubActionsSecret(ctx context.Context, owner, repo, name string) error {
	_, err := withFallback(ctx, c.primary, c.fallback, func(ctx context.Context, transport Transport) (struct{}, error) {
		cleanup, ok := transport.(CleanupTransport)
		if !ok {
			return struct{}{}, ErrGHUnavailable
		}
		return struct{}{}, cleanup.RemoveGitHubActionsSecret(ctx, owner, repo, name)
	})
	return err
}

func (c *Client) RemoveGitHubWorkflowFile(ctx context.Context, owner, repo, path, branch string) error {
	_, err := withFallback(ctx, c.primary, c.fallback, func(ctx context.Context, transport Transport) (struct{}, error) {
		cleanup, ok := transport.(CleanupTransport)
		if !ok {
			return struct{}{}, ErrGHUnavailable
		}
		return struct{}{}, cleanup.RemoveGitHubWorkflowFile(ctx, owner, repo, path, branch)
	})
	return err
}

type CleanupTransport interface {
	RemoveGitHubRepo(ctx context.Context, owner, repo string) error
	RemoveGitHubDeployKey(ctx context.Context, owner, repo string, keyID int64) error
	RemoveGitHubActionsSecret(ctx context.Context, owner, repo, name string) error
	RemoveGitHubWorkflowFile(ctx context.Context, owner, repo, path, branch string) error
}

func (t *CLITransport) RemoveGitHubRepo(ctx context.Context, owner, repo string) error {
	return t.ghAPI(ctx, "DELETE", "/repos/"+repoPath(RepoRef{Owner: owner, Name: repo}), nil, nil)
}

func (t *CLITransport) RemoveGitHubDeployKey(ctx context.Context, owner, repo string, keyID int64) error {
	return t.ghAPI(ctx, "DELETE", "/repos/"+repoPath(RepoRef{Owner: owner, Name: repo})+"/keys/"+strconv.FormatInt(keyID, 10), nil, nil)
}

func (t *CLITransport) RemoveGitHubActionsSecret(ctx context.Context, owner, repo, name string) error {
	return t.ghAPI(ctx, "DELETE", "/repos/"+repoPath(RepoRef{Owner: owner, Name: repo})+"/actions/secrets/"+url.PathEscape(name), nil, nil)
}

func (t *CLITransport) RemoveGitHubWorkflowFile(ctx context.Context, owner, repo, path, branch string) error {
	ref := RepoRef{Owner: owner, Name: repo}
	encodedPath := url.PathEscape(path)
	var current struct {
		SHA string `json:"sha"`
	}
	if err := t.ghAPI(ctx, "GET", "/repos/"+repoPath(ref)+"/contents/"+encodedPath+"?ref="+url.QueryEscape(branch), nil, &current); err != nil {
		return err
	}
	if current.SHA == "" {
		return fmt.Errorf("%w: %s", ErrWorkflowFileMissingSHA, path)
	}
	body := map[string]string{
		"message": "Remove Webox deploy workflow",
		"sha":     current.SHA,
		"branch":  branch,
	}
	return t.ghAPI(ctx, "DELETE", "/repos/"+repoPath(ref)+"/contents/"+encodedPath, body, nil)
}

func (t *RESTTransport) RemoveGitHubRepo(ctx context.Context, owner, repo string) error {
	return t.doJSON(ctx, http.MethodDelete, "/repos/"+repoPath(RepoRef{Owner: owner, Name: repo}), nil, nil)
}

func (t *RESTTransport) RemoveGitHubDeployKey(ctx context.Context, owner, repo string, keyID int64) error {
	return t.doJSON(ctx, http.MethodDelete, "/repos/"+repoPath(RepoRef{Owner: owner, Name: repo})+"/keys/"+strconv.FormatInt(keyID, 10), nil, nil)
}

func (t *RESTTransport) RemoveGitHubActionsSecret(ctx context.Context, owner, repo, name string) error {
	return t.doJSON(ctx, http.MethodDelete, "/repos/"+repoPath(RepoRef{Owner: owner, Name: repo})+"/actions/secrets/"+url.PathEscape(name), nil, nil)
}

func (t *RESTTransport) RemoveGitHubWorkflowFile(ctx context.Context, owner, repo, path, branch string) error {
	ref := RepoRef{Owner: owner, Name: repo}
	encodedPath := url.PathEscape(path)
	var current struct {
		SHA string `json:"sha"`
	}
	if err := t.doJSON(ctx, http.MethodGet, "/repos/"+repoPath(ref)+"/contents/"+encodedPath+"?ref="+url.QueryEscape(branch), nil, &current); err != nil {
		return err
	}
	if current.SHA == "" {
		return fmt.Errorf("%w: %s", ErrWorkflowFileMissingSHA, path)
	}
	body := map[string]string{
		"message": "Remove Webox deploy workflow",
		"sha":     current.SHA,
		"branch":  branch,
	}
	return t.doJSON(ctx, http.MethodDelete, "/repos/"+repoPath(ref)+"/contents/"+encodedPath, body, nil)
}
