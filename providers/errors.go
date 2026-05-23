package providers

import "errors"

// Sentinel errors exposed by the providers package. Higher layers
// (wizard, status, TUI, doctor) branch on these via [errors.Is] to
// decide whether an operation is retryable, idempotent, or fatal.
//
// Each sentinel is intentionally opaque (no exported fields). Adapters
// and registry callers wrap the sentinel with [fmt.Errorf] using the %w
// verb when they need to carry contextual metadata (provider type,
// command name, output snippet) so that `errors.Is(err, ErrFoo)` keeps
// working all the way up the call stack while operator logs stay
// readable.
//
// Adapter-specific errors (for example "Let's Encrypt rate limit") live
// in their own subpackages and follow the same opaque-sentinel pattern.
var (
	// ErrInvalidProviderConfig is returned by [Register] / [New] and
	// every adapter constructor when [ProviderConfig] fails validation
	// (missing host/user, malformed alias, unsupported port, …). The
	// wrapper text identifies the offending field; callers compare via
	// errors.Is only.
	ErrInvalidProviderConfig = errors.New("provider: invalid configuration")

	// ErrUnknownProvider is returned by [New] when the requested
	// provider type has not been registered. Surfaces in `webox
	// doctor` as a missing-adapter error rather than a config error.
	ErrUnknownProvider = errors.New("provider: type not registered")

	// ErrProviderAlreadyRegistered is returned by [Register] when an
	// adapter attempts to register a name that is already taken. The
	// double-registration case is almost always a code bug (two init()
	// blocks with the same name) — we return it instead of panicking
	// so tests can assert on the behavior without process crash.
	ErrProviderAlreadyRegistered = errors.New("provider: type already registered")

	// ErrUnknownOutputFormat is returned by an adapter's parser when
	// the panel's CLI output does not match any of the known
	// shapes (success, exists, not-found, rate-limited, …). The error
	// message includes a short diagnostic snippet — adapters MUST
	// strip ANSI escapes and truncate to a safe length before
	// including any user-visible bytes.
	ErrUnknownOutputFormat = errors.New("provider: unknown CLI output format")

	// ErrOutputTooLarge is returned by adapter parsers when the
	// panel's CLI output exceeds the per-command size cap (1 MiB).
	// Treating oversized output as a fatal parser error prevents
	// runaway memory use if a panel command goes haywire, and keeps
	// our regex-based parsing well-behaved.
	ErrOutputTooLarge = errors.New("provider: command output exceeds size cap")

	// ErrSubdomainExists is returned by [HostingProvider.CreateSubdomain]
	// when the panel reports the subdomain already exists. The wizard
	// treats this as an idempotent "already done" branch.
	ErrSubdomainExists = errors.New("provider: subdomain already exists")

	// ErrNodeVersionUnsupported is returned by [HostingProvider.CreateSubdomain]
	// when the requested Node.js version is rejected by the panel.
	// MVP small.pl wraps the panel-supplied list verbatim; callers may
	// surface the panel diagnostic via err.Error().
	ErrNodeVersionUnsupported = errors.New("provider: node version not supported by panel")

	// ErrAppNotFound is returned by [HostingProvider.RestartNodeApp] /
	// [HostingProvider.CheckStatus] when the panel reports the
	// application does not exist on the account. Idempotent Remove*
	// callers MUST map this onto nil — but the sentinel exists so
	// non-idempotent flows (restart, status) can distinguish absent
	// from broken.
	ErrAppNotFound = errors.New("provider: app not found on account")

	// ErrAppNotNode is returned by [HostingProvider.RestartNodeApp]
	// when the target subdomain exists but is not a Node.js app
	// (static or PHP). The wizard should surface this as a
	// configuration mismatch.
	ErrAppNotNode = errors.New("provider: target domain is not a node app")

	// ErrDNSNotResolving is returned by [HostingProvider.SetupSSL]
	// when the panel cannot reach the domain over HTTP-01. For custom
	// domains this is expected during propagation; the status loop
	// retries until 48 h.
	ErrDNSNotResolving = errors.New("provider: domain dns not resolving")

	// ErrRateLimitLetsEncrypt is returned by [HostingProvider.SetupSSL]
	// when Let's Encrypt rate-limits the account. The status loop
	// backs off until the next day rather than retrying on the
	// regular ticker.
	ErrRateLimitLetsEncrypt = errors.New("provider: lets encrypt rate limit hit")

	// ErrDBNameTaken is returned by [HostingProvider.CreateDatabase]
	// when the requested DB / user name is already in use. The wizard
	// treats this as an idempotent "already done" branch (similar to
	// ErrSubdomainExists).
	ErrDBNameTaken = errors.New("provider: database name already taken")

	// ErrCLINotFound is returned by [HostingProvider.CheckStatus]
	// when the remote panel CLI is missing (exit 127). Doctor flow
	// surfaces this as an actionable "CLI not in PATH" diagnostic.
	ErrCLINotFound = errors.New("provider: panel cli not found in PATH")
)
