package providers

import "context"

// DefaultSSHPort is the assumed remote port when [ProviderConfig.Port]
// is the zero value. Adapters MUST NOT override this on their own; the
// registry normalizes Port at construction time so every layer agrees.
const DefaultSSHPort = 22

// DatabaseKind enumerates the database engines a panel can manage on
// behalf of the user. The canonical [HostingProvider] signature passes
// the kind as a plain string for forward compatibility (adapters MAY
// add new engines without an interface bump); these constants are the
// supported tokens for MVP and act as the single source of truth for
// callers that want to avoid stringly-typed literals scattered through
// business logic.
//
// MVP small.pl supports MySQL and PostgreSQL. Other adapters MAY add
// values in v0.2+ (for example SQLite for cyberpanel); unknown kinds
// MUST surface as a wrapped ErrInvalidProviderConfig at the adapter
// boundary, never at the registry level.
type DatabaseKind = string

const (
	// DatabaseMySQL maps to `devil mysql add/del` on small.pl and
	// equivalent commands on other panels.
	DatabaseMySQL DatabaseKind = "mysql"

	// DatabasePostgres maps to `devil pgsql add/del` on small.pl. Not
	// every small.pl plan exposes PostgreSQL; the adapter probes
	// availability before returning a structured error.
	DatabasePostgres DatabaseKind = "postgresql"
)

// ProviderConfig is the inbound DTO the registry hands to an adapter
// factory. Field-by-field it is a thin projection of `config.Profile`
// (see docs/DESIGN.md §3) — keeping it isolated avoids a circular
// dependency between `providers` and `config` and lets adapters be
// unit-tested without touching the on-disk config layer.
//
// Invariants enforced by [validateConfig]:
//
//   - Alias matches the same regex as `config.Profile.Alias`.
//   - Type, Host, User are non-empty (after trimming).
//   - Port is in [1, 65535]; zero is normalized to [DefaultSSHPort].
//   - Properties is non-nil at the adapter boundary (registry
//     normalizes nil to an empty map so adapters can read keys without
//     a guard).
type ProviderConfig struct {
	Alias      string
	Type       string
	Host       string
	Port       int
	User       string
	Properties map[string]string
}

// ProviderStatus is the result of [HostingProvider.CheckStatus]. It is
// the minimal sanity probe — Webox decorates it with HTTP / SSL / Node
// version data from the higher layer cache (status/), so the adapter
// only needs to answer "can I talk to the panel CLI right now?".
type ProviderStatus struct {
	// SSHConnected is true if the adapter could open an SSH session
	// against the configured host without surfacing
	// ssh.ErrPoolBusy / ErrReconnectExhausted / host-key errors.
	SSHConnected bool

	// CLIInstalled is true if the panel CLI (for small.pl: `devil`)
	// is present in PATH and returns a recognizable version string.
	// False maps to ErrCLINotFound at the caller.
	CLIInstalled bool

	// LatencyMS is the wall-clock duration of the probe in
	// milliseconds. The dashboard uses it as a coarse health
	// indicator (green / yellow / red bucket) — adapters MAY return 0
	// when the probe short-circuits before any I/O.
	LatencyMS int
}

// Subdomain is the result row of [HostingProvider.ListSubdomains]. It
// is intentionally minimal — the dashboard needs to know what is on
// the account and which Node.js version is bound to each subdomain so
// the wizard can detect drift between `config.json` and the panel.
type Subdomain struct {
	// Domain is the fully-qualified subdomain reported by the panel
	// (for small.pl: `<sub>.<user>.smallhost.pl` or a custom domain).
	Domain string

	// Type categorises the subdomain as the panel sees it. MVP
	// values: "nodejs", "static", "php". Unknown values are surfaced
	// verbatim so the dashboard can flag unmanaged stacks.
	Type string

	// NodeVersion is the Node.js version bound to the subdomain when
	// Type == "nodejs"; empty string otherwise. Format follows the
	// panel's reporting (typically major-only: "20", "22").
	NodeVersion string
}

