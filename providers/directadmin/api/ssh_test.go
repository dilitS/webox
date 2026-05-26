package api

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
)

// fakeRunner records every command and returns scripted responses
// keyed by a substring match on the command. Substring matching
// keeps the test bodies short: the SSHFallback embeds the URL
// path inside the curl invocation, so matching on the path is
// enough to disambiguate between endpoints.
type fakeRunner struct {
	mu          sync.Mutex
	scripts     map[string]fakeResponse
	calls       []string
	defaultResp fakeResponse
}

type fakeResponse struct {
	stdout []byte
	stderr []byte
	exit   int
	err    error
}

func newFakeRunner() *fakeRunner {
	return &fakeRunner{scripts: map[string]fakeResponse{}}
}

func (f *fakeRunner) on(substr string, resp fakeResponse) *fakeRunner {
	f.scripts[substr] = resp
	return f
}

func (f *fakeRunner) setDefault(resp fakeResponse) *fakeRunner {
	f.defaultResp = resp
	return f
}

func (f *fakeRunner) Run(_ context.Context, command string) (stdout, stderr []byte, exitCode int, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, command)
	for sub, resp := range f.scripts {
		if strings.Contains(command, sub) {
			return resp.stdout, resp.stderr, resp.exit, resp.err
		}
	}
	return f.defaultResp.stdout, f.defaultResp.stderr, f.defaultResp.exit, f.defaultResp.err
}

func (f *fakeRunner) Calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.calls))
	copy(out, f.calls)
	return out
}

// withStatusCode wraps fixture body bytes with the trailing
// `\n<status>` curl produces via --write-out. The SSHFallback
// expects this suffix on every successful curl invocation.
func withStatusCode(body []byte, status int) []byte {
	out := make([]byte, 0, len(body)+5)
	out = append(out, body...)
	out = append(out, '\n')
	out = append(out, []byte{'0' + byte(status/100), '0' + byte((status/10)%10), '0' + byte(status%10)}...)
	return out
}

