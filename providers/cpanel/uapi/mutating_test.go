package uapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// setMutationsAllowed flips the env-var guard for the duration of
// a single test. We use t.Setenv so the runtime restores the
// previous value automatically — no manual cleanup, no leakage
// between subtests, and parallel-safe at the test level (Setenv
// disables t.Parallel for the test, which is exactly the
// guarantee we need here).
func setMutationsAllowed(t *testing.T, on bool) {
	t.Helper()
	if on {
		t.Setenv(EnvMutationsAllowed, "1")
		return
	}
	t.Setenv(EnvMutationsAllowed, "")
}

func TestMutationsAllowed_RespectsEnv(t *testing.T) {
	cases := []struct {
		name string
		val  string
		want bool
	}{
		{"unset", "", false},
		{"zero", "0", false},
		{"true-literal", "true", false},
		{"one", "1", true},
		{"padded-one", "  1  ", true},
		{"other-digit", "2", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(EnvMutationsAllowed, tc.val)
			if got := MutationsAllowed(); got != tc.want {
				t.Errorf("MutationsAllowed = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestNewHTTPSMutator_ValidatesLikeClient(t *testing.T) {
	cases := []struct {
		name    string
		baseURL string
		user    string
		token   string
		want    error
	}{
		{"plain-http", "http://example.com:2083", "u", "t", ErrInvalidEndpoint},
		{"missing-user", "https://example.com:2083", "", "t", ErrMissingCredentials},
		{"missing-token", "https://example.com:2083", "u", "", ErrMissingCredentials},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewHTTPSMutator(tc.baseURL, tc.user, tc.token, nil)
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestHTTPSMutator_GuardBlocksWhenEnvUnset(t *testing.T) {
	setMutationsAllowed(t, false)
	srv := newTestServerWithBody(t, mustFixture(t, "mut_add_addon_domain_ok.json"), http.StatusOK)
	defer srv.Close()
	m, err := NewHTTPSMutator(srv.URL, "operator", "TOKEN", srv.Client())
	if err != nil {
		t.Fatalf("NewHTTPSMutator: %v", err)
	}

	cases := []struct {
		name string
		call func() error
	}{
		{"add-addon-domain", func() error {
			return m.AddAddonDomain(context.Background(), CreateAddonDomainArgs{NewDomain: "x.example.com", Dir: "public_html/x"})
		}},
		{"add-subdomain", func() error {
			return m.AddSubdomain(context.Background(), CreateSubdomainArgs{Domain: "api", RootDomain: "example.com", Dir: "public_html/api"})
		}},
		{"delete-domain", func() error { return m.DeleteDomain(context.Background(), "x.example.com") }},
		{"delete-subdomain", func() error { return m.DeleteSubdomain(context.Background(), "api.example.com") }},
		{"create-passenger-app", func() error {
			return m.CreatePassengerApp(context.Background(), CreatePassengerAppArgs{Name: "n", Path: "/p", Domain: "d.example.com"})
		}},
		{"edit-passenger-app", func() error {
			return m.EditPassengerApp(context.Background(), EditPassengerAppArgs{Path: "/p"})
		}},
		{"restart-passenger-app", func() error { return m.RestartPassengerApp(context.Background(), "/p") }},
		{"delete-passenger-app", func() error { return m.DeletePassengerApp(context.Background(), "/p") }},
		{"create-mysql-db", func() error { return m.CreateMysqlDatabase(context.Background(), "db") }},
		{"delete-mysql-db", func() error { return m.DeleteMysqlDatabase(context.Background(), "db") }},
		{"create-mysql-user", func() error { return m.CreateMysqlUser(context.Background(), "u", "p") }},
		{"delete-mysql-user", func() error { return m.DeleteMysqlUser(context.Background(), "u") }},
		{"set-mysql-privs", func() error {
			return m.SetMysqlPrivileges(context.Background(), MysqlPrivilegesArgs{Database: "db", User: "u", Privileges: []string{"ALL"}})
		}},
		{"install-ssl", func() error {
			return m.InstallSSL(context.Background(), InstallSSLArgs{Domain: "d.example.com", Cert: "c", Key: "k"})
		}},
		{"start-autossl", func() error { return m.StartAutoSSL(context.Background(), "d.example.com") }},
		{"delete-ssl", func() error { return m.DeleteSSL(context.Background(), "d.example.com") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			if !errors.Is(err, ErrMutationsDisabled) {
				t.Fatalf("got %v, want ErrMutationsDisabled", err)
			}
		})
	}
}

func TestHTTPSMutator_GuardBlocksBeforeNetwork(t *testing.T) {
	setMutationsAllowed(t, false)
	var hits atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":{"status":1}}`))
	}))
	defer srv.Close()
	m, err := NewHTTPSMutator(srv.URL, "operator", "TOKEN", srv.Client())
	if err != nil {
		t.Fatalf("NewHTTPSMutator: %v", err)
	}
	if err := m.AddAddonDomain(context.Background(), CreateAddonDomainArgs{NewDomain: "x.example.com", Dir: "public_html/x"}); !errors.Is(err, ErrMutationsDisabled) {
		t.Fatalf("got %v, want ErrMutationsDisabled", err)
	}
	if hits.Load() != 0 {
		t.Fatalf("guard leaked: server saw %d hits", hits.Load())
	}
}

func TestHTTPSMutator_ValidatesArgs(t *testing.T) {
	setMutationsAllowed(t, true)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":{"status":1}}`))
	}))
	defer srv.Close()
	m, err := NewHTTPSMutator(srv.URL, "operator", "TOKEN", srv.Client())
	if err != nil {
		t.Fatalf("NewHTTPSMutator: %v", err)
	}

	cases := []struct {
		name string
		call func() error
	}{
		{"empty-domain", func() error {
			return m.AddAddonDomain(context.Background(), CreateAddonDomainArgs{Dir: "public_html/x"})
		}},
		{"empty-dir", func() error {
			return m.AddAddonDomain(context.Background(), CreateAddonDomainArgs{NewDomain: "x.example.com"})
		}},
		{"control-char-domain", func() error {
			return m.AddAddonDomain(context.Background(), CreateAddonDomainArgs{NewDomain: "x.example.com\n", Dir: "public_html/x"})
		}},
		{"empty-app-name", func() error {
			return m.CreatePassengerApp(context.Background(), CreatePassengerAppArgs{Path: "/p", Domain: "d"})
		}},
		{"empty-mysql-user", func() error {
			return m.CreateMysqlUser(context.Background(), "", "password")
		}},
		{"empty-mysql-password", func() error {
			return m.CreateMysqlUser(context.Background(), "user", "")
		}},
		{"empty-privileges", func() error {
			return m.SetMysqlPrivileges(context.Background(), MysqlPrivilegesArgs{Database: "db", User: "u"})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); !errors.Is(err, ErrInvalidArgs) {
				t.Fatalf("got %v, want ErrInvalidArgs", err)
			}
		})
	}
}

