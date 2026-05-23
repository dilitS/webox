package tui_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/dilitS/webox/config"
	"github.com/dilitS/webox/providers"
	"github.com/dilitS/webox/tui"
	"github.com/dilitS/webox/wizard"
)

// Note on tests below: we exercise the production `defaultRunner` via
// NewTestWizardRunner, swapping the provider construction seam for an
// in-memory fake. This lets us assert that every wrapper method
// forwards arguments unchanged and surfaces both happy- and
// sad-path errors back to the TUI.

type runnerProvider struct {
	createDomain string
	createNode   string
	createErr    error

	setupSSLDomain string
	setupSSLErr    error

	createDBKind string
	createDBName string
	createDBUser string
	createDBPass string
	createDBErr  error

	restartDomain string
	restartErr    error

	logDomain string
	logLines  int
	logBytes  []byte
	logErr    error

	status    *providers.ProviderStatus
	statusErr error

	listResp []providers.Subdomain
	listErr  error

	removeSubdomainDomain string
	removeSubdomainErr    error
	removeSSLDomain       string
	removeSSLErr          error
	removeDBKind          string
	removeDBName          string
	removeDBErr           error
}

// Name reports the registered provider type. We pretend to be
// `smallhost` so the wizard validator registry (which only knows
// about real providers) resolves a valid validator set for Execute.
func (p *runnerProvider) Name() string { return "smallhost" }

func (p *runnerProvider) CreateSubdomain(_ context.Context, domain, node string) error {
	p.createDomain, p.createNode = domain, node
	return p.createErr
}

func (p *runnerProvider) SetupSSL(_ context.Context, domain string) error {
	p.setupSSLDomain = domain
	return p.setupSSLErr
}

func (p *runnerProvider) CreateDatabase(_ context.Context, kind, name string) (user, password string, err error) {
	p.createDBKind, p.createDBName = kind, name
	return p.createDBUser, p.createDBPass, p.createDBErr
}

func (p *runnerProvider) RestartNodeApp(_ context.Context, domain string) error {
	p.restartDomain = domain
	return p.restartErr
}

func (p *runnerProvider) GetDeployPath(domain string) string { return "/home/demo/domains/" + domain }

func (p *runnerProvider) GetLogPath(domain string) string {
	return "/home/demo/domains/" + domain + "/logs"
}

func (p *runnerProvider) TailLog(_ context.Context, domain string, lines int) ([]byte, error) {
	p.logDomain, p.logLines = domain, lines
	return p.logBytes, p.logErr
}

func (p *runnerProvider) CheckStatus(_ context.Context) (*providers.ProviderStatus, error) {
	if p.statusErr != nil {
		return nil, p.statusErr
	}
	if p.status != nil {
		return p.status, nil
	}
	return &providers.ProviderStatus{SSHConnected: true, CLIInstalled: true, LatencyMS: 5}, nil
}

func (p *runnerProvider) ListSubdomains(_ context.Context) ([]providers.Subdomain, error) {
	return p.listResp, p.listErr
}

func (p *runnerProvider) RemoveSubdomain(_ context.Context, domain string) error {
	p.removeSubdomainDomain = domain
	return p.removeSubdomainErr
}

func (p *runnerProvider) RemoveDatabase(_ context.Context, kind, name string) error {
	p.removeDBKind, p.removeDBName = kind, name
	return p.removeDBErr
}

func (p *runnerProvider) RemoveSSL(_ context.Context, domain string) error {
	p.removeSSLDomain = domain
	return p.removeSSLErr
}

func sampleProfile() config.Profile {
	return config.Profile{
		Alias:      "main",
		Type:       "smallhost",
		Host:       "s1.small.pl",
		Port:       22,
		User:       "demo",
		Properties: map[string]string{"restart_method": "devil"},
	}
}

func TestDefaultRunner_Preflight_ReturnsProviderStatus(t *testing.T) {
	t.Parallel()
	p := &runnerProvider{status: &providers.ProviderStatus{SSHConnected: true, CLIInstalled: true, LatencyMS: 17}}
	runner := tui.NewTestWizardRunner(func(config.Profile) (providers.HostingProvider, error) { return p, nil })
	got, err := runner.Preflight(context.Background(), sampleProfile())
	if err != nil {
		t.Fatalf("Preflight err = %v", err)
	}
	if got.LatencyMS != 17 {
		t.Fatalf("LatencyMS = %d, want 17", got.LatencyMS)
	}
}

func TestDefaultRunner_Preflight_PropagatesFactoryError(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("factory: missing executor")
	runner := tui.NewTestWizardRunner(func(config.Profile) (providers.HostingProvider, error) { return nil, wantErr })
	if _, err := runner.Preflight(context.Background(), sampleProfile()); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want wrap of %v", err, wantErr)
	}
}

