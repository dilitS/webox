package wizard_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/dilitS/webox/providers"
	"github.com/dilitS/webox/wizard"
)

func TestExecuteHappyPathPushesAllCleanups(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pendingPath := filepath.Join(dir, "pending.json")
	fake := newFakeProvider()
	stack := wizard.NewStack(wizard.NewFilePersister(pendingPath, "wizard-1"), "wizard-1")

	plan := wizard.ProvisionPlan{
		ProfileAlias: "main",
		Stack:        wizard.StackNodeExpress,
		Domain:       "app.demo.smallhost.pl",
		NodeVersion:  "22",
		DBKind:       providers.DatabaseMySQL,
		DBName:       "app_main",
	}

	report, err := wizard.Execute(context.Background(), fake, plan, stack)
	if err != nil {
		t.Fatalf("Execute = %v", err)
	}
	if report == nil || !report.Subdomain.OK || !report.SSL.OK || !report.Database.OK {
		t.Fatalf("report = %+v, want all OK", report)
	}
	if report.Credentials == nil || report.Credentials.Password == "" {
		t.Fatal("credentials missing on db success")
	}
	if stack.Len() != 3 {
		t.Fatalf("stack len = %d, want 3", stack.Len())
	}

	loaded, err := wizard.LoadPending(pendingPath)
	if err != nil {
		t.Fatalf("LoadPending = %v", err)
	}
	if loaded == nil || len(loaded.Steps) != 3 {
		t.Fatalf("loaded snapshot = %+v", loaded)
	}
	if loaded.Steps[0].Kind != wizard.ResourceSubdomain ||
		loaded.Steps[1].Kind != wizard.ResourceSSL ||
		loaded.Steps[2].Kind != wizard.ResourceDatabase {
		t.Fatalf("step order = %+v", loaded.Steps)
	}
}

func TestExecuteSkipsDBWhenPlanHasNoneDB(t *testing.T) {
	t.Parallel()
	fake := newFakeProvider()
	stack := wizard.NewStack(nil, "")
	plan := wizard.ProvisionPlan{
		ProfileAlias: "main",
		Stack:        wizard.StackStatic,
		Domain:       "static.demo.smallhost.pl",
		NodeVersion:  "22",
	}
	report, err := wizard.Execute(context.Background(), fake, plan, stack)
	if err != nil {
		t.Fatalf("Execute = %v", err)
	}
	if report.Credentials != nil {
		t.Fatalf("credentials = %+v, want nil for db-less plan", report.Credentials)
	}
	if stack.Len() != 2 {
		t.Fatalf("stack len = %d, want 2 (sub + ssl)", stack.Len())
	}
	for _, c := range fake.Calls() {
		if c == "db:" || len(c) > 3 && c[:3] == "db:" {
			t.Fatalf("provider should not have been asked for db, got call %q", c)
		}
	}
}

func TestExecuteStopsOnSubdomainFailure(t *testing.T) {
	t.Parallel()
	fake := newFakeProvider()
	fake.createSubdomain = []error{providers.ErrSubdomainExists}
	stack := wizard.NewStack(nil, "")
	plan := wizard.ProvisionPlan{
		ProfileAlias: "main",
		Stack:        wizard.StackNodeExpress,
		Domain:       "exists.demo.smallhost.pl",
		NodeVersion:  "22",
		DBKind:       providers.DatabaseMySQL,
		DBName:       "x",
	}
	report, err := wizard.Execute(context.Background(), fake, plan, stack)
	if err == nil {
		t.Fatal("Execute should fail when subdomain exists")
	}
	var execErr *wizard.ExecutionFailedError
	if !errors.As(err, &execErr) || execErr.FailedStep != "subdomain" {
		t.Fatalf("err = %v, want ExecutionFailedError step=subdomain", err)
	}
	if !errors.Is(err, providers.ErrSubdomainExists) {
		t.Fatalf("errors.Is(ErrSubdomainExists) = false, err = %v", err)
	}
	if stack.Len() != 0 {
		t.Fatalf("stack len = %d, want 0 (subdomain failed)", stack.Len())
	}
	if report.Subdomain.OK {
		t.Fatal("subdomain reported OK")
	}
}

