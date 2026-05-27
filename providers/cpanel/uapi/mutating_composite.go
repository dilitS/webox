package uapi

import (
	"context"
	"errors"
)

// CompositeMutator wraps two [Mutator] implementations and tries
// Primary first; on errors.Is(err, ErrTransportUnavailable) it
// fails over to Secondary. Same semantics as [Composite] for the
// read-only path, deliberately copy-pasted (rather than generified
// further) so mutating code stays grep-friendly — auditors looking
// for "what happens when the HTTPS API token is rate-limited
// during a CreateProject" land on a single file with explicit
// method bodies rather than chasing through a third-level generic
// dispatcher.
//
// Composite fall-over is restricted to transport-level failures
// because every other error class would hit the same wall on the
// alternate transport: an auth failure won't disappear by
// switching from HTTPS to SSH, a module-disabled flag is enforced
// at the panel layer.
type CompositeMutator struct {
	Primary   Mutator
	Secondary Mutator
}

// AddAddonDomain satisfies [Mutator].
func (c *CompositeMutator) AddAddonDomain(ctx context.Context, args CreateAddonDomainArgs) error {
	return tryMutate(c, func(m Mutator) error { return m.AddAddonDomain(ctx, args) })
}

// AddSubdomain satisfies [Mutator].
func (c *CompositeMutator) AddSubdomain(ctx context.Context, args CreateSubdomainArgs) error {
	return tryMutate(c, func(m Mutator) error { return m.AddSubdomain(ctx, args) })
}

// DeleteDomain satisfies [Mutator].
func (c *CompositeMutator) DeleteDomain(ctx context.Context, domain string) error {
	return tryMutate(c, func(m Mutator) error { return m.DeleteDomain(ctx, domain) })
}

// DeleteSubdomain satisfies [Mutator].
func (c *CompositeMutator) DeleteSubdomain(ctx context.Context, fqSubdomain string) error {
	return tryMutate(c, func(m Mutator) error { return m.DeleteSubdomain(ctx, fqSubdomain) })
}

// CreatePassengerApp satisfies [Mutator].
func (c *CompositeMutator) CreatePassengerApp(ctx context.Context, args CreatePassengerAppArgs) error {
	return tryMutate(c, func(m Mutator) error { return m.CreatePassengerApp(ctx, args) })
}

// EditPassengerApp satisfies [Mutator].
func (c *CompositeMutator) EditPassengerApp(ctx context.Context, args EditPassengerAppArgs) error {
	return tryMutate(c, func(m Mutator) error { return m.EditPassengerApp(ctx, args) })
}

// RestartPassengerApp satisfies [Mutator].
func (c *CompositeMutator) RestartPassengerApp(ctx context.Context, appPath string) error {
	return tryMutate(c, func(m Mutator) error { return m.RestartPassengerApp(ctx, appPath) })
}

// DeletePassengerApp satisfies [Mutator].
func (c *CompositeMutator) DeletePassengerApp(ctx context.Context, appPath string) error {
	return tryMutate(c, func(m Mutator) error { return m.DeletePassengerApp(ctx, appPath) })
}

// CreateMysqlDatabase satisfies [Mutator].
func (c *CompositeMutator) CreateMysqlDatabase(ctx context.Context, dbName string) error {
	return tryMutate(c, func(m Mutator) error { return m.CreateMysqlDatabase(ctx, dbName) })
}

// DeleteMysqlDatabase satisfies [Mutator].
func (c *CompositeMutator) DeleteMysqlDatabase(ctx context.Context, dbName string) error {
	return tryMutate(c, func(m Mutator) error { return m.DeleteMysqlDatabase(ctx, dbName) })
}

// CreateMysqlUser satisfies [Mutator].
func (c *CompositeMutator) CreateMysqlUser(ctx context.Context, user, password string) error {
	return tryMutate(c, func(m Mutator) error { return m.CreateMysqlUser(ctx, user, password) })
}

// DeleteMysqlUser satisfies [Mutator].
func (c *CompositeMutator) DeleteMysqlUser(ctx context.Context, user string) error {
	return tryMutate(c, func(m Mutator) error { return m.DeleteMysqlUser(ctx, user) })
}

// SetMysqlPrivileges satisfies [Mutator].
func (c *CompositeMutator) SetMysqlPrivileges(ctx context.Context, args MysqlPrivilegesArgs) error {
	return tryMutate(c, func(m Mutator) error { return m.SetMysqlPrivileges(ctx, args) })
}

// InstallSSL satisfies [Mutator].
func (c *CompositeMutator) InstallSSL(ctx context.Context, args InstallSSLArgs) error {
	return tryMutate(c, func(m Mutator) error { return m.InstallSSL(ctx, args) })
}

// StartAutoSSL satisfies [Mutator].
func (c *CompositeMutator) StartAutoSSL(ctx context.Context, domain string) error {
	return tryMutate(c, func(m Mutator) error { return m.StartAutoSSL(ctx, domain) })
}

// DeleteSSL satisfies [Mutator].
func (c *CompositeMutator) DeleteSSL(ctx context.Context, host string) error {
	return tryMutate(c, func(m Mutator) error { return m.DeleteSSL(ctx, host) })
}

// tryMutate runs op against c.Primary; on ErrTransportUnavailable
// it retries against c.Secondary. Every other error (success
// included) surfaces verbatim. The composite intentionally does
// NOT propagate ErrMutationsDisabled to the secondary because both
// transports share the same env-var guard — a "disabled" signal
// on Primary would repeat identically on Secondary.
func tryMutate(c *CompositeMutator, op func(Mutator) error) error {
	if c.Primary == nil && c.Secondary == nil {
		return ErrTransportUnavailable
	}
	if c.Primary != nil {
		err := op(c.Primary)
		if err == nil {
			return nil
		}
		if !errors.Is(err, ErrTransportUnavailable) {
			return err
		}
		if c.Secondary == nil {
			return err
		}
	}
	return op(c.Secondary)
}

// Compile-time assertions that the three mutators satisfy the
// [Mutator] interface. If a new method is added to the interface,
// the build breaks immediately on the missing implementation.
var (
	_ Mutator = (*HTTPSMutator)(nil)
	_ Mutator = (*SSHMutator)(nil)
	_ Mutator = (*CompositeMutator)(nil)
)