// HostingProvider is the canonical Webox contract every hosting-panel
// adapter implements. Sub-packages register themselves via [Register]
// in their init() block; business logic NEVER type-switches on the
// concrete provider — see docs/DESIGN.md §3 and AGENTS.md §2.2.
//
// Method semantics:
//
//   - Every method that performs I/O takes a [context.Context] and
//     respects Done() inside long operations.
//   - Remove* methods are idempotent: "not found on panel" maps to
//     nil. This invariant keeps the LIFO rollback (MVP) and the
//     planned DAG engine (v0.3+) free of "already cleaned up" special
//     cases.
//   - GetDeployPath / GetLogPath are pure functions: no I/O, no logs.
//     Tests treat them like value functions.
//   - Adapters MUST NOT log secrets. Returned errors include only the
//     panel's diagnostic prefix, never the secret payload.
type HostingProvider interface {
	// Name returns the registered provider type (e.g. "smallhost").
	// Used by logging, status banners, and `webox doctor` output.
	Name() string

	// CreateSubdomain provisions a Node.js subdomain on the panel.
	// Returns ErrSubdomainExists if the panel reports the domain
	// already exists, ErrNodeVersionUnsupported if the requested
	// version is rejected.
	CreateSubdomain(ctx context.Context, domain string, nodeVersion string) error

	// SetupSSL provisions a Let's Encrypt certificate for the
	// subdomain. Returns ErrDNSNotResolving / ErrRateLimitLetsEncrypt
	// for the well-known recoverable cases; callers (status loop)
	// schedule a retry instead of surfacing the error to the wizard.
	SetupSSL(ctx context.Context, domain string) error

	// CreateDatabase provisions a database and the matching user.
	// Returns the panel-generated username and password — these are
	// caller-owned secrets and MUST be wiped from memory at the call
	// site (memguard buffer) before the wizard persists them in the
	// keyring. Returns ErrDBNameTaken when the requested name is
	// already in use on the account. The dbType parameter accepts the
	// [DatabaseKind] constants ("mysql", "postgresql"); adapters wrap
	// unknown kinds as ErrInvalidProviderConfig.
	CreateDatabase(ctx context.Context, dbType, dbName string) (user, password string, err error)

	// RestartNodeApp triggers a panel-side restart of the Node.js
	// application bound to the subdomain. ErrAppNotFound /
	// ErrAppNotNode communicate the well-known panel responses to
	// the dashboard.
	RestartNodeApp(ctx context.Context, domain string) error

	// GetDeployPath returns the absolute path the deploy workflow
	// rsyncs build artifacts into. Pure function: no I/O.
	GetDeployPath(domain string) string

	// GetLogPath returns the absolute path containing Node.js stdout
	// / stderr logs. Pure function: no I/O.
	GetLogPath(domain string) string

	// TailLog returns the last `lines` log entries for the Node.js
	// subdomain. Implementations MUST clamp the line count to a
	// safe upper bound and accept zero/negative as "use adapter
	// default". The byte slice contains stdout (and possibly stderr
	// for soft errors such as "log file missing") so the dashboard
	// can render the operator-facing diagnostic verbatim.
	TailLog(ctx context.Context, domain string, lines int) ([]byte, error)

	// CheckStatus performs the cheap sanity probe against the panel
	// (SSH session + CLI version). The result is consumed by the
	// dashboard's health badge and `webox doctor`.
	CheckStatus(ctx context.Context) (*ProviderStatus, error)

	// ListSubdomains enumerates the subdomains currently provisioned
	// on the account so the wizard can detect drift between
	// `config.json` and the panel.
	ListSubdomains(ctx context.Context) ([]Subdomain, error)

	// RemoveSubdomain deprovisions the subdomain. Idempotent: nil
	// when the panel reports "not found".
	RemoveSubdomain(ctx context.Context, domain string) error

	// RemoveDatabase deprovisions the database and its user.
	// Idempotent: nil when the panel reports "not found". The dbType
	// parameter accepts the same tokens as CreateDatabase; adapters
	// MAY choose to probe both engines when invoked without an
	// explicit kind (post-MVP).
	RemoveDatabase(ctx context.Context, dbType, dbName string) error

	// RemoveSSL deprovisions the SSL certificate. Idempotent: nil
	// when the panel reports "no cert".
	RemoveSSL(ctx context.Context, domain string) error
}
