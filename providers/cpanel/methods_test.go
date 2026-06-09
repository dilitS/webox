package cpanel_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dilitS/webox/providers"
	"github.com/dilitS/webox/providers/cpanel"
	"github.com/dilitS/webox/providers/cpanel/uapi"
)

// fakeReader is the in-memory stand-in for [uapi.Reader].
type fakeReader struct {
	mu               sync.Mutex
	domains          *uapi.DomainInfoListResponse
	apps             *uapi.PassengerAppsListResponse
	dbs              *uapi.MysqlListDatabasesResponse
	ssl              *uapi.SSLListKeysResponse
	listDomainsErr   error
	listAppsErr      error
	listDBsErr       error
	listSSLErr       error
	listDomainsCalls int
	listAppsCalls    int
}

func (f *fakeReader) ListDomains(_ context.Context) (*uapi.DomainInfoListResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listDomainsCalls++
	if f.listDomainsErr != nil {
		return nil, f.listDomainsErr
	}
	return f.domains, nil
}

func (f *fakeReader) ListPassengerApps(_ context.Context) (*uapi.PassengerAppsListResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listAppsCalls++
	if f.listAppsErr != nil {
		return nil, f.listAppsErr
	}
	return f.apps, nil
}

func (f *fakeReader) ListMysqlDatabases(_ context.Context) (*uapi.MysqlListDatabasesResponse, error) {
	if f.listDBsErr != nil {
		return nil, f.listDBsErr
	}
	return f.dbs, nil
}

func (f *fakeReader) ListSSLKeys(_ context.Context) (*uapi.SSLListKeysResponse, error) {
	if f.listSSLErr != nil {
		return nil, f.listSSLErr
	}
	return f.ssl, nil
}

// Transport satisfies the post-Sprint-23 uapi.Reader.Transport()
// addition; the in-memory fake has no real transport.
func (f *fakeReader) Transport() string { return "fake" }

// fakeMutator is the in-memory stand-in for [uapi.Mutator]. Each
// method records the call and returns the scripted error.
type fakeMutator struct {
	mu sync.Mutex

	// scripts maps op-name → error; missing entry = success.
	scripts map[string]error
	// callLog preserves the call order; tests assert on it to
	// verify orchestration (create DB → create user → grant
	// privileges, etc.).
	callLog []string

	// captured payloads for argument-shape assertions.
	addAddonArgs     []uapi.CreateAddonDomainArgs
	addSubdomainArgs []uapi.CreateSubdomainArgs
	createAppArgs    []uapi.CreatePassengerAppArgs
	mysqlPrivArgs    []uapi.MysqlPrivilegesArgs
	deletedDomains   []string
	deletedSubs      []string
	deletedApps      []string
	deletedDBs       []string
	deletedUsers     []string
	deletedSSL       []string
	restartedApps    []string
	autoSSLDomains   []string
	createdUsers     []struct{ user, password string }
}

func newFakeMutator() *fakeMutator {
	return &fakeMutator{scripts: map[string]error{}}
}

func (f *fakeMutator) on(op string, err error) *fakeMutator {
	f.scripts[op] = err
	return f
}

func (f *fakeMutator) result(op string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.callLog = append(f.callLog, op)
	return f.scripts[op]
}

func (f *fakeMutator) AddAddonDomain(_ context.Context, a uapi.CreateAddonDomainArgs) error {
	f.mu.Lock()
	f.addAddonArgs = append(f.addAddonArgs, a)
	f.mu.Unlock()
	return f.result("AddAddonDomain")
}

func (f *fakeMutator) AddSubdomain(_ context.Context, a uapi.CreateSubdomainArgs) error {
	f.mu.Lock()
	f.addSubdomainArgs = append(f.addSubdomainArgs, a)
	f.mu.Unlock()
	return f.result("AddSubdomain")
}

func (f *fakeMutator) DeleteDomain(_ context.Context, d string) error {
	f.mu.Lock()
	f.deletedDomains = append(f.deletedDomains, d)
	f.mu.Unlock()
	return f.result("DeleteDomain")
}

