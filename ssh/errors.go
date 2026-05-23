package ssh

import "errors"

// Sentinel errors exposed by the ssh package. Callers compare with
// [errors.Is] — never by string match — because higher layers (TUI,
// providers, doctor) branch on these to decide whether to retry,
// surface a TOFU prompt, or abort the operation entirely.
//
// Each sentinel is intentionally kept opaque (no exported fields). The
// internal callbacks wrap it with [fmt.Errorf] using the %w verb when
// they need to carry contextual metadata such as hostname or
// fingerprint, so that operator logs remain readable while
// `errors.Is(err, ErrHostKeyMismatch)` keeps working all the way up to
// the TUI alert layer.
var (
	// ErrPoolBusy is returned by ssh.Pool.Acquire when no slot becomes
	// available before the caller's context deadline. The caller
	// typically surfaces this as a transient "server busy" toast and
	// retries on next tick — it is not a host-side failure.
	ErrPoolBusy = errors.New("ssh: connection pool exhausted before context deadline")

	// ErrHostKeyUnknown signals that the offered host key is not yet
	// present in the local known_hosts store. TUI must trigger the
	// out-of-band fingerprint confirmation flow described in
	// docs/SECURITY.md §5.3 before allowing the connection.
	ErrHostKeyUnknown = errors.New("ssh: host key not in known_hosts (first connection)")

	// ErrHostKeyMismatch signals that the offered host key differs
	// from the entry already recorded in known_hosts. Per
	// docs/SECURITY.md §5.4 this is treated as a strict block — there
	// is no auto-accept path. The user must remove the known_hosts
	// entry by hand (or re-confirm via the TUI's phrase-confirm flow)
	// after verifying the new fingerprint out-of-band.
	ErrHostKeyMismatch = errors.New("ssh: host key mismatch — refusing connection (manual verification required)")

	// ErrReconnectExhausted is returned after the reconnect policy
	// (3 attempts with 3s/6s/12s backoff, see docs/DESIGN.md §9)
	// has consumed all retries without restoring the session. The
	// caller is expected to surface the last underlying error along
	// with this sentinel.
	ErrReconnectExhausted = errors.New("ssh: reconnect attempts exhausted")

	// ErrHostKeyDBRequired is returned by [BuildClientConfig] when the
	// caller forgets to wire a [HostKeyDB]. Returning a typed sentinel
	// (instead of [cryptossh.InsecureIgnoreHostKey]) is the seam that
	// makes the "strict block on mismatch" guardrail unforgeable —
	// every code path that builds a ClientConfig must explicitly opt
	// into a known_hosts implementation.
	ErrHostKeyDBRequired = errors.New("ssh: HostKeyDB is required — auto-accept is forbidden by SECURITY §5.2")
)
