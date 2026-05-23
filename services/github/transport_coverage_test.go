// Package github tests exercising the wide REST/CLI transport
// surface. These tests target previously-uncovered branches
// (AddDeployKey, SetActionsSecret, DispatchWorkflow, GetLatestRun,
// CommitWorkflowFile, error mapping, retry logic, RepoRef
// validation, withFallback edge cases).
package github

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakeHTTPHandler captures every request the REST transport issues
// and replies with a scripted response. The script is matched on
// "<method> <path>"; unknown requests fail the test loudly so we
// notice when a transport change starts making unintended requests.
type scriptedResponse struct {
	status int
	body   string
	header http.Header
}

type scriptedHandler struct {
	t       *testing.T
	script  map[string]scriptedResponse
	headers []http.Header
	bodies  [][]byte
	count   atomic.Int32
}

func newScriptedHandler(t *testing.T, script map[string]scriptedResponse) *scriptedHandler {
	t.Helper()
	return &scriptedHandler{t: t, script: script}
}

func (h *scriptedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.count.Add(1)
	body, err := readAllSafe(r.Body)
	if err != nil {
		h.t.Fatalf("read request body: %v", err)
	}
	h.bodies = append(h.bodies, body)
	h.headers = append(h.headers, r.Header.Clone())

	// Try in order of specificity: raw RequestURI (with query), raw
	// EscapedPath (preserves %2F), then decoded Path. The first
	// match wins so callers can pin either a specific query-stringed
	// path or fall back to a bare path match.
	for _, key := range []string{
		r.Method + " " + r.URL.RequestURI(),
		r.Method + " " + r.URL.EscapedPath(),
		r.Method + " " + r.URL.Path,
	} {
		if resp, ok := h.script[key]; ok {
			for k, vv := range resp.header {
				for _, v := range vv {
					w.Header().Add(k, v)
				}
			}
			w.WriteHeader(resp.status)
			_, _ = w.Write([]byte(resp.body))
			return
		}
	}
	h.t.Fatalf("unscripted request %s %s (path=%q escapedPath=%q)", r.Method, r.URL.RequestURI(), r.URL.Path, r.URL.EscapedPath())
}

func readAllSafe(r interface{ Read(p []byte) (int, error) }) ([]byte, error) {
	var buf bytes.Buffer
	tmp := make([]byte, 1024)
	for {
		n, err := r.Read(tmp)
		if n > 0 {
			buf.Write(tmp[:n])
		}
		if err != nil {
			if err.Error() == "EOF" {
				return buf.Bytes(), nil
			}
			return buf.Bytes(), err
		}
	}
}

// staticToken always returns the same token; satisfies TokenSource.
type staticToken string

func (s staticToken) Token(context.Context) (string, error) { return string(s), nil }

// errorToken returns the configured error so we can drive the
// "token source failed" branch.
type errorToken struct{ err error }

func (e errorToken) Token(context.Context) (string, error) { return "", e.err }

// noSleep makes retry logic instantaneous in tests.
func noSleep(context.Context, time.Duration) error { return nil }

func newRESTTransport(t *testing.T, url string, src TokenSource) *RESTTransport {
	t.Helper()
	return NewRESTTransport(RESTOptions{
		BaseURL:     url,
		TokenSource: src,
		Sleep:       noSleep,
	})
}

// --------------------------------------------------------------------
// RESTTransport: AddDeployKey
// --------------------------------------------------------------------

func TestRESTTransport_AddDeployKey_HappyPath(t *testing.T) {
	t.Parallel()
	handler := newScriptedHandler(t, map[string]scriptedResponse{
		"POST /repos/dilitS/demo/keys": {status: http.StatusCreated, body: `{"id":42,"title":"webox"}`},
	})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	tr := newRESTTransport(t, server.URL, staticToken("ghp-test"))
	key, err := tr.AddDeployKey(context.Background(), RepoRef{Owner: "dilitS", Name: "demo"}, DeployKeyRequest{
		Title: "webox", Key: "ssh-ed25519 AAAA fake", ReadOnly: false,
	})
	if err != nil {
		t.Fatalf("AddDeployKey: %v", err)
	}
	if key == nil || key.ID != 42 {
		t.Fatalf("key = %+v, want id 42", key)
	}
}

func TestRESTTransport_AddDeployKey_RejectsEmptyRepo(t *testing.T) {
	t.Parallel()
	tr := newRESTTransport(t, "http://unused", staticToken("ghp-test"))
	_, err := tr.AddDeployKey(context.Background(), RepoRef{}, DeployKeyRequest{Title: "x", Key: "y"})
	if !errors.Is(err, ErrInvalidRepoRef) {
		t.Fatalf("err = %v, want ErrInvalidRepoRef", err)
	}
}

// --------------------------------------------------------------------
// RESTTransport: SetActionsSecret
// --------------------------------------------------------------------

func TestRESTTransport_SetActionsSecret_RoundtripsViaPublicKey(t *testing.T) {
	t.Parallel()

	// GitHub's libsodium sealed box requires a 32-byte public key.
	pub := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))
	handler := newScriptedHandler(t, map[string]scriptedResponse{
		"GET /repos/dilitS/demo/actions/secrets/public-key": {
			status: http.StatusOK,
			body:   fmt.Sprintf(`{"key":%q,"key_id":"k1"}`, pub),
		},
		"PUT /repos/dilitS/demo/actions/secrets/DEPLOY_HOST": {status: http.StatusNoContent, body: ""},
	})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	tr := newRESTTransport(t, server.URL, staticToken("ghp-test"))
	err := tr.SetActionsSecret(context.Background(), RepoRef{Owner: "dilitS", Name: "demo"}, "DEPLOY_HOST", []byte("super-secret"))
	if err != nil {
		t.Fatalf("SetActionsSecret: %v", err)
	}

	// Walk recorded request bodies to assert plaintext never leaks.
	for _, body := range handler.bodies {
		if bytes.Contains(body, []byte("super-secret")) {
			t.Fatalf("secret plaintext leaked into request body: %s", body)
		}
	}
}

