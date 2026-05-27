package api

import (
	"context"
	"errors"
)

// Composite wraps two Reader implementations and prefers the
// primary, falling over to the secondary when the primary
// surfaces [ErrTransportUnavailable]. Every other error class —
// authentication, rate-limit, API-disabled, malformed-response —
// surfaces verbatim because the secondary would hit the same
// wall (same credentials, same panel, same upstream service).
//
// Production wiring (Sprint 23 doctor CLI):
//
//	primary   = HTTPS Client     (fast path, direct connection)
//	secondary = SSH SSHFallback  (works through restrictive firewalls)
//
// The composite holds no state of its own beyond the two
// pointers; it's safe to share across goroutines because every
// underlying Reader is concurrency-safe.
type Composite struct {
	Primary   Reader
	Secondary Reader
}

// Compile-time assertion that Composite satisfies Reader. Adding
// a method to Reader without also adding it here will break the
// build.
var _ Reader = (*Composite)(nil)

// NewComposite constructs a composite. Both Readers must be
// non-nil. We chose to return a typed error rather than relying
// on the methods to crash with a nil-pointer dereference — the
// failure mode is identical (the call cannot succeed), but the
// typed error surfaces the wiring bug to whatever caller is
// inspecting the construction site.
func NewComposite(primary, secondary Reader) (*Composite, error) {
	if primary == nil || secondary == nil {
		return nil, ErrCompositeRequiresBothReaders
	}
	return &Composite{Primary: primary, Secondary: secondary}, nil
}

// ErrCompositeRequiresBothReaders fires when [NewComposite] is
// called with a nil Primary or Secondary. Typed so the doctor
// CLI's startup wiring can `errors.Is` against it and surface a
// clear remediation ("doctor was asked for composite mode but
// only one transport is wired").
var ErrCompositeRequiresBothReaders = errors.New("directadmin/api: composite requires both Primary and Secondary readers")

// shouldFailover reports whether err from the Primary justifies
// trying the Secondary. Today: only ErrTransportUnavailable
// (DNS / refused / TLS / timeout). Sprint 21 retro for cpanel
// raised the question of also falling over on rate-limit and
// server-error; deferred because the threat model is different
// (rate-limit usually applies per-source-IP, but SSH+curl from
// the box uses the same upstream LB so it would also be rate-
// limited). If that calculus changes, this is the single
// chokepoint to flip.
func shouldFailover(err error) bool {
	return errors.Is(err, ErrTransportUnavailable)
}

// tryComposite is the generic dispatcher every Reader method
// forwards through. It calls Primary first, falls over to
// Secondary if the failure class qualifies, and surfaces the
// result.
//
// The function is generic over T (the result type) so each
// Reader method stays a one-line forward without runtime type
// assertions. The two captured ops are typed `func(Reader) (T, error)`
// so the compiler enforces the same signature on every call site.
func tryComposite[T any](c *Composite, op func(Reader) (T, error)) (T, error) {
	v, err := op(c.Primary)
	if err == nil {
		return v, nil
	}
	if !shouldFailover(err) {
		return v, err
	}
	return op(c.Secondary)
}

// Whoami satisfies Reader.
func (c *Composite) Whoami(ctx context.Context) (*WhoamiResponse, error) {
	return tryComposite(c, func(r Reader) (*WhoamiResponse, error) { return r.Whoami(ctx) })
}

// ListDomains satisfies Reader.
func (c *Composite) ListDomains(ctx context.Context) ([]Domain, error) {
	return tryComposite(c, func(r Reader) ([]Domain, error) { return r.ListDomains(ctx) })
}

// ListSubdomains satisfies Reader.
func (c *Composite) ListSubdomains(ctx context.Context) ([]Subdomain, error) {
	return tryComposite(c, func(r Reader) ([]Subdomain, error) { return r.ListSubdomains(ctx) })
}

// ListDatabases satisfies Reader.
func (c *Composite) ListDatabases(ctx context.Context) ([]Database, error) {
	return tryComposite(c, func(r Reader) ([]Database, error) { return r.ListDatabases(ctx) })
}

// ListSSLCertificates satisfies Reader.
func (c *Composite) ListSSLCertificates(ctx context.Context) ([]SSLCertificate, error) {
	return tryComposite(c, func(r Reader) ([]SSLCertificate, error) { return r.ListSSLCertificates(ctx) })
}