func (f *fakeMutator) DeleteSubdomain(_ context.Context, d string) error {
	f.mu.Lock()
	f.deletedSubs = append(f.deletedSubs, d)
	f.mu.Unlock()
	return f.result("DeleteSubdomain")
}

func (f *fakeMutator) CreatePassengerApp(_ context.Context, a uapi.CreatePassengerAppArgs) error {
	f.mu.Lock()
	f.createAppArgs = append(f.createAppArgs, a)
	f.mu.Unlock()
	return f.result("CreatePassengerApp")
}

func (f *fakeMutator) EditPassengerApp(_ context.Context, _ uapi.EditPassengerAppArgs) error {
	return f.result("EditPassengerApp")
}

func (f *fakeMutator) RestartPassengerApp(_ context.Context, p string) error {
	f.mu.Lock()
	f.restartedApps = append(f.restartedApps, p)
	f.mu.Unlock()
	return f.result("RestartPassengerApp")
}

func (f *fakeMutator) DeletePassengerApp(_ context.Context, p string) error {
	f.mu.Lock()
	f.deletedApps = append(f.deletedApps, p)
	f.mu.Unlock()
	return f.result("DeletePassengerApp")
}

func (f *fakeMutator) CreateMysqlDatabase(_ context.Context, n string) error {
	return f.result("CreateMysqlDatabase:" + n)
}

func (f *fakeMutator) DeleteMysqlDatabase(_ context.Context, n string) error {
	f.mu.Lock()
	f.deletedDBs = append(f.deletedDBs, n)
	f.mu.Unlock()
	return f.result("DeleteMysqlDatabase:" + n)
}

func (f *fakeMutator) CreateMysqlUser(_ context.Context, u, pw string) error {
	f.mu.Lock()
	f.createdUsers = append(f.createdUsers, struct{ user, password string }{u, pw})
	f.mu.Unlock()
	return f.result("CreateMysqlUser:" + u)
}

func (f *fakeMutator) DeleteMysqlUser(_ context.Context, u string) error {
	f.mu.Lock()
	f.deletedUsers = append(f.deletedUsers, u)
	f.mu.Unlock()
	return f.result("DeleteMysqlUser:" + u)
}

func (f *fakeMutator) SetMysqlPrivileges(_ context.Context, a uapi.MysqlPrivilegesArgs) error {
	f.mu.Lock()
	f.mysqlPrivArgs = append(f.mysqlPrivArgs, a)
	f.mu.Unlock()
	return f.result("SetMysqlPrivileges")
}

func (f *fakeMutator) InstallSSL(_ context.Context, _ uapi.InstallSSLArgs) error {
	return f.result("InstallSSL")
}

func (f *fakeMutator) StartAutoSSL(_ context.Context, d string) error {
	f.mu.Lock()
	f.autoSSLDomains = append(f.autoSSLDomains, d)
	f.mu.Unlock()
	return f.result("StartAutoSSL")
}

func (f *fakeMutator) DeleteSSL(_ context.Context, h string) error {
	f.mu.Lock()
	f.deletedSSL = append(f.deletedSSL, h)
	f.mu.Unlock()
	return f.result("DeleteSSL")
}

// fakeRunner is the in-memory stand-in for [uapi.SSHRunner].
type fakeRunner struct {
	mu       sync.Mutex
	stdout   []byte
	stderr   []byte
	exitCode int
	err      error
	calls    []string
}

func (f *fakeRunner) Run(_ context.Context, cmd string) (stdout, stderr []byte, exitCode int, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, cmd)
	return f.stdout, f.stderr, f.exitCode, f.err
}

// newProvider assembles a Provider with the supplied seams.
func newProvider(t *testing.T, props map[string]string, r uapi.Reader, m uapi.Mutator, run uapi.SSHRunner) *cpanel.Provider {
	t.Helper()
	if props == nil {
		props = map[string]string{}
	}
	cfg := providers.ProviderConfig{
		Alias: "vh", Type: "cpanel", Host: "panel.vh.pl",
		Port: 22, User: "alice", Properties: props,
	}
	p, err := cpanel.New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	cp := p.(*cpanel.Provider)
	if r != nil {
		cp.SetReader(r)
	}
	if m != nil {
		cp.SetMutator(m)
	}
	if run != nil {
		cp.SetSSHRunner(run)
	}
	return cp
}

