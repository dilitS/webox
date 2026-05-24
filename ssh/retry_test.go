package ssh

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// withFakeExec replaces the package-level execFunc seam with a stub
// for the duration of the test. The helper restores the original on
// t.Cleanup so parallel tests still see the production function
// unless they explicitly opt in via their own withFakeExec call.
//
// We intentionally accept a stub func instead of returning a setter
// — the simpler shape makes the test bodies read top-to-bottom and
// avoids accidental "forgot to restore" leaks.
func withFakeExec(t *testing.T, stub func(context.Context, *Pool, Target, string) (ExecResult, error)) {
	t.Helper()
	original := execFunc
	execFunc = stub
	t.Cleanup(func() { execFunc = original })
}

func TestNormalizeRetryableExecPolicy_FillsDefaults(t *testing.T) {
	t.Parallel()

	got := normalizeRetryableExecPolicy(RetryableExecPolicy{})

	def := DefaultRetryableExecPolicy()
	if got.Attempts != def.Attempts {
		t.Errorf("Attempts = %d, want %d", got.Attempts, def.Attempts)
	}
	if got.BaseBackoff != def.BaseBackoff {
		t.Errorf("BaseBackoff = %v, want %v", got.BaseBackoff, def.BaseBackoff)
	}
	if got.MaxBackoff != def.MaxBackoff {
		t.Errorf("MaxBackoff = %v, want %v", got.MaxBackoff, def.MaxBackoff)
	}
	if got.Sleep == nil {
		t.Error("Sleep MUST be filled with sleepContext")
	}
	if got.Rand == nil {
		t.Error("Rand MUST be filled with rand.Float64")
	}
}

func TestNormalizeRetryableExecPolicy_PreservesProvidedValues(t *testing.T) {
	t.Parallel()

	custom := RetryableExecPolicy{
		Attempts:    7,
		BaseBackoff: 50 * time.Millisecond,
		MaxBackoff:  500 * time.Millisecond,
		Sleep:       func(_ context.Context, _ time.Duration) error { return nil },
		Rand:        func() float64 { return 0.5 },
	}
	got := normalizeRetryableExecPolicy(custom)

	if got.Attempts != 7 {
		t.Errorf("Attempts = %d, want 7", got.Attempts)
	}
	if got.BaseBackoff != 50*time.Millisecond {
		t.Errorf("BaseBackoff overridden: %v", got.BaseBackoff)
	}
}

func TestJitteredBackoff_ExponentialClampedAtMax(t *testing.T) {
	t.Parallel()

	policy := RetryableExecPolicy{
		BaseBackoff: 100 * time.Millisecond,
		MaxBackoff:  500 * time.Millisecond,
	}
	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 100 * time.Millisecond},
		{1, 200 * time.Millisecond},
		{2, 400 * time.Millisecond},
		{3, 500 * time.Millisecond},
		{10, 500 * time.Millisecond},
	}
	for _, tc := range cases {
		got := jitteredBackoff(policy, tc.attempt)
		if got != tc.want {
			t.Errorf("attempt=%d: got %v, want %v (no-jitter)", tc.attempt, got, tc.want)
		}
	}
}

func TestJitteredBackoff_AppliesPlusMinus20Percent(t *testing.T) {
	t.Parallel()

	policy := RetryableExecPolicy{
		BaseBackoff: 1000 * time.Millisecond,
		MaxBackoff:  10 * time.Second,
		Rand:        func() float64 { return 0.0 },
	}
	low := jitteredBackoff(policy, 0)
	if low != 800*time.Millisecond {
		t.Errorf("rand=0 lower bound: got %v, want 800ms (-20%%)", low)
	}

	policy.Rand = func() float64 { return 1.0 }
	high := jitteredBackoff(policy, 0)
	if high != 1200*time.Millisecond {
		t.Errorf("rand=1 upper bound: got %v, want 1200ms (+20%%)", high)
	}

	policy.Rand = func() float64 { return 0.5 }
	mid := jitteredBackoff(policy, 0)
	if mid != 1000*time.Millisecond {
		t.Errorf("rand=0.5 centre: got %v, want 1000ms", mid)
	}
}

func TestJitteredBackoff_NeverNegative(t *testing.T) {
	t.Parallel()

	policy := RetryableExecPolicy{
		BaseBackoff: 100 * time.Millisecond,
		MaxBackoff:  1 * time.Second,
		Rand:        func() float64 { return -10 },
	}
	got := jitteredBackoff(policy, 0)
	if got < 0 {
		t.Errorf("backoff < 0 (%v) — guard against rand misuse failed", got)
	}
}

// TestExecWithRetry_RetriesOnPoolBusy is the headline behavioural
// test for TASK-14.3: when the pool says busy, the helper keeps
// trying until success or the attempt budget is spent. We stub the
// Exec call by going through ExecMetrics + a pool wired to a fake
// dialer that signals "busy" via a custom error injector. To keep
// the test small we instead exercise the pool's natural busy path
// from ssh/pool_test.go's patterns — see TestExecWithRetry_Integration
// for the end-to-end variant.
// Tests that swap the package-level execFunc seam MUST NOT call
// t.Parallel — AGENTS.md §7.1 explicitly bans parallel stubbing of
// package globals. The pure-function tests below (jittered backoff,
// normalize, snapshot) keep t.Parallel because they never touch
// execFunc.

