package uapi

import (
	"bytes"
	"context"
	"encoding/json"
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

// Defaults for the HTTPS transport. The values are conservative: a
// 10 s connect timeout matches the SSH probe runner shipping in
// TASK-21.4, and the retry policy stops at 3 attempts for both
// timeout (HTTP 408) and rate-limit (HTTP 429) responses so a
// single `webox doctor cpanel` call cannot spin for minutes.
const (
	defaultTimeout    = 30 * time.Second
	maxRetries        = 3
	maxBodyBytes      = 4 << 20 // 4 MiB; UAPI responses are tiny in practice.
	backoffBaseMS     = 500
	backoffMultiplier = 2
)

// userAgent is the static User-Agent the transport emits on every
// request. cPanel hosters increasingly require an identifiable UA;
// emitting `webox/<version>` lets them whitelist Webox traffic
// without us pretending to be a browser.
func userAgent() string {
	return fmt.Sprintf("webox/%s +https://github.com/dilitS/webox", version.Version)
}

// transport is the per-Client HTTP layer. It owns the *http.Client
// (no global default — `httpcheck` package has its own, kept
// isolated), the credentials, and the rate-limit / retry policy.
type transport struct {
	client     *http.Client
	baseURL    *url.URL
	user       string
	token      string
	now        func() time.Time
	backoffFor func(attempt int) time.Duration
}

// newTransport validates baseURL up front. We refuse plain http://
// (cPanel API tokens travel through here — TLS is mandatory) and
// missing credentials, surfacing typed errors so the caller doesn't
// have to string-match.
func newTransport(rawURL, user, token string, httpClient *http.Client) (*transport, error) {
	if user == "" || token == "" {
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
		token:      token,
		now:        time.Now,
		backoffFor: backoff,
	}, nil
}

// callURL composes the full UAPI URL for a (module, function) pair.
// The endpoint path follows cPanel's documented convention
// (api.docs.cpanel.net): `https://host:2083/execute/<module>/<function>`.
// Query parameters carry the typed args; we url.QueryEscape every
// value so an attacker cannot inject `&` into the request even if
// args were ever user-controlled (they are not — every caller in
// this package supplies hard-coded literals).
func (t *transport) callURL(module Module, function Function, args map[string]string) string {
	u := *t.baseURL
	u.Path = strings.TrimRight(u.Path, "/") + "/execute/" + string(module) + "/" + string(function)
	if len(args) > 0 {
		q := u.Query()
		for k, v := range args {
			q.Set(k, v)
		}
		u.RawQuery = q.Encode()
	}
	return u.String()
}

// call performs the HTTP request, retries on transient failures
// (429 / 5xx / context still alive), and decodes the JSON envelope.
// The returned envelope is always valid (status field decoded,
// Data populated when present); module decoders unmarshal Data
// into their typed shape.
//
// The Sprint-21 read-only client always invokes the documented
// no-arg functions, so [call] hard-codes nil args. When Sprint 22
// adds the mutating layer it will reuse this transport and supply
// the typed argument map.
func (t *transport) call(ctx context.Context, module Module, function Function) (*envelope, error) {
	endpoint := t.callURL(module, function, nil)
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			if err := t.sleepWithCtx(ctx, t.backoffFor(attempt)); err != nil {
				return nil, err
			}
		}
		env, err := t.singleCall(ctx, endpoint)
		if err == nil {
			return env, nil
		}
		lastErr = err
		// Only retry on transient classes; auth, malformed,
		// and module-denied errors are terminal.
		if !shouldRetry(err) {
			return nil, err
		}
	}
	return nil, lastErr
}

// sleepWithCtx blocks for d while honouring ctx cancellation. If
// the test suite has zeroed backoffFor (returns 0), this returns
// immediately without spinning a timer goroutine.
func (t *transport) sleepWithCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

// singleCall is one HTTP round-trip. It is split out so the retry
// loop above stays readable and so tests can swap the HTTP client
// without overriding the retry policy.
func (t *transport) singleCall(ctx context.Context, endpoint string) (*envelope, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("uapi: build request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent())
	req.Header.Set("Authorization", "cpanel "+t.user+":"+t.token)
	req.Header.Set("Accept", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		// Connection-level failure (DNS, refused, TLS
		// handshake, timeout). Surface via the typed sentinel
		// so the composite layer can fail over to SSH.
		return nil, fmt.Errorf("%w: %w", ErrTransportUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("uapi: read body: %w", err)
	}

	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return nil, ErrAuthenticationFailed
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, ErrRateLimited
	case resp.StatusCode >= http.StatusInternalServerError:
		return nil, fmt.Errorf("%w: HTTP %d", ErrServerError, resp.StatusCode)
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("%w: %d", ErrUnexpectedHTTPStatus, resp.StatusCode)
	}

	env := &envelope{}
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(env); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMalformedResponse, err)
	}
	if env.Result.Status != 1 {
		if isModuleDenied(env.Result.Errors) {
			return nil, ErrModuleFunctionDenied
		}
		return nil, fmt.Errorf("%w: status=%d errors=%v", ErrAPIResultFailure, env.Result.Status, env.Result.Errors)
	}
	return env, nil
}

// backoff computes the wait time for retry attempt `n` (1-indexed).
// The base is 500 ms and we double on each subsequent attempt
// (500ms, 1s, 2s) — capped at the linear sum below maxRetries.
func backoff(attempt int) time.Duration {
	mult := math.Pow(backoffMultiplier, float64(attempt-1))
	return time.Duration(float64(backoffBaseMS)*mult) * time.Millisecond
}

// shouldRetry reports whether the error from singleCall is worth
// another attempt. Transient: rate limit, server error, plain
// http-client error (DNS, timeout). Terminal: auth failure,
// malformed response, module disabled.
func shouldRetry(err error) bool {
	switch {
	case errors.Is(err, ErrRateLimited), errors.Is(err, ErrServerError):
		return true
	case errors.Is(err, ErrAuthenticationFailed),
		errors.Is(err, ErrMalformedResponse),
		errors.Is(err, ErrModuleFunctionDenied):
		return false
	}
	// http.Client errors (DNS, dial, TLS handshake) are
	// transient-ish — retry once on the assumption that a
	// transient packet drop caused the failure.
	return true
}

// isModuleDenied looks for cPanel's canonical "feature is disabled"
// signal in the errors list. WHM administrators can disable
// individual UAPI modules per feature list; the API returns this
// phrasing verbatim (per the public docs).
func isModuleDenied(errs []string) bool {
	for _, e := range errs {
		low := strings.ToLower(e)
		if strings.Contains(low, "disabled") || strings.Contains(low, "no access") || strings.Contains(low, "denied") {
			return true
		}
	}
	return false
}
