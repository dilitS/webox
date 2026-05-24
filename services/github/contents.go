//nolint:revive // Contents methods are part of the GitHub service API.
package github

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/url"
)

func (c *Client) CommitWorkflowFile(ctx context.Context, repo RepoRef, req CommitFileRequest) error {
	_, err := withFallback(ctx, c.primary, c.fallback, func(ctx context.Context, transport Transport) (struct{}, error) {
		contents, ok := transport.(ContentsTransport)
		if !ok {
			return struct{}{}, ErrGHUnavailable
		}
		return struct{}{}, contents.CommitWorkflowFile(ctx, repo, req)
	})
	return err
}

type ContentsTransport interface {
	CommitWorkflowFile(ctx context.Context, repo RepoRef, req CommitFileRequest) error
}

func (t *CLITransport) CommitWorkflowFile(ctx context.Context, repo RepoRef, req CommitFileRequest) error {
	if err := repo.validate(); err != nil {
		return err
	}
	if err := validateCommitFileRequest(req); err != nil {
		return err
	}
	body := commitFileBody(req)
	return t.ghAPI(ctx, "PUT", "/repos/"+repoPath(repo)+"/contents/"+url.PathEscape(req.Path), body, nil)
}

func (t *RESTTransport) CommitWorkflowFile(ctx context.Context, repo RepoRef, req CommitFileRequest) error {
	if err := repo.validate(); err != nil {
		return err
	}
	if err := validateCommitFileRequest(req); err != nil {
		return err
	}
	return t.doJSON(ctx, http.MethodPut, "/repos/"+repoPath(repo)+"/contents/"+url.PathEscape(req.Path), commitFileBody(req), nil)
}

func commitFileBody(req CommitFileRequest) map[string]string {
	message := req.Message
	if message == "" {
		message = "Add Webox deploy workflow"
	}
	body := map[string]string{
		"message": message,
		"content": base64.StdEncoding.EncodeToString(req.Content),
	}
	if req.Branch != "" {
		body["branch"] = req.Branch
	}
	return body
}

func validateCommitFileRequest(req CommitFileRequest) error {
	if req.Path == "" || len(req.Content) == 0 {
		return ErrInvalidCommitFileRequest
	}
	return nil
}
