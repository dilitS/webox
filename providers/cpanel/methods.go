package cpanel

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dilitS/webox/providers"
	"github.com/dilitS/webox/providers/cpanel/uapi"
)

// Tail bounds. Empty / negative input from the dashboard maps to
// [defaultTailLines]; anything above [maxTailLines] is silently
// clamped so a stray UI value cannot ship 1 GB of log to the
// local process.
const (
	defaultTailLines = 200
	maxTailLines     = 10000
)

// CreateSubdomain provisions a domain + Passenger application
// pair. The orchestration intentionally lives in the adapter
// (not the wizard) because the panel exposes two endpoints
// (DomainInfo::add_addon_domain or SubDomain::addsubdomain) that
// callers above the adapter should not need to differentiate.
//
// Step order:
//  1. Provision the addon/subdomain (panel-side DNS + vhost).
//  2. Register the Passenger application at the app-root path.
//
// On step 2 failure, step 1 is NOT rolled back here — the
// wizard's LIFO stack pushed the subdomain cleanup before the
// SSL / DB / app steps in [wizard.Execute], so the unwind path
// runs `RemoveSubdomain` automatically. Putting the rollback in
// two places (here AND in the wizard) would risk a double-delete
// on the slow panel and is unnecessary given the LIFO design.
func (p *Provider) CreateSubdomain(ctx context.Context, domain, nodeVersion string) error {
	if err := ValidateDomain(domain); err != nil {
		return fmt.Errorf("%w: %w", providers.ErrInvalidProviderConfig, err)
	}
	if err := ValidateNodeVersion(nodeVersion); err != nil {
		return fmt.Errorf("%w: %w", providers.ErrInvalidProviderConfig, err)
	}
	if p.mutator == nil {
		return errMissingSeam("mutator")
	}

	appRoot := p.resolveAppRoot(domain)
	appPath := p.renderTemplate(p.props.AppRootTemplate, appRoot)
	docRoot := p.renderTemplate(p.props.DeployPathTemplate, appRoot)

	// Step 1: provision the domain entry.
	if err := p.addDomain(ctx, domain, docRoot); err != nil {
		return mapResourceExists(err, providers.ErrSubdomainExists)
	}

	// Step 2: register the Passenger application.
	createArgs := uapi.CreatePassengerAppArgs{
		Name:           appRoot,
		Path:           appPath,
		Domain:         domain,
		BaseURI:        "/",
		DeploymentMode: "production",
		Envvars: map[string]string{
			"NODE_ENV": "production",
		},
	}
	if err := p.mutator.CreatePassengerApp(ctx, createArgs); err != nil {
		// The wizard's LIFO stack already queued the subdomain
		// cleanup before calling CreateSubdomain (see
		// wizard.Execute); a partial-success unwind will run
		// RemoveSubdomain automatically.
		return mapResourceExists(err, providers.ErrSubdomainExists)
	}
	return nil
}

// addDomain dispatches the domain creation onto the correct
// endpoint based on the configured DomainKind.
func (p *Provider) addDomain(ctx context.Context, domain, docRoot string) error {
	switch p.props.DomainKind {
	case domainKindSubdomain:
		label, root := splitDomain(domain)
		if root == "" {
			return fmt.Errorf("%w: cannot derive root domain from %q",
				providers.ErrInvalidProviderConfig, domain)
		}
		return p.mutator.AddSubdomain(ctx, uapi.CreateSubdomainArgs{
			Domain: label, RootDomain: root, Dir: docRoot,
		})
	default:
		return p.mutator.AddAddonDomain(ctx, uapi.CreateAddonDomainArgs{
			NewDomain: domain, Dir: docRoot,
		})
	}
}

