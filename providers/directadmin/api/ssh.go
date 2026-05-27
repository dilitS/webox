package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ErrSSHRunnerRequired is returned when [NewSSHFallback] is called
// with a nil runner. Distinguished from [ErrMissingCredentials]
// because the remediation is different: missing-runner means a
// wiring bug (caller forgot to inject the runner); missing-creds
// means the operator's profile is incomplete.
var ErrSSHRunnerRequired = errors.New("directadmin/api: SSH runner required")

// SSHRunner is the seam between the SSH fallback and the actual
// SSH transport. Implementations execute a command and return
// raw stdout / stderr / exit code. The pool-backed production
// implementation lives in [SSHPoolRunner] (sshpool.go) to keep
// this file dependency-free for unit tests.
//
// command must be a fully-shell-quoted invocation; SSHFallback
// constructs them via [shellQuote] so callers never have to worry
// about untrusted input escaping (Sprint 23 builds every command
// from typed endpoints — there is no untrusted input).
//
// Non-zero exit codes surface via exitCode while err == nil so
// the SSHFallback can inspect stderr to distinguish "API
// disabled" (ErrAPIDisabled) from a generic curl failure.
type SSHRunner interface {
	Run(ctx context.Context, command string) (stdout, stderr []byte, exitCode int, err error)
}

// SSHFallback is the read-only DirectAdmin client driven over
// SSH. Unlike cpanel (which has the first-class `uapi` CLI),
// DirectAdmin's standard SSH integration is to call the Live API
// via loopback HTTPS from inside the box:
//
//	curl -sk --user <user>:<loginkey> https://localhost:2222/api/...
//
// This is useful when the operator's machine can SSH to the box
// but can't reach :2222 directly (restrictive firewalls, NAT,
// IP allow-lists). The fallback handles every transport-level
// failure of the direct HTTPS client.
//
// The fallback does NOT cover the case where /api/* is disabled
// at the panel level — both transports would fail with the same
// ErrAPIDisabled (the loopback call hits the same endpoint with
// the same feature flag). That's a configuration problem, not a
// transport problem; the doctor CLI surfaces a remediation note
// instead of retrying.
type SSHFallback struct {
	runner   SSHRunner
	user     string
	loginKey string
	apiPort  int
}

// Compile-time assertion that SSHFallback satisfies Reader. Will
// refuse to build if Reader gains a method without a matching
// SSHFallback implementation.
var _ Reader = (*SSHFallback)(nil)

// NewSSHFallback validates inputs and returns the fallback.
// apiPort defaults to 2222 (DA standard) when zero is supplied.
func NewSSHFallback(runner SSHRunner, user, loginKey string, apiPort int) (*SSHFallback, error) {
	if runner == nil {
		return nil, ErrSSHRunnerRequired
	}
	if user == "" || loginKey == "" {
		return nil, ErrMissingCredentials
	}
	if apiPort == 0 {
		apiPort = 2222
	}
	return &SSHFallback{runner: runner, user: user, loginKey: loginKey, apiPort: apiPort}, nil
}

// Whoami invokes /api/whoami via loopback curl.
func (s *SSHFallback) Whoami(ctx context.Context) (*WhoamiResponse, error) {
	body, err := s.fetch(ctx, "whoami")
	if err != nil {
		return nil, err
	}
	out := &WhoamiResponse{}
	if jErr := json.Unmarshal(body, out); jErr != nil {
		return nil, fmt.Errorf("%w: whoami: %w", ErrMalformedResponse, jErr)
	}
	return out, nil
}

