// Package e2e_test owns the cross-package wiring tests. The
// cPanel scenario lives here so it exercises the real uapi
// HTTPS transport, the real cpanel adapter, and the wizard's
// LIFO rollback through one composed call site.
//
// Sprint 22 TASK-22.5 — the test deliberately avoids the in-memory
// fakes used by providers/cpanel/methods_test.go and
// wizard/cpanel_integration_test.go. Instead, every UAPI call
// hits a httptest.NewTLSServer returning fixture-backed JSON, so
// a regression in the HTTPS wire format (headers, query
// encoding, retry policy) surfaces here before reaching a live
// panel.
package e2e_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dilitS/webox/providers"
	"github.com/dilitS/webox/providers/cpanel"
	"github.com/dilitS/webox/providers/cpanel/uapi"
	"github.com/dilitS/webox/wizard"
)

// cpanelTestServer is the routing layer that fronts the cPanel
// UAPI fixtures. Every (module, function) lands at a handler the
// test scenario registered; unmapped routes return 404 so the
// failure point is the specific call that did not match.
type cpanelTestServer struct {
	t       *testing.T
	mu      sync.Mutex
	handler map[string]http.HandlerFunc // key: "<module>/<function>"
	calls   []string                    // recorded "<module>/<function>?<args>" log
	failOn  map[string]error            // (module/function) -> 500 with this error body
}

func newCpanelTestServer(t *testing.T) *cpanelTestServer {
	t.Helper()
	return &cpanelTestServer{
		t:       t,
		handler: map[string]http.HandlerFunc{},
		failOn:  map[string]error{},
	}
}

// on registers a handler that responds with the fixture file body
// (loaded from providers/cpanel/uapi/testdata) for a (module,
// function) pair.
func (s *cpanelTestServer) on(module, function, fixture string) *cpanelTestServer {
	body := mustReadFixture(s.t, fixture)
	s.handler[module+"/"+function] = func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}
	return s
}

// onStatus500 marks a (module, function) as failing with a 500
// body for the failure scenarios.
func (s *cpanelTestServer) onStatus500(module, function string) *cpanelTestServer {
	s.handler[module+"/"+function] = func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}
	return s
}

// recordedCalls returns a defensive copy of the call log so the
// test scenario can assert on the order/contents of UAPI calls
// without racing the server goroutine. The method name avoids
// colliding with the unexported `calls` field on the struct
// (Go disallows method-vs-field name collisions on the same type).
func (s *cpanelTestServer) recordedCalls() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.calls))
	copy(out, s.calls)
	return out
}

func (s *cpanelTestServer) buildServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/execute/", func(w http.ResponseWriter, r *http.Request) {
		// Authorisation guard — every UAPI call must carry the
		// `cpanel <user>:<token>` header. A regression in the
		// HTTPS client that drops the header would surface here.
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "cpanel ") {
			s.t.Errorf("missing or malformed Authorization header: %q", auth)
			http.Error(w, "auth", http.StatusUnauthorized)
			return
		}
		if !strings.HasPrefix(r.Header.Get("User-Agent"), "webox/") {
			s.t.Errorf("missing webox/<v> User-Agent: %q", r.Header.Get("User-Agent"))
			http.Error(w, "ua", http.StatusBadRequest)
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/execute/")
		s.mu.Lock()
		query := ""
		if r.URL.RawQuery != "" {
			query = "?" + r.URL.RawQuery
		}
		s.calls = append(s.calls, path+query)
		s.mu.Unlock()

		h, ok := s.handler[path]
		if !ok {
			http.Error(w, "no handler for "+path, http.StatusNotFound)
			return
		}
		h(w, r)
	})
	srv := httptest.NewTLSServer(mux)
	s.t.Cleanup(srv.Close)
	return srv
}