func TestHTTPSMutator_HappyPath_AllMethods(t *testing.T) {
	setMutationsAllowed(t, true)

	cases := []struct {
		name    string
		fixture string
		call    func(*HTTPSMutator) error
	}{
		{"add-addon-domain", "mut_add_addon_domain_ok.json", func(m *HTTPSMutator) error {
			return m.AddAddonDomain(context.Background(), CreateAddonDomainArgs{NewDomain: "client.example.com", Dir: "public_html/client"})
		}},
		{"add-subdomain", "mut_add_addon_domain_ok.json", func(m *HTTPSMutator) error {
			return m.AddSubdomain(context.Background(), CreateSubdomainArgs{Domain: "api", RootDomain: "example.com", Dir: "public_html/api"})
		}},
		{"delete-domain", "mut_del_domain_ok.json", func(m *HTTPSMutator) error {
			return m.DeleteDomain(context.Background(), "client.example.com")
		}},
		{"delete-subdomain", "mut_del_domain_ok.json", func(m *HTTPSMutator) error {
			return m.DeleteSubdomain(context.Background(), "api.example.com")
		}},
		{"create-passenger-app", "mut_create_passenger_app_ok.json", func(m *HTTPSMutator) error {
			return m.CreatePassengerApp(context.Background(), CreatePassengerAppArgs{
				Name:           "shop-backend",
				Path:           "/home/operator/apps/shop-backend",
				Domain:         "shop.example.com",
				BaseURI:        "/",
				DeploymentMode: "production",
				Envvars:        map[string]string{"NODE_ENV": "production", "PORT": "3000"},
			})
		}},
		{"restart-passenger-app", "mut_restart_passenger_app_ok.json", func(m *HTTPSMutator) error {
			return m.RestartPassengerApp(context.Background(), "/home/operator/apps/shop-backend")
		}},
		{"create-mysql-db", "mut_create_mysql_db_ok.json", func(m *HTTPSMutator) error {
			return m.CreateMysqlDatabase(context.Background(), "operator_shopdb")
		}},
		{"create-mysql-user", "mut_create_mysql_user_ok.json", func(m *HTTPSMutator) error {
			return m.CreateMysqlUser(context.Background(), "operator_shopusr", "deadbeef-deadbeef")
		}},
		{"set-mysql-privs", "mut_set_mysql_privileges_ok.json", func(m *HTTPSMutator) error {
			return m.SetMysqlPrivileges(context.Background(), MysqlPrivilegesArgs{
				Database: "operator_shopdb", User: "operator_shopusr", Privileges: []string{"ALL PRIVILEGES"},
			})
		}},
		{"start-autossl", "mut_start_autossl_ok.json", func(m *HTTPSMutator) error {
			return m.StartAutoSSL(context.Background(), "shop.example.com")
		}},
		{"install-ssl", "mut_install_ssl_ok.json", func(m *HTTPSMutator) error {
			return m.InstallSSL(context.Background(), InstallSSLArgs{
				Domain: "shop.example.com",
				Cert:   "-----BEGIN CERTIFICATE-----\nfake-cert\n-----END CERTIFICATE-----",
				Key:    "-----BEGIN PRIVATE KEY-----\nfake-key\n-----END PRIVATE KEY-----",
			})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := newTestServerWithBody(t, mustFixture(t, tc.fixture), http.StatusOK)
			defer srv.Close()
			m, err := NewHTTPSMutator(srv.URL, "operator", "TOKEN", srv.Client())
			if err != nil {
				t.Fatalf("NewHTTPSMutator: %v", err)
			}
			if err := tc.call(m); err != nil {
				t.Fatalf("call: %v", err)
			}
		})
	}
}