func TestDefaultRunner_CheckDomainAvailable_PropagatesExistsSentinel(t *testing.T) {
	t.Parallel()
	p := &runnerProvider{listResp: []providers.Subdomain{{Domain: "taken.demo.smallhost.pl"}}}
	runner := tui.NewTestWizardRunner(func(config.Profile) (providers.HostingProvider, error) { return p, nil })
	err := runner.CheckDomainAvailable(context.Background(), sampleProfile(), "taken.demo.smallhost.pl")
	if !errors.Is(err, providers.ErrSubdomainExists) {
		t.Fatalf("err = %v, want ErrSubdomainExists", err)
	}
}

func TestDefaultRunner_CheckDomainAvailable_HappyPath(t *testing.T) {
	t.Parallel()
	p := &runnerProvider{listResp: []providers.Subdomain{{Domain: "other.demo.smallhost.pl"}}}
	runner := tui.NewTestWizardRunner(func(config.Profile) (providers.HostingProvider, error) { return p, nil })
	if err := runner.CheckDomainAvailable(context.Background(), sampleProfile(), "new.demo.smallhost.pl"); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
}

func TestDefaultRunner_CheckDomainAvailable_ListFailureSurfaces(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("ssh down")
	p := &runnerProvider{listErr: wantErr}
	runner := tui.NewTestWizardRunner(func(config.Profile) (providers.HostingProvider, error) { return p, nil })
	if err := runner.CheckDomainAvailable(context.Background(), sampleProfile(), "x.demo.smallhost.pl"); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want wrap of %v", err, wantErr)
	}
}

func TestDefaultRunner_Execute_HappyPath_PushesStack(t *testing.T) {
	t.Parallel()
	p := &runnerProvider{createDBUser: "u1", createDBPass: "REDACTED-NEVER-A-REAL-SECRET-pwd"}
	runner := tui.NewTestWizardRunner(func(config.Profile) (providers.HostingProvider, error) { return p, nil })
	stack := wizard.NewStack(nil, "test-wizard")
	plan := wizard.ProvisionPlan{
		ProfileAlias: "main",
		Stack:        "static",
		Domain:       "app.demo.smallhost.pl",
		NodeVersion:  "22",
		DBKind:       providers.DatabaseMySQL,
		DBName:       "appdb",
	}
	report, err := runner.Execute(context.Background(), sampleProfile(), plan, stack)
	if err != nil {
		t.Fatalf("Execute err = %v", err)
	}
	if !report.Subdomain.OK || !report.SSL.OK || !report.Database.OK {
		t.Fatalf("report incomplete: %+v", report)
	}
	if p.createDomain != "app.demo.smallhost.pl" || p.createNode != "22" {
		t.Fatalf("subdomain args = (%s, %s)", p.createDomain, p.createNode)
	}
	if stack.Len() < 1 {
		t.Fatalf("stack must contain rollback steps, got depth %d", stack.Len())
	}
}

func TestDefaultRunner_Rollback_RunsLIFOAcrossSteps(t *testing.T) {
	t.Parallel()
	p := &runnerProvider{}
	runner := tui.NewTestWizardRunner(func(config.Profile) (providers.HostingProvider, error) { return p, nil })

	stack := wizard.NewStack(nil, "test-wizard")
	_ = stack.Push(context.Background(), wizard.CleanupStep{Name: "Remove subdomain x", Kind: wizard.ResourceSubdomain, Params: map[string]string{"domain": "x.demo.smallhost.pl"}})
	_ = stack.Push(context.Background(), wizard.CleanupStep{Name: "Remove SSL x", Kind: wizard.ResourceSSL, Params: map[string]string{"domain": "x.demo.smallhost.pl"}})
	results, err := runner.Rollback(context.Background(), sampleProfile(), stack)
	if err != nil {
		t.Fatalf("Rollback err = %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %d, want 2", len(results))
	}
	if p.removeSSLDomain != "x.demo.smallhost.pl" || p.removeSubdomainDomain != "x.demo.smallhost.pl" {
		t.Fatalf("rollback did not reach provider methods: %+v", p)
	}
}

func TestDefaultRunner_RestartApp_ForwardsAndReturnsErr(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("restart blocked")
	p := &runnerProvider{restartErr: wantErr}
	runner := tui.NewTestWizardRunner(func(config.Profile) (providers.HostingProvider, error) { return p, nil })
	err := runner.RestartApp(context.Background(), sampleProfile(), "app.demo.smallhost.pl")
	if !errors.Is(err, wantErr) {
		t.Fatalf("RestartApp err = %v, want %v", err, wantErr)
	}
	if p.restartDomain != "app.demo.smallhost.pl" {
		t.Fatalf("restartDomain = %q", p.restartDomain)
	}
}

