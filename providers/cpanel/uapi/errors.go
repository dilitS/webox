package uapi

import "errors"

// Errors returned by the UAPI client. Every caller branches on these
// via errors.Is — we never compare error strings, and we never wrap
// secrets (tokens, passwords) into the error chain. The transport
// strips the Authorization header before formatting any debug
// representation.
var (
	// ErrInvalidEndpoint signals the constructor rejected the
	// supplied baseURL: missing scheme, plain http, or an invalid
	// host. We never silently upgrade http→https; the caller
	// fixes the config instead.
	ErrInvalidEndpoint = errors.New("uapi: invalid endpoint (require https://host[:port])")

	// ErrMissingCredentials surfaces when neither user nor token
	// is supplied. We refuse to "anonymously" probe a cPanel host
	// because every real-world cPanel rejects anon UAPI anyway.
	ErrMissingCredentials = errors.New("uapi: user and token are required")

	// ErrRateLimited maps the HTTP 429 response a cPanel WHM
	// instance returns when the API token has tripped the
	// throttle. The transport retries with exponential backoff
	// (capped at maxRetries) before surfacing this error.
	ErrRateLimited = errors.New("uapi: rate-limited (HTTP 429) after retries")

	// ErrAuthenticationFailed maps HTTP 401/403. Token rotation
	// is the only remediation; the CLI surfaces this verbatim so
	// operators recognise it without parsing the wrapped message.
	ErrAuthenticationFailed = errors.New("uapi: authentication failed (HTTP 401/403)")

	// ErrServerError maps HTTP 5xx after retries. cPanel WHM
	// occasionally returns 503 during package upgrades; the
	// transport retries those, but persistent 5xx surface as
	// this error so the caller can mark the host degraded.
	ErrServerError = errors.New("uapi: persistent server error (HTTP 5xx)")

	// ErrMalformedResponse signals that the HTTP response body
	// was not valid UAPI envelope JSON. We never try to coerce
	// HTML pages or empty bodies into typed responses.
	ErrMalformedResponse = errors.New("uapi: malformed response body")

	// ErrModuleFunctionDenied corresponds to cPanel's
	// "Feature disabled" / "User does not have access" cases.
	// The remediation is a WHM-side feature list change; we
	// surface it as a distinct error so the CLI can format the
	// recommendation differently from a generic auth failure.
	ErrModuleFunctionDenied = errors.New("uapi: module / function disabled for the account")

	// ErrSprintScopeNotMutable guards the [MutatingClient]
	// stub. Sprint 21 deliberately refuses mutating ops via the
	// type system; calling [MutatingClient].Call always returns
	// this error so the production code path lands in Sprint 22
	// with no shortcuts.
	ErrSprintScopeNotMutable = errors.New("uapi: mutating ops out of scope for Sprint 21 (v0.2-rc read-only)")

	// ErrUnexpectedHTTPStatus surfaces an HTTP status the
	// transport did not recognise (anything outside the
	// explicitly-handled 200/401/403/429/5xx set). We surface
	// the status code via fmt.Errorf("%w: HTTP %d", ...) so the
	// CLI can format the value without sniffing the message.
	ErrUnexpectedHTTPStatus = errors.New("uapi: unexpected HTTP status")

	// ErrAPIResultFailure is the parent for any UAPI envelope
	// with result.status != 1 that did NOT match the more
	// specific [ErrModuleFunctionDenied] pattern. The wrapped
	// message carries the raw `errors` slice for the operator.
	ErrAPIResultFailure = errors.New("uapi: API call returned non-success status")

	// ErrUnknownSSLShape signals SSL.list_keys returned a JSON
	// shape neither the modern object-wrapper nor a top-level
	// array. cPanel does not document any third shape; the
	// error nevertheless exists so the transport never panics
	// on an envelope from a forked WHM build.
	ErrUnknownSSLShape = errors.New("uapi: SSL.list_keys: unrecognised JSON shape")

	// ErrTransportUnavailable signals that the underlying
	// transport (HTTPS or SSH) is unreachable: DNS failure,
	// connection refused, TLS handshake failure, or persistent
	// 5xx after retries on the HTTPS path; missing pool /
	// permanent dial failure on the SSH path. The composite
	// fallback layer fails over to the alternate transport on
	// this error; everything else surfaces verbatim.
	ErrTransportUnavailable = errors.New("uapi: transport unavailable")

	// ErrSSHRunnerRequired surfaces when [NewSSHFallback] is
	// invoked without a runner. Production wiring always
	// supplies one; the typed error keeps the empty-runner
	// failure path observable in tests without scraping
	// strings.
	ErrSSHRunnerRequired = errors.New("uapi: SSH runner is required")

	// ErrMutationsDisabled signals that the operator-side env
	// var WEBOX_CPANEL_MUTATIONS=1 is not set. Every method on
	// the Sprint-22 [Mutator] surface returns this sentinel
	// without making the underlying request, so a missing flag
	// is a fail-closed default rather than an accidental
	// destructive call. The opt-in guard is documented in
	// docs/sprints/sprint-22-cpanel-adapter-mutations.md and
	// docs/SECURITY.md.
	ErrMutationsDisabled = errors.New("uapi: mutations disabled (set WEBOX_CPANEL_MUTATIONS=1 to opt in)")

	// ErrInvalidArgs surfaces when a [Mutator] caller passes an
	// args struct that fails the per-method input validation:
	// empty domain, oversized password, control characters in a
	// path. The adapter layer (providers/cpanel) already validates
	// every public input; this sentinel is defence in depth for
	// the case the package gets reused outside the adapter.
	ErrInvalidArgs = errors.New("uapi: invalid mutating arguments")

	// ErrResourceExists is the panel-level idempotency signal:
	// the resource being created is already present. The
	// adapter maps this onto provider-level sentinels (e.g.
	// providers.ErrSubdomainExists or providers.ErrDBNameTaken)
	// so callers above the transport keep using the canonical
	// idempotent-create semantics.
	ErrResourceExists = errors.New("uapi: resource already exists on panel")

	// ErrResourceNotFound is the panel-level idempotency signal
	// on the delete path: the resource never existed or was
	// already gone. The adapter maps this onto nil for
	// Remove* methods so LIFO rollback stays idempotent.
	ErrResourceNotFound = errors.New("uapi: resource not found on panel")
)
