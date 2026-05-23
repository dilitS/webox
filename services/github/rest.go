//nolint:revive // Transport method names intentionally mirror the public Client API.
package github

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dilitS/webox/internal/log"
)

const defaultBaseURL = "https://api.github.com"
const (
	maxResponseBytes = 1 << 20
	serverErrorMin   = 500
	baseRetryDelay   = 200 * time.Millisecond
	jitterDivisor    = 2
)

type TokenSource interface {
	Token(ctx context.Context) (string, error)
}

type RESTOptions struct {
	BaseURL     string
	HTTPClient  *http.Client
	TokenSource TokenSource
	Sleep       func(context.Context, time.Duration) error
}

type RESTTransport struct {
	baseURL     string
	httpClient  *http.Client
	tokenSource TokenSource
	sleep       func(context.Context, time.Duration) error
}

func NewRESTTransport(opts RESTOptions) *RESTTransport {
	if opts.BaseURL == "" {
		opts.BaseURL = defaultBaseURL
	}
	if opts.HTTPClient == nil {
		opts.HTTPClient = http.DefaultClient
	}
	if opts.Sleep == nil {
		opts.Sleep = sleepContext
	}
	return &RESTTransport{
		baseURL:     strings.TrimRight(opts.BaseURL, "/"),
		httpClient:  opts.HTTPClient,
		tokenSource: opts.TokenSource,
		sleep:       opts.Sleep,
	}
}

func (t *RESTTransport) CreateRepo(ctx context.Context, req CreateRepoRequest) (*Repository, error) {
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
		path = "/orgs/" + url.PathEscape(req.Owner) + "/repos"
	}
	var response restRepository
	if err := t.doJSON(ctx, http.MethodPost, path, body, &response); err != nil {
		return nil, err
	}
	return response.repository(), nil
}

func (t *RESTTransport) AddDeployKey(ctx context.Context, repo RepoRef, req DeployKeyRequest) (*DeployKey, error) {
	if err := repo.validate(); err != nil {
		return nil, err
	}
	body := map[string]any{
		"title":     req.Title,
		"key":       req.Key,
		"read_only": req.ReadOnly,
	}
	var response DeployKey
	if err := t.doJSON(ctx, http.MethodPost, "/repos/"+repoPath(repo)+"/keys", body, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (t *RESTTransport) SetActionsSecret(ctx context.Context, repo RepoRef, name string, value []byte) error {
	if err := repo.validate(); err != nil {
		return err
	}
	var key actionsPublicKey
	if err := t.doJSON(ctx, http.MethodGet, "/repos/"+repoPath(repo)+"/actions/secrets/public-key", nil, &key); err != nil {
		return err
	}
	encrypted, err := EncryptSecretForGitHub(key.Key, value)
	if err != nil {
		return err
	}
	body := map[string]string{
		"encrypted_value": encrypted,
		"key_id":          key.KeyID,
	}
	return t.doJSON(ctx, http.MethodPut, "/repos/"+repoPath(repo)+"/actions/secrets/"+url.PathEscape(name), body, nil)
}

func (t *RESTTransport) DispatchWorkflow(ctx context.Context, repo RepoRef, req DispatchWorkflowRequest) (*WorkflowDispatch, error) {
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
	err := t.doJSON(ctx, http.MethodPost, "/repos/"+repoPath(repo)+"/actions/workflows/"+url.PathEscape(req.WorkflowID)+"/dispatches", body, &response)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrWorkflowDispatchFailed, err)
	}
	return &response, nil
}

// GetWorkflowSteps fetches the workflow run's jobs+steps via REST. The
// payload mirrors the gh CLI projection so callers can switch
// transports transparently.
func (t *RESTTransport) GetWorkflowSteps(ctx context.Context, repo RepoRef, runID int64) ([]Step, error) {
	if err := repo.validate(); err != nil {
		return nil, err
	}
	if runID <= 0 {
		return nil, ErrRunNotFound
	}
	var response jobsResponse
	if err := t.doJSON(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/actions/runs/%d/jobs", repoPath(repo), runID), nil, &response); err != nil {
		if errors.Is(err, ErrHTTPUnexpectedStatus) && strings.Contains(err.Error(), "404") {
			return nil, ErrRunNotFound
		}
		return nil, err
	}
	if len(response.Jobs) == 0 {
		return nil, ErrRunNotFound
	}
	steps := response.flatten()
	if len(steps) == 0 {
		return nil, ErrStepsParseError
	}
	return steps, nil
}

// GetWorkflowLogs returns ErrPATScopeInsufficient: the REST endpoint
// streams a zip we don't unpack in-process. The CI/CD modal expects
// the gh CLI to be available for logs; rely on the fallback chain in
// [Client.GetWorkflowLogs] (gh first → REST returns this typed error
// → modal surfaces the "logs require gh CLI" hint).
func (t *RESTTransport) GetWorkflowLogs(_ context.Context, _ RepoRef, _ int64, _ int) ([]WorkflowLogLine, error) {
	return nil, fmt.Errorf("%w: REST log fallback unsupported; install gh CLI", ErrPATScopeInsufficient)
}

