package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dilitS/webox/internal/version"
)

// Defaults for the HTTPS transport. Identical surface to the
// cpanel/uapi transport (Sprint 21) so future contributors don't
// have to re-learn the retry semantics per provider — both
// adapters now follow the same: 3-attempt budget, 500ms × 2ⁿ
// backoff, 30s default per-call timeout, 4 MiB body cap.
const (
	defaultTimeout    = 30 * time.Second
	maxRetries        = 3
	maxBodyBytes      = 4 << 20 // 4 MiB; DA list responses are tiny in practice.
	backoffBaseMS     = 500
	backoffMultiplier = 2
)

// userAgent identifies Webox to DA hosters that whitelist API
// traffic. Some hosting providers (notably JBMC-partner installs)
// gate requests behind a UA allow-list; emitting webox/<version>
// + the repo URL gives them an unambiguous identifier instead of
// requiring us to pretend to be a browser.
func userAgent() string {
	return fmt.Sprintf("webox/%s +https://github.com/dilitS/webox", version.Version)
}

// transport is the per-Client HTTP layer. Owns the *http.Client
// (no global default — `httpcheck` and `cpanel/uapi` keep their
// own isolated clients), the credentials, and the retry policy.
//
// The two seams (`now`, `backoffFor`) exist for tests: the
// retry-loop test injects a zero-backoff so the suite doesn't
// spend 3.5s per retry-classify subtest.
type transport struct {
	client     *http.Client
	baseURL    *url.URL
	user       string
	loginKey   string
	now        func() time.Time
	backoffFor func(attempt int) time.Duration
}

// newTransport validates the base URL up front and refuses any
// scheme except `https`. DA login keys travel in the Basic-auth
// header on every request — plain HTTP would surface them on the
// wire, which is the exact threat model the typed sentinel
// `ErrInvalidEndpoint` was designed to short-circuit.
//
// The caller is expected to supply a pre-built `*http.Client`
// (e.g. with a custom CA pool or pinned TLS config); if nil,
// the constructor wraps a vanilla client with the package's
// default timeout. Reusing a caller-supplied client preserves
// connection-pool benefits across multiple Client instances.
func newTransport(rawURL, user, loginKey string, httpClient *http.Client) (*transport, error) {
	if user == "" || loginKey == "" {
		return nil, ErrMissingCredentials
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return nil, ErrInvalidEndpoint
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	return &transport{
		client:     httpClient,
		baseURL:    parsed,
		user:       user,
		loginKey:   loginKey,
		now:        time.Now,
		backoffFor: backoff,
	}, nil
}

// endpointURL composes the full URL for an endpoint. The DA Live
// API rule: every path lives under `/api/`. We allow the caller
// to thread a single positional argument (typically the username)
// because three of the read-only endpoints embed it in the path
// (`/api/users/<user>/domains`, etc.). The pattern uses %s, not
// `{user}`, so the substitution is sprintf-safe and not an
// open-ended template engine.
func (t *transport) endpointURL(ep Endpoint, args ...any) string {
	u := *t.baseURL
	path := string(ep)
	if len(args) > 0 {
		path = fmt.Sprintf(path, args...)
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/api/" + strings.TrimLeft(path, "/")
	return u.String()
}

// call issues one HTTP GET, retries on transient classes, and
// returns the raw response body for the decoder to unmarshal.
// The retry loop is identical to cpanel/uapi's, so the wall-clock
// behaviour and exit-code policy across the two adapters' doctor
// CLIs is consistent.
func (t *transport) call(ctx context.Context, endpoint string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			if err := t.sleepWithCtx(ctx, t.backoffFor(attempt)); err != nil {
				return nil, err
			}
		}
		body, err := t.singleCall(ctx, endpoint)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !shouldRetry(err) {
			return nil, err
		}
	}
	return nil, lastErr
}

// sleepWithCtx blocks for d while honouring ctx cancellation.
// Returns the wrapped context error so the composite layer can
// fall over on cancellation-mid-retry. Mirrors the cpanel/uapi
// implementation precisely — Sprint 21 retro flagged the
// originally-naïve ctx.Err() return as a UNREACHABLE-vs-FAILED
// classification bug.
func (t *transport) sleepWithCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	select {
	case <-ctx.Done():
		return fmt.Errorf("%w: %w", ErrTransportUnavailable, ctx.Err())
	case <-time.After(d):
		return nil
	}
}

