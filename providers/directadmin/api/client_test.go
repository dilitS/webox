package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// mustFixture loads a JSON fixture from testdata/ or fails the test
// immediately. Mirrors the cpanel/uapi helper pattern.
func mustFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

// newTestServer wraps httptest.NewTLSServer with the DA-specific
// authentication header check. Every endpoint must carry
// `Authorization: Basic <base64(user:loginkey)>` — a regression
// that drops this header would otherwise leak past the unit
// tests and into prod.
func newTestServer(t *testing.T, body []byte, status int) *httptest.Server {
	t.Helper()
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, key, ok := r.BasicAuth()
		if !ok || user == "" || key == "" {
			t.Errorf("missing or malformed Basic auth header")
			http.Error(w, "auth", http.StatusUnauthorized)
			return
		}
		ua := r.Header.Get("User-Agent")
		if !strings.HasPrefix(ua, "webox/") {
			t.Errorf("expected User-Agent webox/<v>, got %q", ua)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
}

// newTestClient returns a Client wired to srv with a short timeout
// so a misbehaving handler can't lock up the suite.
func newTestClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	httpClient := srv.Client()
	httpClient.Timeout = 5 * time.Second
	c, err := NewClient(srv.URL, "alice", "LOGINKEY", httpClient)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	// Strip backoff so retry tests don't burn 3.5s each.
	c.t.backoffFor = func(int) time.Duration { return 0 }
	return c
}

func TestNewClient_RejectsBadInputs(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, baseURL, user, key string
		want                     error
	}{
		{"plain-http", "http://example.com:2222", "u", "k", ErrInvalidEndpoint},
		{"missing-host", "https://", "u", "k", ErrInvalidEndpoint},
		{"bad-scheme", "ftp://example.com", "u", "k", ErrInvalidEndpoint},
		{"missing-user", "https://example.com:2222", "", "k", ErrMissingCredentials},
		{"missing-key", "https://example.com:2222", "u", "", ErrMissingCredentials},
		{"missing-both", "https://example.com:2222", "", "", ErrMissingCredentials},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewClient(tc.baseURL, tc.user, tc.key, nil)
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestClient_Whoami_HappyPath(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t, mustFixture(t, "whoami_ok.json"), http.StatusOK)
	defer srv.Close()
	c := newTestClient(t, srv)

	got, err := c.Whoami(context.Background())
	if err != nil {
		t.Fatalf("Whoami: %v", err)
	}
	if got.Username != "alice" || got.UserType != "user" {
		t.Fatalf("unexpected payload: %+v", got)
	}
	if len(got.Scopes) != 4 {
		t.Fatalf("expected 4 scopes, got %d", len(got.Scopes))
	}
}

func TestClient_ListDomains_HappyPath_SortedByName(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t, mustFixture(t, "list_domains_ok.json"), http.StatusOK)
	defer srv.Close()
	c := newTestClient(t, srv)

	got, err := c.ListDomains(context.Background())
	if err != nil {
		t.Fatalf("ListDomains: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 domains, got %d", len(got))
	}
	// Stable-sort by Name — alice.example.com < blog.example.com < shop.example.com
	want := []string{"alice.example.com", "blog.example.com", "shop.example.com"}
	for i, d := range got {
		if d.Name != want[i] {
			t.Fatalf("sort drift @%d: %q, want %q", i, d.Name, want[i])
		}
	}
	if !got[0].Primary {
		t.Fatal("alice.example.com expected to be primary")
	}
}

// TestClient_ListDomains_AcceptsLegacyWrapperShape covers the
// `{"data": [...]}` wrapper shape some DA installs return instead
// of the modern `{"domains": [...]}` wrapper.
func TestClient_ListDomains_AcceptsLegacyWrapperShape(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t, mustFixture(t, "list_domains_legacy_wrapper.json"), http.StatusOK)
	defer srv.Close()
	c := newTestClient(t, srv)

	got, err := c.ListDomains(context.Background())
	if err != nil {
		t.Fatalf("ListDomains (legacy wrapper): %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 domains, got %d", len(got))
	}
}