func mustReadFixture(t *testing.T, name string) []byte {
	t.Helper()
	// The fixture lives two packages up — /providers/cpanel/uapi/testdata.
	b, err := os.ReadFile(filepath.Join("..", "..", "providers", "cpanel", "uapi", "testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

// buildCpanelProviderForE2E wires a real cpanel.Provider to the
// httptest server through the production uapi.Client +
// uapi.HTTPSMutator constructors. No fakes — the test exercises
// the actual transport layer.
func buildCpanelProviderForE2E(t *testing.T, srv *httptest.Server) *cpanel.Provider {
	t.Helper()

	httpClient := srv.Client()
	httpClient.Timeout = 5 * time.Second

	reader, err := uapi.NewClient(srv.URL, "alice", "TOKEN", httpClient)
	if err != nil {
		t.Fatalf("uapi.NewClient: %v", err)
	}
	mutator, err := uapi.NewHTTPSMutator(srv.URL, "alice", "TOKEN", httpClient)
	if err != nil {
		t.Fatalf("uapi.NewHTTPSMutator: %v", err)
	}

	cfg := providers.ProviderConfig{
		Alias: "vh", Type: "cpanel", Host: "panel.vh.pl", Port: 22, User: "alice",
		Properties: map[string]string{
			"restart_method":       "passenger",
			"node_selector":        "cloudlinux_selector",
			"ssl_provider":         "autossl",
			"domain_kind":          "addon",
			"app_root_template":    "/home/{user}/nodejs/{app_root}",
			"deploy_path_template": "/home/{user}/nodejs/{app_root}/public",
			"log_path_template":    "/home/{user}/nodejs/{app_root}/logs",
		},
	}
	p, err := cpanel.New(cfg)
	if err != nil {
		t.Fatalf("cpanel.New: %v", err)
	}
	cp := p.(*cpanel.Provider)
	cp.SetReader(reader)
	cp.SetMutator(mutator)
	return cp
}

// TestE2E_Cpanel_HappyPath_FullWizardExecution drives the cPanel
// adapter through wizard.Execute with **real** UAPI HTTPS transport
// against a fixture-backed test server. The scenario mirrors the
// post-Sprint-22 production path:
//
//  1. AddAddonDomain        — adds shop.example.com
//  2. CreatePassengerApp    — registers the Node.js Selector app
//  3. StartAutoSSL          — kicks off AutoSSL for the new domain
//  4. CreateMysqlDatabase   — provisions shopdb
//  5. CreateMysqlUser       — provisions the matching user
//  6. SetMysqlPrivileges    — wires user → db
//
// Each step must push its cleanup onto the LIFO stack so a later
// failure can unwind it.
func TestE2E_Cpanel_HappyPath_FullWizardExecution(t *testing.T) {
	t.Setenv("WEBOX_CPANEL_MUTATIONS", "1")

	srv := newCpanelTestServer(t).
		// Domain probe — wizard checks for collisions first.
		on("DomainInfo", "list_domains", "list_domains_ok.json").
		on("PassengerApps", "list_applications", "list_passenger_apps_ok.json").
		// Mutation chain.
		on("DomainInfo", "add_addon_domain", "mut_add_addon_domain_ok.json").
		on("PassengerApps", "create_application", "mut_create_passenger_app_ok.json").
		on("SSL", "start_autossl_check", "mut_start_autossl_ok.json").
		on("Mysql", "create_database", "mut_create_mysql_db_ok.json").
		on("Mysql", "create_user", "mut_create_mysql_user_ok.json").
		on("Mysql", "set_privileges_on_database", "mut_set_mysql_privileges_ok.json").
		buildServer()

	provider := buildCpanelProviderForE2E(t, srv)

	stack := wizard.NewStack(nil, "e2e-cp-happy")
	plan := wizard.ProvisionPlan{
		ProfileAlias: "vh",
		Stack:        wizard.StackNodeExpress,
		Domain:       "shop.example.com",
		NodeVersion:  "22",
		DBKind:       providers.DatabaseMySQL,
		DBName:       "shopdb",
	}

	report, err := wizard.Execute(context.Background(), provider, plan, stack)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !report.Subdomain.OK || !report.SSL.OK || !report.Database.OK {
		t.Fatalf("report not all-OK: %+v", report)
	}
	if report.Credentials == nil || report.Credentials.Password == "" {
		t.Fatalf("credentials missing")
	}
	if got := stack.Len(); got != 3 {
		t.Fatalf("stack len = %d, want 3", got)
	}
}

// TestE2E_Cpanel_SSLFailure_RollbackUnwindsSubdomain proves the
// LIFO rollback runs end-to-end against the real HTTPS transport.
// The SSL endpoint returns a 500; the wizard surfaces the error,
// the caller drives the stack rollback, and we assert the cPanel
// adapter issued DeletePassengerApp + DeleteDomain in the right
// order to unwind the subdomain step.
func TestE2E_Cpanel_SSLFailure_RollbackUnwindsSubdomain(t *testing.T) {
	t.Setenv("WEBOX_CPANEL_MUTATIONS", "1")

	server := newCpanelTestServer(t).
		on("DomainInfo", "list_domains", "list_domains_ok.json").
		on("PassengerApps", "list_applications", "list_passenger_apps_ok.json").
		on("DomainInfo", "add_addon_domain", "mut_add_addon_domain_ok.json").
		on("PassengerApps", "create_application", "mut_create_passenger_app_ok.json").
		// SSL endpoint fails — wizard halts at SetupSSL.
		onStatus500("SSL", "start_autossl_check").
		// Rollback path: adapter calls DeletePassengerApp +
		// DomainInfo/del_domain (idempotent — fixtures return "ok").
		on("PassengerApps", "delete_application", "mut_create_passenger_app_ok.json").
		on("DomainInfo", "del_domain", "mut_del_domain_ok.json")

	srv := server.buildServer()
	provider := buildCpanelProviderForE2E(t, srv)

	stack := wizard.NewStack(nil, "e2e-cp-rb")
	plan := wizard.ProvisionPlan{
		ProfileAlias: "vh",
		Stack:        wizard.StackNodeExpress,
		Domain:       "shop.example.com",
		NodeVersion:  "22",
		DBKind:       providers.DatabaseMySQL,
		DBName:       "shopdb",
	}

	_, err := wizard.Execute(context.Background(), provider, plan, stack)
	if err == nil {
		t.Fatal("expected SSL step to fail")
	}
	// Stack must hold the subdomain step at this point —
	// SSL never pushed because SetupSSL failed.
	if got := stack.Len(); got != 1 {
		t.Fatalf("pre-rollback stack len = %d, want 1", got)
	}

	runner := wizard.MakeStepRunner(provider)
	results, _ := stack.Rollback(context.Background(), runner)
	if len(results) != 1 {
		t.Fatalf("rollback results = %d, want 1", len(results))
	}
	if results[0].Err != nil {
		t.Fatalf("rollback returned error: %v", results[0].Err)
	}

	// Assert the rollback's UAPI footprint hit delete_application
	// then delete_domain — the cpanel adapter's RemoveSubdomain
	// must clean the Passenger app BEFORE the domain (the panel
	// otherwise refuses to drop a domain with a live app).
	calls := server.recordedCalls()
	delAppIdx := -1
	delDomIdx := -1
	for i, c := range calls {
		switch {
		case strings.HasPrefix(c, "PassengerApps/delete_application"):
			delAppIdx = i
		case strings.HasPrefix(c, "DomainInfo/del_domain"):
			delDomIdx = i
		}
	}
	if delAppIdx < 0 || delDomIdx < 0 {
		t.Fatalf("rollback did not invoke both delete endpoints: %v", calls)
	}
	if delAppIdx > delDomIdx {
		t.Fatalf("delete_application (%d) must run BEFORE delete_domain (%d): %v",
			delAppIdx, delDomIdx, calls)
	}
}

// TestE2E_Cpanel_DomainCollision_DetectedBeforeMutation proves the
// wizard's pre-check stops the flow before any mutation lands when
// the chosen domain already exists in the panel's enumerated apps.
//
// The fixture's app list contains "shop.example.com" so the check
// must short-circuit; we assert no mutating endpoint was hit.
func TestE2E_Cpanel_DomainCollision_DetectedBeforeMutation(t *testing.T) {
	t.Setenv("WEBOX_CPANEL_MUTATIONS", "1")

	server := newCpanelTestServer(t).
		on("PassengerApps", "list_applications", "list_passenger_apps_ok.json")
	srv := server.buildServer()
	provider := buildCpanelProviderForE2E(t, srv)

	// The fixture list (list_passenger_apps_ok.json) contains
	// "shop.example.com" — confirmed by inspection. We pass that
	// exact value through CheckSubdomainAvailable.
	err := wizard.CheckSubdomainAvailable(context.Background(), provider, "shop.example.com")
	if !errors.Is(err, providers.ErrSubdomainExists) {
		t.Fatalf("expected ErrSubdomainExists, got %v", err)
	}

	for _, c := range server.recordedCalls() {
		if strings.HasPrefix(c, "PassengerApps/create_application") ||
			strings.HasPrefix(c, "DomainInfo/add_addon_domain") {
			t.Fatalf("collision check leaked a mutation: %s", c)
		}
	}
}

// TestE2E_Cpanel_TokenExpired_SurfacesAuthErrorWithoutRetry confirms
// the HTTPS transport classifies a 401 from the panel as terminal
// (no retry storm) and the wizard surfaces the underlying
// authentication sentinel.
//
// The scenario matches a real-world cPanel token rotation: the
// operator's API token was rotated on the panel side mid-deploy;
// the next UAPI call returns 401. Without the terminal-classify
// guard the wizard would retry 3× with backoff (~3.5s of dead
// time) before giving up — confirming `shouldRetry` in
// providers/cpanel/uapi/transport.go does the right thing.
func TestE2E_Cpanel_TokenExpired_SurfacesAuthErrorWithoutRetry(t *testing.T) {
	t.Setenv("WEBOX_CPANEL_MUTATIONS", "1")

	var hits atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)

	provider := buildCpanelProviderForE2E(t, srv)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := provider.ListSubdomains(ctx)
	if !errors.Is(err, uapi.ErrAuthenticationFailed) {
		t.Fatalf("expected ErrAuthenticationFailed, got %v", err)
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("auth failure was retried %d times — should be 1 (terminal)", got)
	}
}
