package uapi

import (
	"context"
	"errors"
)

// Reader is the closed read-only interface both [Client] and
// [SSHFallback] satisfy. The composite client and the future
// cPanel adapter speak to this interface so swapping transports
// requires no caller change.
//
// Only the four Sprint 21 modules appear here; new read-only
// methods land in this interface explicitly (and then in both
// transports) to keep the type system enforcing parity.
type Reader interface {
	ListDomains(ctx context.Context) (*DomainInfoListResponse, error)
	ListPassengerApps(ctx context.Context) (*PassengerAppsListResponse, error)
	ListMysqlDatabases(ctx context.Context) (*MysqlListDatabasesResponse, error)
	ListSSLKeys(ctx context.Context) (*SSLListKeysResponse, error)

	// Transport returns a short, stable label for the transport
	// powering this Reader. Used by `webox doctor cpanel` to render
	// the transport hint next to each section without resorting to
	// a runtime type-switch in the caller. Stable label values:
	// "HTTPS", "SSH", "HTTPS+SSH" (composite with both wired), "?"
	// (composite with neither wired — a programmer error).
	Transport() string
}

// Composite tries Primary first; on
// errors.Is(err, ErrTransportUnavailable) it falls back to
// Secondary. Any other error (auth failure, rate-limit,
// malformed body, module disabled) surfaces verbatim — those are
// not transport-level failures and the secondary would hit the
// same wall.
//
// Either Primary or Secondary may be nil — a nil entry is treated
// as "not configured", and the composite returns its error
// verbatim from whichever side is wired. Both nil is an explicit
// programmer error and returns [ErrTransportUnavailable].
type Composite struct {
	Primary   Reader
	Secondary Reader
}

// ListDomains satisfies [Reader].
func (c *Composite) ListDomains(ctx context.Context) (*DomainInfoListResponse, error) {
	return tryComposite(c, func(r Reader) (*DomainInfoListResponse, error) { return r.ListDomains(ctx) })
}

// ListPassengerApps satisfies [Reader].
func (c *Composite) ListPassengerApps(ctx context.Context) (*PassengerAppsListResponse, error) {
	return tryComposite(c, func(r Reader) (*PassengerAppsListResponse, error) { return r.ListPassengerApps(ctx) })
}

// ListMysqlDatabases satisfies [Reader].
func (c *Composite) ListMysqlDatabases(ctx context.Context) (*MysqlListDatabasesResponse, error) {
	return tryComposite(c, func(r Reader) (*MysqlListDatabasesResponse, error) { return r.ListMysqlDatabases(ctx) })
}

// ListSSLKeys satisfies [Reader].
func (c *Composite) ListSSLKeys(ctx context.Context) (*SSLListKeysResponse, error) {
	return tryComposite(c, func(r Reader) (*SSLListKeysResponse, error) { return r.ListSSLKeys(ctx) })
}

// Transport satisfies [Reader]. Renders the wiring snapshot:
//   - both legs configured → "HTTPS+SSH"
//   - only Primary → whatever Primary reports (typically "HTTPS")
//   - only Secondary → whatever Secondary reports (typically "SSH")
//   - neither → "?" (matches the programmer-error path in tryComposite)
//
// Delegating to the underlying Reader's Transport instead of
// hardcoding strings means a future read-only adapter that surfaces
// a third transport (e.g. CLI plugin) will plug in without touching
// this composite.
func (c *Composite) Transport() string {
	switch {
	case c.Primary != nil && c.Secondary != nil:
		return c.Primary.Transport() + "+" + c.Secondary.Transport()
	case c.Primary != nil:
		return c.Primary.Transport()
	case c.Secondary != nil:
		return c.Secondary.Transport()
	default:
		return "?"
	}
}

// tryComposite runs `op` against c.Primary; on
// ErrTransportUnavailable it retries against c.Secondary. Either
// transport returning anything else (including success) is
// surfaced verbatim. The Go generic keeps the dispatcher type-safe
// without runtime assertions.
func tryComposite[T any](c *Composite, op func(Reader) (T, error)) (T, error) {
	var zero T
	if c.Primary == nil && c.Secondary == nil {
		return zero, ErrTransportUnavailable
	}
	if c.Primary != nil {
		out, err := op(c.Primary)
		if err == nil {
			return out, nil
		}
		if !errors.Is(err, ErrTransportUnavailable) {
			return zero, err
		}
		if c.Secondary == nil {
			return zero, err
		}
	}
	return op(c.Secondary)
}

// Compile-time assertions that the three implementations satisfy
// [Reader]. If a new method is added to the interface, the
// build breaks immediately on the missing implementation.
var (
	_ Reader = (*Client)(nil)
	_ Reader = (*SSHFallback)(nil)
	_ Reader = (*Composite)(nil)
)