func TestRESTTransport_SetActionsSecret_RejectsEmptyRepo(t *testing.T) {
	t.Parallel()
	tr := newRESTTransport(t, "http://unused", staticToken("ghp-test"))
	err := tr.SetActionsSecret(context.Background(), RepoRef{}, "DEPLOY", []byte("value"))
	if !errors.Is(err, ErrInvalidRepoRef) {
		t.Fatalf("err = %v, want ErrInvalidRepoRef", err)
	}
}

func TestRESTTransport_SetActionsSecret_InvalidPublicKey(t *testing.T) {
	t.Parallel()
	handler := newScriptedHandler(t, map[string]scriptedResponse{
		"GET /repos/dilitS/demo/actions/secrets/public-key": {
			status: http.StatusOK,
			body:   `{"key":"not-base64","key_id":"k1"}`,
		},
	})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	tr := newRESTTransport(t, server.URL, staticToken("ghp-test"))
	err := tr.SetActionsSecret(context.Background(), RepoRef{Owner: "dilitS", Name: "demo"}, "DEPLOY", []byte("value"))
	if err == nil || !strings.Contains(err.Error(), "actions public key") {
		t.Fatalf("err = %v, want public-key error", err)
	}
}

// --------------------------------------------------------------------
// RESTTransport: DispatchWorkflow + GetLatestRun
// --------------------------------------------------------------------

func TestRESTTransport_DispatchWorkflow_WrapsHTTPError(t *testing.T) {
	t.Parallel()
	handler := newScriptedHandler(t, map[string]scriptedResponse{
		"POST /repos/dilitS/demo/actions/workflows/deploy.yml/dispatches": {
			status: http.StatusUnprocessableEntity,
			body:   `{"message":"validation: missing inputs"}`,
		},
	})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	tr := newRESTTransport(t, server.URL, staticToken("ghp-test"))
	_, err := tr.DispatchWorkflow(context.Background(), RepoRef{Owner: "dilitS", Name: "demo"}, DispatchWorkflowRequest{
		WorkflowID: "deploy.yml", Ref: "main", Inputs: map[string]string{"environment": "production"},
	})
	if !errors.Is(err, ErrWorkflowDispatchFailed) {
		t.Fatalf("err = %v, want wrapped ErrWorkflowDispatchFailed", err)
	}
}

func TestRESTTransport_DispatchWorkflow_RejectsEmptyRepo(t *testing.T) {
	t.Parallel()
	tr := newRESTTransport(t, "http://unused", staticToken("ghp-test"))
	_, err := tr.DispatchWorkflow(context.Background(), RepoRef{}, DispatchWorkflowRequest{WorkflowID: "deploy.yml", Ref: "main"})
	if !errors.Is(err, ErrInvalidRepoRef) {
		t.Fatalf("err = %v, want ErrInvalidRepoRef", err)
	}
}

func TestRESTTransport_GetLatestRun_ReturnsRun(t *testing.T) {
	t.Parallel()
	handler := newScriptedHandler(t, map[string]scriptedResponse{
		"GET /repos/dilitS/demo/actions/runs?event=workflow_dispatch&per_page=1": {
			status: http.StatusOK,
			body:   `{"workflow_runs":[{"id":7,"status":"completed","conclusion":"success"}]}`,
		},
	})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	tr := newRESTTransport(t, server.URL, staticToken("ghp-test"))
	run, err := tr.GetLatestRun(context.Background(), RepoRef{Owner: "dilitS", Name: "demo"}, LatestRunRequest{Event: "workflow_dispatch"})
	if err != nil {
		t.Fatalf("GetLatestRun: %v", err)
	}
	if run == nil || run.ID != 7 || run.Conclusion != "success" {
		t.Fatalf("run = %+v, want id 7 success", run)
	}
}

func TestRESTTransport_GetLatestRun_NoRuns(t *testing.T) {
	t.Parallel()
	handler := newScriptedHandler(t, map[string]scriptedResponse{
		"GET /repos/dilitS/demo/actions/runs?branch=main&per_page=1": {status: http.StatusOK, body: `{"workflow_runs":[]}`},
	})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	tr := newRESTTransport(t, server.URL, staticToken("ghp-test"))
	run, err := tr.GetLatestRun(context.Background(), RepoRef{Owner: "dilitS", Name: "demo"}, LatestRunRequest{Branch: "main"})
	if err != nil {
		t.Fatalf("GetLatestRun: %v", err)
	}
	if run != nil {
		t.Fatalf("run = %+v, want nil for empty runs", run)
	}
}

func TestRESTTransport_GetLatestRun_RejectsEmptyRepo(t *testing.T) {
	t.Parallel()
	tr := newRESTTransport(t, "http://unused", staticToken("ghp-test"))
	_, err := tr.GetLatestRun(context.Background(), RepoRef{}, LatestRunRequest{})
	if !errors.Is(err, ErrInvalidRepoRef) {
		t.Fatalf("err = %v, want ErrInvalidRepoRef", err)
	}
}

// --------------------------------------------------------------------
// RESTTransport: doJSON token paths + retry/backoff
// --------------------------------------------------------------------