func TestExecuteStopsOnSSLFailureWithSubdomainPushed(t *testing.T) {
	t.Parallel()
	fake := newFakeProvider()
	fake.setupSSL = []error{providers.ErrDNSNotResolving}
	stack := wizard.NewStack(nil, "")
	plan := wizard.ProvisionPlan{
		ProfileAlias: "main",
		Stack:        wizard.StackStatic,
		Domain:       "fresh.demo.smallhost.pl",
		NodeVersion:  "22",
	}
	report, err := wizard.Execute(context.Background(), fake, plan, stack)
	if err == nil {
		t.Fatal("Execute should fail on SSL")
	}
	if !errors.Is(err, providers.ErrDNSNotResolving) {
		t.Fatalf("errors.Is(ErrDNSNotResolving) = false, err = %v", err)
	}
	if !report.Subdomain.OK || report.SSL.OK {
		t.Fatalf("report = %+v", report)
	}
	if stack.Len() != 1 || stack.Steps()[0].Kind != wizard.ResourceSubdomain {
		t.Fatalf("stack = %+v, want only subdomain", stack.Steps())
	}
}

func TestExecuteStopsOnDBFailure(t *testing.T) {
	t.Parallel()
	fake := newFakeProvider()
	fake.createDatabase = []dbResult{{err: providers.ErrDBNameTaken}}
	stack := wizard.NewStack(nil, "")
	plan := wizard.ProvisionPlan{
		ProfileAlias: "main",
		Stack:        wizard.StackNodeExpress,
		Domain:       "db.demo.smallhost.pl",
		NodeVersion:  "22",
		DBKind:       providers.DatabaseMySQL,
		DBName:       "taken",
	}
	report, err := wizard.Execute(context.Background(), fake, plan, stack)
	if err == nil {
		t.Fatal("Execute should fail on DB")
	}
	if !errors.Is(err, providers.ErrDBNameTaken) {
		t.Fatalf("errors.Is(ErrDBNameTaken) = false, err = %v", err)
	}
	if report.Subdomain.OK == false || report.SSL.OK == false {
		t.Fatalf("report = %+v", report)
	}
	if stack.Len() != 2 {
		t.Fatalf("stack len = %d, want 2 (sub + ssl)", stack.Len())
	}
}

func TestExecuteValidatesPlanBeforeAnyCall(t *testing.T) {
	t.Parallel()
	fake := newFakeProvider()
	stack := wizard.NewStack(nil, "")
	_, err := wizard.Execute(context.Background(), fake, wizard.ProvisionPlan{Stack: "unknown"}, stack)
	if !errors.Is(err, wizard.ErrInvalidPlan) {
		t.Fatalf("err = %v, want ErrInvalidPlan", err)
	}
	if len(fake.Calls()) != 0 {
		t.Fatalf("provider called despite invalid plan, calls = %+v", fake.Calls())
	}
}

func TestExecuteRollbackRoundtripIsIdempotent(t *testing.T) {
	t.Parallel()

	fake := newFakeProvider()
	fake.setupSSL = []error{providers.ErrDNSNotResolving}
	stack := wizard.NewStack(nil, "")
	plan := wizard.ProvisionPlan{
		ProfileAlias: "main",
		Stack:        wizard.StackStatic,
		Domain:       "x.demo.smallhost.pl",
		NodeVersion:  "22",
	}
	if _, err := wizard.Execute(context.Background(), fake, plan, stack); err == nil {
		t.Fatal("expected ssl failure")
	}
	if _, err := stack.Rollback(context.Background(), wizard.MakeStepRunner(fake)); err != nil {
		t.Fatalf("first rollback = %v", err)
	}
	if _, err := stack.Rollback(context.Background(), wizard.MakeStepRunner(fake)); err != nil {
		t.Fatalf("second rollback (idempotent) = %v", err)
	}
}