// Compile-time interface satisfaction guard. If a method signature
// drifts away from the contract, this test won't compile.
func TestProvider_SatisfiesHostingProvider(t *testing.T) {
	t.Parallel()
	var _ providers.HostingProvider = (*cpanel.Provider)(nil)
}

func TestProvider_CreateSubdomain_HappyPath_AddonDomain(t *testing.T) {
	t.Parallel()
	m := newFakeMutator()
	p := newProvider(t, nil, nil, m, nil)

	err := p.CreateSubdomain(context.Background(), "shop.example.com", "22")
	if err != nil {
		t.Fatalf("CreateSubdomain: %v", err)
	}
	if got, want := m.callLog, []string{"AddAddonDomain", "CreatePassengerApp"}; !equalStrings(got, want) {
		t.Fatalf("call order = %v, want %v", got, want)
	}
	if len(m.addAddonArgs) != 1 || m.addAddonArgs[0].NewDomain != "shop.example.com" {
		t.Fatalf("addon args mismatch: %+v", m.addAddonArgs)
	}
	if len(m.createAppArgs) != 1 {
		t.Fatalf("expected 1 CreatePassengerApp call, got %d", len(m.createAppArgs))
	}
	app := m.createAppArgs[0]
	if app.Name != "shop-example-com" {
		t.Fatalf("app name = %q, want shop-example-com", app.Name)
	}
	if app.Path != "/home/alice/nodejs/shop-example-com" {
		t.Fatalf("app path = %q", app.Path)
	}
	if app.Domain != "shop.example.com" {
		t.Fatalf("app domain = %q", app.Domain)
	}
	if app.Envvars["NODE_ENV"] != "production" {
		t.Fatalf("NODE_ENV missing or wrong: %+v", app.Envvars)
	}
}

func TestProvider_CreateSubdomain_SubdomainKind_RoutesToSubdomainEndpoint(t *testing.T) {
	t.Parallel()
	m := newFakeMutator()
	p := newProvider(t, map[string]string{"domain_kind": "subdomain"}, nil, m, nil)

	err := p.CreateSubdomain(context.Background(), "api.example.com", "22")
	if err != nil {
		t.Fatalf("CreateSubdomain: %v", err)
	}
	if got, want := m.callLog, []string{"AddSubdomain", "CreatePassengerApp"}; !equalStrings(got, want) {
		t.Fatalf("call order = %v, want %v", got, want)
	}
	if len(m.addSubdomainArgs) != 1 {
		t.Fatalf("addSubdomain not called")
	}
	args := m.addSubdomainArgs[0]
	if args.Domain != "api" || args.RootDomain != "example.com" {
		t.Fatalf("subdomain args mismatch: %+v", args)
	}
}

func TestProvider_CreateSubdomain_DomainExists_ReturnsSentinel(t *testing.T) {
	t.Parallel()
	m := newFakeMutator().on("AddAddonDomain", uapi.ErrResourceExists)
	p := newProvider(t, nil, nil, m, nil)

	err := p.CreateSubdomain(context.Background(), "shop.example.com", "22")
	if !errors.Is(err, providers.ErrSubdomainExists) {
		t.Fatalf("expected ErrSubdomainExists, got %v", err)
	}
	if len(m.createAppArgs) != 0 {
		t.Fatalf("CreatePassengerApp should not run when domain step failed")
	}
}

func TestProvider_CreateSubdomain_AppExists_ReturnsSentinel(t *testing.T) {
	t.Parallel()
	m := newFakeMutator().on("CreatePassengerApp", uapi.ErrResourceExists)
	p := newProvider(t, nil, nil, m, nil)

	err := p.CreateSubdomain(context.Background(), "shop.example.com", "22")
	if !errors.Is(err, providers.ErrSubdomainExists) {
		t.Fatalf("expected ErrSubdomainExists, got %v", err)
	}
}