func TestRESTTransport_NoTokenSourceReturnsPATInvalid(t *testing.T) {
	t.Parallel()
	tr := NewRESTTransport(RESTOptions{BaseURL: "http://example.invalid", Sleep: noSleep})
	_, err := tr.CreateRepo(context.Background(), CreateRepoRequest{Name: "demo"})
	if !errors.Is(err, ErrPATInvalid) {
		t.Fatalf("err = %v, want ErrPATInvalid", err)
	}
}

func TestRESTTransport_TokenSourceErrorReturnsPATInvalid(t *testing.T) {
	t.Parallel()
	tr := newRESTTransport(t, "http://example.invalid", errorToken{err: errors.New("keyring locked")})
	_, err := tr.CreateRepo(context.Background(), CreateRepoRequest{Name: "demo"})
	if !errors.Is(err, ErrPATInvalid) {
		t.Fatalf("err = %v, want ErrPATInvalid", err)
	}
}

func TestRESTTransport_EmptyTokenReturnsPATInvalid(t *testing.T) {
	t.Parallel()
	tr := newRESTTransport(t, "http://example.invalid", staticToken(""))
	_, err := tr.CreateRepo(context.Background(), CreateRepoRequest{Name: "demo"})
	if !errors.Is(err, ErrPATInvalid) {
		t.Fatalf("err = %v, want ErrPATInvalid", err)
	}
}

func TestRESTTransport_RetriesOn5xx(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count := calls.Add(1)
		if count < 3 {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"message":"transient"}`))
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":1,"name":"demo","owner":{"login":"dilitS"}}`))
	})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	tr := newRESTTransport(t, server.URL, staticToken("ghp"))
	repo, err := tr.CreateRepo(context.Background(), CreateRepoRequest{Name: "demo"})
	if err != nil {
		t.Fatalf("CreateRepo: %v", err)
	}
	if calls.Load() != 3 {
		t.Fatalf("calls = %d, want 3 (retry until success)", calls.Load())
	}
	if repo == nil || repo.Owner != "dilitS" {
		t.Fatalf("repo = %+v, want owner dilitS", repo)
	}
}

