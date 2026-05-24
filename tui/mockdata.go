package tui

// Mock-data fixtures for the offline cockpit (`webox --mock`).
// Magic numbers (build numbers, SSL day counters, step durations,
// terminal sizes) are intentional and exempted in `.golangci.yml`
// — they reproduce the reference cockpit image and are not
// algorithmic parameters.

import (
	"context"
	"time"

	"github.com/dilitS/webox/config"
	"github.com/dilitS/webox/providers"
	ghsvc "github.com/dilitS/webox/services/github"
	"github.com/dilitS/webox/status"
	"github.com/dilitS/webox/tui/bento"
	"github.com/dilitS/webox/wizard"
)

// MockOptions builds an [Options] value pre-loaded with realistic
// demo data so the cockpit boots end-to-end without touching SSH,
// HTTP probes, the keyring, or GitHub. The launcher (`cmd/webox`)
// wires it behind `--mock` / `WEBOX_MOCK=1`.
//
// The mock is deterministic: every fetcher returns the same payload
// every call so screenshots and recording sessions stay reproducible.
//
// Security note: the mock fixtures contain ONLY synthetic data
// (`shop-ease.io`, `dilitS-demo`, fake commit SHAs, fake build
// numbers). Nothing here looks like a real secret — the redactor's
// regression corpus stays intact.
func MockOptions(configPath string) Options {
	cfg := MockConfig()
	now := mockNow()

	cicd := mockCICDSnapshots(cfg)
	pipelineFetcher := func(_ context.Context, ref ghsvc.RepoRef, _ string) (PipelineFetchResult, error) {
		for _, entry := range cicd {
			if entry.Run == nil {
				continue
			}
			return PipelineFetchResult{
				Run: &ghsvc.WorkflowRun{
					ID:         entry.Run.RunID,
					RunNumber:  entry.Run.RunNumber,
					Status:     entry.Run.Status,
					Conclusion: entry.Run.Conclusion,
				},
				Steps: entry.Steps,
			}, nil
		}
		_ = ref
		return PipelineFetchResult{}, ghsvc.ErrRunNotFound
	}

	lastDeployFetcher := func(_ context.Context, _ ghsvc.RepoRef, _ string) (*ghsvc.WorkflowRun, error) {
		t := now.Add(-2 * time.Hour)
		return &ghsvc.WorkflowRun{
			RunNumber:  412,
			Status:     "completed",
			Conclusion: "success",
			UpdatedAt:  t,
			StartedAt:  &t,
		}, nil
	}

	statusesByID := MockProjectStatuses(cfg, now)
	statusFetcher := func(_ context.Context, projects []config.Project, _ *status.Cache) ([]ProjectStatus, error) {
		out := make([]ProjectStatus, 0, len(projects))
		for _, p := range projects {
			if got, ok := statusesByID[p.ID]; ok {
				out = append(out, got)
				continue
			}
			out = append(out, ProjectStatus{ProjectID: p.ID, State: ProjectUnknown})
		}
		return out, nil
	}

	return Options{
		ConfigPath:       configPath,
		Cache:            status.NewCache(status.Options{}),
		FetchStatuses:    statusFetcher,
		GitHubLastDeploy: lastDeployFetcher,
		GitHubPipeline:   pipelineFetcher,
		GitHubLogs:       MockGitHubLogsFetcher,
		RefreshInterval:  30 * time.Second,
		InitialWidth:     140,
		InitialHeight:    40,
		Now:              mockNow,
		PreloadedConfig:  cfg,
		WizardRunner:     newMockWizardRunner(),
		MockHeaderMetrics: HeaderMetricsSnapshot{
			ProfileAlias: "us-east-1",
			UptimeLabel:  "24d 11h",
			LoadLabel:    "0.12, 0.28, 0.31",
			RAMLabel:     "3.4/8.0 GB (42%)",
			RTTLabel:     "18ms (SF01)",
			UpdatedAt:    now,
		},
		MockLiveLogLines:  MockLiveLogLines(),
		MockCICDSnapshots: cicd,
	}
}