func TestProvider_CreateSubdomain_InvalidInputs(t *testing.T) {
	t.Parallel()
	m := newFakeMutator()
	p := newProvider(t, nil, nil, m, nil)

	cases := []struct {
		name        string
		domain, ver string
	}{
		{"bad_domain", "INVALID DOMAIN", "22"},
		{"bad_version", "shop.example.com", "22;echo"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := p.CreateSubdomain(context.Background(), tc.domain, tc.ver)
			if !errors.Is(err, providers.ErrInvalidProviderConfig) {
				t.Fatalf("expected ErrInvalidProviderConfig, got %v", err)
			}
		})
	}
}

func TestProvider_CreateSubdomain_MissingSeam(t *testing.T) {
	t.Parallel()
	p := newProvider(t, nil, nil, nil, nil)
	err := p.CreateSubdomain(context.Background(), "shop.example.com", "22")
	if !errors.Is(err, providers.ErrUnknownOutputFormat) {
		t.Fatalf("expected ErrUnknownOutputFormat for missing seam, got %v", err)
	}
}

func TestProvider_SetupSSL_AutoSSL_HappyPath(t *testing.T) {
	t.Parallel()
	m := newFakeMutator()
	p := newProvider(t, nil, nil, m, nil)

	err := p.SetupSSL(context.Background(), "shop.example.com")
	if err != nil {
		t.Fatalf("SetupSSL: %v", err)
	}
	if got, want := m.autoSSLDomains, []string{"shop.example.com"}; !equalStrings(got, want) {
		t.Fatalf("StartAutoSSL domains = %v, want %v", got, want)
	}
}

func TestProvider_SetupSSL_AutoSSL_DNSNotResolving(t *testing.T) {
	t.Parallel()
	m := newFakeMutator().on("StartAutoSSL", uapi.ErrModuleFunctionDenied)
	p := newProvider(t, nil, nil, m, nil)

	err := p.SetupSSL(context.Background(), "shop.example.com")
	if !errors.Is(err, providers.ErrDNSNotResolving) {
		t.Fatalf("expected ErrDNSNotResolving, got %v", err)
	}
}

func TestProvider_SetupSSL_ManualRejected(t *testing.T) {
	t.Parallel()
	m := newFakeMutator()
	p := newProvider(t, map[string]string{"ssl_provider": "manual"}, nil, m, nil)

	err := p.SetupSSL(context.Background(), "shop.example.com")
	if !errors.Is(err, providers.ErrInvalidProviderConfig) {
		t.Fatalf("expected ErrInvalidProviderConfig for manual mode, got %v", err)
	}
}

func TestProvider_CreateDatabase_HappyPath(t *testing.T) {
	t.Parallel()
	m := newFakeMutator()
	p := newProvider(t, nil, nil, m, nil)

	user, password, err := p.CreateDatabase(context.Background(), providers.DatabaseMySQL, "shopdb")
	if err != nil {
		t.Fatalf("CreateDatabase: %v", err)
	}
	if user != "shopdb" {
		t.Fatalf("user = %q, want shopdb", user)
	}
	if len(password) < 16 {
		t.Fatalf("password too short: len=%d", len(password))
	}
	wantOrder := []string{
		"CreateMysqlDatabase:shopdb",
		"CreateMysqlUser:shopdb",
		"SetMysqlPrivileges",
	}
	if !equalStrings(m.callLog, wantOrder) {
		t.Fatalf("call order = %v, want %v", m.callLog, wantOrder)
	}
	if len(m.mysqlPrivArgs) != 1 {
		t.Fatalf("SetMysqlPrivileges not called")
	}
	if got := m.mysqlPrivArgs[0]; got.Database != "shopdb" || got.User != "shopdb" {
		t.Fatalf("priv args mismatch: %+v", got)
	}
	if got, want := m.mysqlPrivArgs[0].Privileges, []string{"ALL PRIVILEGES"}; !equalStrings(got, want) {
		t.Fatalf("privileges = %v, want %v", got, want)
	}
}

