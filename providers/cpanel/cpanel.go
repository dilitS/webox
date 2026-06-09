package cpanel

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dilitS/webox/providers"
	"github.com/dilitS/webox/providers/cpanel/uapi"
)

// Provider is the [providers.HostingProvider] adapter for
// cPanel-based shared hosting. The adapter composes three
// transport seams which the wiring step (or tests) injects via
// the [Provider.SetReader], [Provider.SetMutator],
// [Provider.SetSSHRunner] setters:
//
//   - Reader   — read-only UAPI calls (list domains, list apps).
//   - Mutator  — mutating UAPI calls (create app, create DB).
//   - SSHRunner — raw shell exec for [Provider.TailLog].
//
// Decoupling the adapter from the concrete clients keeps the
// LIFO rollback testable with in-memory fakes and lets future
// transports (e.g. GraphQL WHM API) plug in without touching the
// method bodies.
type Provider struct {
	cfg     providers.ProviderConfig
	props   Properties
	reader  uapi.Reader
	mutator uapi.Mutator
	runner  uapi.SSHRunner
	now     func() time.Time
}

// SetReader installs the read-only UAPI seam. Tests inject a
// fake; production wiring passes the [uapi.Composite] built from
// HTTPSClient + SSHFallback.
func (p *Provider) SetReader(r uapi.Reader) { p.reader = r }

// SetMutator installs the mutating UAPI seam. Same shape as
// [SetReader]; production wiring passes the [uapi.CompositeMutator].
func (p *Provider) SetMutator(m uapi.Mutator) { p.mutator = m }

// SetSSHRunner installs the raw SSH command seam used by
// [Provider.TailLog]. Production wiring passes the
// [uapi.SSHPoolRunner].
func (p *Provider) SetSSHRunner(r uapi.SSHRunner) { p.runner = r }

// SetClock installs the clock function the adapter uses for
// latency measurement in [Provider.CheckStatus]. Passing nil
// restores the default (time.Now); useful for tests that want
// to reset the seam between subtests.
func (p *Provider) SetClock(now func() time.Time) {
	if now == nil {
		p.now = time.Now
		return
	}
	p.now = now
}

// Name satisfies [providers.HostingProvider].
func (p *Provider) Name() string { return providerName }

// Config exposes the normalised configuration for tests and the
// debug TUI. Returns a value copy.
func (p *Provider) Config() providers.ProviderConfig { return p.cfg }

// Properties exposes the parsed properties bag for tests and the
// debug TUI. Returns a value copy.
func (p *Provider) Properties() Properties { return p.props }

// New is the [providers.Factory] registered under "cpanel". It
// runs the adapter-specific Properties validation; the registry
// already validated the shared invariants (alias / host / user /
// port / Properties non-nil) before calling us.
func New(cfg providers.ProviderConfig) (providers.HostingProvider, error) {
	if cfg.Type != providerName {
		return nil, fmt.Errorf("%w: cpanel factory invoked with type %q",
			providers.ErrInvalidProviderConfig, cfg.Type)
	}
	props, err := parseProperties(cfg.Properties)
	if err != nil {
		return nil, err
	}
	return &Provider{cfg: cfg, props: props, now: time.Now}, nil
}

// clock returns the currently installed clock function. New
// providers initialised through the registry might not have hit
// SetClock yet, so we fall back to [time.Now] to keep
// [CheckStatus] thread-safe even before the test setup runs.
func (p *Provider) clock() func() time.Time {
	if p.now == nil {
		return time.Now
	}
	return p.now
}

// resolveAppRoot computes the per-app slug from a fully-qualified
// domain. Dots become dashes (`shop.example.com` →
// `shop-example-com`) so the slug round-trips into filesystem
// paths and Passenger application names without panel quoting
// gymnastics.
func (p *Provider) resolveAppRoot(domain string) string {
	return strings.ReplaceAll(domain, ".", "-")
}

// renderTemplate substitutes `{user}` and `{app_root}` into the
// template. Pulled out as a free method so tests can pin the
// substitution behaviour without spinning up a real Provider.
func (p *Provider) renderTemplate(tmpl, appRoot string) string {
	out := strings.ReplaceAll(tmpl, "{user}", p.cfg.User)
	out = strings.ReplaceAll(out, "{app_root}", appRoot)
	return out
}

// splitDomain returns the leftmost label and the rest of the
// domain, e.g. `("shop", "example.com")` from "shop.example.com".
// Used by [Provider.CreateSubdomain] when the operator opted into
// subdomain-kind provisioning.
func splitDomain(domain string) (label, root string) {
	idx := strings.IndexByte(domain, '.')
	if idx <= 0 || idx >= len(domain)-1 {
		return domain, ""
	}
	return domain[:idx], domain[idx+1:]
}

// generatePassword returns a URL-safe random password suitable
// for the MySQL user the adapter creates alongside the database.
// 192 bits of entropy is overkill for a per-app DB password
// (cPanel typically caps at 64 chars) but the abundance is free
// and the constant choice keeps audits simple.
func generatePassword() (string, error) {
	const entropyBytes = 24 // 24 bytes ≈ 192 bits.
	buf := make([]byte, entropyBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("cpanel: csprng for password: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// errMissingSeam wraps [providers.ErrUnknownOutputFormat] when an
// adapter method needs a seam (reader / mutator / runner) that
// the wiring step never installed. Surfacing this as the
// `unknown output format` sentinel matches smallhost's "executor
// not configured" pattern — both translate to "the adapter is
// not yet ready to talk to the panel".
func errMissingSeam(name string) error {
	return fmt.Errorf("%w: cpanel %s seam not configured (call Set%s)",
		providers.ErrUnknownOutputFormat, name, strings.Title(strings.ToLower(name))) //nolint:staticcheck // strings.Title sufficient for ASCII names.
}

// mapResourceExists folds the panel's idempotent-create signal
// (uapi.ErrResourceExists) onto the canonical provider sentinel
// (providers.ErrSubdomainExists / ErrDBNameTaken / ErrAppNotFound,
// per caller). The mapping is intentionally pushed out of each
// method so the call site stays declarative.
func mapResourceExists(err, exists error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, uapi.ErrResourceExists) {
		return exists
	}
	return err
}

// mapResourceNotFound folds [uapi.ErrResourceNotFound] onto nil
// for the idempotent Remove* path. Anything else surfaces
// verbatim so non-idempotent flows can still distinguish absent
// from broken.
func mapResourceNotFound(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, uapi.ErrResourceNotFound) {
		return nil
	}
	return err
}

func init() {
	if err := providers.Register(providerName, New); err != nil {
		panic(fmt.Sprintf("cpanel: register: %v", err))
	}
	if err := providers.RegisterPlanValidators(providerName, providers.PlanValidators{
		ValidateDomain:      ValidateDomain,
		ValidateNodeVersion: ValidateNodeVersion,
		ValidateDBName:      ValidateDBName,
	}); err != nil {
		panic(fmt.Sprintf("cpanel: register plan validators: %v", err))
	}
}