// TestClient_ListDomains_AcceptsBareArrayShape covers the bare
// top-level array some CMD-aliased endpoints return.
func TestClient_ListDomains_AcceptsBareArrayShape(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t, mustFixture(t, "list_domains_bare_array.json"), http.StatusOK)
	defer srv.Close()
	c := newTestClient(t, srv)

	got, err := c.ListDomains(context.Background())
	if err != nil {
		t.Fatalf("ListDomains (bare array): %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 domains, got %d", len(got))
	}
}

func TestClient_ListSubdomains_HappyPath(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t, mustFixture(t, "list_subdomains_ok.json"), http.StatusOK)
	defer srv.Close()
	c := newTestClient(t, srv)

	got, err := c.ListSubdomains(context.Background())
	if err != nil {
		t.Fatalf("ListSubdomains: %v", err)
	}
	if len(got) != 2 || got[0].Name != "api.shop.example.com" {
		t.Fatalf("unexpected subdomains: %+v", got)
	}
}

func TestClient_ListDatabases_HappyPath(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t, mustFixture(t, "list_databases_ok.json"), http.StatusOK)
	defer srv.Close()
	c := newTestClient(t, srv)

	got, err := c.ListDatabases(context.Background())
	if err != nil {
		t.Fatalf("ListDatabases: %v", err)
	}
	if len(got) != 2 || got[0].Name != "alice_blogdb" {
		t.Fatalf("expected alice_blogdb first (alpha order), got %+v", got)
	}
}

func TestClient_ListSSLCertificates_HappyPath(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t, mustFixture(t, "list_ssl_certificates_ok.json"), http.StatusOK)
	defer srv.Close()
	c := newTestClient(t, srv)

	got, err := c.ListSSLCertificates(context.Background())
	if err != nil {
		t.Fatalf("ListSSLCertificates: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 certs, got %d", len(got))
	}
	if !got[0].LetsEncrypt {
		t.Fatal("alice.example.com should be LE-managed")
	}
}

func TestClient_AuthFailure_NotRetried(t *testing.T) {
	t.Parallel()
	var hits atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.Whoami(context.Background())
	if !errors.Is(err, ErrAuthenticationFailed) {
		t.Fatalf("expected ErrAuthenticationFailed, got %v", err)
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("auth failure retried %d times; expected 1 (terminal)", got)
	}
}

func TestClient_APIDisabled_VariantsMapToTypedSentinel(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		status int
		body   []byte
	}{
		{"404-on-/api", http.StatusNotFound, []byte("Not Found")},
		{"503-with-canonical-body", http.StatusServiceUnavailable, mustFixture(t, "error_api_disabled_503.json")},
		{"503-with-loose-phrase", http.StatusServiceUnavailable, []byte(`{"error":"feature is not available"}`)},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write(tc.body)
			}))
			defer srv.Close()
			c := newTestClient(t, srv)
			_, err := c.Whoami(context.Background())
			if !errors.Is(err, ErrAPIDisabled) {
				t.Fatalf("got %v, want ErrAPIDisabled", err)
			}
		})
	}
}

func TestClient_503PlainRetriesThenSurfacesServerError(t *testing.T) {
	t.Parallel()
	var hits atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)

	_, err := c.Whoami(context.Background())
	if !errors.Is(err, ErrServerError) {
		t.Fatalf("expected ErrServerError, got %v", err)
	}
	if got := hits.Load(); got != maxRetries+1 {
		t.Fatalf("retry count = %d, want %d (initial + maxRetries)", got, maxRetries+1)
	}
}

func TestClient_RateLimited_RetriesUntilBudgetExhausted(t *testing.T) {
	t.Parallel()
	var hits atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)

	_, err := c.Whoami(context.Background())
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited, got %v", err)
	}
	if got := hits.Load(); got != maxRetries+1 {
		t.Fatalf("retry count = %d", got)
	}
}

