package wizard_test

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"github.com/dilitS/webox/providers"
	"github.com/dilitS/webox/providers/cpanel"
	"github.com/dilitS/webox/providers/cpanel/uapi"
	"github.com/dilitS/webox/wizard"
)

// fakeCpanelMutator is a thin in-memory [uapi.Mutator] used by the
// wizard×cpanel integration tests. It mirrors the fakeMutator pattern
// from providers/cpanel/methods_test.go but lives in `wizard_test` so
// the wizard package never depends on the cpanel test package.
type fakeCpanelMutator struct {
	mu      sync.Mutex
	scripts map[string]error
	callLog []string
}

func newFakeCpanelMutator() *fakeCpanelMutator {
	return &fakeCpanelMutator{scripts: map[string]error{}}
}

func (f *fakeCpanelMutator) on(op string, err error) *fakeCpanelMutator {
	f.scripts[op] = err
	return f
}

func (f *fakeCpanelMutator) result(op string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.callLog = append(f.callLog, op)
	return f.scripts[op]
}

func (f *fakeCpanelMutator) Calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.callLog))
	copy(out, f.callLog)
	return out
}

func (f *fakeCpanelMutator) AddAddonDomain(_ context.Context, _ uapi.CreateAddonDomainArgs) error {
	return f.result("AddAddonDomain")
}

func (f *fakeCpanelMutator) AddSubdomain(_ context.Context, _ uapi.CreateSubdomainArgs) error {
	return f.result("AddSubdomain")
}

func (f *fakeCpanelMutator) DeleteDomain(_ context.Context, _ string) error {
	return f.result("DeleteDomain")
}

func (f *fakeCpanelMutator) DeleteSubdomain(_ context.Context, _ string) error {
	return f.result("DeleteSubdomain")
}

func (f *fakeCpanelMutator) CreatePassengerApp(_ context.Context, _ uapi.CreatePassengerAppArgs) error {
	return f.result("CreatePassengerApp")
}

func (f *fakeCpanelMutator) EditPassengerApp(_ context.Context, _ uapi.EditPassengerAppArgs) error {
	return f.result("EditPassengerApp")
}

func (f *fakeCpanelMutator) RestartPassengerApp(_ context.Context, _ string) error {
	return f.result("RestartPassengerApp")
}

func (f *fakeCpanelMutator) DeletePassengerApp(_ context.Context, _ string) error {
	return f.result("DeletePassengerApp")
}

func (f *fakeCpanelMutator) CreateMysqlDatabase(_ context.Context, _ string) error {
	return f.result("CreateMysqlDatabase")
}

func (f *fakeCpanelMutator) DeleteMysqlDatabase(_ context.Context, _ string) error {
	return f.result("DeleteMysqlDatabase")
}

func (f *fakeCpanelMutator) CreateMysqlUser(_ context.Context, _, _ string) error {
	return f.result("CreateMysqlUser")
}

func (f *fakeCpanelMutator) DeleteMysqlUser(_ context.Context, _ string) error {
	return f.result("DeleteMysqlUser")
}

func (f *fakeCpanelMutator) SetMysqlPrivileges(_ context.Context, _ uapi.MysqlPrivilegesArgs) error {
	return f.result("SetMysqlPrivileges")
}

func (f *fakeCpanelMutator) InstallSSL(_ context.Context, _ uapi.InstallSSLArgs) error {
	return f.result("InstallSSL")
}

func (f *fakeCpanelMutator) StartAutoSSL(_ context.Context, _ string) error {
	return f.result("StartAutoSSL")
}

func (f *fakeCpanelMutator) DeleteSSL(_ context.Context, _ string) error {
	return f.result("DeleteSSL")
}

// fakeCpanelReader serves the ListDomains / ListPassengerApps calls
// the wizard uses for preflight + duplicate-domain detection.
type fakeCpanelReader struct {
	domains *uapi.DomainInfoListResponse
	apps    *uapi.PassengerAppsListResponse
}

func (f *fakeCpanelReader) ListDomains(_ context.Context) (*uapi.DomainInfoListResponse, error) {
	if f.domains == nil {
		return &uapi.DomainInfoListResponse{}, nil
	}
	return f.domains, nil
}

func (f *fakeCpanelReader) ListPassengerApps(_ context.Context) (*uapi.PassengerAppsListResponse, error) {
	if f.apps == nil {
		return &uapi.PassengerAppsListResponse{}, nil
	}
	return f.apps, nil
}

func (f *fakeCpanelReader) ListMysqlDatabases(_ context.Context) (*uapi.MysqlListDatabasesResponse, error) {
	return &uapi.MysqlListDatabasesResponse{}, nil
}

func (f *fakeCpanelReader) ListSSLKeys(_ context.Context) (*uapi.SSLListKeysResponse, error) {
	return &uapi.SSLListKeysResponse{}, nil
}

// Transport satisfies the post-Sprint-23 uapi.Reader.Transport()
// addition; the in-memory fake has no real transport.
func (f *fakeCpanelReader) Transport() string { return "fake" }

// newCpanelProvider mirrors the cloudlinux-selector preset's
// capability layout: addon domains, AutoSSL, MySQL.
func newCpanelProvider(t *testing.T, r uapi.Reader, m uapi.Mutator) providers.HostingProvider {
	t.Helper()
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
	if r != nil {
		cp.SetReader(r)
	}
	if m != nil {
		cp.SetMutator(m)
	}
	return cp
}