func TestProvider_CreateDatabase_DBNameTaken(t *testing.T) {
	t.Parallel()
	m := newFakeMutator().on("CreateMysqlDatabase:shopdb", uapi.ErrResourceExists)
	p := newProvider(t, nil, nil, m, nil)

	_, _, err := p.CreateDatabase(context.Background(), providers.DatabaseMySQL, "shopdb")
	if !errors.Is(err, providers.ErrDBNameTaken) {
		t.Fatalf("expected ErrDBNameTaken, got %v", err)
	}
}

func TestProvider_CreateDatabase_UserCreationFails_RollsBackDB(t *testing.T) {
	t.Parallel()
	m := newFakeMutator().on("CreateMysqlUser:shopdb", errors.New("boom"))
	p := newProvider(t, nil, nil, m, nil)

	_, _, err := p.CreateDatabase(context.Background(), providers.DatabaseMySQL, "shopdb")
	if err == nil || strings.Contains(err.Error(), "rollback") {
		t.Fatalf("unexpected error shape: %v", err)
	}
	if !containsString(m.callLog, "DeleteMysqlDatabase:shopdb") {
		t.Fatalf("expected rollback DeleteMysqlDatabase, got log %v", m.callLog)
	}
}

func TestProvider_CreateDatabase_PrivilegesFail_RollsBackUserAndDB(t *testing.T) {
	t.Parallel()
	m := newFakeMutator().on("SetMysqlPrivileges", errors.New("boom"))
	p := newProvider(t, nil, nil, m, nil)

	_, _, err := p.CreateDatabase(context.Background(), providers.DatabaseMySQL, "shopdb")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !containsString(m.callLog, "DeleteMysqlUser:shopdb") {
		t.Fatalf("expected rollback DeleteMysqlUser, got %v", m.callLog)
	}
	if !containsString(m.callLog, "DeleteMysqlDatabase:shopdb") {
		t.Fatalf("expected rollback DeleteMysqlDatabase, got %v", m.callLog)
	}
}

func TestProvider_CreateDatabase_RejectsPostgres(t *testing.T) {
	t.Parallel()
	m := newFakeMutator()
	p := newProvider(t, nil, nil, m, nil)

	_, _, err := p.CreateDatabase(context.Background(), providers.DatabasePostgres, "shopdb")
	if !errors.Is(err, providers.ErrInvalidProviderConfig) {
		t.Fatalf("expected ErrInvalidProviderConfig for postgres, got %v", err)
	}
}

func TestProvider_RestartNodeApp_HappyPath(t *testing.T) {
	t.Parallel()
	m := newFakeMutator()
	p := newProvider(t, nil, nil, m, nil)

	if err := p.RestartNodeApp(context.Background(), "shop.example.com"); err != nil {
		t.Fatalf("RestartNodeApp: %v", err)
	}
	if got, want := m.restartedApps, []string{"/home/alice/nodejs/shop-example-com"}; !equalStrings(got, want) {
		t.Fatalf("restarted paths = %v, want %v", got, want)
	}
}

func TestProvider_RestartNodeApp_NotFound(t *testing.T) {
	t.Parallel()
	m := newFakeMutator().on("RestartPassengerApp", uapi.ErrResourceNotFound)
	p := newProvider(t, nil, nil, m, nil)

	err := p.RestartNodeApp(context.Background(), "shop.example.com")
	if !errors.Is(err, providers.ErrAppNotFound) {
		t.Fatalf("expected ErrAppNotFound, got %v", err)
	}
}

func TestProvider_GetDeployPath_GetLogPath(t *testing.T) {
	t.Parallel()
	p := newProvider(t, nil, nil, nil, nil)

	if got, want := p.GetDeployPath("shop.example.com"), "/home/alice/nodejs/shop-example-com/public"; got != want {
		t.Fatalf("DeployPath = %q, want %q", got, want)
	}
	if got, want := p.GetLogPath("shop.example.com"), "/home/alice/nodejs/shop-example-com/logs"; got != want {
		t.Fatalf("LogPath = %q, want %q", got, want)
	}
	if p.GetDeployPath("invalid domain") != "" {
		t.Fatalf("invalid domain should yield empty path")
	}
}