// SetupSSL provisions SSL for the domain. AutoSSL is async on
// the panel side; the adapter returns immediately after the
// trigger. Manual provider mode would require a cert+key pair
// outside the wizard's scope, so it surfaces a typed error.
func (p *Provider) SetupSSL(ctx context.Context, domain string) error {
	if err := ValidateDomain(domain); err != nil {
		return fmt.Errorf("%w: %w", providers.ErrInvalidProviderConfig, err)
	}
	if p.mutator == nil {
		return errMissingSeam("mutator")
	}
	switch p.props.SSLProvider {
	case sslAutoSSL:
		if err := p.mutator.StartAutoSSL(ctx, domain); err != nil {
			if errors.Is(err, uapi.ErrModuleFunctionDenied) {
				return providers.ErrDNSNotResolving
			}
			return err
		}
		if p.props.AutoSSLPollSeconds > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(p.props.AutoSSLPollSeconds) * time.Second):
			}
		}
		return nil
	case sslManual:
		return fmt.Errorf("%w: ssl_provider=manual requires cert+key out of wizard scope",
			providers.ErrInvalidProviderConfig)
	default:
		return fmt.Errorf("%w: ssl_provider %q not handled",
			providers.ErrInvalidProviderConfig, p.props.SSLProvider)
	}
}

// CreateDatabase orchestrates three UAPI calls — create_database,
// create_user, set_privileges — and returns the panel-prefixed
// username plus the generated password. Callers MUST move the
// password into a memguard buffer immediately (the wizard
// already does this).
func (p *Provider) CreateDatabase(ctx context.Context, dbType, dbName string) (user, password string, err error) {
	if dbType != providers.DatabaseMySQL {
		return "", "", fmt.Errorf("%w: cpanel adapter supports MySQL only in Sprint 22 (got %q)",
			providers.ErrInvalidProviderConfig, dbType)
	}
	if vErr := ValidateDBName(dbName); vErr != nil {
		return "", "", fmt.Errorf("%w: %w", providers.ErrInvalidProviderConfig, vErr)
	}
	if p.mutator == nil {
		return "", "", errMissingSeam("mutator")
	}

	password, err = generatePassword()
	if err != nil {
		return "", "", err
	}

	if cErr := p.mutator.CreateMysqlDatabase(ctx, dbName); cErr != nil {
		return "", "", mapResourceExists(cErr, providers.ErrDBNameTaken)
	}

	user = dbName // cPanel prefixes both DB and user with the account login.
	if uErr := p.mutator.CreateMysqlUser(ctx, user, password); uErr != nil {
		// Best-effort rollback of the database — same pattern
		// as smallhost. Surface the original error; wizard
		// rollback will replay RemoveDatabase if the operator
		// chooses to unwind.
		_ = p.mutator.DeleteMysqlDatabase(ctx, dbName)
		return "", "", mapResourceExists(uErr, providers.ErrDBNameTaken)
	}

	if pErr := p.mutator.SetMysqlPrivileges(ctx, uapi.MysqlPrivilegesArgs{
		Database:   dbName,
		User:       user,
		Privileges: []string{"ALL PRIVILEGES"},
	}); pErr != nil {
		// Best-effort rollback of user + DB.
		_ = p.mutator.DeleteMysqlUser(ctx, user)
		_ = p.mutator.DeleteMysqlDatabase(ctx, dbName)
		return "", "", pErr
	}

	// Returning the un-prefixed slug (matching the operator's
	// input) keeps the wizard's UI consistent; the panel
	// prepends `<accountuser>_` server-side and the dashboard
	// re-derives the full name when needed.
	return user, password, nil
}

// RestartNodeApp triggers the Passenger restart for the
// application at the domain's app-root path. cPanel's restart
// endpoint takes the absolute path, not the application name —
// `path` is the stable identifier.
func (p *Provider) RestartNodeApp(ctx context.Context, domain string) error {
	if err := ValidateDomain(domain); err != nil {
		return fmt.Errorf("%w: %w", providers.ErrInvalidProviderConfig, err)
	}
	if p.mutator == nil {
		return errMissingSeam("mutator")
	}
	appPath := p.renderTemplate(p.props.AppRootTemplate, p.resolveAppRoot(domain))
	if err := p.mutator.RestartPassengerApp(ctx, appPath); err != nil {
		if errors.Is(err, uapi.ErrResourceNotFound) {
			return providers.ErrAppNotFound
		}
		return err
	}
	return nil
}

