package smallhost

import (
	"context"
	"errors"
	"fmt"

	"github.com/dilitS/webox/providers"
)

// errNotImplemented is the stand-in returned by adapter methods that
// will be implemented in TASK-03.6 (parser ↔ ssh.Exec wiring). It
// wraps providers.ErrUnknownOutputFormat so the type assertions in
// higher layers (wizard, status loop) still work — they will treat
// the stub as if the panel returned an unparseable response, which
// matches the "fail closed" rule from SECURITY §3.3.
//
// Tests for the adapter MUST NOT see this error past TASK-03.6.
var errNotImplemented = errors.New("smallhost: method not implemented yet (TASK-03.6)")

// CreateSubdomain is implemented in TASK-03.6.
func (p *Provider) CreateSubdomain(_ context.Context, _, _ string) error {
	return fmt.Errorf("%w: %w", providers.ErrUnknownOutputFormat, errNotImplemented)
}

// SetupSSL is implemented in TASK-03.6.
func (p *Provider) SetupSSL(_ context.Context, _ string) error {
	return fmt.Errorf("%w: %w", providers.ErrUnknownOutputFormat, errNotImplemented)
}

// CreateDatabase is implemented in TASK-03.6.
func (p *Provider) CreateDatabase(_ context.Context, _, _ string) (user, password string, err error) {
	return "", "", fmt.Errorf("%w: %w", providers.ErrUnknownOutputFormat, errNotImplemented)
}

// RestartNodeApp is implemented in TASK-03.6.
func (p *Provider) RestartNodeApp(_ context.Context, _ string) error {
	return fmt.Errorf("%w: %w", providers.ErrUnknownOutputFormat, errNotImplemented)
}

// GetDeployPath is implemented in TASK-03.3.
func (p *Provider) GetDeployPath(_ string) string { return "" }

// GetLogPath is implemented in TASK-03.3.
func (p *Provider) GetLogPath(_ string) string { return "" }

// CheckStatus is implemented in TASK-03.6.
func (p *Provider) CheckStatus(_ context.Context) (*providers.ProviderStatus, error) {
	return nil, fmt.Errorf("%w: %w", providers.ErrUnknownOutputFormat, errNotImplemented)
}

// ListSubdomains is implemented in TASK-03.6.
func (p *Provider) ListSubdomains(_ context.Context) ([]providers.Subdomain, error) {
	return nil, fmt.Errorf("%w: %w", providers.ErrUnknownOutputFormat, errNotImplemented)
}

// RemoveSubdomain is implemented in TASK-03.6.
func (p *Provider) RemoveSubdomain(_ context.Context, _ string) error {
	return fmt.Errorf("%w: %w", providers.ErrUnknownOutputFormat, errNotImplemented)
}

// RemoveDatabase is implemented in TASK-03.6.
func (p *Provider) RemoveDatabase(_ context.Context, _, _ string) error {
	return fmt.Errorf("%w: %w", providers.ErrUnknownOutputFormat, errNotImplemented)
}

// RemoveSSL is implemented in TASK-03.6.
func (p *Provider) RemoveSSL(_ context.Context, _ string) error {
	return fmt.Errorf("%w: %w", providers.ErrUnknownOutputFormat, errNotImplemented)
}