func TestProvider_TailLog_HappyPath(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{stdout: []byte("hello\nworld\n")}
	p := newProvider(t, nil, nil, nil, r)

	out, err := p.TailLog(context.Background(), "shop.example.com", 50)
	if err != nil {
		t.Fatalf("TailLog: %v", err)
	}
	if string(out) != "hello\nworld\n" {
		t.Fatalf("output mismatch: %q", out)
	}
	if len(r.calls) != 1 {
		t.Fatalf("expected 1 runner call, got %d", len(r.calls))
	}
	cmd := r.calls[0]
	wantSubstrings := []string{
		"tail",
		"-n 50",
		"/home/alice/nodejs/shop-example-com/logs/app.log",
		"/home/alice/nodejs/shop-example-com/logs/error.log",
	}
	for _, s := range wantSubstrings {
		if !strings.Contains(cmd, s) {
			t.Fatalf("command missing %q: %s", s, cmd)
		}
	}
}

func TestProvider_TailLog_ClampsLines(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		input     int
		wantInCmd string
	}{
		{"zero_uses_default", 0, "-n 200"},
		{"negative_uses_default", -5, "-n 200"},
		{"above_cap_clamps", 100000, "-n 10000"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := &fakeRunner{stdout: []byte("ok")}
			p := newProvider(t, nil, nil, nil, r)
			if _, err := p.TailLog(context.Background(), "shop.example.com", tc.input); err != nil {
				t.Fatalf("TailLog: %v", err)
			}
			if !strings.Contains(r.calls[0], tc.wantInCmd) {
				t.Fatalf("command = %q, missing %q", r.calls[0], tc.wantInCmd)
			}
		})
	}
}

func TestProvider_TailLog_NonZeroExit_ReturnsCombined(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{
		stdout:   []byte(""),
		stderr:   []byte("tail: cannot open"),
		exitCode: 1,
	}
	p := newProvider(t, nil, nil, nil, r)
	out, err := p.TailLog(context.Background(), "shop.example.com", 10)
	if err != nil {
		t.Fatalf("TailLog: %v", err)
	}
	if !strings.Contains(string(out), "tail: cannot open") {
		t.Fatalf("expected stderr passthrough, got %q", out)
	}
}

func TestProvider_CheckStatus_HappyPath(t *testing.T) {
	t.Parallel()
	r := &fakeReader{domains: &uapi.DomainInfoListResponse{MainDomain: "primary.example.com"}}
	p := newProvider(t, nil, r, nil, nil)
	p.SetClock(stubClock())

	status, err := p.CheckStatus(context.Background())
	if err != nil {
		t.Fatalf("CheckStatus: %v", err)
	}
	if !status.SSHConnected || !status.CLIInstalled {
		t.Fatalf("status should be all-green, got %+v", status)
	}
	if r.listDomainsCalls != 1 {
		t.Fatalf("expected exactly one ListDomains call, got %d", r.listDomainsCalls)
	}
}

func TestProvider_CheckStatus_AuthFailure(t *testing.T) {
	t.Parallel()
	r := &fakeReader{listDomainsErr: uapi.ErrAuthenticationFailed}
	p := newProvider(t, nil, r, nil, nil)

	status, err := p.CheckStatus(context.Background())
	if !errors.Is(err, providers.ErrCLINotFound) {
		t.Fatalf("expected ErrCLINotFound, got %v", err)
	}
	if status == nil || status.SSHConnected {
		t.Fatalf("status should report disconnected: %+v", status)
	}
}

