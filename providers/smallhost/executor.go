package smallhost

import (
	"context"
	"errors"
	"fmt"

	"github.com/dilitS/webox/providers"
	wssh "github.com/dilitS/webox/ssh"
)

// Executor is the narrow seam between the smallhost adapter and the
// SSH transport. Production wiring (see [NewSSHExecutor]) delegates to
// ssh.Exec against a shared connection pool; tests substitute a
// deterministic in-memory recorder so the parser ↔ method coupling
// can be exercised without a live SSH server.
//
// The interface deliberately exposes only the command-execution
// surface — connection lifecycle, keepalives, and reconnects live
// inside ssh/Pool. Adapters stay decoupled from those details and
// keep their command surface auditable.
type Executor interface {
	// Exec runs command on the remote host and returns the captured
	// stdout/stderr/exit-status. ctx cancellation MUST close the
	// remote session promptly. Implementations MUST NOT log the
	// command verbatim — adapters guarantee the command tokens are
	// already whitelisted but logging at the transport layer would
	// duplicate the redactor work in `internal/log`.
	Exec(ctx context.Context, command string) (wssh.ExecResult, error)
}

// errExecutorNotConfigured is the sentinel returned by adapter
// methods when [Provider.SetExecutor] was never called. It is wrapped
// with providers.ErrUnknownOutputFormat at the call site so higher
// layers treat the missing wiring the same way they treat a panel
// outage — fail-closed.
var errExecutorNotConfigured = errors.New("smallhost: executor not configured (call SetExecutor)")

// SetExecutor installs the [Executor] the adapter uses for SSH
// commands. The wizard layer wires this in once per profile (using
// the shared ssh.Pool); tests wire it in per-test (using
// [NewFakeExecutor]).
//
// Passing a nil executor is treated as "unconfigure" — useful for
// tests that want to assert the fail-closed branch.
func (p *Provider) SetExecutor(e Executor) { p.executor = e }

// NewSSHExecutor adapts the package-level ssh.Exec into the [Executor]
// surface required by smallhost. The pool and target are captured at
// construction; consequently every Exec call routes through the same
// pool bucket, which is what `ssh_pool_max` is meant to enforce.
//
// The returned executor is safe for concurrent use because ssh.Exec
// + Pool are: Acquire is bounded by the per-host semaphore, Release
// returns the client immediately after the session exits.
func NewSSHExecutor(pool *wssh.Pool, target wssh.Target) Executor {
	return &sshExecutor{pool: pool, target: target}
}

type sshExecutor struct {
	pool   *wssh.Pool
	target wssh.Target
}

// Exec satisfies [Executor] by delegating to ssh.Exec against the
// pool + target captured at construction. The smallhost adapter
// owns no transport state of its own — every retry / reconnect
// decision lives inside ssh.Pool.
func (e *sshExecutor) Exec(ctx context.Context, command string) (wssh.ExecResult, error) {
	return wssh.Exec(ctx, e.pool, e.target, command)
}

// exec is the in-adapter helper that surfaces "executor not
// configured" as a typed error. Methods call it instead of touching
// p.executor directly so the nil-check has exactly one home.
func (p *Provider) exec(ctx context.Context, command string) (wssh.ExecResult, error) {
	if p.executor == nil {
		return wssh.ExecResult{}, fmt.Errorf("%w: %w", providers.ErrUnknownOutputFormat, errExecutorNotConfigured)
	}
	return p.executor.Exec(ctx, command)
}
