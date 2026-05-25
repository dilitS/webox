package uapi

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

// fakeSSHRunner is the unit-test seam for [SSHRunner]. It records
// the last command and returns canned stdout/stderr/exitCode/err.
type fakeSSHRunner struct {
	lastCommand string
	stdout      []byte
	stderr      []byte
	exitCode    int
	err         error
}

func (f *fakeSSHRunner) Run(_ context.Context, command string) (stdout, stderr []byte, exitCode int, err error) {
	f.lastCommand = command
	return f.stdout, f.stderr, f.exitCode, f.err
}

func TestNewSSHFallback_Validates(t *testing.T) {
	if _, err := NewSSHFallback(&fakeSSHRunner{}, ""); !errors.Is(err, ErrMissingCredentials) {
		t.Fatalf("empty user: got %v, want ErrMissingCredentials", err)
	}
	if _, err := NewSSHFallback(nil, "operator"); !errors.Is(err, ErrSSHRunnerRequired) {
		t.Fatalf("nil runner: got %v, want ErrSSHRunnerRequired", err)
	}
}

func TestSSHFallback_ListDomains_BuildsCommand(t *testing.T) {
	fake := &fakeSSHRunner{stdout: mustFixture(t, "list_domains_ok.json")}
	s, err := NewSSHFallback(fake, "operator")
	if err != nil {
		t.Fatalf("NewSSHFallback: %v", err)
	}
	got, err := s.ListDomains(context.Background())
	if err != nil {
		t.Fatalf("ListDomains: %v", err)
	}
	if got.MainDomain != "example.com" {
		t.Errorf("MainDomain = %q, want example.com", got.MainDomain)
	}
	want := "uapi --user='operator' --output=jsonpretty 'DomainInfo' 'list_domains'"
	if fake.lastCommand != want {
		t.Errorf("command = %q, want %q", fake.lastCommand, want)
	}
}

func TestSSHFallback_AllReadersUseSameDecoders(t *testing.T) {
	cases := []struct {
		name    string
		fixture string
		call    func(*SSHFallback) (any, error)
	}{
		{"passenger-apps", "list_passenger_apps_ok.json", func(s *SSHFallback) (any, error) { return s.ListPassengerApps(context.Background()) }},
		{"mysql-dbs", "list_mysql_databases_ok.json", func(s *SSHFallback) (any, error) { return s.ListMysqlDatabases(context.Background()) }},
		{"ssl-keys", "list_ssl_keys_ok.json", func(s *SSHFallback) (any, error) { return s.ListSSLKeys(context.Background()) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeSSHRunner{stdout: mustFixture(t, tc.fixture)}
			s, err := NewSSHFallback(fake, "operator")
			if err != nil {
				t.Fatalf("NewSSHFallback: %v", err)
			}
			got, err := tc.call(s)
			if err != nil {
				t.Fatalf("call: %v", err)
			}
			if got == nil {
				t.Fatal("got nil result")
			}
		})
	}
}

func TestSSHFallback_ModuleDisabledViaStderr(t *testing.T) {
	fake := &fakeSSHRunner{
		stderr:   []byte("Sorry, the feature you are using is disabled for this account."),
		exitCode: 1,
	}
	s, err := NewSSHFallback(fake, "operator")
	if err != nil {
		t.Fatalf("NewSSHFallback: %v", err)
	}
	_, err = s.ListDomains(context.Background())
	if !errors.Is(err, ErrModuleFunctionDenied) {
		t.Fatalf("got %v, want ErrModuleFunctionDenied", err)
	}
}

func TestSSHFallback_ModuleDisabledViaEnvelope(t *testing.T) {
	fake := &fakeSSHRunner{stdout: mustFixture(t, "error_module_denied.json")}
	s, err := NewSSHFallback(fake, "operator")
	if err != nil {
		t.Fatalf("NewSSHFallback: %v", err)
	}
	_, err = s.ListDomains(context.Background())
	if !errors.Is(err, ErrModuleFunctionDenied) {
		t.Fatalf("got %v, want ErrModuleFunctionDenied", err)
	}
}

