// Package api implements a read-only client for DirectAdmin's Live
// JSON API (`/api/*` endpoints, OpenAPI 2.0 spec) plus an SSH
// fallback that shells out to the panel's bundled `da-cli` binary.
//
// The shape mirrors the Sprint 21 cpanel/uapi package on purpose:
// callers compose a `Reader` interface, the transport layer is
// HTTPS-only (constructor rejects `http://`), and every failure
// surfaces a typed sentinel so consumers can `errors.Is` against
// the named class instead of string-matching. This package is
// imported by [providers/directadmin] (Sprint 24) and the
// `webox doctor directadmin` diagnostic CLI (Sprint 23 TASK-23.3).
//
// What's in scope for Sprint 23:
//   - HTTPS transport with Basic-auth via login key (NOT password).
//   - Read-only list endpoints: domains, subdomains, databases, SSL.
//   - Retry policy (500 ms × 2ⁿ × 3 attempts) for transient classes.
//   - SSH fallback via `da-cli` (TASK-23.2).
//   - Composite{Primary, Secondary} with HTTPS-first preference.
//
// Out of scope: mutating client (`MutatingClient`, Sprint 24+),
// adapter integration with `providers.HostingProvider`
// (`providers/directadmin/directadmin.go`, Sprint 24+), legacy
// `/CMD_API_*` parser (deferred — `ErrAPIDisabled` surfaces the
// degraded panel so operators can ask their host to enable Live
// API or fall back to a CMD-only profile in v0.4+).
package api

import "errors"

// ErrInvalidEndpoint is returned when the constructor receives a
// malformed base URL (empty host, non-`https` scheme, or unparseable
// URL). DA login keys travel through the `Authorization` header in
// every request — letting plain HTTP through would surface them on
// the wire, so the constructor blocks it up front.
var ErrInvalidEndpoint = errors.New("directadmin/api: invalid endpoint")

// ErrMissingCredentials surfaces when the constructor is called
// with an empty user or login key. The DA Live API rejects every
// request without `Authorization`; failing fast with a typed error
// keeps the failure mode predictable for the doctor CLI.
var ErrMissingCredentials = errors.New("directadmin/api: missing credentials")

// ErrAuthenticationFailed maps HTTP 401 / 403 responses. DA returns
// 401 for an invalid login key, 403 for a key that lacks the
// required scope (DA supports per-key scope limits since 1.62);
// we collapse both to the same sentinel because the remediation
// is identical: rotate the key.
var ErrAuthenticationFailed = errors.New("directadmin/api: authentication failed")

// ErrRateLimited maps HTTP 429. DA's anti-bruteforce module is
// active by default and rate-limits read calls too; the transport
// honours the standard retry budget but surfaces the sentinel when
// the budget is exhausted so the caller can decide whether to back
// further off or surface a UI hint.
var ErrRateLimited = errors.New("directadmin/api: rate limited")

// ErrServerError maps HTTP 5xx responses (excluding 503, which is
// classified as `ErrAPIDisabled` when the body matches DA's
// canonical "Live API is disabled" wording — that one's not a
// server error, it's a configuration choice).
var ErrServerError = errors.New("directadmin/api: server error")

// ErrMalformedResponse fires when the transport gets a 200 but the
// body fails to decode against the expected JSON shape. Surfaces
// either a corrupt response or a DA version mismatch we don't yet
// know about (defensive — every decoder maps unknown fields onto
// the zero value, but a top-level type drift will trip this).
var ErrMalformedResponse = errors.New("directadmin/api: malformed response")

// ErrTransportUnavailable is the catch-all for network-level
// failures (DNS, TCP refused, TLS handshake, deadline exceeded).
// The composite layer (TASK-23.2) keys its fall-over decision on
// this sentinel: when HTTPS surfaces `ErrTransportUnavailable`,
// the composite falls over to SSH; every other class (auth,
// rate-limit, API-disabled, etc.) surfaces verbatim because SSH
// would hit the same wall.
var ErrTransportUnavailable = errors.New("directadmin/api: transport unavailable")

// ErrAPIDisabled fires when the panel's Live API surface is turned
// off (older DA installs ship with `CMD_API_*` only; some hosters
// disable `/api/*` for security policy). HTTP signal: 404 on every
// `/api/*` endpoint, or a 503 with body containing "Live API" /
// "disabled" wording. The doctor CLI surfaces this with a
// `DISABLED` section status + actionable note ("contact your host
// to enable the Live API, or switch to a CMD-only profile").
var ErrAPIDisabled = errors.New("directadmin/api: live API disabled")

// ErrUnexpectedHTTPStatus is the fallback when the response status
// doesn't match any of the typed classes above. The wrapped value
// carries the actual status code so the caller can decide.
var ErrUnexpectedHTTPStatus = errors.New("directadmin/api: unexpected HTTP status")