// mockNow returns a fixed timestamp so demo screenshots match the
// reference image (14:32:01 UTC). Production users boot in real time;
// only the mock launcher pins this clock.
func mockNow() time.Time {
	return time.Date(2026, 5, 24, 14, 32, 1, 0, time.UTC)
}

// MockConfig returns the deterministic config rendered in the
// `--mock` mode. The six projects mirror the reference image: four
// healthy (ShopEase-Web, API-Gateway, two Dashboards), one degraded
// (Auth-Service), one outage (Payment-UI).
func MockConfig() *config.Config {
	return &config.Config{
		SchemaVersion: config.Current,
		Profiles: []config.Profile{
			{Alias: "us-east-1", Type: "smallhost", Host: "demo.smallhost.pl", Port: 22, User: "shopease"},
		},
		Projects: []config.Project{
			{ID: "p1", Domain: "ShopEase-Web", ProfileAlias: "us-east-1", Repo: "dilitS-demo/shopease-web", Stack: "node-express", NodeVersion: "v20.11.0"},
			{ID: "p2", Domain: "API-Gateway", ProfileAlias: "us-east-1", Repo: "dilitS-demo/api-gateway", Stack: "node-express", NodeVersion: "v20.11.0"},
			{ID: "p3", Domain: "Auth-Service", ProfileAlias: "us-east-1", Repo: "dilitS-demo/auth-service", Stack: "node-express", NodeVersion: "v20.11.0"},
			{ID: "p4", Domain: "Dashboard", ProfileAlias: "us-east-1", Repo: "dilitS-demo/dashboard", Stack: "vite-react", NodeVersion: "v20.11.0"},
			{ID: "p5", Domain: "Dashboard-Admin", ProfileAlias: "us-east-1", Repo: "dilitS-demo/dashboard-admin", Stack: "vite-react", NodeVersion: "v20.11.0"},
			{ID: "p6", Domain: "Payment-UI", ProfileAlias: "us-east-1", Repo: "dilitS-demo/payment-ui", Stack: "vite-react", NodeVersion: "v18.20.0"},
		},
	}
}

// MockProjectStatuses returns the per-project status map used by the
// mock fetcher. Status mix matches the reference image exactly.
func MockProjectStatuses(cfg *config.Config, now time.Time) map[string]ProjectStatus {
	if cfg == nil {
		return map[string]ProjectStatus{}
	}
	statuses := map[string]ProjectStatus{
		"p1": {ProjectID: "p1", State: ProjectOnline, HTTPHealth: "200 OK", SSLDaysLeft: 114, NodeVersion: "v20.11.0", LastDeploy: "2h ago · success", FetchedAt: now},
		"p2": {ProjectID: "p2", State: ProjectOnline, HTTPHealth: "200 OK", SSLDaysLeft: 89, NodeVersion: "v20.11.0", LastDeploy: "11m ago · success", FetchedAt: now},
		"p3": {ProjectID: "p3", State: ProjectBuilding, HTTPHealth: "200 (degraded)", SSLDaysLeft: 28, NodeVersion: "v20.11.0", LastDeploy: "4h ago · success", FetchedAt: now},
		"p4": {ProjectID: "p4", State: ProjectOnline, HTTPHealth: "200 OK", SSLDaysLeft: 76, NodeVersion: "v20.11.0", LastDeploy: "1d ago · success", FetchedAt: now},
		"p5": {ProjectID: "p5", State: ProjectBuilding, HTTPHealth: "200 OK", SSLDaysLeft: 64, NodeVersion: "v20.11.0", LastDeploy: "3d ago · success", FetchedAt: now},
		"p6": {ProjectID: "p6", State: ProjectOffline, HTTPHealth: "502 Bad Gateway", SSLDaysLeft: 4, NodeVersion: "v18.20.0", LastDeploy: "8h ago · failure", FetchedAt: now},
	}
	return statuses
}