func TestClient_TransportUnavailable_MapsConnectionRefused(t *testing.T) {
	t.Parallel()
	// RFC 5737 TEST-NET-1 — guaranteed unreachable.
	c, err := NewClient("https://192.0.2.1:2222", "alice", "KEY", &http.Client{
		Timeout: 500 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	c.t.backoffFor = func(int) time.Duration { return 0 }
	_, err = c.Whoami(context.Background())
	if !errors.Is(err, ErrTransportUnavailable) {
		t.Fatalf("expected ErrTransportUnavailable, got %v", err)
	}
}

func TestClient_ContextCancelDuringRetry_SurfacesAsUnavailable(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, "alice", "KEY", srv.Client())
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	// Force a non-zero backoff so the cancellation lands in
	// `sleepWithCtx` rather than the first call.
	c.t.backoffFor = func(int) time.Duration { return 100 * time.Millisecond }

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err = c.Whoami(ctx)
	if !errors.Is(err, ErrTransportUnavailable) {
		t.Fatalf("expected ErrTransportUnavailable (ctx-cancelled), got %v", err)
	}
}

func TestClient_MalformedResponse_SurfacesTypedError(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t, []byte("this is not json"), http.StatusOK)
	defer srv.Close()
	c := newTestClient(t, srv)

	_, err := c.Whoami(context.Background())
	if !errors.Is(err, ErrMalformedResponse) {
		t.Fatalf("expected ErrMalformedResponse, got %v", err)
	}
}

func TestClient_EmptyBody_ReturnsEmptyList(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t, []byte(""), http.StatusOK)
	defer srv.Close()
	c := newTestClient(t, srv)

	got, err := c.ListDomains(context.Background())
	if err != nil {
		t.Fatalf("ListDomains (empty body): %v", err)
	}
	if len(got) > 0 {
		t.Fatalf("expected empty list, got %+v", got)
	}
}

func TestEndpointURL_AppliesAPIPrefix(t *testing.T) {
	t.Parallel()
	c, err := NewClient("https://da.example.com:2222", "alice", "KEY", nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	tests := []struct {
		ep   Endpoint
		args []any
		want string
	}{
		{EndpointWhoami, nil, "https://da.example.com:2222/api/whoami"},
		{EndpointListDomains, []any{"alice"}, "https://da.example.com:2222/api/users/alice/domains"},
		{EndpointListSubdomains, []any{"alice"}, "https://da.example.com:2222/api/users/alice/subdomains"},
		{EndpointListDatabases, []any{"alice"}, "https://da.example.com:2222/api/users/alice/databases"},
		{EndpointListSSLCertificates, []any{"alice"}, "https://da.example.com:2222/api/users/alice/ssl/certificates"},
	}
	for _, tc := range tests {
		got := c.t.endpointURL(tc.ep, tc.args...)
		if got != tc.want {
			t.Errorf("endpoint %q args %v: got %q, want %q", tc.ep, tc.args, got, tc.want)
		}
	}
}

func TestShouldRetry_TerminalAndTransientClassification(t *testing.T) {
	t.Parallel()
	terminal := []error{
		ErrAuthenticationFailed,
		ErrMalformedResponse,
		ErrAPIDisabled,
		ErrUnexpectedHTTPStatus,
	}
	for _, err := range terminal {
		if shouldRetry(err) {
			t.Errorf("expected terminal: %v", err)
		}
	}
	transient := []error{
		ErrRateLimited,
		ErrServerError,
	}
	for _, err := range transient {
		if !shouldRetry(err) {
			t.Errorf("expected transient: %v", err)
		}
	}
	// Wrapped transport errors → transient.
	if !shouldRetry(errors.New("dial tcp: i/o timeout")) {
		t.Error("expected wrapped http.Client error to be transient")
	}
}

func TestIsAPIDisabledBody_RecognisesCanonicalPhrases(t *testing.T) {
	t.Parallel()
	positive := [][]byte{
		[]byte(`{"error":"Live API is disabled"}`),
		[]byte(`{"error":"the api is disabled by hoster policy"}`),
		[]byte(`feature is not available on this plan`),
		mustFixture(t, "error_api_disabled_503.json"),
	}
	for _, body := range positive {
		if !isAPIDisabledBody(body) {
			t.Errorf("expected positive match: %q", body)
		}
	}
	negative := [][]byte{
		[]byte(`{"error":"internal server error"}`),
		[]byte(`{"error":"DB connection lost"}`),
		[]byte(``),
	}
	for _, body := range negative {
		if isAPIDisabledBody(body) {
			t.Errorf("expected negative match: %q", body)
		}
	}
}