// TestWizard_HappyPath_CpanelCloudlinuxSelector_Provisions verifies
// the wizard drives a full node-express plan through the cpanel
// provider end-to-end: subdomain → SSL → MySQL DB, with the LIFO
// stack persisted into pending_cleanups.json.
//
// This test catches drift between the wizard contract and the
// cpanel adapter: any signature mismatch or unexpected sentinel
// surfaces here instead of in production.
func TestWizard_HappyPath_CpanelCloudlinuxSelector_Provisions(t *testing.T) {
	t.Parallel()

	mut := newFakeCpanelMutator()
	rdr := &fakeCpanelReader{}
	provider := newCpanelProvider(t, rdr, mut)

	tmp := t.TempDir()
	stack := wizard.NewStack(
		wizard.NewFilePersister(filepath.Join(tmp, "pending.json"), "wiz-cp-1"),
		"wiz-cp-1",
	)

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
		t.Fatalf("credentials missing on successful DB step")
	}
	if got := len(report.Credentials.Password); got < 16 {
		t.Fatalf("password length = %d, want >= 16", got)
	}
	if stack.Len() != 3 {
		t.Fatalf("rollback stack len = %d, want 3 (sub+ssl+db)", stack.Len())
	}

	want := []string{
		"AddAddonDomain",      // CreateSubdomain step 1
		"CreatePassengerApp",  // CreateSubdomain step 2
		"StartAutoSSL",        // SetupSSL
		"CreateMysqlDatabase", // CreateDatabase step 1
		"CreateMysqlUser",     // CreateDatabase step 2
		"SetMysqlPrivileges",  // CreateDatabase step 3
	}
	if got := mut.Calls(); !sameOrder(got, want) {
		t.Fatalf("call order = %v, want %v", got, want)
	}
}

// TestWizard_DBFailure_RollsBackCpanelStack verifies the wizard's
// LIFO rollback drives the cpanel adapter through RemoveSSL +
// RemoveSubdomain on a DB failure.
func TestWizard_DBFailure_RollsBackCpanelStack(t *testing.T) {
	t.Parallel()

	mut := newFakeCpanelMutator().on("CreateMysqlDatabase", errors.New("connection reset by peer"))
	provider := newCpanelProvider(t, &fakeCpanelReader{}, mut)
	stack := wizard.NewStack(nil, "wiz-cp-rb")

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
		t.Fatal("expected execution to fail at DB step")
	}
	if stack.Len() != 2 {
		t.Fatalf("stack len pre-rollback = %d, want 2 (sub+ssl)", stack.Len())
	}

	runner := wizard.MakeStepRunner(provider)
	results, _ := stack.Rollback(context.Background(), runner)
	if len(results) != 2 {
		t.Fatalf("rollback results = %d, want 2", len(results))
	}
	for i, r := range results {
		if r.Err != nil {
			t.Fatalf("rollback[%d] returned error: %v", i, r.Err)
		}
	}
	calls := mut.Calls()
	want := []string{
		"DeleteSSL",          // SSL rolled back first (LIFO)
		"DeletePassengerApp", // then subdomain — first delete the app
		"DeleteDomain",       // then the domain
	}
	// The rollback runs DeleteSSL first; ordering between
	// DeletePassengerApp and DeleteDomain inside RemoveSubdomain
	// is enforced by the adapter.
	tail := calls[len(calls)-3:]
	if !sameOrder(tail, want) {
		t.Fatalf("rollback tail = %v, want %v (full log: %v)", tail, want, calls)
	}
}

// TestWizard_CpanelValidatorsResolveViaRegistry asserts that the
// wizard resolves cpanel's plan validators through the
// [providers.PlanValidatorsFor] indirection, NOT by importing the
// cpanel package directly. This is the gate that proves AGENTS.md
// §2.2 ("business logic never knows a provider by name") holds for
// the cpanel adapter.
func TestWizard_CpanelValidatorsResolveViaRegistry(t *testing.T) {
	t.Parallel()
	set, err := providers.PlanValidatorsFor("cpanel")
	if err != nil {
		t.Fatalf("PlanValidatorsFor(cpanel): %v", err)
	}
	if !set.IsComplete() {
		t.Fatalf("validator set incomplete")
	}
	if err := set.ValidateDomain("shop.example.com"); err != nil {
		t.Fatalf("ValidateDomain: %v", err)
	}
	if err := set.ValidateDomain("INVALID DOMAIN"); err == nil {
		t.Fatal("ValidateDomain accepted invalid input")
	}
	if err := set.ValidateNodeVersion("22"); err != nil {
		t.Fatalf("ValidateNodeVersion: %v", err)
	}
	if err := set.ValidateDBName("shopdb"); err != nil {
		t.Fatalf("ValidateDBName: %v", err)
	}
}

// TestWizard_CpanelDomainCollisionReported verifies the wizard's
// duplicate-domain pre-check sees the cpanel adapter's enumerated
// PassengerApps as the source of truth.
func TestWizard_CpanelDomainCollisionReported(t *testing.T) {
	t.Parallel()
	rdr := &fakeCpanelReader{apps: &uapi.PassengerAppsListResponse{
		Applications: []uapi.PassengerApp{
			{Name: "shop", Domain: "shop.example.com", Path: "/home/alice/nodejs/shop"},
		},
	}}
	provider := newCpanelProvider(t, rdr, newFakeCpanelMutator())

	err := wizard.CheckSubdomainAvailable(context.Background(), provider, "shop.example.com")
	if !errors.Is(err, providers.ErrSubdomainExists) {
		t.Fatalf("expected ErrSubdomainExists, got %v", err)
	}
}

// sameOrder is a small helper local to this file to avoid name
// clashes with the methods_test.go equivalents from the cpanel
// package (which lives outside the wizard_test package).
func sameOrder(a, b []string) bool {
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