// mockCICDSnapshots returns a deterministic CI/CD payload for the
// first project in cfg.Projects (the one focused by default). The
// other projects render the cyan placeholder until selected.
//
// Unexported because the return type ([cicdSnapshotEntry]) is
// internal — callers should drive the dashboard through
// [MockOptions] instead.
func mockCICDSnapshots(cfg *config.Config) map[string]cicdSnapshotEntry {
	out := make(map[string]cicdSnapshotEntry)
	if cfg == nil || len(cfg.Projects) == 0 {
		return out
	}
	first := cfg.Projects[0]
	now := mockNow().Add(-20 * time.Minute)
	out[first.ID] = cicdSnapshotEntry{
		Run: &cicdRunSummary{
			RunID:      482917312,
			RunNumber:  412,
			Status:     "completed",
			Conclusion: "success",
			HeaderTime: now,
			Duration:   "1m 42s",
		},
		Steps: []ghsvc.Step{
			{Number: 1, Name: "Git Checkout", Status: "completed", Conclusion: "success", DurationMs: 2000},
			{Number: 2, Name: "Install Deps", Status: "completed", Conclusion: "success", DurationMs: 12000},
			{Number: 3, Name: "Code Lint", Status: "completed", Conclusion: "success", DurationMs: 7000},
			{Number: 4, Name: "Build Artifact", Status: "completed", Conclusion: "success", DurationMs: 38000},
			{Number: 5, Name: "Unit Tests", Status: "completed", Conclusion: "success", DurationMs: 19000},
			{Number: 6, Name: "Deploy (Prod)", Status: "completed", Conclusion: "success", DurationMs: 22000},
		},
		FetchedAt: mockNow(),
	}
	return out
}

// MockLiveLogLines returns the seed buffer rendered in the Live
// Server Logs tile when `--mock` is active. The lines mirror the
// reference image's INFO/WARN/DEBUG mix so the operator can verify
// the colour mapping at a glance.
//
// Sprint-14 addition: two extra lines reference the new
// `--debug-trace` and host-key modal subsystems so the mock demo
// surfaces them even before the operator interacts with the
// cockpit. The lines are NOT marked Redacted because they contain
// no secret-shaped content.
func MockLiveLogLines() []LiveLogLine {
	return []LiveLogLine{
		{Level: "INFO", Text: "[14:32:10] INFO - API-Gateway: Incoming GET /users (status: 200)"},
		{Level: "WARN", Text: "[14:32:11] WARN - Auth-Service: High latency detected (450ms)"},
		{Level: "INFO", Text: "[14:32:12] INFO - ShopEase-Web: Served /products in 88ms"},
		{Level: "DEBUG", Text: "[14:32:14] DEBUG - Worker #2: Cache hit for key 'prod:list'"},
		{Level: "INFO", Text: "[14:32:15] INFO - API-Gateway: Healthcheck OK (12ms)"},
		{Level: "INFO", Text: "[14:32:16] INFO - cockpit: telemetry.Sink = Disabled (no --debug-trace flag)"},
		{Level: "DEBUG", Text: "[14:32:17] DEBUG - ssh.pool: MaxPerHost=3, ExecMetrics{acquires=0, busy=0, retries=0}"},
		{Level: "ERROR", Text: "[14:32:18] ERROR - Payment-UI: 502 backend timeout", Redacted: true},
	}
}

// MockGitHubLogsFetcher is the in-memory replacement for
// [GitHubLogsFetcher]. It returns a short, deterministic workflow log
// for the F8 modal so the operator can demo the surface end-to-end.
func MockGitHubLogsFetcher(_ context.Context, _ ghsvc.RepoRef, _ int64, maxLines int) ([]ghsvc.WorkflowLogLine, error) {
	source := []ghsvc.WorkflowLogLine{
		{StepName: "Git Checkout", Raw: "Cloning into 'shopease-web'..."},
		{StepName: "Git Checkout", Raw: "Repository cloned at 3fdc34d"},
		{StepName: "Install Deps", Raw: "npm ci --omit=dev"},
		{StepName: "Install Deps", Raw: "added 412 packages in 11s"},
		{StepName: "Code Lint", Raw: "eslint . --max-warnings=0"},
		{StepName: "Code Lint", Raw: "✓ No problems detected (linted 192 files)"},
		{StepName: "Build Artifact", Raw: "vite build --mode production"},
		{StepName: "Build Artifact", Raw: "dist/index.html  0.45 kB │ gzip:  0.27 kB"},
		{StepName: "Unit Tests", Raw: "vitest run --coverage"},
		{StepName: "Unit Tests", Raw: "Tests  68 passed, 0 failed (duration 18s)"},
		{StepName: "Deploy (Prod)", Raw: "rsync -avz --delete dist/ shop-ease.io:/var/www/"},
		{StepName: "Deploy (Prod)", Raw: "Deployment complete (uploaded 12 files)"},
	}
	if maxLines > 0 && len(source) > maxLines {
		source = source[len(source)-maxLines:]
	}
	return source, nil
}

