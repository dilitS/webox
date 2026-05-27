package uapi

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// SSHRunner is the seam between the SSH fallback and the actual
// SSH transport. Implementations execute a command and return raw
// stdout / stderr / exit code. The pool-backed production
// implementation lives in [SSHPoolRunner] under sshpool.go to
// keep this file dependency-free for unit tests.
//
// command must be a fully-shell-quoted invocation; SSHFallback
// constructs them via [shellQuote] so callers never have to worry
// about untrusted input escaping (Sprint 21 builds every command
// from typed Module / Function constants — there is no untrusted
// input in this code path).
//
// stdout / stderr / exitCode are returned verbatim from the
// underlying transport. err is non-nil only when the transport
// itself failed (dial / session / context-cancel); non-zero exits
// surface via exitCode while err == nil so the [SSHFallback] can
// inspect stderr to distinguish "module disabled" from a generic
// API error.
type SSHRunner interface {
	Run(ctx context.Context, command string) (stdout, stderr []byte, exitCode int, err error)
}

// SSHFallback is the read-only cPanel client driven over SSH. It
// shells out to `uapi --user=<user> --output=jsonpretty <Module>
// <function>` and parses the standard envelope, sharing every
// decoder with the HTTPS [Client]. The two clients are
// interchangeable on the read path; the composite layer chooses
// between them based on whether [ErrTransportUnavailable] surfaces
// from the HTTPS attempt.
type SSHFallback struct {
	runner SSHRunner
	user   string
}

// NewSSHFallback validates user (required for the `uapi --user`
// flag) and returns the typed fallback. The runner is whatever the
// caller chooses to inject; production wiring uses
// [SSHPoolRunner] which adapts the package-level [ssh.Pool].
func NewSSHFallback(runner SSHRunner, user string) (*SSHFallback, error) {
	if user == "" {
		return nil, ErrMissingCredentials
	}
	if runner == nil {
		return nil, ErrSSHRunnerRequired
	}
	return &SSHFallback{runner: runner, user: user}, nil
}

// ListDomains executes `uapi DomainInfo list_domains` over SSH.
func (s *SSHFallback) ListDomains(ctx context.Context) (*DomainInfoListResponse, error) {
	env, err := s.call(ctx, ModuleDomainInfo, FunctionDomainInfoList)
	if err != nil {
		return nil, err
	}
	out := &DomainInfoListResponse{}
	if uErr := json.Unmarshal(env.Result.Data, out); uErr != nil {
		return nil, fmt.Errorf("%w: DomainInfo.list_domains: %w", ErrMalformedResponse, uErr)
	}
	return out, nil
}

// ListPassengerApps executes `uapi PassengerApps list_applications`.
func (s *SSHFallback) ListPassengerApps(ctx context.Context) (*PassengerAppsListResponse, error) {
	env, err := s.call(ctx, ModulePassengerApps, FunctionPassengerAppsList)
	if err != nil {
		return nil, err
	}
	apps, dErr := decodePassengerApps(env.Result.Data)
	if dErr != nil {
		return nil, fmt.Errorf("%w: PassengerApps.list_applications: %w", ErrMalformedResponse, dErr)
	}
	return &PassengerAppsListResponse{Applications: apps}, nil
}

// ListMysqlDatabases executes `uapi Mysql list_databases`.
func (s *SSHFallback) ListMysqlDatabases(ctx context.Context) (*MysqlListDatabasesResponse, error) {
	env, err := s.call(ctx, ModuleMysql, FunctionMysqlListDatabases)
	if err != nil {
		return nil, err
	}
	dbs, dErr := decodeMysqlDatabases(env.Result.Data)
	if dErr != nil {
		return nil, fmt.Errorf("%w: Mysql.list_databases: %w", ErrMalformedResponse, dErr)
	}
	return &MysqlListDatabasesResponse{Databases: dbs}, nil
}

// ListSSLKeys executes `uapi SSL list_keys`.
func (s *SSHFallback) ListSSLKeys(ctx context.Context) (*SSLListKeysResponse, error) {
	env, err := s.call(ctx, ModuleSSL, FunctionSSLListKeys)
	if err != nil {
		return nil, err
	}
	keys, dErr := decodeSSLKeys(env.Result.Data)
	if dErr != nil {
		return nil, fmt.Errorf("%w: SSL.list_keys: %w", ErrMalformedResponse, dErr)
	}
	return &SSLListKeysResponse{Keys: keys}, nil
}

// Transport satisfies [Reader] and returns the constant "SSH".
// See [Client.Transport] for the rationale.
func (*SSHFallback) Transport() string { return "SSH" }

// call runs a UAPI invocation over SSH and decodes the standard
// envelope. exit-code 1 + "Sorry, the feature you are using is
// disabled" stderr is the canonical "module disabled" path; we map
// it to [ErrModuleFunctionDenied] just like the HTTPS client.
func (s *SSHFallback) call(ctx context.Context, module Module, function Function) (*envelope, error) {
	cmd := buildUAPICommand(s.user, module, function)
	stdout, stderr, code, err := s.runner.Run(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("uapi: ssh run: %w", err)
	}
	if code != 0 {
		if looksLikeModuleDenied(stderr) || looksLikeModuleDenied(stdout) {
			return nil, ErrModuleFunctionDenied
		}
		return nil, fmt.Errorf("%w: exit=%d stderr=%q", ErrAPIResultFailure, code, truncate(stderr))
	}
	env := &envelope{}
	if dErr := json.Unmarshal(stdout, env); dErr != nil {
		return nil, fmt.Errorf("%w: %w", ErrMalformedResponse, dErr)
	}
	if env.Result.Status != 1 {
		if isModuleDenied(env.Result.Errors) {
			return nil, ErrModuleFunctionDenied
		}
		return nil, fmt.Errorf("%w: status=%d errors=%v", ErrAPIResultFailure, env.Result.Status, env.Result.Errors)
	}
	return env, nil
}

// buildUAPICommand renders the exact shell command the SSH runner
// executes. We hard-code `--output=jsonpretty` so the envelope
// parser is identical to the HTTPS path. Every component (user,
// module, function) is shell-quoted; in Sprint 21 these are all
// typed constants supplied by the package itself, but the quoting
// is defence-in-depth for the day the args map becomes user-typed.
func buildUAPICommand(user string, module Module, function Function) string {
	return fmt.Sprintf("uapi --user=%s --output=jsonpretty %s %s",
		shellQuote(user), shellQuote(string(module)), shellQuote(string(function)))
}

// shellQuote wraps s in single quotes and escapes any embedded
// single quote via the `'\”` idiom. This is the same approach the
// `ssh` package uses for `devil` invocations on small.pl.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// looksLikeModuleDenied returns true when the byte slice contains
// the documented "feature disabled" phrasing. The SSH fallback
// sometimes surfaces these via stderr+exit=1 instead of an envelope
// with status=0, so we check both before falling through to the
// generic API-failure path.
func looksLikeModuleDenied(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	low := strings.ToLower(string(b))
	return strings.Contains(low, "disabled") ||
		strings.Contains(low, "no access") ||
		strings.Contains(low, "denied")
}

// truncate returns at most 240 chars from b for use in error
// messages. We cap because cPanel verbose-error stderr can run
// to hundreds of lines; logs care about the first line.
func truncate(b []byte) string {
	const limit = 240
	if len(b) <= limit {
		return string(b)
	}
	return string(b[:limit]) + "..."
}