func TestExecuteRollbackFailureSurfacesAggregated(t *testing.T) {
	t.Parallel()

	fake := newFakeProvider()
	fake.setupSSL = []error{providers.ErrRateLimitLetsEncrypt}
	fake.removeSubdomain = []error{errors.New("panel disconnected")}
	stack := wizard.NewStack(nil, "")
	plan := wizard.ProvisionPlan{
		ProfileAlias: "main",
		Stack:        wizard.StackStatic,
		Domain:       "x.demo.smallhost.pl",
		NodeVersion:  "22",
	}
	if _, err := wizard.Execute(context.Background(), fake, plan, stack); err == nil {
		t.Fatal("ssl failure expected")
	}
	results, err := stack.Rollback(context.Background(), wizard.MakeStepRunner(fake))
	if err == nil {
		t.Fatal("rollback should fail because remove subdomain errors")
	}
	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}
}

func TestPreflightHappyPath(t *testing.T) {
	t.Parallel()
	fake := newFakeProvider()
	status, err := wizard.Preflight(context.Background(), fake)
	if err != nil {
		t.Fatalf("Preflight = %v", err)
	}
	if status == nil || !status.SSHConnected || !status.CLIInstalled {
		t.Fatalf("status = %+v", status)
	}
}

func TestPreflightCLIMissing(t *testing.T) {
	t.Parallel()
	fake := newFakeProvider()
	fake.checkStatus = []checkStatusResult{{
		status: &providers.ProviderStatus{SSHConnected: true, CLIInstalled: false},
		err:    providers.ErrCLINotFound,
	}}
	_, err := wizard.Preflight(context.Background(), fake)
	if !errors.Is(err, providers.ErrCLINotFound) {
		t.Fatalf("err = %v, want ErrCLINotFound", err)
	}
}

func TestPreflightNilProvider(t *testing.T) {
	t.Parallel()
	_, err := wizard.Preflight(context.Background(), nil)
	if !errors.Is(err, wizard.ErrInvalidPlan) {
		t.Fatalf("err = %v, want ErrInvalidPlan", err)
	}
}

func TestCheckSubdomainAvailableDetectsCollision(t *testing.T) {
	t.Parallel()
	fake := newFakeProvider()
	fake.listSubdomains = [][]providers.Subdomain{{
		{Domain: "app.demo.smallhost.pl", Type: "nodejs", NodeVersion: "22"},
	}}
	err := wizard.CheckSubdomainAvailable(context.Background(), fake, "app.demo.smallhost.pl")
	if !errors.Is(err, providers.ErrSubdomainExists) {
		t.Fatalf("err = %v, want ErrSubdomainExists", err)
	}
}

func TestCheckSubdomainAvailableAccepts(t *testing.T) {
	t.Parallel()
	fake := newFakeProvider()
	fake.listSubdomains = [][]providers.Subdomain{{
		{Domain: "other.demo.smallhost.pl", Type: "nodejs", NodeVersion: "22"},
	}}
	if err := wizard.CheckSubdomainAvailable(context.Background(), fake, "app.demo.smallhost.pl"); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
}

func TestIsRecoverable(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{"subdomain exists", providers.ErrSubdomainExists, true},
		{"dns", providers.ErrDNSNotResolving, true},
		{"rate limit", providers.ErrRateLimitLetsEncrypt, true},
		{"db taken", providers.ErrDBNameTaken, true},
		{"node version", providers.ErrNodeVersionUnsupported, true},
		{"app not found", providers.ErrAppNotFound, false},
		{"nil", nil, false},
		{"context cancel", context.Canceled, false},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := wizard.IsRecoverable(tc.err); got != tc.want {
				t.Fatalf("IsRecoverable(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
