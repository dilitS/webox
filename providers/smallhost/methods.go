package smallhost

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/dilitS/webox/providers"
	wssh "github.com/dilitS/webox/ssh"
)

// Command tokens are bound at compile time so a shell-injected
// `domain` cannot reach the panel through string formatting. The
// adapter builds commands by concatenating these tokens with
// already-validated identifiers (domain via [ValidateDomain], dbName
// via [ValidateDBName]).
//
// We deliberately keep one constant per token rather than building a
// builder helper: every command path appears verbatim in a grep,
// which makes code review trivial and prevents accidental option
// drift between commands.
const (
	cmdDevil        = "devil"
	cmdWwwAdd       = "www add"
	cmdWwwDel       = "www del"
	cmdWwwList      = "www list"
	cmdWwwRestart   = "www restart"
	cmdSSLWwwAdd    = "ssl www add"
	cmdSSLWwwDel    = "ssl www del"
	cmdVhostList    = "vhost list"
	cmdMysqlAdd     = "mysql add"
	cmdMysqlDel     = "mysql del"
	cmdPgsqlAdd     = "pgsql add"
	cmdPgsqlDel     = "pgsql del"
	cmdDevilVersion = "devil --version"
	tokenNodeJS     = "nodejs"
	tokenLE         = "le"
)

// exitCLINotFound is the conventional exit status used by shells when
// the requested binary is missing from PATH. The status loop maps it
// to providers.ErrCLINotFound so the doctor flow can surface an
// actionable diagnostic.
const exitCLINotFound = 127

// nowFn is the seam used by [Provider.CheckStatus] to measure
// latency. Tests substitute a stub clock to make the latency field
// deterministic.
var nowFn = time.Now

// dbNamePattern guards database / table identifiers before they
// reach `devil mysql add` or `devil pgsql add`. SQL identifier
// constraints across MySQL and PostgreSQL converge on
// `[A-Za-z0-9_]+` with a length cap; we are stricter (lowercase
// only) to keep deterministic round-tripping through `config.json`.
var dbNamePattern = regexp.MustCompile(`^[a-z0-9_]{1,32}$`)