// GetDeployPath returns the absolute path the deploy workflow
// rsyncs build artefacts into. Pure function: no I/O.
func (p *Provider) GetDeployPath(domain string) string {
	if err := ValidateDomain(domain); err != nil {
		return ""
	}
	return p.renderTemplate(p.props.DeployPathTemplate, p.resolveAppRoot(domain))
}

// GetLogPath returns the absolute path containing Node.js
// stdout/stderr logs. Pure function: no I/O.
func (p *Provider) GetLogPath(domain string) string {
	if err := ValidateDomain(domain); err != nil {
		return ""
	}
	return p.renderTemplate(p.props.LogPathTemplate, p.resolveAppRoot(domain))
}

// TailLog returns the last N log entries via SSH `tail`. Same
// shape as smallhost.TailLog but the file layout follows the
// cPanel/Passenger convention.
func (p *Provider) TailLog(ctx context.Context, domain string, lines int) ([]byte, error) {
	if err := ValidateDomain(domain); err != nil {
		return nil, fmt.Errorf("%w: %w", providers.ErrInvalidProviderConfig, err)
	}
	if p.runner == nil {
		return nil, errMissingSeam("ssh-runner")
	}
	lines = clampTailLines(lines)
	logDir := p.GetLogPath(domain)
	if logDir == "" {
		return nil, fmt.Errorf("%w: log path unavailable", providers.ErrInvalidProviderConfig)
	}
	// cPanel + Passenger logs land in two files: the
	// application's stdout (`app.log`) and stderr (`error.log`).
	// `tail` with `--` separator stops shell from misreading
	// any leading `-` in the file names; the names are static.
	cmd := strings.Join([]string{
		"tail",
		"-n", strconv.Itoa(lines),
		"--",
		logDir + "/app.log",
		logDir + "/error.log",
	}, " ")
	stdout, stderr, code, err := p.runner.Run(ctx, cmd)
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return combineTailOutput(stdout, stderr), nil
	}
	return stdout, nil
}

// CheckStatus performs the cheap probe required by the
// dashboard's health badge. cPanel does not expose a "version"
// endpoint that survives across cPanel versions; we use
// `DomainInfo.list_domains` as a proxy because it works on
// every supported version and the dashboard already needs the
// data anyway.
func (p *Provider) CheckStatus(ctx context.Context) (*providers.ProviderStatus, error) {
	if p.reader == nil {
		return nil, errMissingSeam("reader")
	}
	now := p.clock()
	start := now()
	_, err := p.reader.ListDomains(ctx)
	elapsed := now().Sub(start)
	latencyMS := int(elapsed / time.Millisecond)
	if err != nil {
		status := &providers.ProviderStatus{
			SSHConnected: false,
			CLIInstalled: false,
			LatencyMS:    latencyMS,
		}
		if errors.Is(err, uapi.ErrAuthenticationFailed) {
			return status, providers.ErrCLINotFound
		}
		return status, err
	}
	return &providers.ProviderStatus{
		SSHConnected: true,
		CLIInstalled: true,
		LatencyMS:    latencyMS,
	}, nil
}

// ListSubdomains enumerates the Passenger applications on the
// account. We use PassengerApps::list_applications (not
// DomainInfo) because the wizard cares about the Node.js
// version bound to each app — DomainInfo lists every vhost
// including PHP and static.
func (p *Provider) ListSubdomains(ctx context.Context) ([]providers.Subdomain, error) {
	if p.reader == nil {
		return nil, errMissingSeam("reader")
	}
	resp, err := p.reader.ListPassengerApps(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]providers.Subdomain, 0, len(resp.Applications))
	for _, app := range resp.Applications {
		out = append(out, providers.Subdomain{
			Domain:      app.Domain,
			Type:        "nodejs",
			NodeVersion: "", // cPanel doesn't expose Node version via PassengerApps; future probe via Selector.
		})
	}
	return out, nil
}

