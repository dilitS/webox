//nolint:revive // Transport method names intentionally mirror the public Client API.
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/dilitS/webox/internal/log"
)

type CommandRunner interface {
	Run(ctx context.Context, name string, args []string, stdin []byte) (stdout, stderr []byte, err error)
}

type execCommandRunner struct{}

func (execCommandRunner) Run(ctx context.Context, name string, args []string, stdin []byte) (stdoutBytes, stderrBytes []byte, err error) {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // G204: production passes "gh"; tests use the runner seam.
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

type CLITransport struct {
	runner CommandRunner
}

func NewCLITransport(runner CommandRunner) *CLITransport {
	if runner == nil {
		runner = execCommandRunner{}
	}
	return &CLITransport{runner: runner}
}

func (t *CLITransport) CreateRepo(ctx context.Context, req CreateRepoRequest) (*Repository, error) {
	visibility := string(req.Visibility)
	if visibility == "" {
		visibility = string(VisibilityPrivate)
	}
	body := map[string]any{
		"name":        req.Name,
		"description": req.Description,
		"private":     visibility != string(VisibilityPublic),
		"auto_init":   req.AutoInit,
	}
	path := "/user/repos"
	if req.Owner != "" {
		path = "/orgs/" + req.Owner + "/repos"
	}
	var response restRepository
	if err := t.ghAPI(ctx, "POST", path, body, &response); err != nil {
		return nil, err
	}
	return response.repository(), nil
}

func (t *CLITransport) AddDeployKey(ctx context.Context, repo RepoRef, req DeployKeyRequest) (*DeployKey, error) {
	if err := repo.validate(); err != nil {
		return nil, err
	}
	body := map[string]any{
		"title":     req.Title,
		"key":       req.Key,
		"read_only": req.ReadOnly,
	}
	var response DeployKey
	if err := t.ghAPI(ctx, "POST", fmt.Sprintf("/repos/%s/keys", repo.FullName()), body, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (t *CLITransport) SetActionsSecret(ctx context.Context, repo RepoRef, name string, value []byte) error {
	if err := repo.validate(); err != nil {
		return err
	}
	args := []string{"secret", "set", name, "--repo", repo.FullName(), "--body", "-"}
	_, stderr, err := t.runner.Run(ctx, "gh", args, value)
	if err != nil {
		return wrapCLIError("set actions secret", stderr, err)
	}
	return nil
}

func (t *CLITransport) DispatchWorkflow(ctx context.Context, repo RepoRef, req DispatchWorkflowRequest) (*WorkflowDispatch, error) {
	if err := repo.validate(); err != nil {
		return nil, err
	}
	body := map[string]any{
		"ref": req.Ref,
	}
	if len(req.Inputs) > 0 {
		body["inputs"] = req.Inputs
	}
	var response WorkflowDispatch
	err := t.ghAPI(ctx, "POST", fmt.Sprintf("/repos/%s/actions/workflows/%s/dispatches", repo.FullName(), req.WorkflowID), body, &response)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrWorkflowDispatchFailed, err)
	}
	return &response, nil
}

func (t *CLITransport) GetLatestRun(ctx context.Context, repo RepoRef, req LatestRunRequest) (*WorkflowRun, error) {
	if err := repo.validate(); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/repos/%s/actions/runs?per_page=1", repo.FullName())
	if req.Branch != "" {
		path += "&branch=" + req.Branch
	}
	if req.Event != "" {
		path += "&event=" + req.Event
	}
	var response workflowRunsResponse
	if err := t.ghAPI(ctx, "GET", path, nil, &response); err != nil {
		return nil, err
	}
	if len(response.WorkflowRuns) == 0 {
		return nil, nil
	}
	return &response.WorkflowRuns[0], nil
}

func (t *CLITransport) ghAPI(ctx context.Context, method, path string, body, out any) error {
	args := []string{"api", "--method", method, path}
	var stdin []byte
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("github: marshal gh api body: %w", err)
		}
		stdin = raw
		args = append(args, "--input", "-")
	}
	stdout, stderr, err := t.runner.Run(ctx, "gh", args, stdin)
	if err != nil {
		return wrapCLIError(method+" "+path, stderr, err)
	}
	if out == nil || strings.TrimSpace(string(stdout)) == "" {
		return nil
	}
	if err := json.Unmarshal(stdout, out); err != nil {
		return fmt.Errorf("github: parse gh api response for %s %s: %w", method, path, err)
	}
	return nil
}

func wrapCLIError(operation string, stderr []byte, err error) error {
	redacted := strings.TrimSpace(log.Redact(string(stderr)))
	if errors.Is(err, exec.ErrNotFound) || strings.Contains(redacted, "command not found") {
		return fmt.Errorf("%w: %s", ErrGHUnavailable, operation)
	}
	if strings.Contains(redacted, "HTTP 401") || strings.Contains(redacted, "authentication required") {
		return fmt.Errorf("%w: %s", ErrPATInvalid, operation)
	}
	if strings.Contains(redacted, "HTTP 403") {
		return fmt.Errorf("%w: %s: %s", ErrPATScopeInsufficient, operation, redacted)
	}
	if strings.Contains(redacted, "HTTP 422") && strings.Contains(strings.ToLower(redacted), "already") {
		return fmt.Errorf("%w: %s: %s", ErrRepoExists, operation, redacted)
	}
	if redacted == "" {
		redacted = log.Redact(err.Error())
	}
	return fmt.Errorf("github: gh %s: %s: %w", operation, redacted, err)
}