func TestNewSSHFallback_RejectsBadInputs(t *testing.T) {
	t.Parallel()
	runner := newFakeRunner()
	cases := []struct {
		name, user, key string
		runner          SSHRunner
		want            error
	}{
		{"nil-runner", "u", "k", nil, ErrSSHRunnerRequired},
		{"missing-user", "", "k", runner, ErrMissingCredentials},
		{"missing-key", "u", "", runner, ErrMissingCredentials},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewSSHFallback(tc.runner, tc.user, tc.key, 2222)
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestNewSSHFallback_AppliesDefaultPort(t *testing.T) {
	t.Parallel()
	f := newFakeRunner().setDefault(fakeResponse{
		stdout: withStatusCode(mustFixture(t, "whoami_ok.json"), 200),
	})
	s, err := NewSSHFallback(f, "alice", "KEY", 0)
	if err != nil {
		t.Fatalf("NewSSHFallback: %v", err)
	}
	if _, err := s.Whoami(context.Background()); err != nil {
		t.Fatalf("Whoami: %v", err)
	}
	if len(f.Calls()) != 1 {
		t.Fatalf("expected 1 call, got %d", len(f.Calls()))
	}
	if !strings.Contains(f.Calls()[0], "https://localhost:2222/api/whoami") {
		t.Fatalf("expected default port 2222 in URL, got: %s", f.Calls()[0])
	}
}

func TestSSHFallback_Whoami_HappyPath(t *testing.T) {
	t.Parallel()
	f := newFakeRunner().setDefault(fakeResponse{
		stdout: withStatusCode(mustFixture(t, "whoami_ok.json"), 200),
	})
	s, err := NewSSHFallback(f, "alice", "KEY", 2222)
	if err != nil {
		t.Fatalf("NewSSHFallback: %v", err)
	}
	got, err := s.Whoami(context.Background())
	if err != nil {
		t.Fatalf("Whoami: %v", err)
	}
	if got.Username != "alice" {
		t.Fatalf("unexpected: %+v", got)
	}
}

func TestSSHFallback_AllListMethods_RouteToCorrectEndpoint(t *testing.T) {
	t.Parallel()
	f := newFakeRunner().
		on("/api/users/alice/domains", fakeResponse{stdout: withStatusCode(mustFixture(t, "list_domains_ok.json"), 200)}).
		on("/api/users/alice/subdomains", fakeResponse{stdout: withStatusCode(mustFixture(t, "list_subdomains_ok.json"), 200)}).
		on("/api/users/alice/databases", fakeResponse{stdout: withStatusCode(mustFixture(t, "list_databases_ok.json"), 200)}).
		on("/api/users/alice/ssl/certificates", fakeResponse{stdout: withStatusCode(mustFixture(t, "list_ssl_certificates_ok.json"), 200)})

	s, err := NewSSHFallback(f, "alice", "KEY", 2222)
	if err != nil {
		t.Fatalf("NewSSHFallback: %v", err)
	}
	ctx := context.Background()

	if d, err := s.ListDomains(ctx); err != nil || len(d) != 3 {
		t.Fatalf("ListDomains: %v / %d", err, len(d))
	}
	if d, err := s.ListSubdomains(ctx); err != nil || len(d) != 2 {
		t.Fatalf("ListSubdomains: %v / %d", err, len(d))
	}
	if d, err := s.ListDatabases(ctx); err != nil || len(d) != 2 {
		t.Fatalf("ListDatabases: %v / %d", err, len(d))
	}
	if d, err := s.ListSSLCertificates(ctx); err != nil || len(d) != 2 {
		t.Fatalf("ListSSLCertificates: %v / %d", err, len(d))
	}
}

func TestSSHFallback_HTTPStatus404_MapsToAPIDisabled(t *testing.T) {
	t.Parallel()
	f := newFakeRunner().setDefault(fakeResponse{
		stdout: withStatusCode([]byte("Not Found"), 404),
	})
	s, _ := NewSSHFallback(f, "alice", "KEY", 2222)
	_, err := s.Whoami(context.Background())
	if !errors.Is(err, ErrAPIDisabled) {
		t.Fatalf("expected ErrAPIDisabled, got %v", err)
	}
}

func TestSSHFallback_HTTPStatus401_MapsToAuthFailed(t *testing.T) {
	t.Parallel()
	f := newFakeRunner().setDefault(fakeResponse{
		stdout: withStatusCode([]byte(`{"error":"invalid login key"}`), 401),
	})
	s, _ := NewSSHFallback(f, "alice", "KEY", 2222)
	_, err := s.Whoami(context.Background())
	if !errors.Is(err, ErrAuthenticationFailed) {
		t.Fatalf("expected ErrAuthenticationFailed, got %v", err)
	}
}

func TestSSHFallback_CurlNonZeroExit_MapsToTransportUnavailable(t *testing.T) {
	t.Parallel()
	f := newFakeRunner().setDefault(fakeResponse{
		stdout: nil,
		stderr: []byte("curl: (7) Failed to connect to localhost port 2222"),
		exit:   7,
	})
	s, _ := NewSSHFallback(f, "alice", "KEY", 2222)
	_, err := s.Whoami(context.Background())
	if !errors.Is(err, ErrTransportUnavailable) {
		t.Fatalf("expected ErrTransportUnavailable, got %v", err)
	}
}

func TestSSHFallback_TransportErrorBubblesUp(t *testing.T) {
	t.Parallel()
	wrapped := errors.New("ssh dial failed")
	f := newFakeRunner().setDefault(fakeResponse{
		err: wrapped,
	})
	s, _ := NewSSHFallback(f, "alice", "KEY", 2222)
	_, err := s.Whoami(context.Background())
	// We don't wrap the runner's error — it's the runner's job
	// to surface ErrTransportUnavailable (SSHPoolRunner does so).
	// Verify the error passes through unchanged.
	if !errors.Is(err, wrapped) {
		t.Fatalf("expected wrapped error to bubble up, got %v", err)
	}
}

func TestSSHFallback_ShellQuotingDefendsAgainstInjection(t *testing.T) {
	t.Parallel()
	// Construct a user with shell metacharacters. Sprint 23 only
	// sources these from typed credentials, but the defence-in-depth
	// quoting still protects future user-typed args (the cpanel/uapi
	// shellQuote table-driven tests cover the same threat model).
	f := newFakeRunner().setDefault(fakeResponse{
		stdout: withStatusCode(mustFixture(t, "whoami_ok.json"), 200),
	})
	s, err := NewSSHFallback(f, "alice; rm -rf /", "KEY`whoami`", 2222)
	if err != nil {
		t.Fatalf("NewSSHFallback: %v", err)
	}
	if _, err := s.Whoami(context.Background()); err != nil {
		t.Fatalf("Whoami: %v", err)
	}
	cmd := f.Calls()[0]
	// Verify the metacharacters are inside single quotes; the
	// `; rm -rf /` substring must be enclosed.
	if !strings.Contains(cmd, "'alice; rm -rf /:KEY`whoami`'") {
		t.Fatalf("metacharacters not single-quoted in command: %s", cmd)
	}
}

func TestSplitOnLastNewline_HappyPath(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in       string
		wantBody string
		wantCode string
		wantOK   bool
	}{
		{"body\n200", "body", "200", true},
		{`{"x":1}` + "\n200", `{"x":1}`, "200", true},
		{"multiline\nbody\n503", "multiline\nbody", "503", true},
		{"no newline", "", "", false},
		{"", "", "", false},
	}
	for _, tc := range cases {
		body, code, ok := splitOnLastNewline([]byte(tc.in))
		if ok != tc.wantOK {
			t.Errorf("%q: ok=%v, want %v", tc.in, ok, tc.wantOK)
			continue
		}
		if string(body) != tc.wantBody {
			t.Errorf("%q: body=%q, want %q", tc.in, body, tc.wantBody)
		}
		if code != tc.wantCode {
			t.Errorf("%q: code=%q, want %q", tc.in, code, tc.wantCode)
		}
	}
}

func TestShellQuote_HandlesEmbeddedSingleQuotes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"plain", "'plain'"},
		{"O'Brien", `'O'\''Brien'`},
		{"$(rm -rf /)", "'$(rm -rf /)'"},
		{"`whoami`", "'`whoami`'"},
		{"a; b", "'a; b'"},
		{`back\slash`, `'back\slash'`},
		{"", "''"},
	}
	for _, tc := range cases {
		got := shellQuote(tc.in)
		if got != tc.want {
			t.Errorf("shellQuote(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