func TestHTTPSMutator_IdempotencyErrorMapping(t *testing.T) {
	setMutationsAllowed(t, true)
	cases := []struct {
		name    string
		fixture string
		want    error
		call    func(*HTTPSMutator) error
	}{
		{"add-addon-domain-exists", "mut_add_addon_domain_exists.json", ErrResourceExists, func(m *HTTPSMutator) error {
			return m.AddAddonDomain(context.Background(), CreateAddonDomainArgs{NewDomain: "client.example.com", Dir: "public_html/client"})
		}},
		{"del-domain-not-found", "mut_del_domain_not_found.json", ErrResourceNotFound, func(m *HTTPSMutator) error {
			return m.DeleteDomain(context.Background(), "client.example.com")
		}},
		{"create-passenger-exists", "mut_create_passenger_app_exists.json", ErrResourceExists, func(m *HTTPSMutator) error {
			return m.CreatePassengerApp(context.Background(), CreatePassengerAppArgs{Name: "shop-backend", Path: "/p", Domain: "d.example.com"})
		}},
		{"create-mysql-db-exists", "mut_create_mysql_db_exists.json", ErrResourceExists, func(m *HTTPSMutator) error {
			return m.CreateMysqlDatabase(context.Background(), "operator_shopdb")
		}},
		{"delete-ssl-not-found", "mut_delete_ssl_not_found.json", ErrResourceNotFound, func(m *HTTPSMutator) error {
			return m.DeleteSSL(context.Background(), "shop.example.com")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := newTestServerWithBody(t, mustFixture(t, tc.fixture), http.StatusOK)
			defer srv.Close()
			m, err := NewHTTPSMutator(srv.URL, "operator", "TOKEN", srv.Client())
			if err != nil {
				t.Fatalf("NewHTTPSMutator: %v", err)
			}
			if err := tc.call(m); !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestHTTPSMutator_AuthAndRateLimit(t *testing.T) {
	setMutationsAllowed(t, true)

	cases := []struct {
		name   string
		status int
		want   error
	}{
		{"auth-failed-401", http.StatusUnauthorized, ErrAuthenticationFailed},
		{"auth-failed-403", http.StatusForbidden, ErrAuthenticationFailed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
			}))
			defer srv.Close()
			m, err := NewHTTPSMutator(srv.URL, "operator", "TOKEN", srv.Client())
			if err != nil {
				t.Fatalf("NewHTTPSMutator: %v", err)
			}
			err = m.AddAddonDomain(context.Background(), CreateAddonDomainArgs{NewDomain: "x.example.com", Dir: "public_html/x"})
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestNewSSHMutator_Validates(t *testing.T) {
	if _, err := NewSSHMutator(&fakeSSHRunner{}, ""); !errors.Is(err, ErrMissingCredentials) {
		t.Fatalf("empty user: got %v, want ErrMissingCredentials", err)
	}
	if _, err := NewSSHMutator(nil, "operator"); !errors.Is(err, ErrSSHRunnerRequired) {
		t.Fatalf("nil runner: got %v, want ErrSSHRunnerRequired", err)
	}
}

func TestSSHMutator_GuardBlocksWhenEnvUnset(t *testing.T) {
	setMutationsAllowed(t, false)
	fake := &fakeSSHRunner{}
	m, err := NewSSHMutator(fake, "operator")
	if err != nil {
		t.Fatalf("NewSSHMutator: %v", err)
	}
	if err := m.AddAddonDomain(context.Background(), CreateAddonDomainArgs{NewDomain: "x.example.com", Dir: "public_html/x"}); !errors.Is(err, ErrMutationsDisabled) {
		t.Fatalf("got %v, want ErrMutationsDisabled", err)
	}
	if fake.lastCommand != "" {
		t.Fatalf("guard leaked: runner saw %q", fake.lastCommand)
	}
}

func TestSSHMutator_BuildsCommandWithSortedArgs(t *testing.T) {
	setMutationsAllowed(t, true)
	fake := &fakeSSHRunner{stdout: mustFixture(t, "mut_create_passenger_app_ok.json")}
	m, err := NewSSHMutator(fake, "operator")
	if err != nil {
		t.Fatalf("NewSSHMutator: %v", err)
	}
	err = m.CreatePassengerApp(context.Background(), CreatePassengerAppArgs{
		Name:           "shop-backend",
		Path:           "/home/operator/apps/shop-backend",
		Domain:         "shop.example.com",
		BaseURI:        "/",
		DeploymentMode: "production",
		Envvars:        map[string]string{"PORT": "3000", "NODE_ENV": "production"},
	})
	if err != nil {
		t.Fatalf("CreatePassengerApp: %v", err)
	}

	want := "uapi --user='operator' --output=jsonpretty 'PassengerApps' 'create_application'" +
		" 'base_uri'='/'" +
		" 'deployment_mode'='production'" +
		" 'domain'='shop.example.com'" +
		" 'envvar.NODE_ENV'='production'" +
		" 'envvar.PORT'='3000'" +
		" 'name'='shop-backend'" +
		" 'path'='/home/operator/apps/shop-backend'"
	if fake.lastCommand != want {
		t.Errorf("command mismatch\n got: %s\nwant: %s", fake.lastCommand, want)
	}
}

func TestSSHMutator_ShellInjectionContained(t *testing.T) {
	setMutationsAllowed(t, true)
	fake := &fakeSSHRunner{stdout: mustFixture(t, "mut_add_addon_domain_ok.json")}
	m, err := NewSSHMutator(fake, "operator")
	if err != nil {
		t.Fatalf("NewSSHMutator: %v", err)
	}
	hostile := "x.example.com'; rm -rf /; '"
	err = m.AddAddonDomain(context.Background(), CreateAddonDomainArgs{NewDomain: hostile, Dir: "public_html/x"})
	if err != nil {
		t.Fatalf("AddAddonDomain: %v", err)
	}
	if strings.Contains(fake.lastCommand, "; rm -rf /;") && !strings.Contains(fake.lastCommand, `'x.example.com'\''; rm -rf /; '\'''`) {
		t.Fatalf("shell-quote did not escape the payload: %s", fake.lastCommand)
	}
}

func TestSSHMutator_HappyPath_AllMethods(t *testing.T) {
	setMutationsAllowed(t, true)

	cases := []struct {
		name    string
		fixture string
		call    func(*SSHMutator) error
	}{
		{"add-addon-domain", "mut_add_addon_domain_ok.json", func(m *SSHMutator) error {
			return m.AddAddonDomain(context.Background(), CreateAddonDomainArgs{NewDomain: "client.example.com", Dir: "public_html/client"})
		}},
		{"create-passenger-app", "mut_create_passenger_app_ok.json", func(m *SSHMutator) error {
			return m.CreatePassengerApp(context.Background(), CreatePassengerAppArgs{Name: "n", Path: "/p", Domain: "d.example.com"})
		}},
		{"create-mysql-db", "mut_create_mysql_db_ok.json", func(m *SSHMutator) error {
			return m.CreateMysqlDatabase(context.Background(), "operator_shopdb")
		}},
		{"start-autossl", "mut_start_autossl_ok.json", func(m *SSHMutator) error {
			return m.StartAutoSSL(context.Background(), "shop.example.com")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeSSHRunner{stdout: mustFixture(t, tc.fixture)}
			m, err := NewSSHMutator(fake, "operator")
			if err != nil {
				t.Fatalf("NewSSHMutator: %v", err)
			}
			if err := tc.call(m); err != nil {
				t.Fatalf("call: %v", err)
			}
		})
	}
}

func TestSSHMutator_ModuleDisabledViaStderr(t *testing.T) {
	setMutationsAllowed(t, true)
	fake := &fakeSSHRunner{
		stderr:   []byte("Sorry, the feature you are using is disabled for this account."),
		exitCode: 1,
	}
	m, err := NewSSHMutator(fake, "operator")
	if err != nil {
		t.Fatalf("NewSSHMutator: %v", err)
	}
	err = m.AddAddonDomain(context.Background(), CreateAddonDomainArgs{NewDomain: "x.example.com", Dir: "public_html/x"})
	if !errors.Is(err, ErrModuleFunctionDenied) {
		t.Fatalf("got %v, want ErrModuleFunctionDenied", err)
	}
}

func TestCompositeMutator_PrefersPrimary(t *testing.T) {
	setMutationsAllowed(t, true)

	var primaryCalled, secondaryCalled atomic.Int32
	primary := &fakeMutator{onAddDomain: func(context.Context, CreateAddonDomainArgs) error {
		primaryCalled.Add(1)
		return nil
	}}
	secondary := &fakeMutator{onAddDomain: func(context.Context, CreateAddonDomainArgs) error {
		secondaryCalled.Add(1)
		return nil
	}}
	c := &CompositeMutator{Primary: primary, Secondary: secondary}
	if err := c.AddAddonDomain(context.Background(), CreateAddonDomainArgs{NewDomain: "x.example.com", Dir: "public_html/x"}); err != nil {
		t.Fatalf("AddAddonDomain: %v", err)
	}
	if primaryCalled.Load() != 1 || secondaryCalled.Load() != 0 {
		t.Fatalf("primary=%d secondary=%d, want 1/0", primaryCalled.Load(), secondaryCalled.Load())
	}
}

func TestCompositeMutator_FailsOverOnTransportUnavailable(t *testing.T) {
	setMutationsAllowed(t, true)

	var primaryCalled, secondaryCalled atomic.Int32
	primary := &fakeMutator{onAddDomain: func(context.Context, CreateAddonDomainArgs) error {
		primaryCalled.Add(1)
		return ErrTransportUnavailable
	}}
	secondary := &fakeMutator{onAddDomain: func(context.Context, CreateAddonDomainArgs) error {
		secondaryCalled.Add(1)
		return nil
	}}
	c := &CompositeMutator{Primary: primary, Secondary: secondary}
	if err := c.AddAddonDomain(context.Background(), CreateAddonDomainArgs{NewDomain: "x.example.com", Dir: "public_html/x"}); err != nil {
		t.Fatalf("AddAddonDomain: %v", err)
	}
	if primaryCalled.Load() != 1 || secondaryCalled.Load() != 1 {
		t.Fatalf("primary=%d secondary=%d, want 1/1", primaryCalled.Load(), secondaryCalled.Load())
	}
}

func TestCompositeMutator_DoesNotFailOverOnAuthError(t *testing.T) {
	setMutationsAllowed(t, true)

	var primaryCalled, secondaryCalled atomic.Int32
	primary := &fakeMutator{onAddDomain: func(context.Context, CreateAddonDomainArgs) error {
		primaryCalled.Add(1)
		return ErrAuthenticationFailed
	}}
	secondary := &fakeMutator{onAddDomain: func(context.Context, CreateAddonDomainArgs) error {
		secondaryCalled.Add(1)
		return nil
	}}
	c := &CompositeMutator{Primary: primary, Secondary: secondary}
	err := c.AddAddonDomain(context.Background(), CreateAddonDomainArgs{NewDomain: "x.example.com", Dir: "public_html/x"})
	if !errors.Is(err, ErrAuthenticationFailed) {
		t.Fatalf("got %v, want ErrAuthenticationFailed", err)
	}
	if primaryCalled.Load() != 1 || secondaryCalled.Load() != 0 {
		t.Fatalf("primary=%d secondary=%d, want 1/0 (no fallback on auth)", primaryCalled.Load(), secondaryCalled.Load())
	}
}

func TestCompositeMutator_BothNilReturnsTransportUnavailable(t *testing.T) {
	setMutationsAllowed(t, true)
	c := &CompositeMutator{}
	err := c.AddAddonDomain(context.Background(), CreateAddonDomainArgs{NewDomain: "x.example.com", Dir: "public_html/x"})
	if !errors.Is(err, ErrTransportUnavailable) {
		t.Fatalf("got %v, want ErrTransportUnavailable", err)
	}
}

func TestClassifyMutationError_PassesThroughNonAPIErrors(t *testing.T) {
	in := errors.New("connection refused")
	got := classifyMutationError(in)
	if !errors.Is(got, in) {
		t.Fatalf("got %v, want passthrough of %v", got, in)
	}
}

func TestClassifyMutationError_NilSafe(t *testing.T) {
	if got := classifyMutationError(nil); got != nil {
		t.Fatalf("nil-classify returned %v", got)
	}
}

// fakeMutator is the in-memory Mutator used by the composite
// tests. Every method is wired through a callback set per test
// so we can choreograph the primary/secondary handshake without
// spinning up two real httptest servers.
type fakeMutator struct {
	onAddDomain func(context.Context, CreateAddonDomainArgs) error
}

func (f *fakeMutator) AddAddonDomain(ctx context.Context, args CreateAddonDomainArgs) error {
	if f.onAddDomain == nil {
		return nil
	}
	return f.onAddDomain(ctx, args)
}

func (f *fakeMutator) AddSubdomain(_ context.Context, _ CreateSubdomainArgs) error { return nil }
func (f *fakeMutator) DeleteDomain(_ context.Context, _ string) error              { return nil }
func (f *fakeMutator) DeleteSubdomain(_ context.Context, _ string) error           { return nil }
func (f *fakeMutator) CreatePassengerApp(_ context.Context, _ CreatePassengerAppArgs) error {
	return nil
}

func (f *fakeMutator) EditPassengerApp(_ context.Context, _ EditPassengerAppArgs) error { return nil }

func (f *fakeMutator) RestartPassengerApp(_ context.Context, _ string) error { return nil }

func (f *fakeMutator) DeletePassengerApp(_ context.Context, _ string) error { return nil }

func (f *fakeMutator) CreateMysqlDatabase(_ context.Context, _ string) error { return nil }

func (f *fakeMutator) DeleteMysqlDatabase(_ context.Context, _ string) error { return nil }

func (f *fakeMutator) CreateMysqlUser(_ context.Context, _, _ string) error { return nil }

func (f *fakeMutator) DeleteMysqlUser(_ context.Context, _ string) error { return nil }

func (f *fakeMutator) SetMysqlPrivileges(_ context.Context, _ MysqlPrivilegesArgs) error {
	return nil
}
func (f *fakeMutator) InstallSSL(_ context.Context, _ InstallSSLArgs) error { return nil }
func (f *fakeMutator) StartAutoSSL(_ context.Context, _ string) error       { return nil }
func (f *fakeMutator) DeleteSSL(_ context.Context, _ string) error          { return nil }
