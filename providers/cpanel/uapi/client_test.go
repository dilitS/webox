package uapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func mustFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

func newTestServerWithBody(t *testing.T, body []byte, status int) *httptest.Server {
	t.Helper()
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Errorf("missing Authorization header")
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "cpanel ") {
			t.Errorf("Authorization header should start with `cpanel `, got %q", r.Header.Get("Authorization"))
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

func newTestClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	httpClient := srv.Client()
	httpClient.Timeout = 5 * time.Second
	c, err := NewClient(srv.URL, "operator", "TOKEN", httpClient)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

func TestNewClient_RejectsBadInputs(t *testing.T) {
	cases := []struct {
		name    string
		baseURL string
		user    string
		token   string
		want    error
	}{
		{"plain-http", "http://example.com:2083", "u", "t", ErrInvalidEndpoint},
		{"missing-host", "https://", "u", "t", ErrInvalidEndpoint},
		{"bad-scheme", "ftp://example.com", "u", "t", ErrInvalidEndpoint},
		{"missing-user", "https://example.com:2083", "", "t", ErrMissingCredentials},
		{"missing-token", "https://example.com:2083", "u", "", ErrMissingCredentials},
		{"missing-both", "https://example.com:2083", "", "", ErrMissingCredentials},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewClient(tc.baseURL, tc.user, tc.token, nil)
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestClient_ListDomains_HappyPath(t *testing.T) {
	srv := newTestServerWithBody(t, mustFixture(t, "list_domains_ok.json"), http.StatusOK)
	defer srv.Close()

	c := newTestClient(t, srv)
	got, err := c.ListDomains(context.Background())
	if err != nil {
		t.Fatalf("ListDomains: %v", err)
	}
	if got.MainDomain != "example.com" {
		t.Errorf("MainDomain = %q, want example.com", got.MainDomain)
	}
	if len(got.SubDomains) != 2 || got.SubDomains[0] != "api.example.com" {
		t.Errorf("SubDomains = %v, want [api.example.com blog.example.com]", got.SubDomains)
	}
	if len(got.AddonDomains) != 1 || got.AddonDomains[0] != "second-example.com" {
		t.Errorf("AddonDomains = %v", got.AddonDomains)
	}
	if len(got.ParkedDomains) != 1 || got.ParkedDomains[0] != "example.net" {
		t.Errorf("ParkedDomains = %v", got.ParkedDomains)
	}
}

func TestClient_ListPassengerApps_ModernShape(t *testing.T) {
	srv := newTestServerWithBody(t, mustFixture(t, "list_passenger_apps_ok.json"), http.StatusOK)
	defer srv.Close()

	c := newTestClient(t, srv)
	got, err := c.ListPassengerApps(context.Background())
	if err != nil {
		t.Fatalf("ListPassengerApps: %v", err)
	}
	if len(got.Applications) != 2 {
		t.Fatalf("len(apps) = %d, want 2", len(got.Applications))
	}
	if got.Applications[0].Name != "marketing-cms" || got.Applications[1].Name != "shop-backend" {
		t.Errorf("apps should be sorted asc by Name: got %s, %s", got.Applications[0].Name, got.Applications[1].Name)
	}
	if got.Applications[1].EnvironmentX["NODE_ENV"] != "production" {
		t.Errorf("envvars missing: %v", got.Applications[1].EnvironmentX)
	}
}

func TestClient_ListPassengerApps_LegacyMapShape(t *testing.T) {
	srv := newTestServerWithBody(t, mustFixture(t, "list_passenger_apps_legacy.json"), http.StatusOK)
	defer srv.Close()

	c := newTestClient(t, srv)
	got, err := c.ListPassengerApps(context.Background())
	if err != nil {
		t.Fatalf("ListPassengerApps: %v", err)
	}
	if len(got.Applications) != 2 {
		t.Fatalf("len(apps) = %d, want 2", len(got.Applications))
	}
	gotNames := []string{got.Applications[0].Name, got.Applications[1].Name}
	if gotNames[0] != "marketing-cms" || gotNames[1] != "shop-backend" {
		t.Errorf("legacy map shape lost Name keys, got %v", gotNames)
	}
}

func TestClient_ListMysqlDatabases_HappyPath(t *testing.T) {
	srv := newTestServerWithBody(t, mustFixture(t, "list_mysql_databases_ok.json"), http.StatusOK)
	defer srv.Close()

	c := newTestClient(t, srv)
	got, err := c.ListMysqlDatabases(context.Background())
	if err != nil {
		t.Fatalf("ListMysqlDatabases: %v", err)
	}
	if len(got.Databases) != 2 {
		t.Fatalf("len(dbs) = %d, want 2", len(got.Databases))
	}
	if got.Databases[0].Name != "operator_blog" || got.Databases[1].Name != "operator_shop" {
		t.Errorf("dbs should be sorted asc: %v", got.Databases)
	}
	if got.Databases[1].DiskUsage != 8421376 {
		t.Errorf("DiskUsage = %d, want 8421376", got.Databases[1].DiskUsage)
	}
}

func TestClient_ListSSLKeys_HappyPath(t *testing.T) {
	srv := newTestServerWithBody(t, mustFixture(t, "list_ssl_keys_ok.json"), http.StatusOK)
	defer srv.Close()

	c := newTestClient(t, srv)
	got, err := c.ListSSLKeys(context.Background())
	if err != nil {
		t.Fatalf("ListSSLKeys: %v", err)
	}
	if len(got.Keys) != 2 {
		t.Fatalf("len(keys) = %d, want 2", len(got.Keys))
	}
	if got.Keys[0].NotAfter == "" {
		t.Error("NotAfter should be populated for renewal scheduling")
	}
}

func TestClient_AuthErrorIsTerminal(t *testing.T) {
	srv := newTestServerWithBody(t, []byte(`{"result":{"status":0,"errors":["unauthorized"]}}`), http.StatusUnauthorized)
	defer srv.Close()
	c := newTestClient(t, srv)
	_, err := c.ListDomains(context.Background())
	if !errors.Is(err, ErrAuthenticationFailed) {
		t.Fatalf("got %v, want ErrAuthenticationFailed", err)
	}
}

func TestClient_RateLimitedIsTerminalAfterRetries(t *testing.T) {
	var calls int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"result":{"status":0,"errors":["rate limit exceeded"]}}`)
	})
	srv := httptest.NewTLSServer(handler)
	defer srv.Close()

	httpClient := srv.Client()
	httpClient.Timeout = 5 * time.Second
	c, err := NewClient(srv.URL, "operator", "TOKEN", httpClient)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	speedUpBackoff(t, c)

	_, err = c.ListDomains(context.Background())
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("got %v, want ErrRateLimited", err)
	}
	if got := atomic.LoadInt32(&calls); got != int32(maxRetries+1) {
		t.Errorf("got %d HTTP calls, want %d (initial + %d retries)", got, maxRetries+1, maxRetries)
	}
}

func TestClient_ServerErrorRetriesThenSurfaces(t *testing.T) {
	var calls int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, "oops")
	}))
	defer srv.Close()

	httpClient := srv.Client()
	httpClient.Timeout = 5 * time.Second
	c, err := NewClient(srv.URL, "operator", "TOKEN", httpClient)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	speedUpBackoff(t, c)

	_, err = c.ListDomains(context.Background())
	if !errors.Is(err, ErrServerError) {
		t.Fatalf("got %v, want ErrServerError", err)
	}
	if got := atomic.LoadInt32(&calls); got != int32(maxRetries+1) {
		t.Errorf("got %d HTTP calls, want %d", got, maxRetries+1)
	}
}

func TestClient_MalformedBodyTerminates(t *testing.T) {
	srv := newTestServerWithBody(t, []byte(`<html>not json</html>`), http.StatusOK)
	defer srv.Close()
	c := newTestClient(t, srv)
	_, err := c.ListDomains(context.Background())
	if !errors.Is(err, ErrMalformedResponse) {
		t.Fatalf("got %v, want ErrMalformedResponse", err)
	}
}

func TestClient_ModuleDeniedIsTerminal(t *testing.T) {
	srv := newTestServerWithBody(t, mustFixture(t, "error_module_denied.json"), http.StatusOK)
	defer srv.Close()
	c := newTestClient(t, srv)
	_, err := c.ListDomains(context.Background())
	if !errors.Is(err, ErrModuleFunctionDenied) {
		t.Fatalf("got %v, want ErrModuleFunctionDenied", err)
	}
}

func TestClient_GenericStatusZeroIsGenericError(t *testing.T) {
	srv := newTestServerWithBody(t, mustFixture(t, "error_invalid_envelope.json"), http.StatusOK)
	defer srv.Close()
	c := newTestClient(t, srv)
	_, err := c.ListDomains(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if errors.Is(err, ErrModuleFunctionDenied) {
		t.Fatalf("generic error should NOT match ErrModuleFunctionDenied: %v", err)
	}
	if errors.Is(err, ErrAuthenticationFailed) {
		t.Fatalf("generic error should NOT match ErrAuthenticationFailed: %v", err)
	}
}

func TestClient_ContextCancelInterrupts(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{"result":{"status":1,"data":{}}}`)
		}
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.ListDomains(ctx)
	if err == nil {
		t.Fatal("expected context error, got nil")
	}
}

func TestMutatingClient_AlwaysReturnsScopeError(t *testing.T) {
	m := NewStubMutatingClient()
	err := m.Call(context.Background(), ModuleDomainInfo, FunctionDomainInfoList, nil)
	if !errors.Is(err, ErrSprintScopeNotMutable) {
		t.Fatalf("got %v, want ErrSprintScopeNotMutable", err)
	}
}

func TestAllReadOnlyModules_StableSet(t *testing.T) {
	got := AllReadOnlyModules()
	want := []Module{ModuleDomainInfo, ModulePassengerApps, ModuleMysql, ModuleSSL}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("[%d] got %s, want %s", i, got[i], want[i])
		}
	}
}

// speedUpBackoff replaces the transport's backoff with a zero
// duration so the retry loop completes in test time. We can't
// reach into the unexported field from outside the package, so
// this helper lives in the same package as transport.go.
func speedUpBackoff(t *testing.T, c *Client) {
	t.Helper()
	c.transport.backoffFor = func(int) time.Duration { return 0 }
}