// mockWizardRunner is the offline replacement for [DefaultWizardRunner].
// Every method short-circuits with a hard-coded payload so the mock
// cockpit never reaches the network. Mutating operations (restart,
// renew, log tail) report success after a 0-duration "sleep" so the
// progress affordances in the TUI still render.
type mockWizardRunner struct{}

func newMockWizardRunner() WizardRunner { return &mockWizardRunner{} }

// Preflight satisfies [WizardRunner] with an always-ready stub.
func (mockWizardRunner) Preflight(context.Context, config.Profile) (*providers.ProviderStatus, error) {
	return &providers.ProviderStatus{SSHConnected: true, CLIInstalled: true}, nil
}

// CheckDomainAvailable satisfies [WizardRunner]; the mock keeps every
// domain available so the create-wizard demo never gets stuck.
func (mockWizardRunner) CheckDomainAvailable(context.Context, config.Profile, string) error {
	return nil
}

// Execute satisfies [WizardRunner] by returning an empty provision
// report. No cleanup steps are pushed, so [Rollback] is a noop too.
func (mockWizardRunner) Execute(_ context.Context, _ config.Profile, _ wizard.ProvisionPlan, _ *wizard.Stack) (*wizard.ProvisionReport, error) {
	return &wizard.ProvisionReport{}, nil
}

// Rollback satisfies [WizardRunner]; the mock has nothing to roll back.
func (mockWizardRunner) Rollback(context.Context, config.Profile, *wizard.Stack) ([]wizard.CleanupResult, error) {
	return nil, nil
}

// RestartApp satisfies [WizardRunner]; the mock reports success
// without performing any I/O.
func (mockWizardRunner) RestartApp(context.Context, config.Profile, string) error { return nil }

// RenewSSL satisfies [WizardRunner]; the mock reports success
// without touching the provider.
func (mockWizardRunner) RenewSSL(context.Context, config.Profile, string) error { return nil }

// TailLog satisfies [WizardRunner] by returning a small canned log
// excerpt. The same lines are used by the dashboard tile so the modal
// flow stays consistent.
func (mockWizardRunner) TailLog(_ context.Context, _ config.Profile, _ string, _ int) ([]byte, error) {
	body := "[14:32:10] INFO - API-Gateway: Incoming GET /users (status: 200)\n" +
		"[14:32:11] WARN - Auth-Service: High latency detected (450ms)\n" +
		"[14:32:14] DEBUG - Worker #2: Cache hit for key 'prod:list'\n"
	return []byte(body), nil
}

// ListProviderSubdomains satisfies [WizardRunner]; the mock pretends
// the panel has no extra subdomains beyond the ones already in
// [MockConfig].
func (mockWizardRunner) ListProviderSubdomains(context.Context, config.Profile) ([]providers.Subdomain, error) {
	return nil, nil
}

// MockBento is exposed for callers that want to render the cockpit
// outside the TUI lifecycle (e.g. snapshot generators). It builds the
// tile list from a stand-alone snapshot without a Model.
func MockBento() []bento.BentoTile {
	cfg := MockConfig()
	statuses := MockProjectStatuses(cfg, mockNow())
	rows := make([]bento.ProjectRowSnapshot, 0, len(cfg.Projects))
	for idx, p := range cfg.Projects {
		state := "UNKNOWN"
		if st, ok := statuses[p.ID]; ok {
			state = string(st.State)
		}
		rows = append(rows, bento.ProjectRowSnapshot{
			Name:     p.Domain,
			State:    state,
			Selected: idx == 0,
		})
	}
	return []bento.BentoTile{
		bento.NewProjectsTile(rows),
	}
}