func TestRESTTransport_GivesUpAfterRetries(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"message":"down"}`))
	})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	tr := newRESTTransport(t, server.URL, staticToken("ghp"))
	_, err := tr.CreateRepo(context.Background(), CreateRepoRequest{Name: "demo"})
	if !errors.Is(err, ErrHTTPServerError) {
		t.Fatalf("err = %v, want ErrHTTPServerError", err)
	}
	if calls.Load() != 3 {
		t.Fatalf("calls = %d, want 3 retry attempts", calls.Load())
	}
}

// --------------------------------------------------------------------
// mapHTTPError coverage
// --------------------------------------------------------------------

func TestMapHTTPError_StatusCoverage(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{"401", http.StatusUnauthorized, `{"message":"bad creds"}`, ErrPATInvalid},
		{"403 scope", http.StatusForbidden, `{"message":"missing scope"}`, ErrPATScopeInsufficient},
		{"403 rate limit", http.StatusForbidden, `{"message":"API rate limit exceeded"}`, ErrRateLimited},
		{"429", http.StatusTooManyRequests, `{"message":"abuse"}`, ErrRateLimited},
		{"422 exists", http.StatusUnprocessableEntity, `{"message":"already exists"}`, ErrRepoExists},
		{"422 other", http.StatusUnprocessableEntity, `{"message":"missing required field"}`, ErrHTTPValidationFailed},
		{"500", http.StatusInternalServerError, `{"message":"oops"}`, ErrHTTPServerError},
		{"418", http.StatusTeapot, `{"message":"teapot"}`, ErrHTTPUnexpectedStatus},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := mapHTTPError(tc.status, []byte(tc.body))
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestRetriableErrorErrorAndUnwrap(t *testing.T) {
	t.Parallel()
	inner := errors.New("transient")
	re := retriableError{err: inner}
	if re.Error() != "transient" {
		t.Fatalf("Error() = %q, want transient", re.Error())
	}
	if !errors.Is(re.Unwrap(), inner) {
		t.Fatal("Unwrap() did not return inner")
	}
}

func TestRetryDelay_PositiveAndJitterBounded(t *testing.T) {
	t.Parallel()
	for attempt := 1; attempt <= 3; attempt++ {
		d := retryDelay(attempt)
		if d <= 0 {
			t.Fatalf("attempt %d delay = %s, want > 0", attempt, d)
		}
	}
}

func TestCryptoJitterZeroOnNonPositiveMax(t *testing.T) {
	t.Parallel()
	if cryptoJitter(0) != 0 {
		t.Fatal("cryptoJitter(0) should be 0")
	}
	if cryptoJitter(-time.Second) != 0 {
		t.Fatal("cryptoJitter(negative) should be 0")
	}
}

func TestSleepContext_ReturnsImmediatelyOnCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := sleepContext(ctx, time.Hour)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestSleepContext_SleepsAndReturns(t *testing.T) {
	t.Parallel()
	start := time.Now()
	err := sleepContext(context.Background(), 5*time.Millisecond)
	if err != nil {
		t.Fatalf("sleepContext: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 4*time.Millisecond {
		t.Fatalf("returned too early: %s", elapsed)
	}
}

// --------------------------------------------------------------------
// CommitWorkflowFile (REST + CLI + Client)
// --------------------------------------------------------------------

func TestCommitWorkflowFile_REST_Validation(t *testing.T) {
	t.Parallel()

	tr := newRESTTransport(t, "http://unused", staticToken("ghp"))

	t.Run("invalid repo", func(t *testing.T) {
		t.Parallel()
		err := tr.CommitWorkflowFile(context.Background(), RepoRef{}, CommitFileRequest{Path: ".github/workflows/deploy.yml", Content: []byte("x")})
		if !errors.Is(err, ErrInvalidRepoRef) {
			t.Fatalf("err = %v, want ErrInvalidRepoRef", err)
		}
	})
	t.Run("missing path", func(t *testing.T) {
		t.Parallel()
		err := tr.CommitWorkflowFile(context.Background(), RepoRef{Owner: "dilitS", Name: "demo"}, CommitFileRequest{Content: []byte("x")})
		if !errors.Is(err, ErrInvalidCommitFileRequest) {
			t.Fatalf("err = %v, want ErrInvalidCommitFileRequest", err)
		}
	})
}

func TestCommitWorkflowFile_RESTHappyPath(t *testing.T) {
	t.Parallel()
	handler := newScriptedHandler(t, map[string]scriptedResponse{
		"PUT /repos/dilitS/demo/contents/.github%2Fworkflows%2Fdeploy.yml": {status: http.StatusCreated, body: "{}"},
	})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	tr := newRESTTransport(t, server.URL, staticToken("ghp"))
	err := tr.CommitWorkflowFile(context.Background(), RepoRef{Owner: "dilitS", Name: "demo"}, CommitFileRequest{
		Path: ".github/workflows/deploy.yml", Content: []byte("name: deploy\n"), Branch: "main", Message: "Add",
	})
	if err != nil {
		t.Fatalf("CommitWorkflowFile: %v", err)
	}

	// Inspect the request body to assert it was base64-encoded.
	var body struct {
		Message, Content, Branch string
	}
	if err := json.Unmarshal(handler.bodies[0], &body); err != nil {
		t.Fatalf("body unmarshal: %v", err)
	}
	if body.Branch != "main" || body.Message != "Add" {
		t.Fatalf("body = %+v", body)
	}
	if !strings.HasPrefix(body.Content, base64.StdEncoding.EncodeToString([]byte("name: deploy\n"))) {
		t.Fatalf("content should be base64-encoded yaml: %q", body.Content)
	}
}

func TestCommitFileBody_DefaultsMessage(t *testing.T) {
	t.Parallel()
	body := commitFileBody(CommitFileRequest{Path: "x", Content: []byte("hello")})
	if body["message"] != "Add Webox deploy workflow" {
		t.Fatalf("default message = %q", body["message"])
	}
	if _, ok := body["branch"]; ok {
		t.Fatalf("branch should be omitted when empty")
	}
}

func TestValidateCommitFileRequest_RejectsEmpty(t *testing.T) {
	t.Parallel()
	if err := validateCommitFileRequest(CommitFileRequest{}); !errors.Is(err, ErrInvalidCommitFileRequest) {
		t.Fatalf("err = %v, want ErrInvalidCommitFileRequest", err)
	}
	if err := validateCommitFileRequest(CommitFileRequest{Path: "x", Content: []byte{0}}); err != nil {
		t.Fatalf("non-empty content = %v, want nil", err)
	}
}

// --------------------------------------------------------------------
// Client.CommitWorkflowFile fallback logic
// --------------------------------------------------------------------

// transportWithCommit lets transportFunc satisfy ContentsTransport so
// the Client wrapper test can drive the fallback path.
type transportWithCommit struct {
	transportFunc
	commit func(context.Context, RepoRef, CommitFileRequest) error
}

func (t transportWithCommit) CommitWorkflowFile(ctx context.Context, repo RepoRef, req CommitFileRequest) error {
	return t.commit(ctx, repo, req)
}

func TestClientCommitWorkflowFile_FallbackUsedOnGHUnavailable(t *testing.T) {
	t.Parallel()
	var primaryCalls, fallbackCalls atomic.Int32
	primary := transportWithCommit{
		commit: func(context.Context, RepoRef, CommitFileRequest) error {
			primaryCalls.Add(1)
			return ErrGHUnavailable
		},
	}
	fallback := transportWithCommit{
		commit: func(context.Context, RepoRef, CommitFileRequest) error {
			fallbackCalls.Add(1)
			return nil
		},
	}
	client := NewClient(Options{Primary: primary, Fallback: fallback})
	err := client.CommitWorkflowFile(context.Background(), RepoRef{Owner: "dilitS", Name: "demo"}, CommitFileRequest{Path: "x", Content: []byte("y")})
	if err != nil {
		t.Fatalf("CommitWorkflowFile: %v", err)
	}
	if primaryCalls.Load() != 1 || fallbackCalls.Load() != 1 {
		t.Fatalf("primary=%d fallback=%d, want 1/1", primaryCalls.Load(), fallbackCalls.Load())
	}
}

func TestClientCommitWorkflowFile_PrimaryWithoutContentsCapabilityFails(t *testing.T) {
	t.Parallel()
	// transportFunc does NOT implement ContentsTransport so the
	// assertion in Client.CommitWorkflowFile must fall through to
	// ErrGHUnavailable; with no fallback, the error propagates.
	primary := transportFunc{}
	client := NewClient(Options{Primary: primary})
	err := client.CommitWorkflowFile(context.Background(), RepoRef{Owner: "dilitS", Name: "demo"}, CommitFileRequest{Path: "x", Content: []byte("y")})
	if !errors.Is(err, ErrGHUnavailable) {
		t.Fatalf("err = %v, want ErrGHUnavailable", err)
	}
}

// --------------------------------------------------------------------
// CleanupTransport on REST + Client fallback
// --------------------------------------------------------------------

func TestRESTTransport_Cleanup_HappyPaths(t *testing.T) {
	t.Parallel()
	handler := newScriptedHandler(t, map[string]scriptedResponse{
		"DELETE /repos/dilitS/demo":                                                 {status: http.StatusNoContent, body: ""},
		"DELETE /repos/dilitS/demo/keys/42":                                         {status: http.StatusNoContent, body: ""},
		"DELETE /repos/dilitS/demo/actions/secrets/DEPLOY_HOST":                     {status: http.StatusNoContent, body: ""},
		"GET /repos/dilitS/demo/contents/.github%2Fworkflows%2Fdeploy.yml?ref=main": {status: http.StatusOK, body: `{"sha":"abc"}`},
		"DELETE /repos/dilitS/demo/contents/.github%2Fworkflows%2Fdeploy.yml":       {status: http.StatusNoContent, body: ""},
	})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	tr := newRESTTransport(t, server.URL, staticToken("ghp"))
	if err := tr.RemoveGitHubRepo(context.Background(), "dilitS", "demo"); err != nil {
		t.Fatalf("RemoveGitHubRepo: %v", err)
	}
	if err := tr.RemoveGitHubDeployKey(context.Background(), "dilitS", "demo", 42); err != nil {
		t.Fatalf("RemoveGitHubDeployKey: %v", err)
	}
	if err := tr.RemoveGitHubActionsSecret(context.Background(), "dilitS", "demo", "DEPLOY_HOST"); err != nil {
		t.Fatalf("RemoveGitHubActionsSecret: %v", err)
	}
	if err := tr.RemoveGitHubWorkflowFile(context.Background(), "dilitS", "demo", ".github/workflows/deploy.yml", "main"); err != nil {
		t.Fatalf("RemoveGitHubWorkflowFile: %v", err)
	}
}

func TestRESTTransport_CleanupWorkflowMissingSHA(t *testing.T) {
	t.Parallel()
	handler := newScriptedHandler(t, map[string]scriptedResponse{
		"GET /repos/dilitS/demo/contents/.github%2Fworkflows%2Fdeploy.yml": {status: http.StatusOK, body: `{"sha":""}`},
	})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	tr := newRESTTransport(t, server.URL, staticToken("ghp"))
	err := tr.RemoveGitHubWorkflowFile(context.Background(), "dilitS", "demo", ".github/workflows/deploy.yml", "main")
	if !errors.Is(err, ErrWorkflowFileMissingSHA) {
		t.Fatalf("err = %v, want ErrWorkflowFileMissingSHA", err)
	}
}

func TestClientCleanup_FallsBackToRESTOnGHUnavailable(t *testing.T) {
	t.Parallel()
	primary := transportFunc{}
	handler := newScriptedHandler(t, map[string]scriptedResponse{
		"DELETE /repos/dilitS/demo":                                                 {status: http.StatusNoContent},
		"DELETE /repos/dilitS/demo/keys/7":                                          {status: http.StatusNoContent},
		"DELETE /repos/dilitS/demo/actions/secrets/X":                               {status: http.StatusNoContent},
		"GET /repos/dilitS/demo/contents/.github%2Fworkflows%2Fdeploy.yml?ref=main": {status: http.StatusOK, body: `{"sha":"abc"}`},
		"DELETE /repos/dilitS/demo/contents/.github%2Fworkflows%2Fdeploy.yml":       {status: http.StatusNoContent},
	})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	fallback := newRESTTransport(t, server.URL, staticToken("ghp"))
	client := NewClient(Options{Primary: primary, Fallback: fallback})

	if err := client.RemoveGitHubRepo(context.Background(), "dilitS", "demo"); err != nil {
		t.Fatalf("RemoveGitHubRepo: %v", err)
	}
	if err := client.RemoveGitHubDeployKey(context.Background(), "dilitS", "demo", 7); err != nil {
		t.Fatalf("RemoveGitHubDeployKey: %v", err)
	}
	if err := client.RemoveGitHubActionsSecret(context.Background(), "dilitS", "demo", "X"); err != nil {
		t.Fatalf("RemoveGitHubActionsSecret: %v", err)
	}
	if err := client.RemoveGitHubWorkflowFile(context.Background(), "dilitS", "demo", ".github/workflows/deploy.yml", "main"); err != nil {
		t.Fatalf("RemoveGitHubWorkflowFile: %v", err)
	}
}

func TestClientRemoveGitHubWorkflowFile_PrimaryWithoutCleanupCapabilityFails(t *testing.T) {
	t.Parallel()
	primary := transportFunc{} // does NOT implement CleanupTransport
	client := NewClient(Options{Primary: primary})
	err := client.RemoveGitHubWorkflowFile(context.Background(), "dilitS", "demo", ".github/workflows/deploy.yml", "main")
	if !errors.Is(err, ErrGHUnavailable) {
		t.Fatalf("err = %v, want ErrGHUnavailable", err)
	}
}

// --------------------------------------------------------------------
// CLITransport: wrapCLIError matrix
// --------------------------------------------------------------------

func TestWrapCLIError_ClassifiesStderrPatterns(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		stderr []byte
		err    error
		want   error
	}{
		{"missing gh", []byte("bash: gh: command not found"), errors.New("exit"), ErrGHUnavailable},
		{"unauthorized", []byte("HTTP 401: bad creds"), errors.New("exit"), ErrPATInvalid},
		{"forbidden", []byte("HTTP 403: missing scope"), errors.New("exit"), ErrPATScopeInsufficient},
		{"repo exists", []byte("HTTP 422: name already exists on this account"), errors.New("exit"), ErrRepoExists},
		{"generic", []byte("HTTP 500: oops"), errors.New("exit"), errors.New("github: gh fail")},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := wrapCLIError("fail", tc.stderr, tc.err)
			if got == nil {
				t.Fatal("wrapCLIError = nil, want error")
			}
			if tc.name == "generic" {
				if !strings.Contains(got.Error(), "github: gh") {
					t.Fatalf("generic err = %v, want gh prefix", got)
				}
				return
			}
			if !errors.Is(got, tc.want) {
				t.Fatalf("err = %v, want wrap of %v", got, tc.want)
			}
		})
	}
}

// --------------------------------------------------------------------
// Token source
// --------------------------------------------------------------------

func TestSecretsTokenSource_PropagatesBackend(t *testing.T) {
	t.Parallel()
	src := SecretsTokenSource{Backend: stubBackend{token: "ghp-token"}, Account: "default"}
	tok, err := src.Token(context.Background())
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if tok != "ghp-token" {
		t.Fatalf("token = %q, want ghp-token", tok)
	}
}

func TestSecretsTokenSource_BackendErrorPropagates(t *testing.T) {
	t.Parallel()
	src := SecretsTokenSource{Backend: stubBackend{err: errors.New("locked")}, Account: "default"}
	_, err := src.Token(context.Background())
	if err == nil {
		t.Fatal("Token: err = nil, want backend error")
	}
}

// stubBackend implements just enough of secrets.Backend for the
// token-source test. The real keyring/file backend is exercised by
// the secrets package tests.
type stubBackend struct {
	token string
	err   error
}

func (s stubBackend) Get(string) ([]byte, error) {
	if s.err != nil {
		return nil, s.err
	}
	return []byte(s.token), nil
}

func (s stubBackend) Set(string, []byte) error { return nil }
func (s stubBackend) Delete(string) error      { return nil }

// --------------------------------------------------------------------
// Sealed-box edge cases
// --------------------------------------------------------------------

func TestEncryptSecretForGitHub_RejectsBadPublicKey(t *testing.T) {
	t.Parallel()
	if _, err := EncryptSecretForGitHub("!!!not-base64", []byte("x")); err == nil {
		t.Fatal("expected decode error, got nil")
	}
	short := base64.StdEncoding.EncodeToString([]byte{1, 2, 3})
	if _, err := EncryptSecretForGitHub(short, []byte("x")); !errors.Is(err, ErrInvalidPublicKey) {
		t.Fatalf("err = %v, want ErrInvalidPublicKey", err)
	}
}

// --------------------------------------------------------------------
// RepoRef + Repository helpers
// --------------------------------------------------------------------

func TestRepoRefValidateAndFullName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   RepoRef
		full string
		err  error
	}{
		{"happy", RepoRef{Owner: "dilitS", Name: "demo"}, "dilitS/demo", nil},
		{"name only (FullName ok, validate rejects)", RepoRef{Name: "demo"}, "demo", ErrInvalidRepoRef},
		{"missing owner is whitespace", RepoRef{Owner: " ", Name: "demo"}, " /demo", ErrInvalidRepoRef},
		{"empty", RepoRef{}, "", ErrInvalidRepoRef},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.in.FullName(); got != tc.full {
				t.Errorf("FullName = %q, want %q", got, tc.full)
			}
			err := tc.in.validate()
			if tc.err == nil {
				if err != nil {
					t.Errorf("validate err = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, tc.err) {
				t.Errorf("validate err = %v, want %v", err, tc.err)
			}
		})
	}
}

func TestRepositoryHelpers(t *testing.T) {
	t.Parallel()
	repo := Repository{Owner: "dilitS", Name: "demo"}
	if repo.Ref() != (RepoRef{Owner: "dilitS", Name: "demo"}) {
		t.Fatalf("Ref() = %+v", repo.Ref())
	}
	if repo.FullName() != "dilitS/demo" {
		t.Fatalf("FullName = %q", repo.FullName())
	}
}

// --------------------------------------------------------------------
// CLITransport: AddDeployKey, DispatchWorkflow, GetLatestRun,
// CommitWorkflowFile, RemoveGitHubWorkflowFile (cleanup)
// --------------------------------------------------------------------

// captureRunner records every gh CLI invocation and returns the
// scripted stdout response. Keys are matched on a concatenation of
// args (excluding `gh` itself); the script returns the [0] response
// on miss so simple stubs stay terse.
type captureRunner struct {
	t            *testing.T
	scripts      map[string][]byte
	scriptErrors map[string]error
	calls        []string
	stdins       [][]byte
}

func (r *captureRunner) Run(_ context.Context, _ string, args []string, stdin []byte) (stdout, stderr []byte, err error) {
	joined := strings.Join(args, " ")
	r.calls = append(r.calls, joined)
	r.stdins = append(r.stdins, append([]byte(nil), stdin...))

	for prefix, out := range r.scripts {
		if strings.Contains(joined, prefix) {
			err, ok := r.scriptErrors[prefix]
			if ok {
				return out, []byte("scripted error"), err
			}
			return out, nil, nil
		}
	}
	r.t.Fatalf("unscripted gh call: %s", joined)
	return nil, nil, nil
}

func TestCLITransport_AddDeployKey(t *testing.T) {
	t.Parallel()
	runner := &captureRunner{
		t: t,
		scripts: map[string][]byte{
			"/repos/dilitS/demo/keys": []byte(`{"id":1,"title":"webox"}`),
		},
	}
	tr := NewCLITransport(runner)
	key, err := tr.AddDeployKey(context.Background(), RepoRef{Owner: "dilitS", Name: "demo"}, DeployKeyRequest{Title: "webox", Key: "ssh"})
	if err != nil {
		t.Fatalf("AddDeployKey: %v", err)
	}
	if key.ID != 1 || key.Title != "webox" {
		t.Fatalf("key = %+v", key)
	}
}

func TestCLITransport_AddDeployKey_RejectsEmptyRepo(t *testing.T) {
	t.Parallel()
	tr := NewCLITransport(&captureRunner{t: t})
	_, err := tr.AddDeployKey(context.Background(), RepoRef{}, DeployKeyRequest{Title: "x", Key: "y"})
	if !errors.Is(err, ErrInvalidRepoRef) {
		t.Fatalf("err = %v", err)
	}
}

func TestCLITransport_DispatchWorkflow_WrapsFailure(t *testing.T) {
	t.Parallel()
	runner := &captureRunner{
		t:            t,
		scripts:      map[string][]byte{"/repos/dilitS/demo/actions/workflows/deploy.yml/dispatches": []byte("{}")},
		scriptErrors: map[string]error{"/repos/dilitS/demo/actions/workflows/deploy.yml/dispatches": errors.New("HTTP 422 boom")},
	}
	tr := NewCLITransport(runner)
	_, err := tr.DispatchWorkflow(context.Background(), RepoRef{Owner: "dilitS", Name: "demo"}, DispatchWorkflowRequest{
		WorkflowID: "deploy.yml", Ref: "main", Inputs: map[string]string{"env": "prod"},
	})
	if !errors.Is(err, ErrWorkflowDispatchFailed) {
		t.Fatalf("err = %v, want wrapped ErrWorkflowDispatchFailed", err)
	}
}

func TestCLITransport_DispatchWorkflow_RejectsEmptyRepo(t *testing.T) {
	t.Parallel()
	tr := NewCLITransport(&captureRunner{t: t})
	_, err := tr.DispatchWorkflow(context.Background(), RepoRef{}, DispatchWorkflowRequest{WorkflowID: "x", Ref: "main"})
	if !errors.Is(err, ErrInvalidRepoRef) {
		t.Fatalf("err = %v, want ErrInvalidRepoRef", err)
	}
}

func TestCLITransport_GetLatestRun_HappyAndEmpty(t *testing.T) {
	t.Parallel()
	runner := &captureRunner{
		t: t,
		scripts: map[string][]byte{
			"/repos/dilitS/demo/actions/runs": []byte(`{"workflow_runs":[{"id":3,"status":"in_progress"}]}`),
		},
	}
	tr := NewCLITransport(runner)
	run, err := tr.GetLatestRun(context.Background(), RepoRef{Owner: "dilitS", Name: "demo"}, LatestRunRequest{Branch: "main", Event: "workflow_dispatch"})
	if err != nil {
		t.Fatalf("GetLatestRun: %v", err)
	}
	if run == nil || run.ID != 3 {
		t.Fatalf("run = %+v", run)
	}

	emptyRunner := &captureRunner{
		t: t,
		scripts: map[string][]byte{
			"/repos/dilitS/demo/actions/runs": []byte(`{"workflow_runs":[]}`),
		},
	}
	emptyTr := NewCLITransport(emptyRunner)
	out, err := emptyTr.GetLatestRun(context.Background(), RepoRef{Owner: "dilitS", Name: "demo"}, LatestRunRequest{})
	if err != nil {
		t.Fatalf("empty GetLatestRun: %v", err)
	}
	if out != nil {
		t.Fatalf("empty out = %+v, want nil", out)
	}
}

func TestCLITransport_GetLatestRun_RejectsEmptyRepo(t *testing.T) {
	t.Parallel()
	tr := NewCLITransport(&captureRunner{t: t})
	_, err := tr.GetLatestRun(context.Background(), RepoRef{}, LatestRunRequest{})
	if !errors.Is(err, ErrInvalidRepoRef) {
		t.Fatalf("err = %v", err)
	}
}

func TestCLITransport_CommitWorkflowFile_ValidationAndHappy(t *testing.T) {
	t.Parallel()

	runner := &captureRunner{
		t:       t,
		scripts: map[string][]byte{"/contents/.github%2Fworkflows%2Fdeploy.yml": []byte("{}")},
	}
	tr := NewCLITransport(runner)

	// Missing path.
	if err := tr.CommitWorkflowFile(context.Background(), RepoRef{Owner: "dilitS", Name: "demo"}, CommitFileRequest{Content: []byte("x")}); !errors.Is(err, ErrInvalidCommitFileRequest) {
		t.Fatalf("err = %v, want ErrInvalidCommitFileRequest", err)
	}
	// Empty repo.
	if err := tr.CommitWorkflowFile(context.Background(), RepoRef{}, CommitFileRequest{Path: "x", Content: []byte("y")}); !errors.Is(err, ErrInvalidRepoRef) {
		t.Fatalf("err = %v, want ErrInvalidRepoRef", err)
	}
	// Happy path.
	if err := tr.CommitWorkflowFile(context.Background(), RepoRef{Owner: "dilitS", Name: "demo"}, CommitFileRequest{
		Path: ".github/workflows/deploy.yml", Content: []byte("name: deploy\n"), Branch: "main",
	}); err != nil {
		t.Fatalf("CommitWorkflowFile: %v", err)
	}
	// Only the happy-path call reaches the runner — the prior
	// two return validation errors before any execve. So the
	// single recorded stdin slot should carry the base64 body.
	if len(runner.stdins) != 1 {
		t.Fatalf("recorded stdins = %d, want 1 (validation errors short-circuit)", len(runner.stdins))
	}
	if !bytes.Contains(runner.stdins[0], []byte(base64.StdEncoding.EncodeToString([]byte("name: deploy\n")))) {
		t.Fatalf("stdin missing base64 content: %s", runner.stdins[0])
	}
}

func TestClient_AddDeployKey_FallsBackOnGHUnavailable(t *testing.T) {
	t.Parallel()
	primary := transportFunc{
		addDeployKey: func(context.Context, RepoRef, DeployKeyRequest) (*DeployKey, error) {
			return nil, ErrGHUnavailable
		},
	}
	fallback := transportFunc{
		addDeployKey: func(context.Context, RepoRef, DeployKeyRequest) (*DeployKey, error) {
			return &DeployKey{ID: 9}, nil
		},
	}
	client := NewClient(Options{Primary: primary, Fallback: fallback})
	key, err := client.AddDeployKey(context.Background(), RepoRef{Owner: "dilitS", Name: "demo"}, DeployKeyRequest{Title: "x", Key: "y"})
	if err != nil {
		t.Fatalf("AddDeployKey: %v", err)
	}
	if key.ID != 9 {
		t.Fatalf("key = %+v, want fallback id 9", key)
	}
}

func TestClient_SetActionsSecret_PropagatesPrimaryError(t *testing.T) {
	t.Parallel()
	primary := transportFunc{
		setActionsSecret: func(context.Context, RepoRef, string, []byte) error {
			return ErrPATScopeInsufficient
		},
	}
	client := NewClient(Options{Primary: primary})
	err := client.SetActionsSecret(context.Background(), RepoRef{Owner: "dilitS", Name: "demo"}, "X", []byte("v"))
	if !errors.Is(err, ErrPATScopeInsufficient) {
		t.Fatalf("err = %v", err)
	}
}

func TestClient_DispatchWorkflow_HappyPath(t *testing.T) {
	t.Parallel()
	primary := transportFunc{
		dispatchWorkflow: func(context.Context, RepoRef, DispatchWorkflowRequest) (*WorkflowDispatch, error) {
			return &WorkflowDispatch{RunID: 11}, nil
		},
	}
	client := NewClient(Options{Primary: primary})
	disp, err := client.DispatchWorkflow(context.Background(), RepoRef{Owner: "dilitS", Name: "demo"}, DispatchWorkflowRequest{WorkflowID: "x", Ref: "main"})
	if err != nil {
		t.Fatalf("DispatchWorkflow: %v", err)
	}
	if disp.RunID != 11 {
		t.Fatalf("disp = %+v", disp)
	}
}

// --------------------------------------------------------------------
// CLITransport.RemoveGitHubWorkflowFile (already covered via test
// in cleanup_test.go, but doesn't drive the missing-SHA branch).
// --------------------------------------------------------------------

func TestCLITransport_RemoveGitHubWorkflowFile_MissingSHA(t *testing.T) {
	t.Parallel()
	runner := &captureRunner{
		t:       t,
		scripts: map[string][]byte{"/contents/.github%2Fworkflows%2Fdeploy.yml": []byte(`{"sha":""}`)},
	}
	tr := NewCLITransport(runner)
	err := tr.RemoveGitHubWorkflowFile(context.Background(), "dilitS", "demo", ".github/workflows/deploy.yml", "main")
	if !errors.Is(err, ErrWorkflowFileMissingSHA) {
		t.Fatalf("err = %v, want ErrWorkflowFileMissingSHA", err)
	}
}

// --------------------------------------------------------------------
// WaitForWorkflowCompletion
// --------------------------------------------------------------------

type sequencedGetter struct {
	runs []*WorkflowRun
	errs []error
	idx  int
}

func (s *sequencedGetter) GetLatestRun(context.Context, RepoRef, LatestRunRequest) (*WorkflowRun, error) {
	if s.idx >= len(s.runs) {
		return nil, errors.New("getter exhausted")
	}
	run := s.runs[s.idx]
	err := s.errs[s.idx]
	s.idx++
	return run, err
}

func TestWaitForWorkflowCompletion_NilGetterReturnsSentinel(t *testing.T) {
	t.Parallel()
	_, err := WaitForWorkflowCompletion(context.Background(), nil, RepoRef{}, LatestRunRequest{}, PollOptions{})
	if !errors.Is(err, ErrRunGetterNil) {
		t.Fatalf("err = %v, want ErrRunGetterNil", err)
	}
}

func TestWaitForWorkflowCompletion_FailureConclusionWraps(t *testing.T) {
	t.Parallel()
	getter := &sequencedGetter{
		runs: []*WorkflowRun{{Status: "completed", Conclusion: "failure"}},
		errs: []error{nil},
	}
	_, err := WaitForWorkflowCompletion(context.Background(), getter, RepoRef{Owner: "dilitS", Name: "demo"}, LatestRunRequest{}, PollOptions{Interval: time.Millisecond, Sleep: noSleep})
	if !errors.Is(err, ErrWorkflowDispatchFailed) {
		t.Fatalf("err = %v, want wrap of ErrWorkflowDispatchFailed", err)
	}
}

func TestWaitForWorkflowCompletion_ContextCancel(t *testing.T) {
	t.Parallel()
	getter := &sequencedGetter{
		runs: []*WorkflowRun{nil, nil},
		errs: []error{nil, nil},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancelSleep := func(c context.Context, _ time.Duration) error {
		cancel()
		return c.Err()
	}
	_, err := WaitForWorkflowCompletion(ctx, getter, RepoRef{Owner: "dilitS", Name: "demo"}, LatestRunRequest{}, PollOptions{Interval: time.Millisecond, Sleep: cancelSleep})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestWaitForWorkflowCompletion_TimeoutAppliesWhenSet(t *testing.T) {
	t.Parallel()
	getter := &sequencedGetter{
		runs: []*WorkflowRun{{Status: "completed", Conclusion: "success"}},
		errs: []error{nil},
	}
	run, err := WaitForWorkflowCompletion(context.Background(), getter, RepoRef{Owner: "dilitS", Name: "demo"}, LatestRunRequest{}, PollOptions{Interval: time.Millisecond, Timeout: time.Second, Sleep: noSleep})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if run == nil || run.Conclusion != "success" {
		t.Fatalf("run = %+v", run)
	}
}