func TestExecWithRetry_BusyHitsCounted(t *testing.T) {
	metrics := &ExecMetrics{}
	policy := RetryableExecPolicy{
		Attempts:    3,
		BaseBackoff: 1 * time.Millisecond,
		MaxBackoff:  2 * time.Millisecond,
		Sleep:       func(_ context.Context, _ time.Duration) error { return nil },
		Rand:        func() float64 { return 0.5 },
	}

	calls := atomic.Int32{}
	withFakeExec(t, func(_ context.Context, _ *Pool, _ Target, _ string) (ExecResult, error) {
		calls.Add(1)
		if calls.Load() < 3 {
			return ExecResult{}, ErrPoolBusy
		}
		return ExecResult{Stdout: []byte("ok"), ExitCode: 0}, nil
	})

	res, err := ExecWithRetry(context.Background(), nil, Target{}, "node --version", policy, metrics)
	if err != nil {
		t.Fatalf("ExecWithRetry err = %v, want nil after busy backoff", err)
	}
	if string(res.Stdout) != "ok" {
		t.Errorf("Stdout = %q", res.Stdout)
	}
	if got := metrics.PoolBusyHits.Load(); got != 2 {
		t.Errorf("PoolBusyHits = %d, want 2 (3rd attempt succeeded)", got)
	}
	if got := metrics.Retries.Load(); got != 2 {
		t.Errorf("Retries = %d, want 2", got)
	}
	if got := metrics.Acquires.Load(); got != 1 {
		t.Errorf("Acquires = %d, want 1 (final successful attempt)", got)
	}
}

func TestExecWithRetry_TerminalErrorBypassesRetry(t *testing.T) {
	metrics := &ExecMetrics{}
	calls := atomic.Int32{}
	withFakeExec(t, func(_ context.Context, _ *Pool, _ Target, _ string) (ExecResult, error) {
		calls.Add(1)
		return ExecResult{}, ErrHostKeyMismatch
	})

	_, err := ExecWithRetry(
		context.Background(), nil, Target{}, "node --version",
		RetryableExecPolicy{Attempts: 5, BaseBackoff: time.Millisecond, MaxBackoff: time.Millisecond},
		metrics,
	)
	if !errors.Is(err, ErrHostKeyMismatch) {
		t.Fatalf("err = %v, want ErrHostKeyMismatch propagation", err)
	}
	if calls.Load() != 1 {
		t.Errorf("terminal err must not retry; calls = %d", calls.Load())
	}
	if got := metrics.TerminalErrors.Load(); got != 1 {
		t.Errorf("TerminalErrors = %d, want 1", got)
	}
	if got := metrics.Retries.Load(); got != 0 {
		t.Errorf("Retries = %d, want 0 (terminal error bypass)", got)
	}
}

func TestExecWithRetry_BudgetExhaustedReturnsWrappedError(t *testing.T) {
	withFakeExec(t, func(_ context.Context, _ *Pool, _ Target, _ string) (ExecResult, error) {
		return ExecResult{}, ErrPoolBusy
	})

	_, err := ExecWithRetry(
		context.Background(), nil, Target{}, "tail -f",
		RetryableExecPolicy{
			Attempts: 2, BaseBackoff: time.Microsecond, MaxBackoff: time.Microsecond,
			Sleep: func(_ context.Context, _ time.Duration) error { return nil },
		},
		nil,
	)
	if !errors.Is(err, ErrPoolBusy) {
		t.Fatalf("err = %v, want errors.Is(_, ErrPoolBusy) after budget exhaustion", err)
	}
	if !strings.Contains(err.Error(), "after 2 attempts") {
		t.Errorf("wrap should mention attempt count: %v", err)
	}
}

func TestExecWithRetry_ContextCancellationStopsRetry(t *testing.T) {
	withFakeExec(t, func(_ context.Context, _ *Pool, _ Target, _ string) (ExecResult, error) {
		return ExecResult{}, ErrPoolBusy
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := ExecWithRetry(ctx, nil, Target{}, "tail -f", DefaultRetryableExecPolicy(), nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestExecMetricsSnapshot_NilSafeAndConsistent(t *testing.T) {
	t.Parallel()

	if got := (*ExecMetrics)(nil).Snapshot(); got != (ExecMetricsSnapshot{}) {
		t.Errorf("nil receiver Snapshot must yield zero value, got %+v", got)
	}

	m := &ExecMetrics{}
	m.Acquires.Add(3)
	m.PoolBusyHits.Add(2)
	m.Retries.Add(2)
	m.TerminalErrors.Add(1)

	snap := m.Snapshot()
	if snap.Acquires != 3 || snap.PoolBusyHits != 2 || snap.Retries != 2 || snap.TerminalErrors != 1 {
		t.Errorf("snapshot drift: %+v", snap)
	}
}
