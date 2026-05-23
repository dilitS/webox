package wizard_test

import (
	"context"
	"sync"

	"github.com/dilitS/webox/providers"
)

// fakeProvider is the deterministic in-memory HostingProvider used by
// every wizard execution + rollback test. Each method has a scripted
// behaviour (success / error sequence) and records every call so
// tests can assert order, params, and rollback dispatch without
// going near SSH.
type fakeProvider struct {
	mu                sync.Mutex
	createSubdomain   []error
	setupSSL          []error
	createDatabase    []dbResult
	removeSubdomain   []error
	removeSSL         []error
	removeDatabase    []error
	listSubdomains    [][]providers.Subdomain
	checkStatus       []checkStatusResult
	calls             []string
	createSubdomainCt int
	setupSSLCt        int
	createDatabaseCt  int
	removeSubdomainCt int
	removeSSLCt       int
	removeDatabaseCt  int
	listSubdomainsCt  int
	checkStatusCt     int
}

type dbResult struct {
	user, password string
	err            error
}

type checkStatusResult struct {
	status *providers.ProviderStatus
	err    error
}

func newFakeProvider() *fakeProvider { return &fakeProvider{} }

func (f *fakeProvider) record(call string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, call)
}

func (f *fakeProvider) Calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.calls))
	copy(out, f.calls)
	return out
}

func (f *fakeProvider) Name() string { return "fake" }

func (f *fakeProvider) CreateSubdomain(_ context.Context, domain, _ string) error {
	f.record("create:" + domain)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createSubdomainCt >= len(f.createSubdomain) {
		return nil
	}
	err := f.createSubdomain[f.createSubdomainCt]
	f.createSubdomainCt++
	return err
}

func (f *fakeProvider) SetupSSL(_ context.Context, domain string) error {
	f.record("ssl:" + domain)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.setupSSLCt >= len(f.setupSSL) {
		return nil
	}
	err := f.setupSSL[f.setupSSLCt]
	f.setupSSLCt++
	return err
}

func (f *fakeProvider) CreateDatabase(_ context.Context, dbKind, dbName string) (user, password string, err error) {
	f.record("db:" + dbKind + ":" + dbName)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createDatabaseCt >= len(f.createDatabase) {
		return "u", "REDACTED-NEVER-A-REAL-SECRET-pwd", nil
	}
	res := f.createDatabase[f.createDatabaseCt]
	f.createDatabaseCt++
	return res.user, res.password, res.err
}

func (f *fakeProvider) RestartNodeApp(context.Context, string) error { return nil }
func (f *fakeProvider) GetDeployPath(string) string                  { return "/tmp/deploy" }
func (f *fakeProvider) GetLogPath(string) string                     { return "/tmp/logs" }

func (f *fakeProvider) CheckStatus(context.Context) (*providers.ProviderStatus, error) {
	f.record("status")
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.checkStatusCt >= len(f.checkStatus) {
		return &providers.ProviderStatus{SSHConnected: true, CLIInstalled: true, LatencyMS: 1}, nil
	}
	res := f.checkStatus[f.checkStatusCt]
	f.checkStatusCt++
	return res.status, res.err
}

func (f *fakeProvider) ListSubdomains(context.Context) ([]providers.Subdomain, error) {
	f.record("list")
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listSubdomainsCt >= len(f.listSubdomains) {
		return nil, nil
	}
	out := f.listSubdomains[f.listSubdomainsCt]
	f.listSubdomainsCt++
	return out, nil
}

func (f *fakeProvider) RemoveSubdomain(_ context.Context, domain string) error {
	f.record("remove:" + domain)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.removeSubdomainCt >= len(f.removeSubdomain) {
		return nil
	}
	err := f.removeSubdomain[f.removeSubdomainCt]
	f.removeSubdomainCt++
	return err
}

func (f *fakeProvider) RemoveSSL(_ context.Context, domain string) error {
	f.record("remove-ssl:" + domain)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.removeSSLCt >= len(f.removeSSL) {
		return nil
	}
	err := f.removeSSL[f.removeSSLCt]
	f.removeSSLCt++
	return err
}

func (f *fakeProvider) RemoveDatabase(_ context.Context, dbKind, dbName string) error {
	f.record("remove-db:" + dbKind + ":" + dbName)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.removeDatabaseCt >= len(f.removeDatabase) {
		return nil
	}
	err := f.removeDatabase[f.removeDatabaseCt]
	f.removeDatabaseCt++
	return err
}
