package ssh

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"sync/atomic"
	"time"
)

// RetryableExecPolicy controls how [ExecWithRetry] backs off when the
// pool reports `ErrPoolBusy`. The defaults below mirror the SWR
// cache's freshness budget (a single status refresh has ~5 s before
// the operator perceives the spinner as stuck) and are deliberately
// short — `ErrPoolBusy` means the pool's MaxPerHost semaphore is
// saturated, which on small.pl resolves in tens of milliseconds.
//
// Backoff is jittered ±20 % to avoid the "thundering herd" pattern
// when the cockpit fires the periodic status refresh and every
// project goroutine wakes up on the same tick.
//
// The policy applies ONLY to `ErrPoolBusy`. Terminal errors —
// `ErrHostKeyMismatch`, `ErrHostKeyUnknown`, `ssh: handshake failed`,
// command exit codes — are returned immediately so the caller's
// idempotency / safety logic can decide what to do.
type RetryableExecPolicy struct {
	// Attempts is the maximum number of `Exec` invocations including
	// the first one. Values <2 disable retry (pure passthrough).
	Attempts int

	// BaseBackoff is the delay before the SECOND attempt; subsequent
	// attempts use `BaseBackoff * 2^n` with ±20 % jitter.
	BaseBackoff time.Duration

	// MaxBackoff caps the per-attempt sleep so a long tail does not
	// outlive the caller's context deadline. The context is still
	// authoritative — Sleep aborts immediately on ctx.Done.
	MaxBackoff time.Duration

	// Sleep is injectable so unit tests can replace it with an
	// instant returns. Defaults to a context-aware timer.
	Sleep SleepFunc

	// Rand is injectable for deterministic jitter in tests. Defaults
	// to a global rand source. The contract is `Rand() ∈ [0,1)`.
	Rand func() float64
}

// ExecMetrics is a lock-free, atomic-incremented set of counters
// surfaced via [PoolMetrics] for the `--debug-trace` JSONL stream
// (TASK-14.6). All counters are CUMULATIVE since process start. The
// type is intentionally simple — exposing it as a struct rather than
// a Prometheus collector keeps Webox dependency-free.
type ExecMetrics struct {
	// Acquires counts every successful `pool.Acquire` from
	// `ExecWithRetry`. Useful to compare against `Retries` to see
	// whether the pool is hot.
	Acquires atomic.Uint64

	// PoolBusyHits counts ErrPoolBusy responses across ALL attempts
	// (including those that later succeeded after a retry). A
	// non-zero value with stable Acquires means the cockpit is
	// over-issuing parallel ops — consider singleflight at the
	// caller.
	PoolBusyHits atomic.Uint64

	// Retries counts how many *additional* Exec attempts the helper
	// issued beyond the first one. `Retries == 0` and a non-zero
	// `PoolBusyHits` would indicate the policy is configured with
	// `Attempts: 1` (passthrough) — surfaced via doctor.
	Retries atomic.Uint64

	// TerminalErrors counts host-key / auth / handshake / command
	// failures that bypass the retry layer. Operator-visible spike
	// here usually means a credential rotation went wrong or a
	// provider outage.
	TerminalErrors atomic.Uint64
}

// Snapshot returns a value copy of the current counters. Reading
// each atomic individually is non-atomic at the struct level — by
// design — because we never need a perfectly consistent snapshot
// (drift between counters is at most one event behind real time).
func (m *ExecMetrics) Snapshot() ExecMetricsSnapshot {
	if m == nil {
		return ExecMetricsSnapshot{}
	}
	return ExecMetricsSnapshot{
		Acquires:       m.Acquires.Load(),
		PoolBusyHits:   m.PoolBusyHits.Load(),
		Retries:        m.Retries.Load(),
		TerminalErrors: m.TerminalErrors.Load(),
	}
}

// ExecMetricsSnapshot is the JSON-stable read view of the counters.
// Field names are short on purpose so the `--debug-trace` payload
// stays grep-friendly without consuming bandwidth.
type ExecMetricsSnapshot struct {
	Acquires       uint64 `json:"acquires"`
	PoolBusyHits   uint64 `json:"pool_busy_hits"`
	Retries        uint64 `json:"retries"`
	TerminalErrors uint64 `json:"terminal_errors"`
}