// ListDomains invokes /api/users/<user>/domains.
func (s *SSHFallback) ListDomains(ctx context.Context) ([]Domain, error) {
	body, err := s.fetch(ctx, "users/"+s.user+"/domains")
	if err != nil {
		return nil, err
	}
	out, dErr := decodeList[Domain](body, "domains")
	if dErr != nil {
		return nil, fmt.Errorf("%w: list_domains: %w", ErrMalformedResponse, dErr)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// ListSubdomains invokes /api/users/<user>/subdomains.
func (s *SSHFallback) ListSubdomains(ctx context.Context) ([]Subdomain, error) {
	body, err := s.fetch(ctx, "users/"+s.user+"/subdomains")
	if err != nil {
		return nil, err
	}
	out, dErr := decodeList[Subdomain](body, "subdomains")
	if dErr != nil {
		return nil, fmt.Errorf("%w: list_subdomains: %w", ErrMalformedResponse, dErr)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// ListDatabases invokes /api/users/<user>/databases.
func (s *SSHFallback) ListDatabases(ctx context.Context) ([]Database, error) {
	body, err := s.fetch(ctx, "users/"+s.user+"/databases")
	if err != nil {
		return nil, err
	}
	out, dErr := decodeList[Database](body, "databases")
	if dErr != nil {
		return nil, fmt.Errorf("%w: list_databases: %w", ErrMalformedResponse, dErr)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// ListSSLCertificates invokes /api/users/<user>/ssl/certificates.
func (s *SSHFallback) ListSSLCertificates(ctx context.Context) ([]SSLCertificate, error) {
	body, err := s.fetch(ctx, "users/"+s.user+"/ssl/certificates")
	if err != nil {
		return nil, err
	}
	out, dErr := decodeList[SSLCertificate](body, "certificates")
	if dErr != nil {
		return nil, fmt.Errorf("%w: list_ssl_certificates: %w", ErrMalformedResponse, dErr)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Domain < out[j].Domain })
	return out, nil
}

// Transport satisfies [Reader] and returns the constant "SSH".
// See [Client.Transport] for the rationale.
func (*SSHFallback) Transport() string { return "SSH" }

// fetch builds a loopback curl invocation, runs it via the SSH
// runner, and classifies the result. The command shape:
//
//	curl -sk --max-time 30 \
//	     --user <user>:<key> \
//	     --write-out '\n%{http_code}' \
//	     https://localhost:<port>/api/<path>
//
// The trailing `--write-out '\n%{http_code}'` appends the HTTP
// status to stdout so we can classify the response identically
// to the HTTPS client even though curl's exit code only
// distinguishes "request completed" from "transport-level
// failure".
//
// All shell arguments flow through [shellQuote] so a malicious
// user/loginKey (Properties-supplied) cannot inject a second
// command — defence-in-depth even though Sprint 23 only sources
// these from the operator's typed credentials, never from
// untrusted input.
func (s *SSHFallback) fetch(ctx context.Context, path string) ([]byte, error) {
	url := fmt.Sprintf("https://localhost:%d/api/%s", s.apiPort, path)
	cmd := strings.Join([]string{
		"curl", "-sk",
		"--max-time", "30",
		"--user", shellQuote(s.user + ":" + s.loginKey),
		"--write-out", shellQuote(`\n%{http_code}`),
		shellQuote(url),
	}, " ")

	stdout, stderr, exitCode, err := s.runner.Run(ctx, cmd)
	if err != nil {
		return nil, err // already wrapped with ErrTransportUnavailable by the pool runner.
	}
	if exitCode != 0 {
		// curl couldn't even complete the request — typically
		// means DNS or TCP failure on the remote box. Map to
		// transport-unavailable so the composite layer fails
		// over correctly.
		return nil, fmt.Errorf("%w: curl exit %d: %s",
			ErrTransportUnavailable, exitCode, strings.TrimSpace(string(stderr)))
	}

	// Strip the trailing `\n<http-code>` curl appended via
	// --write-out. The body is everything before the last
	// newline; the status follows.
	bodyBytes, statusStr, ok := splitOnLastNewline(stdout)
	if !ok {
		return nil, fmt.Errorf("%w: curl output did not contain http_code suffix", ErrMalformedResponse)
	}
	status := 0
	if _, sErr := fmt.Sscanf(statusStr, "%d", &status); sErr != nil {
		return nil, fmt.Errorf("%w: parse http_code %q: %w", ErrMalformedResponse, statusStr, sErr)
	}
	if cErr := classifyHTTPStatus(status, bodyBytes); cErr != nil {
		return nil, cErr
	}
	return bodyBytes, nil
}

// splitOnLastNewline returns the bytes before the last newline
// and the string content after. Used to peel the HTTP status
// curl appended via --write-out. Returns ok=false if there is
// no newline in the input (impossible in practice because curl
// always inserts the literal `\n` we requested).
func splitOnLastNewline(b []byte) (body []byte, statusStr string, ok bool) {
	idx := -1
	for i := len(b) - 1; i >= 0; i-- {
		if b[i] == '\n' {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil, "", false
	}
	return b[:idx], strings.TrimSpace(string(b[idx+1:])), true
}

// shellQuote wraps s in single quotes with embedded single
// quotes escaped via the `'\”` trick. Identical to the cpanel
// helper because shell quoting rules don't change per provider.
// Defence-in-depth: callers in this package only pass typed
// constants, but the quoting protects future user-typed args.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