// nodeVersionPattern accepts the majors small.pl currently exposes
// without hardcoding the supported list — the panel is the
// authority. We refuse anything that looks like a shell flag or
// pathological string so the command builder stays safe.
var nodeVersionPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,16}$`)

// ErrInvalidDBName is the sentinel returned by ValidateDBName when the
// input fails the defensive check.
var ErrInvalidDBName = errors.New("smallhost: invalid database name")

// ErrInvalidNodeVersion is the sentinel returned by ValidateNodeVersion
// when the input fails the defensive check.
var ErrInvalidNodeVersion = errors.New("smallhost: invalid node version")

// ValidateDBName rejects DB names that would be unsafe to substitute
// into a panel command. Wraps [ErrInvalidDBName].
func ValidateDBName(name string) error {
	if name == "" {
		return fmt.Errorf("%w: empty", ErrInvalidDBName)
	}
	if !dbNamePattern.MatchString(name) {
		return fmt.Errorf("%w: %q does not match %s", ErrInvalidDBName, name, dbNamePattern.String())
	}
	return nil
}

// ValidateNodeVersion rejects node versions that would be unsafe to
// substitute into `devil www add`. Wraps [ErrInvalidNodeVersion].
func ValidateNodeVersion(version string) error {
	if version == "" {
		return fmt.Errorf("%w: empty", ErrInvalidNodeVersion)
	}
	if !nodeVersionPattern.MatchString(version) {
		return fmt.Errorf("%w: %q has unexpected characters", ErrInvalidNodeVersion, version)
	}
	return nil
}

// CreateSubdomain provisions a Node.js subdomain via `devil www add`.
func (p *Provider) CreateSubdomain(ctx context.Context, domain, nodeVersion string) error {
	if err := ValidateDomain(domain); err != nil {
		return fmt.Errorf("%w: %w", providers.ErrInvalidProviderConfig, err)
	}
	if err := ValidateNodeVersion(nodeVersion); err != nil {
		return fmt.Errorf("%w: %w", providers.ErrInvalidProviderConfig, err)
	}
	cmd := strings.Join([]string{cmdDevil, cmdWwwAdd, domain, tokenNodeJS, nodeVersion}, " ")
	out, err := p.exec(ctx, cmd)
	if err != nil {
		return err
	}
	if _, parseErr := parseWwwAdd(combine(out)); parseErr != nil {
		return parseErr
	}
	return nil
}

// RestartNodeApp restarts the Node.js app bound to domain via
// `devil www restart`.
func (p *Provider) RestartNodeApp(ctx context.Context, domain string) error {
	if err := ValidateDomain(domain); err != nil {
		return fmt.Errorf("%w: %w", providers.ErrInvalidProviderConfig, err)
	}
	cmd := strings.Join([]string{cmdDevil, cmdWwwRestart, domain}, " ")
	out, err := p.exec(ctx, cmd)
	if err != nil {
		return err
	}
	return parseWwwRestart(combine(out))
}

// ListSubdomains enumerates the subdomains currently provisioned on
// the account.
func (p *Provider) ListSubdomains(ctx context.Context) ([]providers.Subdomain, error) {
	cmd := cmdDevil + " " + cmdWwwList
	out, err := p.exec(ctx, cmd)
	if err != nil {
		return nil, err
	}
	return parseWwwList(combine(out))
}

// RemoveSubdomain deletes the subdomain via `devil www del`.
// Idempotent: "not found" maps to nil so LIFO rollback survives
// partial-success replays.
func (p *Provider) RemoveSubdomain(ctx context.Context, domain string) error {
	if err := ValidateDomain(domain); err != nil {
		return fmt.Errorf("%w: %w", providers.ErrInvalidProviderConfig, err)
	}
	cmd := strings.Join([]string{cmdDevil, cmdWwwDel, domain}, " ")
	out, err := p.exec(ctx, cmd)
	if err != nil {
		return err
	}
	return parseDeleteOutcome(combine(out), "Deleted")
}

// SetupSSL provisions Let's Encrypt for domain. Resolves the account
// IP via `devil vhost list` first so the adapter does not need a
// configured IP per profile.
func (p *Provider) SetupSSL(ctx context.Context, domain string) error {
	if err := ValidateDomain(domain); err != nil {
		return fmt.Errorf("%w: %w", providers.ErrInvalidProviderConfig, err)
	}
	ip, err := p.accountIP(ctx)
	if err != nil {
		return err
	}
	cmd := strings.Join([]string{cmdDevil, cmdSSLWwwAdd, ip, tokenLE, tokenLE, domain}, " ")
	out, err := p.exec(ctx, cmd)
	if err != nil {
		return err
	}
	return parseSSLAdd(combine(out))
}

// RemoveSSL removes the SSL certificate for domain. Idempotent on
// not-found / no-cert.
func (p *Provider) RemoveSSL(ctx context.Context, domain string) error {
	if err := ValidateDomain(domain); err != nil {
		return fmt.Errorf("%w: %w", providers.ErrInvalidProviderConfig, err)
	}
	ip, err := p.accountIP(ctx)
	if err != nil {
		return err
	}
	cmd := strings.Join([]string{cmdDevil, cmdSSLWwwDel, ip, domain}, " ")
	out, err := p.exec(ctx, cmd)
	if err != nil {
		return err
	}
	return parseSSLDelete(combine(out))
}

// CreateDatabase provisions a MySQL or PostgreSQL database. Returns
// the panel-generated username and password — callers MUST move
// password into a memguard.LockedBuffer immediately and zero the
// returned string before any logging or persistence.
func (p *Provider) CreateDatabase(ctx context.Context, dbType, dbName string) (user, password string, err error) {
	if err := ValidateDBName(dbName); err != nil {
		return "", "", fmt.Errorf("%w: %w", providers.ErrInvalidProviderConfig, err)
	}
	subCmd, err := dbAddSubcommand(dbType)
	if err != nil {
		return "", "", err
	}
	cmd := strings.Join([]string{cmdDevil, subCmd, dbName}, " ")
	out, execErr := p.exec(ctx, cmd)
	if execErr != nil {
		return "", "", execErr
	}
	res, parseErr := parseDBAdd(combine(out))
	if parseErr != nil {
		return "", "", parseErr
	}
	return res.User, res.Password, nil
}

// RemoveDatabase removes a MySQL or PostgreSQL database. Idempotent
// on not-found.
func (p *Provider) RemoveDatabase(ctx context.Context, dbType, dbName string) error {
	if err := ValidateDBName(dbName); err != nil {
		return fmt.Errorf("%w: %w", providers.ErrInvalidProviderConfig, err)
	}
	subCmd, err := dbDeleteSubcommand(dbType)
	if err != nil {
		return err
	}
	cmd := strings.Join([]string{cmdDevil, subCmd, dbName}, " ")
	out, execErr := p.exec(ctx, cmd)
	if execErr != nil {
		return execErr
	}
	return parseDBDelete(combine(out))
}

// CheckStatus runs the cheap probe required by the dashboard's
// health badge: it confirms the panel CLI exists and reports its
// version. ErrCLINotFound is returned when the remote shell reports
// exit 127 ("command not found").
func (p *Provider) CheckStatus(ctx context.Context) (*providers.ProviderStatus, error) {
	start := nowFn()
	out, err := p.exec(ctx, cmdDevilVersion)
	elapsed := nowFn().Sub(start)
	latencyMS := int(elapsed / time.Millisecond)
	if err != nil {
		return &providers.ProviderStatus{
			SSHConnected: false,
			CLIInstalled: false,
			LatencyMS:    latencyMS,
		}, err
	}
	status := &providers.ProviderStatus{
		SSHConnected: true,
		CLIInstalled: out.ExitCode == 0,
		LatencyMS:    latencyMS,
	}
	if out.ExitCode == exitCLINotFound {
		return status, providers.ErrCLINotFound
	}
	return status, nil
}

// accountIP looks up the account IP via `devil vhost list`. SetupSSL
// and RemoveSSL share this helper because the IP is stable per
// account.
func (p *Provider) accountIP(ctx context.Context) (string, error) {
	cmd := cmdDevil + " " + cmdVhostList
	out, err := p.exec(ctx, cmd)
	if err != nil {
		return "", err
	}
	_, ip, parseErr := parseVhostList(combine(out))
	if parseErr != nil {
		return "", parseErr
	}
	return ip, nil
}

// dbAddSubcommand maps a [providers.DatabaseKind]-shaped string to
// the panel sub-command. Unknown kinds surface as
// ErrInvalidProviderConfig.
func dbAddSubcommand(dbType string) (string, error) {
	switch dbType {
	case providers.DatabaseMySQL:
		return cmdMysqlAdd, nil
	case providers.DatabasePostgres:
		return cmdPgsqlAdd, nil
	default:
		return "", fmt.Errorf("%w: unsupported database kind %q", providers.ErrInvalidProviderConfig, dbType)
	}
}

func dbDeleteSubcommand(dbType string) (string, error) {
	switch dbType {
	case providers.DatabaseMySQL:
		return cmdMysqlDel, nil
	case providers.DatabasePostgres:
		return cmdPgsqlDel, nil
	default:
		return "", fmt.Errorf("%w: unsupported database kind %q", providers.ErrInvalidProviderConfig, dbType)
	}
}

// parseDeleteOutcome is the shared idempotent-delete predicate used
// by RemoveSubdomain. Accepts the success prefix and the panel's
// "not found" wording.
func parseDeleteOutcome(raw []byte, successPrefix string) error {
	clean, err := stripAndNormalize(raw)
	if err != nil {
		return err
	}
	text := strings.TrimSpace(string(clean))
	switch {
	case strings.HasPrefix(text, successPrefix):
		return nil
	case strings.HasPrefix(text, "not found"):
		return nil
	}
	return fmt.Errorf("%w: delete outcome unrecognized (%d bytes)", providers.ErrUnknownOutputFormat, len(raw))
}

// combine merges stdout + stderr into a single buffer for the
// parsers. The Devil panel writes "happy path" lines to stdout and
// failure diagnostics to stderr; the parsers branch on the textual
// shape, so a unified view keeps them simple and avoids missing the
// stderr-only "not found" cases.
func combine(out wssh.ExecResult) []byte {
	if len(out.Stderr) == 0 {
		return out.Stdout
	}
	if len(out.Stdout) == 0 {
		return out.Stderr
	}
	buf := make([]byte, 0, len(out.Stdout)+1+len(out.Stderr))
	buf = append(buf, out.Stdout...)
	buf = append(buf, '\n')
	buf = append(buf, out.Stderr...)
	return buf
}