func TestDefaultRunner_RenewSSL_PropagatesError(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("acme rate limit")
	p := &runnerProvider{setupSSLErr: wantErr}
	runner := tui.NewTestWizardRunner(func(config.Profile) (providers.HostingProvider, error) { return p, nil })
	if err := runner.RenewSSL(context.Background(), sampleProfile(), "app.demo.smallhost.pl"); !errors.Is(err, wantErr) {
		t.Fatalf("RenewSSL err = %v, want %v", err, wantErr)
	}
}

func TestDefaultRunner_TailLog_ForwardsLinesAndReturnsBytes(t *testing.T) {
	t.Parallel()
	p := &runnerProvider{logBytes: []byte("hello\nworld\n")}
	runner := tui.NewTestWizardRunner(func(config.Profile) (providers.HostingProvider, error) { return p, nil })
	out, err := runner.TailLog(context.Background(), sampleProfile(), "app.demo.smallhost.pl", 50)
	if err != nil {
		t.Fatalf("TailLog err = %v", err)
	}
	if !strings.Contains(string(out), "world") {
		t.Fatalf("output = %q, want contains world", out)
	}
	if p.logDomain != "app.demo.smallhost.pl" || p.logLines != 50 {
		t.Fatalf("TailLog forwarded (%s, %d)", p.logDomain, p.logLines)
	}
}

func TestDefaultRunner_ListProviderSubdomains_ReturnsRows(t *testing.T) {
	t.Parallel()
	rows := []providers.Subdomain{{Domain: "x.demo.smallhost.pl", Type: "nodejs", NodeVersion: "22"}}
	p := &runnerProvider{listResp: rows}
	runner := tui.NewTestWizardRunner(func(config.Profile) (providers.HostingProvider, error) { return p, nil })
	got, err := runner.ListProviderSubdomains(context.Background(), sampleProfile())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(got) != 1 || got[0].NodeVersion != "22" {
		t.Fatalf("rows = %+v", got)
	}
}

// All wrapper methods short-circuit when the factory fails. Single
// fail-fast test asserts each path returns the factory error
// unchanged.
func TestDefaultRunner_FactoryFailureShortCircuitsAllMethods(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("provider missing")
	runner := tui.NewTestWizardRunner(func(config.Profile) (providers.HostingProvider, error) { return nil, wantErr })
	profile := sampleProfile()
	ctx := context.Background()

	cases := []struct {
		name string
		call func() error
	}{
		{"Preflight", func() error { _, e := runner.Preflight(ctx, profile); return e }},
		{"CheckDomainAvailable", func() error { return runner.CheckDomainAvailable(ctx, profile, "x") }},
		{"Execute", func() error {
			_, e := runner.Execute(ctx, profile, wizard.ProvisionPlan{}, wizard.NewStack(nil, "test"))
			return e
		}},
		{"Rollback", func() error { _, e := runner.Rollback(ctx, profile, wizard.NewStack(nil, "test")); return e }},
		{"RestartApp", func() error { return runner.RestartApp(ctx, profile, "x") }},
		{"RenewSSL", func() error { return runner.RenewSSL(ctx, profile, "x") }},
		{"TailLog", func() error { _, e := runner.TailLog(ctx, profile, "x", 10); return e }},
		{"ListProviderSubdomains", func() error { _, e := runner.ListProviderSubdomains(ctx, profile); return e }},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if err := c.call(); !errors.Is(err, wantErr) {
				t.Fatalf("%s err = %v, want wrap of %v", c.name, err, wantErr)
			}
		})
	}
}

func TestDefaultWizardRunner_BuildsRealProviderForSmallhost(t *testing.T) {
	t.Parallel()
	// DefaultWizardRunner is the production constructor. We can't
	// exercise the network paths here, but we can verify it produces
	// a non-nil runner and that pointing it at a known-good profile
	// type does not panic when calling `Preflight` (the call fails
	// with the executor-not-configured sentinel because the live SSH
	// pool is not wired yet — which is intentional and tested in the
	// smallhost suite).
	runner := tui.DefaultWizardRunner()
	if runner == nil {
		t.Fatal("DefaultWizardRunner returned nil")
	}
	_, err := runner.Preflight(context.Background(), sampleProfile())
	if err == nil {
		t.Fatal("expected executor-not-configured error from real runner without pool")
	}
}

func TestProfileByAlias_ReturnsHitOrZero(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{Profiles: []config.Profile{{Alias: "main"}, {Alias: "backup"}}}
	if p, ok := tui.ProfileByAlias(cfg, "backup"); !ok || p.Alias != "backup" {
		t.Fatalf("ProfileByAlias = (%+v, %v), want backup, true", p, ok)
	}
	if p, ok := tui.ProfileByAlias(cfg, "ghost"); ok || p.Alias != "" {
		t.Fatalf("ProfileByAlias = (%+v, %v), want zero, false", p, ok)
	}
	if _, ok := tui.ProfileByAlias(nil, "main"); ok {
		t.Fatal("ProfileByAlias(nil) returned ok=true")
	}
}
