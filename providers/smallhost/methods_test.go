package smallhost_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dilitS/webox/providers"
	"github.com/dilitS/webox/providers/smallhost"
)

// methodFixture is a tiny convenience helper that loads a fixture
// file relative to testing/fixtures/devil and returns its bytes. The
// methods tests live in the smallhost_test package — they do not
// have access to the internal loadFixture used by parser tests.
func methodFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testing", "fixtures", "devil", name))
	if err != nil {
		t.Fatalf("load fixture %s: %v", name, err)
	}
	return data
}

// newProvider builds a smallhost.Provider with a fake executor
// installed and returns both for assertions.
func newProvider(t *testing.T) (*smallhost.Provider, *fakeExecutor) {
	t.Helper()
	provider, err := smallhost.New(validConfig())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	sp, ok := provider.(*smallhost.Provider)
	if !ok {
		t.Fatalf("New returned %T, want *smallhost.Provider", provider)
	}
	exec := newFakeExecutor()
	sp.SetExecutor(exec)
	return sp, exec
}

func TestCreateSubdomain_HappyPath(t *testing.T) {
	p, exec := newProvider(t)
	const domain = "app.webox-test.smallhost.pl"
	exec.On("devil www add "+domain+" nodejs 24", fakeResponse{
		stdout: methodFixture(t, "www_add_ok.txt"),
	})

	if err := p.CreateSubdomain(context.Background(), domain, "24"); err != nil {
		t.Fatalf("CreateSubdomain: %v", err)
	}
	if got := exec.Calls(); len(got) != 1 || got[0] != "devil www add "+domain+" nodejs 24" {
		t.Fatalf("calls = %v", got)
	}
}

func TestCreateSubdomain_RejectsBadDomain(t *testing.T) {
	p, exec := newProvider(t)
	err := p.CreateSubdomain(context.Background(), "evil$(rm).example.com", "24")
	if !errors.Is(err, providers.ErrInvalidProviderConfig) {
		t.Fatalf("err = %v, want ErrInvalidProviderConfig", err)
	}
	if len(exec.Calls()) != 0 {
		t.Errorf("calls = %v, want none (fail before exec)", exec.Calls())
	}
}

func TestCreateSubdomain_RejectsBadNodeVersion(t *testing.T) {
	p, exec := newProvider(t)
	err := p.CreateSubdomain(context.Background(), "app.example.com", "$(curl evil)")
	if !errors.Is(err, providers.ErrInvalidProviderConfig) {
		t.Fatalf("err = %v, want ErrInvalidProviderConfig", err)
	}
	if len(exec.Calls()) != 0 {
		t.Errorf("calls = %v, want none", exec.Calls())
	}
}

