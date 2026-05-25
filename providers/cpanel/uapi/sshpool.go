package uapi

import (
	"context"
	"errors"
	"fmt"

	cryptossh "golang.org/x/crypto/ssh"

	wssh "github.com/dilitS/webox/ssh"
)

// SSHPoolRunner adapts the project's pooled SSH client
// ([wssh.Pool] + [wssh.Target]) to the [SSHRunner] seam. It is
// the production wiring used by the future cPanel adapter; tests
// stay on an in-memory fake.
//
// The runner does not retry on its own — the SSH pool already
// implements DESIGN §9 reconnect logic, and TASK-21.2's caller is
// `webox doctor cpanel`, which surfaces transient failures back to
// the operator rather than blindly retrying.
type SSHPoolRunner struct {
	Pool   *wssh.Pool
	Target wssh.Target
}

// Run satisfies [SSHRunner] by delegating to [wssh.Exec]. The
// returned exit code is the SSH-reported command exit status (-1
// on dial / session failure, real exit code otherwise). Non-zero
// exits surface as (stdout, stderr, exitCode, nil) so the
// [SSHFallback] decides whether the failure is "feature disabled"
// or a generic API error; genuine transport errors (dial / session
// failure) surface as [ErrTransportUnavailable].
func (r *SSHPoolRunner) Run(ctx context.Context, command string) (stdout, stderr []byte, exitCode int, err error) {
	if r.Pool == nil {
		return nil, nil, -1, ErrTransportUnavailable
	}
	result, execErr := wssh.Exec(ctx, r.Pool, r.Target, command)
	if execErr == nil {
		return result.Stdout, result.Stderr, result.ExitCode, nil
	}
	var exitErr *cryptossh.ExitError
	if errors.As(execErr, &exitErr) {
		return result.Stdout, result.Stderr, result.ExitCode, nil
	}
	return result.Stdout, result.Stderr, result.ExitCode, fmt.Errorf("%w: %w", ErrTransportUnavailable, execErr)
}