func (t *RESTTransport) GetLatestRun(ctx context.Context, repo RepoRef, req LatestRunRequest) (*WorkflowRun, error) {
	if err := repo.validate(); err != nil {
		return nil, err
	}
	values := url.Values{"per_page": []string{"1"}}
	if req.Branch != "" {
		values.Set("branch", req.Branch)
	}
	if req.Event != "" {
		values.Set("event", req.Event)
	}
	var response workflowRunsResponse
	if err := t.doJSON(ctx, http.MethodGet, "/repos/"+repoPath(repo)+"/actions/runs?"+values.Encode(), nil, &response); err != nil {
		return nil, err
	}
	if len(response.WorkflowRuns) == 0 {
		return nil, nil
	}
	return &response.WorkflowRuns[0], nil
}

func (t *RESTTransport) doJSON(ctx context.Context, method, path string, body, out any) error {
	if t.tokenSource == nil {
		return ErrPATInvalid
	}
	token, err := t.tokenSource.Token(ctx)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrPATInvalid, err)
	}
	if token == "" {
		return ErrPATInvalid
	}

	var payload []byte
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("github: marshal request: %w", err)
		}
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			if err := t.sleep(ctx, retryDelay(attempt)); err != nil {
				return err
			}
		}
		lastErr = t.doOnce(ctx, method, path, payload, token, out)
		if lastErr == nil || !isRetriable(lastErr) {
			return lastErr
		}
	}
	return lastErr
}

func (t *RESTTransport) doOnce(ctx context.Context, method, path string, payload []byte, token string, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, t.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("github: build request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("github: http request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("github: read response: %w", err)
	}
	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		if out == nil || len(bytes.TrimSpace(raw)) == 0 {
			return nil
		}
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("github: parse response: %w", err)
		}
		return nil
	}
	return mapHTTPError(resp.StatusCode, raw)
}

type retriableError struct {
	err error
}

func (e retriableError) Error() string { return e.err.Error() }
func (e retriableError) Unwrap() error { return e.err }

func isRetriable(err error) bool {
	var retry retriableError
	return errors.As(err, &retry)
}

func mapHTTPError(status int, raw []byte) error {
	body := strings.TrimSpace(log.Redact(string(raw)))
	switch status {
	case http.StatusUnauthorized:
		return fmt.Errorf("%w: %s", ErrPATInvalid, body)
	case http.StatusForbidden:
		if strings.Contains(strings.ToLower(body), "rate limit") {
			return retriableError{err: fmt.Errorf("%w: %s", ErrRateLimited, body)}
		}
		return fmt.Errorf("%w: %s", ErrPATScopeInsufficient, body)
	case http.StatusTooManyRequests:
		return retriableError{err: fmt.Errorf("%w: %s", ErrRateLimited, body)}
	case http.StatusUnprocessableEntity:
		if strings.Contains(strings.ToLower(body), "already") {
			return fmt.Errorf("%w: %s", ErrRepoExists, body)
		}
		return fmt.Errorf("%w: %s", ErrHTTPValidationFailed, body)
	default:
		if status >= serverErrorMin {
			return retriableError{err: fmt.Errorf("%w: %d: %s", ErrHTTPServerError, status, body)}
		}
		return fmt.Errorf("%w: %d: %s", ErrHTTPUnexpectedStatus, status, body)
	}
}

func retryDelay(attempt int) time.Duration {
	base := time.Duration(1<<attempt) * baseRetryDelay
	jitter := cryptoJitter(base / jitterDivisor)
	return base + jitter
}

func cryptoJitter(maxDelay time.Duration) time.Duration {
	if maxDelay <= 0 {
		return 0
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(maxDelay)))
	if err != nil {
		return 0
	}
	return time.Duration(n.Int64())
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func repoPath(repo RepoRef) string {
	return url.PathEscape(repo.Owner) + "/" + url.PathEscape(repo.Name)
}

type restRepository struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	HTMLURL  string `json:"html_url"`
	SSHURL   string `json:"ssh_url"`
	Owner    struct {
		Login string `json:"login"`
	} `json:"owner"`
}

func (r restRepository) repository() *Repository {
	return &Repository{
		ID:      r.ID,
		Owner:   r.Owner.Login,
		Name:    r.Name,
		HTMLURL: r.HTMLURL,
		SSHURL:  r.SSHURL,
	}
}

type actionsPublicKey struct {
	KeyID string `json:"key_id"`
	Key   string `json:"key"`
}

type workflowRunsResponse struct {
	WorkflowRuns []WorkflowRun `json:"workflow_runs"`
}