// singleCall performs one HTTP round-trip. Split from `call` so
// the retry loop stays readable and so tests can swap the HTTP
// client without overriding the retry policy.
func (t *transport) singleCall(ctx context.Context, endpoint string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("directadmin/api: build request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent())
	req.SetBasicAuth(t.user, t.loginKey)
	req.Header.Set("Accept", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		// Connection-level failure (DNS, refused, TLS, timeout).
		// Surface via the typed sentinel so the composite layer
		// can fall over to SSH.
		return nil, fmt.Errorf("%w: %w", ErrTransportUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("directadmin/api: read body: %w", err)
	}

	return body, classifyHTTPStatus(resp.StatusCode, body)
}

// classifyHTTPStatus maps the response status (and body, for the
// disambiguation between "server error" and "API disabled" on
// 503) onto a typed sentinel. Anything ≥ 400 surfaces an error;
// 200-class returns nil so the caller knows to decode the body.
func classifyHTTPStatus(status int, body []byte) error {
	switch {
	case status >= 200 && status < 300:
		return nil
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return ErrAuthenticationFailed
	case status == http.StatusTooManyRequests:
		return ErrRateLimited
	case status == http.StatusNotFound:
		// DA returns 404 on every `/api/*` endpoint when the
		// Live API surface is disabled at the panel level. The
		// canonical CMD_API endpoints still work in that case,
		// but Sprint 23 doesn't ship a legacy adapter — we
		// surface the typed sentinel so the doctor CLI can
		// suggest the right remediation.
		return ErrAPIDisabled
	case status == http.StatusServiceUnavailable:
		// 503 splits two ways: DA's "Live API is disabled"
		// canonical body vs. a transient panel restart. The
		// body sniffing is cheap and the heuristic is the same
		// DA-recommended pattern from their docs.
		if isAPIDisabledBody(body) {
			return ErrAPIDisabled
		}
		return fmt.Errorf("%w: HTTP %d", ErrServerError, status)
	case status >= http.StatusInternalServerError:
		return fmt.Errorf("%w: HTTP %d", ErrServerError, status)
	default:
		return fmt.Errorf("%w: %d", ErrUnexpectedHTTPStatus, status)
	}
}

// isAPIDisabledBody scans the 503 body for DA's canonical "Live
// API disabled" wording. The phrases come from public DA support
// threads (JBMC docs link out to them under "Why is /api/ returning 503?").
// Defensive: the match is lowercase-substring so phrasing drift
// across DA versions is tolerated.
func isAPIDisabledBody(body []byte) bool {
	low := strings.ToLower(string(body))
	for _, phrase := range []string{
		"live api",
		"api disabled",
		"api is disabled",
		"feature is not available",
	} {
		if strings.Contains(low, phrase) {
			return true
		}
	}
	return false
}

// shouldRetry reports whether the error is worth another attempt.
// Mirrors the cpanel/uapi policy:
//   - Transient: rate-limit, server error, raw http.Client errors
//     (DNS / dial / TLS handshake) — all wrapped under
//     ErrTransportUnavailable by `singleCall`.
//   - Terminal: auth, malformed body, API disabled, unexpected
//     HTTP status. Retrying these just burns the backoff budget
//     with no benefit.
func shouldRetry(err error) bool {
	switch {
	case errors.Is(err, ErrRateLimited), errors.Is(err, ErrServerError):
		return true
	case errors.Is(err, ErrAuthenticationFailed),
		errors.Is(err, ErrMalformedResponse),
		errors.Is(err, ErrAPIDisabled),
		errors.Is(err, ErrUnexpectedHTTPStatus):
		return false
	}
	// Wrapped http.Client errors (DNS, dial, TLS handshake) are
	// transient — retry once on the assumption a packet drop
	// caused the failure.
	return true
}

// backoff computes the wait time for attempt n (1-indexed):
// 500ms, 1s, 2s. Capped by maxRetries=3 so the total worst-case
// wall-clock is ~3.5s for a request that ultimately fails.
func backoff(attempt int) time.Duration {
	mult := math.Pow(backoffMultiplier, float64(attempt-1))
	return time.Duration(float64(backoffBaseMS)*mult) * time.Millisecond
}