const (
	// defaultRetryAttempts mirrors the cockpit's tolerance for the
	// pool-busy backpressure case: enough retries to absorb a
	// burst of N parallel goroutines saturating MaxPerHost=3, few
	// enough that a genuinely stuck remote surfaces inside the
	// 5 s SWR freshness budget.
	defaultRetryAttempts    = 4
	defaultRetryBaseBackoff = 100 * time.Millisecond
	defaultRetryMaxBackoff  = 1 * time.Second
)

// DefaultRetryableExecPolicy returns the production-recommended
// policy: 4 attempts, 100 ms base / 1 s cap. With ±20 % jitter the
// total worst-case wall clock is ~2.3 s, comfortably inside the
// cockpit's 5 s status-refresh tick (DESIGN §8).
func DefaultRetryableExecPolicy() RetryableExecPolicy {
	return RetryableExecPolicy{
		Attempts:    defaultRetryAttempts,
		BaseBackoff: defaultRetryBaseBackoff,
		MaxBackoff:  defaultRetryMaxBackoff,
	}
}

// execFunc is the indirection seam ExecWithRetry uses so unit tests
// can stub the underlying pool.Exec call without spinning up a fake
// SSHMock server. Production code never assigns this; tests do via
// withFakeExec (see retry_test.go). Initialised lazily to break the
// initialisation cycle with the Exec function defined in exec.go.
var execFunc = Exec

// ExecWithRetry runs `command` through the pool and transparently
// retries when the pool reports `ErrPoolBusy`. Every other error
// class is terminal and returned immediately so the caller can
// distinguish "wait and try again" from "fix something and try
// again". When `metrics` is non-nil the helper updates its atomic
// counters for observability.
//
// Idempotency contract: callers MUST only pass commands that are
// safe to repeat (typically read-only ops like `node --version` or
// `tail`). State-mutating commands MUST use [Exec] directly so the
// provider's parser can inspect the remote side after the first
// attempt before deciding whether to replay (DESIGN §9).
func ExecWithRetry(
	ctx context.Context,
	pool *Pool,
	target Target,
	command string,
	policy RetryableExecPolicy,
	metrics *ExecMetrics,
) (ExecResult, error) {
	policy = normalizeRetryableExecPolicy(policy)

	var lastErr error
	var lastResult ExecResult
	for attempt := range policy.Attempts {
		if err := ctx.Err(); err != nil {
			return lastResult, err
		}
		result, err := execFunc(ctx, pool, target, command)
		switch {
		case err == nil:
			if metrics != nil {
				metrics.Acquires.Add(1)
			}
			return result, nil
		case errors.Is(err, ErrPoolBusy):
			if metrics != nil {
				metrics.PoolBusyHits.Add(1)
			}
			lastErr = err
			lastResult = result
			if attempt == policy.Attempts-1 {
				return result, fmt.Errorf("exec %q: %w (after %d attempts)",
					command, err, policy.Attempts)
			}
			backoff := jitteredBackoff(policy, attempt)
			if metrics != nil {
				metrics.Retries.Add(1)
			}
			if sleepErr := policy.Sleep(ctx, backoff); sleepErr != nil {
				return result, sleepErr
			}
		default:
			if metrics != nil {
				metrics.TerminalErrors.Add(1)
			}
			return result, err
		}
	}
	return lastResult, lastErr
}

// jitteredBackoff computes `BaseBackoff * 2^attempt` then applies a
// ±20 % jitter and clamps at MaxBackoff. Pure function for testability.
func jitteredBackoff(policy RetryableExecPolicy, attempt int) time.Duration {
	exp := policy.BaseBackoff << attempt
	if exp <= 0 || exp > policy.MaxBackoff {
		exp = policy.MaxBackoff
	}
	if policy.Rand == nil {
		return exp
	}
	jitter := (policy.Rand()*0.4 - 0.2) //nolint:mnd // ±20 % spread documented above.
	scaled := float64(exp) * (1 + jitter)
	if scaled < 0 {
		scaled = 0
	}
	return time.Duration(scaled)
}

func normalizeRetryableExecPolicy(p RetryableExecPolicy) RetryableExecPolicy {
	if p.Attempts <= 0 {
		p.Attempts = DefaultRetryableExecPolicy().Attempts
	}
	if p.BaseBackoff <= 0 {
		p.BaseBackoff = DefaultRetryableExecPolicy().BaseBackoff
	}
	if p.MaxBackoff <= 0 {
		p.MaxBackoff = DefaultRetryableExecPolicy().MaxBackoff
	}
	if p.Sleep == nil {
		p.Sleep = sleepContext
	}
	if p.Rand == nil {
		p.Rand = rand.Float64
	}
	return p
}