// RemoveSubdomain deprovisions the domain + Passenger
// application pair. Idempotent: panel "not found" maps to nil.
func (p *Provider) RemoveSubdomain(ctx context.Context, domain string) error {
	if err := ValidateDomain(domain); err != nil {
		return fmt.Errorf("%w: %w", providers.ErrInvalidProviderConfig, err)
	}
	if p.mutator == nil {
		return errMissingSeam("mutator")
	}
	appPath := p.renderTemplate(p.props.AppRootTemplate, p.resolveAppRoot(domain))
	// Order matters: delete the Passenger app first, then the
	// domain. Reversing the order leaves the vhost without a
	// Passenger config and the panel logs a warning every
	// time the app dir is touched until we delete it.
	if err := p.mutator.DeletePassengerApp(ctx, appPath); err != nil {
		if err := mapResourceNotFound(err); err != nil {
			return err
		}
	}
	if err := p.deleteDomain(ctx, domain); err != nil {
		return mapResourceNotFound(err)
	}
	return nil
}

// deleteDomain dispatches the delete call onto the same endpoint
// the create call used. The wizard guarantees DomainKind is the
// same value used during creation because it persists with the
// profile.
func (p *Provider) deleteDomain(ctx context.Context, domain string) error {
	if p.props.DomainKind == domainKindSubdomain {
		return p.mutator.DeleteSubdomain(ctx, domain)
	}
	return p.mutator.DeleteDomain(ctx, domain)
}

// RemoveDatabase deprovisions the database and the matching
// user. Idempotent.
func (p *Provider) RemoveDatabase(ctx context.Context, dbType, dbName string) error {
	if dbType != providers.DatabaseMySQL {
		return fmt.Errorf("%w: cpanel adapter supports MySQL only in Sprint 22 (got %q)",
			providers.ErrInvalidProviderConfig, dbType)
	}
	if err := ValidateDBName(dbName); err != nil {
		return fmt.Errorf("%w: %w", providers.ErrInvalidProviderConfig, err)
	}
	if p.mutator == nil {
		return errMissingSeam("mutator")
	}
	if err := p.mutator.DeleteMysqlUser(ctx, dbName); err != nil {
		if err := mapResourceNotFound(err); err != nil {
			return err
		}
	}
	if err := p.mutator.DeleteMysqlDatabase(ctx, dbName); err != nil {
		return mapResourceNotFound(err)
	}
	return nil
}

// RemoveSSL deprovisions the SSL certificate. Idempotent.
func (p *Provider) RemoveSSL(ctx context.Context, domain string) error {
	if err := ValidateDomain(domain); err != nil {
		return fmt.Errorf("%w: %w", providers.ErrInvalidProviderConfig, err)
	}
	if p.mutator == nil {
		return errMissingSeam("mutator")
	}
	return mapResourceNotFound(p.mutator.DeleteSSL(ctx, domain))
}

// clampTailLines mirrors smallhost.clampTailLines so the two
// adapters present consistent bounds to the dashboard.
func clampTailLines(n int) int {
	if n <= 0 {
		return defaultTailLines
	}
	if n > maxTailLines {
		return maxTailLines
	}
	return n
}

// combineTailOutput mirrors smallhost.combine. cPanel + Passenger
// write "log file does not exist" diagnostics to stderr with
// exit-code 1, but the diagnostic itself is the most useful
// thing the operator can see — so we return the combined view.
func combineTailOutput(stdout, stderr []byte) []byte {
	if len(stderr) == 0 {
		return stdout
	}
	if len(stdout) == 0 {
		return stderr
	}
	buf := make([]byte, 0, len(stdout)+1+len(stderr))
	buf = append(buf, stdout...)
	buf = append(buf, '\n')
	buf = append(buf, stderr...)
	return buf
}