func TestSSHFallback_TransportErrorWrapped(t *testing.T) {
	netErr := errors.New("ssh: connection refused")
	fake := &fakeSSHRunner{err: netErr}
	s, err := NewSSHFallback(fake, "operator")
	if err != nil {
		t.Fatalf("NewSSHFallback: %v", err)
	}
	_, err = s.ListDomains(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, netErr) {
		t.Errorf("expected wrapped %v, got %v", netErr, err)
	}
}

func TestSSHFallback_MalformedStdoutMaps(t *testing.T) {
	fake := &fakeSSHRunner{stdout: []byte("not json")}
	s, err := NewSSHFallback(fake, "operator")
	if err != nil {
		t.Fatalf("NewSSHFallback: %v", err)
	}
	_, err = s.ListDomains(context.Background())
	if !errors.Is(err, ErrMalformedResponse) {
		t.Fatalf("got %v, want ErrMalformedResponse", err)
	}
}

func TestShellQuote_EscapesSingleQuotes(t *testing.T) {
	cases := map[string]string{
		"operator":      `'operator'`,
		"with space":    `'with space'`,
		"O'Brien":       `'O'\''Brien'`,
		"a'b'c":         `'a'\''b'\''c'`,
		"":              `''`,
		"!@#$%^&*()":    `'!@#$%^&*()'`,
		"$(rm -rf /)":   `'$(rm -rf /)'`,
		"; ls; #":       `'; ls; #'`,
		"`whoami`":      "'`whoami`'",
		"\\backslash\\": `'\backslash\'`,
	}
	for in, want := range cases {
		if got := shellQuote(in); got != want {
			t.Errorf("shellQuote(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestComposite_PrimaryWinsOnSuccess(t *testing.T) {
	primary := newFakeReader().withDomains(&DomainInfoListResponse{MainDomain: "primary.example.com"})
	secondary := newFakeReader().withDomains(&DomainInfoListResponse{MainDomain: "secondary.example.com"})
	c := &Composite{Primary: primary, Secondary: secondary}

	got, err := c.ListDomains(context.Background())
	if err != nil {
		t.Fatalf("ListDomains: %v", err)
	}
	if got.MainDomain != "primary.example.com" {
		t.Errorf("MainDomain = %q, want primary.example.com", got.MainDomain)
	}
	if atomic.LoadInt32(&secondary.calls) != 0 {
		t.Errorf("secondary called %d times, want 0", secondary.calls)
	}
}

func TestComposite_FallsOverOnTransportUnavailable(t *testing.T) {
	primary := newFakeReader().withErr(ErrTransportUnavailable)
	secondary := newFakeReader().withDomains(&DomainInfoListResponse{MainDomain: "secondary.example.com"})
	c := &Composite{Primary: primary, Secondary: secondary}

	got, err := c.ListDomains(context.Background())
	if err != nil {
		t.Fatalf("ListDomains: %v", err)
	}
	if got.MainDomain != "secondary.example.com" {
		t.Errorf("MainDomain = %q, want secondary.example.com", got.MainDomain)
	}
	if atomic.LoadInt32(&primary.calls) != 1 || atomic.LoadInt32(&secondary.calls) != 1 {
		t.Errorf("calls = (%d, %d), want (1, 1)", primary.calls, secondary.calls)
	}
}

func TestComposite_AuthFailureSurfacesWithoutFallover(t *testing.T) {
	primary := newFakeReader().withErr(ErrAuthenticationFailed)
	secondary := newFakeReader().withDomains(&DomainInfoListResponse{MainDomain: "secondary.example.com"})
	c := &Composite{Primary: primary, Secondary: secondary}

	_, err := c.ListDomains(context.Background())
	if !errors.Is(err, ErrAuthenticationFailed) {
		t.Fatalf("got %v, want ErrAuthenticationFailed", err)
	}
	if atomic.LoadInt32(&secondary.calls) != 0 {
		t.Errorf("secondary called %d times, want 0 (auth errors should not fail over)", secondary.calls)
	}
}

func TestComposite_BothNilReturnsTransportError(t *testing.T) {
	c := &Composite{}
	_, err := c.ListDomains(context.Background())
	if !errors.Is(err, ErrTransportUnavailable) {
		t.Fatalf("got %v, want ErrTransportUnavailable", err)
	}
}

func TestComposite_SecondaryOnly(t *testing.T) {
	secondary := newFakeReader().withDomains(&DomainInfoListResponse{MainDomain: "secondary.example.com"})
	c := &Composite{Secondary: secondary}

	got, err := c.ListDomains(context.Background())
	if err != nil {
		t.Fatalf("ListDomains: %v", err)
	}
	if got.MainDomain != "secondary.example.com" {
		t.Errorf("MainDomain = %q, want secondary.example.com", got.MainDomain)
	}
}

func TestComposite_AllFourMethodsRoundTrip(t *testing.T) {
	primary := newFakeReader().
		withDomains(&DomainInfoListResponse{MainDomain: "example.com"}).
		withApps(&PassengerAppsListResponse{Applications: []PassengerApp{{Name: "shop"}}}).
		withDBs(&MysqlListDatabasesResponse{Databases: []MysqlDatabase{{Name: "operator_shop"}}}).
		withKeys(&SSLListKeysResponse{Keys: []SSLKey{{FriendlyName: "example.com"}}})
	c := &Composite{Primary: primary}

	if d, err := c.ListDomains(context.Background()); err != nil || d.MainDomain != "example.com" {
		t.Errorf("ListDomains: %v %v", d, err)
	}
	if a, err := c.ListPassengerApps(context.Background()); err != nil || len(a.Applications) != 1 {
		t.Errorf("ListPassengerApps: %v %v", a, err)
	}
	if m, err := c.ListMysqlDatabases(context.Background()); err != nil || len(m.Databases) != 1 {
		t.Errorf("ListMysqlDatabases: %v %v", m, err)
	}
	if k, err := c.ListSSLKeys(context.Background()); err != nil || len(k.Keys) != 1 {
		t.Errorf("ListSSLKeys: %v %v", k, err)
	}
}

func TestClient_HTTPSConnectionRefusedMapsToTransportUnavailable(t *testing.T) {
	// Use a deliberately unreachable IP+port. 192.0.2.0/24 is
	// RFC 5737 TEST-NET-1 so the dial fails fast on every CI
	// runner without depending on local firewall behaviour.
	c, err := NewClient("https://192.0.2.1:2083", "operator", "TOKEN", &http.Client{Transport: &http.Transport{
		DisableKeepAlives: true,
	}})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	speedUpBackoff(t, c)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	c.transport.client.Timeout = 250 * time.Millisecond

	_, err = c.ListDomains(ctx)
	if !errors.Is(err, ErrTransportUnavailable) {
		t.Fatalf("got %v, want ErrTransportUnavailable", err)
	}
}

// fakeReader satisfies the [Reader] interface using inlined typed
// fixtures. The withX builders make table-driven tests read like
// English.
type fakeReader struct {
	calls   int32
	err     error
	domains *DomainInfoListResponse
	apps    *PassengerAppsListResponse
	dbs     *MysqlListDatabasesResponse
	keys    *SSLListKeysResponse
}

func newFakeReader() *fakeReader { return &fakeReader{} }

func (f *fakeReader) withErr(err error) *fakeReader { f.err = err; return f }
func (f *fakeReader) withDomains(d *DomainInfoListResponse) *fakeReader {
	f.domains = d
	return f
}

func (f *fakeReader) withApps(a *PassengerAppsListResponse) *fakeReader {
	f.apps = a
	return f
}

func (f *fakeReader) withDBs(m *MysqlListDatabasesResponse) *fakeReader {
	f.dbs = m
	return f
}

func (f *fakeReader) withKeys(k *SSLListKeysResponse) *fakeReader {
	f.keys = k
	return f
}

func (f *fakeReader) ListDomains(_ context.Context) (*DomainInfoListResponse, error) {
	atomic.AddInt32(&f.calls, 1)
	return f.domains, f.err
}

func (f *fakeReader) ListPassengerApps(_ context.Context) (*PassengerAppsListResponse, error) {
	atomic.AddInt32(&f.calls, 1)
	return f.apps, f.err
}

func (f *fakeReader) ListMysqlDatabases(_ context.Context) (*MysqlListDatabasesResponse, error) {
	atomic.AddInt32(&f.calls, 1)
	return f.dbs, f.err
}

func (f *fakeReader) ListSSLKeys(_ context.Context) (*SSLListKeysResponse, error) {
	atomic.AddInt32(&f.calls, 1)
	return f.keys, f.err
}