func TestProvider_ListSubdomains(t *testing.T) {
	t.Parallel()
	r := &fakeReader{apps: &uapi.PassengerAppsListResponse{
		Applications: []uapi.PassengerApp{
			{Name: "shop", Domain: "shop.example.com", Path: "/home/alice/nodejs/shop"},
			{Name: "blog", Domain: "blog.example.com", Path: "/home/alice/nodejs/blog"},
		},
	}}
	p := newProvider(t, nil, r, nil, nil)

	subs, err := p.ListSubdomains(context.Background())
	if err != nil {
		t.Fatalf("ListSubdomains: %v", err)
	}
	if len(subs) != 2 {
		t.Fatalf("expected 2 subs, got %d", len(subs))
	}
	if subs[0].Domain != "shop.example.com" || subs[0].Type != "nodejs" {
		t.Fatalf("sub[0] mismatch: %+v", subs[0])
	}
}

func TestProvider_RemoveSubdomain_Idempotent(t *testing.T) {
	t.Parallel()
	m := newFakeMutator().
		on("DeletePassengerApp", uapi.ErrResourceNotFound).
		on("DeleteDomain", uapi.ErrResourceNotFound)
	p := newProvider(t, nil, nil, m, nil)

	if err := p.RemoveSubdomain(context.Background(), "ghost.example.com"); err != nil {
		t.Fatalf("RemoveSubdomain should be idempotent, got %v", err)
	}
	wantOrder := []string{"DeletePassengerApp", "DeleteDomain"}
	if !equalStrings(m.callLog, wantOrder) {
		t.Fatalf("order = %v, want %v", m.callLog, wantOrder)
	}
}

func TestProvider_RemoveSubdomain_SubdomainKind(t *testing.T) {
	t.Parallel()
	m := newFakeMutator()
	p := newProvider(t, map[string]string{"domain_kind": "subdomain"}, nil, m, nil)

	if err := p.RemoveSubdomain(context.Background(), "api.example.com"); err != nil {
		t.Fatalf("RemoveSubdomain: %v", err)
	}
	if got, want := m.deletedSubs, []string{"api.example.com"}; !equalStrings(got, want) {
		t.Fatalf("deletedSubs = %v, want %v", got, want)
	}
	if len(m.deletedDomains) != 0 {
		t.Fatalf("DeleteDomain should not run in subdomain mode")
	}
}

func TestProvider_RemoveDatabase_Idempotent(t *testing.T) {
	t.Parallel()
	m := newFakeMutator().
		on("DeleteMysqlUser:shopdb", uapi.ErrResourceNotFound).
		on("DeleteMysqlDatabase:shopdb", uapi.ErrResourceNotFound)
	p := newProvider(t, nil, nil, m, nil)

	if err := p.RemoveDatabase(context.Background(), providers.DatabaseMySQL, "shopdb"); err != nil {
		t.Fatalf("RemoveDatabase should be idempotent, got %v", err)
	}
}

func TestProvider_RemoveSSL_Idempotent(t *testing.T) {
	t.Parallel()
	m := newFakeMutator().on("DeleteSSL", uapi.ErrResourceNotFound)
	p := newProvider(t, nil, nil, m, nil)

	if err := p.RemoveSSL(context.Background(), "shop.example.com"); err != nil {
		t.Fatalf("RemoveSSL should be idempotent, got %v", err)
	}
}

func TestProvider_RemoveSSL_PropagatesNonIdempotent(t *testing.T) {
	t.Parallel()
	hardErr := errors.New("transport: tcp reset")
	m := newFakeMutator().on("DeleteSSL", hardErr)
	p := newProvider(t, nil, nil, m, nil)

	err := p.RemoveSSL(context.Background(), "shop.example.com")
	if err == nil || !strings.Contains(err.Error(), "tcp reset") {
		t.Fatalf("expected propagated error, got %v", err)
	}
}

// equalStrings is a tiny no-allocation comparison helper. Easier to
// read in failure messages than reflect.DeepEqual on string slices.
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

// stubClock returns a deterministic clock that advances by 10ms per
// call — enough resolution for the latency assertion without
// time.Sleep churn.
func stubClock() func() time.Time {
	base := time.Date(2026, 5, 26, 0, 0, 0, 0, time.UTC)
	var calls int
	return func() time.Time {
		t := base.Add(time.Duration(calls) * 10 * time.Millisecond)
		calls++
		return t
	}
}