func TestCreateSubdomain_MapsParserSentinels(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		wantErr error
	}{
		{"exists", "www_add_exists.txt", providers.ErrSubdomainExists},
		{"invalid_node", "www_add_invalid_node.txt", providers.ErrNodeVersionUnsupported},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, exec := newProvider(t)
			exec.On("devil www add app.example.com nodejs 24", fakeResponse{
				stdout: methodFixture(t, tt.fixture),
			})
			err := p.CreateSubdomain(context.Background(), "app.example.com", "24")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestRestartNodeApp_Variants(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		wantErr error
	}{
		{"ok", "www_restart_ok.txt", nil},
		{"not_found", "www_restart_not_found.txt", providers.ErrAppNotFound},
		{"not_node", "www_restart_not_node.txt", providers.ErrAppNotNode},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, exec := newProvider(t)
			exec.On("devil www restart app.example.com", fakeResponse{
				stdout: methodFixture(t, tt.fixture),
			})
			err := p.RestartNodeApp(context.Background(), "app.example.com")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestListSubdomains(t *testing.T) {
	p, exec := newProvider(t)
	exec.On("devil www list", fakeResponse{
		stdout: methodFixture(t, "www_list_5.txt"),
	})
	got, err := p.ListSubdomains(context.Background())
	if err != nil {
		t.Fatalf("ListSubdomains: %v", err)
	}
	if len(got) != 5 {
		t.Errorf("len(got) = %d, want 5", len(got))
	}
}

func TestRemoveSubdomain_IdempotentOnNotFound(t *testing.T) {
	p, exec := newProvider(t)
	exec.On("devil www del app.example.com", fakeResponse{
		stdout: []byte("not found: app.example.com\n"),
	})
	if err := p.RemoveSubdomain(context.Background(), "app.example.com"); err != nil {
		t.Fatalf("RemoveSubdomain not idempotent: %v", err)
	}
}

func TestRemoveSubdomain_Success(t *testing.T) {
	p, exec := newProvider(t)
	exec.On("devil www del app.example.com", fakeResponse{
		stdout: []byte("Deleted app.example.com\n"),
	})
	if err := p.RemoveSubdomain(context.Background(), "app.example.com"); err != nil {
		t.Fatalf("RemoveSubdomain: %v", err)
	}
}

func TestSetupSSL_HappyPath(t *testing.T) {
	p, exec := newProvider(t)
	exec.On("devil vhost list", fakeResponse{
		stdout: methodFixture(t, "vhost_list.txt"),
	})
	exec.On("devil ssl www add 203.0.113.10 le le app.webox-test.smallhost.pl", fakeResponse{
		stdout: methodFixture(t, "ssl_add_ok.txt"),
	})

	if err := p.SetupSSL(context.Background(), "app.webox-test.smallhost.pl"); err != nil {
		t.Fatalf("SetupSSL: %v", err)
	}
	if got := exec.Calls(); len(got) != 2 {
		t.Fatalf("calls = %v, want 2", got)
	}
}

func TestSetupSSL_DNSNotReady(t *testing.T) {
	p, exec := newProvider(t)
	exec.On("devil vhost list", fakeResponse{stdout: methodFixture(t, "vhost_list.txt")})
	exec.On("devil ssl www add 203.0.113.10 le le app.example.com", fakeResponse{
		stdout: methodFixture(t, "ssl_add_dns_not_ready.txt"),
	})
	err := p.SetupSSL(context.Background(), "app.example.com")
	if !errors.Is(err, providers.ErrDNSNotResolving) {
		t.Fatalf("err = %v, want ErrDNSNotResolving", err)
	}
}

func TestCreateDatabase_HappyPath(t *testing.T) {
	p, exec := newProvider(t)
	exec.On("devil mysql add myapp_prod", fakeResponse{
		stdout: methodFixture(t, "mysql_add_ok.txt"),
	})
	user, pass, err := p.CreateDatabase(context.Background(), providers.DatabaseMySQL, "myapp_prod")
	if err != nil {
		t.Fatalf("CreateDatabase: %v", err)
	}
	if user != "myapp_prod" {
		t.Errorf("user = %q, want myapp_prod", user)
	}
	if !strings.HasPrefix(pass, "REDACTED-NEVER-A-REAL-SECRET-") {
		t.Errorf("password = %q, want tripwire prefix", pass)
	}
}

func TestCreateDatabase_RejectsBadName(t *testing.T) {
	p, exec := newProvider(t)
	_, _, err := p.CreateDatabase(context.Background(), providers.DatabaseMySQL, "Bad-Name")
	if !errors.Is(err, providers.ErrInvalidProviderConfig) {
		t.Fatalf("err = %v, want ErrInvalidProviderConfig", err)
	}
	if len(exec.Calls()) != 0 {
		t.Errorf("calls = %v, want none", exec.Calls())
	}
}

func TestCreateDatabase_RejectsUnknownKind(t *testing.T) {
	p, exec := newProvider(t)
	_, _, err := p.CreateDatabase(context.Background(), "sqlite", "myapp_prod")
	if !errors.Is(err, providers.ErrInvalidProviderConfig) {
		t.Fatalf("err = %v, want ErrInvalidProviderConfig", err)
	}
	if len(exec.Calls()) != 0 {
		t.Errorf("calls = %v, want none", exec.Calls())
	}
}

func TestCreateDatabase_PostgresUsesPgsqlAdd(t *testing.T) {
	p, exec := newProvider(t)
	exec.On("devil pgsql add myapp_prod", fakeResponse{
		stdout: methodFixture(t, "mysql_add_ok.txt"),
	})
	if _, _, err := p.CreateDatabase(context.Background(), providers.DatabasePostgres, "myapp_prod"); err != nil {
		t.Fatalf("CreateDatabase: %v", err)
	}
}

func TestRemoveDatabase_IdempotentOnNotFound(t *testing.T) {
	p, exec := newProvider(t)
	exec.On("devil mysql del myapp_prod", fakeResponse{
		stdout: methodFixture(t, "mysql_del_not_found.txt"),
	})
	if err := p.RemoveDatabase(context.Background(), providers.DatabaseMySQL, "myapp_prod"); err != nil {
		t.Fatalf("RemoveDatabase not idempotent: %v", err)
	}
}

func TestRemoveSSL_IdempotentOnNoCert(t *testing.T) {
	p, exec := newProvider(t)
	exec.On("devil vhost list", fakeResponse{stdout: methodFixture(t, "vhost_list.txt")})
	exec.On("devil ssl www del 203.0.113.10 app.example.com", fakeResponse{
		stdout: methodFixture(t, "ssl_del_no_cert.txt"),
	})
	if err := p.RemoveSSL(context.Background(), "app.example.com"); err != nil {
		t.Fatalf("RemoveSSL not idempotent: %v", err)
	}
}

func TestCheckStatus_HappyPath(t *testing.T) {
	p, exec := newProvider(t)
	exec.On("devil --version", fakeResponse{
		stdout:   []byte("devil 1.0.0\n"),
		exitCode: 0,
	})
	status, err := p.CheckStatus(context.Background())
	if err != nil {
		t.Fatalf("CheckStatus: %v", err)
	}
	if !status.SSHConnected {
		t.Error("SSHConnected = false, want true")
	}
	if !status.CLIInstalled {
		t.Error("CLIInstalled = false, want true")
	}
}

func TestCheckStatus_CLINotFound(t *testing.T) {
	p, exec := newProvider(t)
	exec.On("devil --version", fakeResponse{
		stderr:   []byte("bash: devil: command not found\n"),
		exitCode: 127,
	})
	_, err := p.CheckStatus(context.Background())
	if !errors.Is(err, providers.ErrCLINotFound) {
		t.Fatalf("err = %v, want ErrCLINotFound", err)
	}
}

// TestCheckStatus_LatencyUsesInjectedClock asserts that the
// per-instance clock seam ([Provider.SetClock]) drives the
// LatencyMS field. Two t0/t1 readings differ by 42 ms; we expect
// the report to surface that exact difference rather than a real
// wall-clock value.
func TestCheckStatus_LatencyUsesInjectedClock(t *testing.T) {
	t.Parallel()
	p, exec := newProvider(t)
	exec.On("devil --version", fakeResponse{stdout: []byte("devil 1.0.0\n")})

	base := time.Unix(1_700_000_000, 0).UTC()
	readings := []time.Time{base, base.Add(42 * time.Millisecond)}
	idx := 0
	p.SetClock(func() time.Time {
		ts := readings[idx%len(readings)]
		idx++
		return ts
	})

	status, err := p.CheckStatus(context.Background())
	if err != nil {
		t.Fatalf("CheckStatus: %v", err)
	}
	if status.LatencyMS != 42 {
		t.Fatalf("LatencyMS = %d, want 42 (injected clock delta)", status.LatencyMS)
	}
}

// TestCheckStatus_SetClockNilRestoresDefault asserts that passing
// nil to SetClock is the documented way to revert to time.Now —
// otherwise tests that share a provider across subtests would leak
// stub clocks.
func TestCheckStatus_SetClockNilRestoresDefault(t *testing.T) {
	t.Parallel()
	p, exec := newProvider(t)
	exec.On("devil --version", fakeResponse{stdout: []byte("devil 1.0.0\n")})

	p.SetClock(func() time.Time { return time.Unix(0, 0).UTC() })
	p.SetClock(nil)
	status, err := p.CheckStatus(context.Background())
	if err != nil {
		t.Fatalf("CheckStatus: %v", err)
	}
	if status.LatencyMS < 0 {
		t.Fatalf("LatencyMS = %d, want non-negative", status.LatencyMS)
	}
}

func TestMethods_FailClosedWithoutExecutor(t *testing.T) {
	provider, err := smallhost.New(validConfig())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	sp, ok := provider.(*smallhost.Provider)
	if !ok {
		t.Fatalf("New returned %T, want *smallhost.Provider", provider)
	}

	tests := []struct {
		name string
		run  func() error
	}{
		{"CreateSubdomain", func() error {
			return sp.CreateSubdomain(context.Background(), "app.example.com", "24")
		}},
		{"RestartNodeApp", func() error { return sp.RestartNodeApp(context.Background(), "app.example.com") }},
		{"ListSubdomains", func() error {
			_, err := sp.ListSubdomains(context.Background())
			return err
		}},
		{"SetupSSL", func() error { return sp.SetupSSL(context.Background(), "app.example.com") }},
		{"RemoveSubdomain", func() error { return sp.RemoveSubdomain(context.Background(), "app.example.com") }},
		{"CreateDatabase", func() error {
			_, _, err := sp.CreateDatabase(context.Background(), providers.DatabaseMySQL, "appdb")
			return err
		}},
		{"RemoveDatabase", func() error {
			return sp.RemoveDatabase(context.Background(), providers.DatabaseMySQL, "appdb")
		}},
		{"RemoveSSL", func() error { return sp.RemoveSSL(context.Background(), "app.example.com") }},
		{"CheckStatus", func() error {
			_, err := sp.CheckStatus(context.Background())
			return err
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run()
			if err == nil {
				t.Fatal("expected error without executor, got nil")
			}
			if !errors.Is(err, providers.ErrUnknownOutputFormat) {
				t.Fatalf("err = %v, want wrap of ErrUnknownOutputFormat", err)
			}
		})
	}
}

func TestTailLog_HappyPath(t *testing.T) {
	p, exec := newProvider(t)
	const domain = "app.webox-test.smallhost.pl"
	logPath := p.GetLogPath(domain)
	cmd := "tail -n 50 -- " + logPath + "/node.log " + logPath + "/error.log"
	exec.On(cmd, fakeResponse{stdout: []byte("[info] booted on :3000\n[info] request OK\n")})

	out, err := p.TailLog(context.Background(), domain, 50)
	if err != nil {
		t.Fatalf("TailLog: %v", err)
	}
	if want := "booted on :3000"; !strings.Contains(string(out), want) {
		t.Fatalf("output missing %q: %s", want, out)
	}
	if got := exec.Calls(); len(got) != 1 || got[0] != cmd {
		t.Fatalf("calls = %v, want [%q]", got, cmd)
	}
}

func TestTailLog_DefaultsAndClampsLineCount(t *testing.T) {
	tests := []struct {
		name      string
		lines     int
		wantInCmd string
	}{
		{"zero defaults to 200", 0, "tail -n 200 --"},
		{"negative defaults to 200", -5, "tail -n 200 --"},
		{"over cap clamps to 10000", 99999, "tail -n 10000 --"},
		{"within bounds passes through", 25, "tail -n 25 --"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			p, exec := newProvider(t)
			const domain = "app.webox-test.smallhost.pl"
			logPath := p.GetLogPath(domain)
			cmd := tt.wantInCmd + " " + logPath + "/node.log " + logPath + "/error.log"
			exec.On(cmd, fakeResponse{stdout: []byte("ok\n")})
			if _, err := p.TailLog(context.Background(), domain, tt.lines); err != nil {
				t.Fatalf("TailLog: %v", err)
			}
			if got := exec.Calls(); len(got) != 1 || got[0] != cmd {
				t.Fatalf("calls = %v, want %q", got, cmd)
			}
		})
	}
}

func TestTailLog_RejectsBadDomain(t *testing.T) {
	p, exec := newProvider(t)
	_, err := p.TailLog(context.Background(), "evil$(rm).example.com", 100)
	if !errors.Is(err, providers.ErrInvalidProviderConfig) {
		t.Fatalf("err = %v, want ErrInvalidProviderConfig", err)
	}
	if len(exec.Calls()) != 0 {
		t.Errorf("calls = %v, want none", exec.Calls())
	}
}

func TestTailLog_MissingFilesReturnsCombinedOutput(t *testing.T) {
	p, exec := newProvider(t)
	const domain = "app.webox-test.smallhost.pl"
	logPath := p.GetLogPath(domain)
	cmd := "tail -n 100 -- " + logPath + "/node.log " + logPath + "/error.log"
	exec.On(cmd, fakeResponse{
		stdout:   []byte("==> error.log <==\nstarted\n"),
		stderr:   []byte("tail: cannot open '" + logPath + "/node.log' for reading: No such file or directory\n"),
		exitCode: 1,
		err:      errors.New("Process exited with status 1"),
	})
	out, err := p.TailLog(context.Background(), domain, 100)
	if err != nil {
		t.Fatalf("TailLog should swallow exit 1 from tail: %v", err)
	}
	if !strings.Contains(string(out), "started") {
		t.Fatalf("missing stdout fragment: %s", out)
	}
	if !strings.Contains(string(out), "No such file or directory") {
		t.Fatalf("missing stderr fragment: %s", out)
	}
}

func TestValidateDBName(t *testing.T) {
	good := []string{"myapp", "myapp_prod", "x1"}
	for _, n := range good {
		if err := smallhost.ValidateDBName(n); err != nil {
			t.Errorf("ValidateDBName(%q) = %v, want nil", n, err)
		}
	}
	bad := []string{"", "MyApp", "my-app", "my app", strings.Repeat("a", 33), "$(curl)"}
	for _, n := range bad {
		if err := smallhost.ValidateDBName(n); err == nil {
			t.Errorf("ValidateDBName(%q) = nil, want error", n)
		}
	}
}

func TestValidateNodeVersion(t *testing.T) {
	good := []string{"24", "22.1.0", "v20", "lts"}
	for _, n := range good {
		if err := smallhost.ValidateNodeVersion(n); err != nil {
			t.Errorf("ValidateNodeVersion(%q) = %v, want nil", n, err)
		}
	}
	bad := []string{"", "$(rm)", " 24", "24;ls", strings.Repeat("9", 17)}
	for _, n := range bad {
		if err := smallhost.ValidateNodeVersion(n); err == nil {
			t.Errorf("ValidateNodeVersion(%q) = nil, want error", n)
		}
	}
}
