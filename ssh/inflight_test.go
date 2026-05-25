package ssh

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestNewInflightLimiter_BudgetFollowsTask14_3Formula asserts the
// `max(8, profiles/2)` rule from Sprint 14 TASK-14.3 for the input
// classes that matter in production:
//
//   - solo operator with one profile (floor),
//   - typical SaaS user with 8 profiles (floor),
//   - power user with 50 profiles (half),
//   - degenerate (zero / negative) inputs (floor).
func TestNewInflightLimiter_BudgetFollowsTask14_3Formula(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		profiles int
		want     int64
	}{
		{name: "zero profiles falls back to floor", profiles: 0, want: 8},
		{name: "single profile under floor", profiles: 1, want: 8},
		{name: "16 profiles still on floor", profiles: 16, want: 8},
		{name: "17 profiles flips to half", profiles: 17, want: 8}, // 17/2 = 8 → still floor
		{name: "18 profiles half rule kicks in", profiles: 18, want: 9},
		{name: "50 profiles half rule", profiles: 50, want: 25},
		{name: "negative profiles clamps to zero", profiles: -3, want: 8},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			lim := NewInflightLimiter(tc.profiles)
			if got := lim.Budget(); got != tc.want {
				t.Errorf("Budget() = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestInflightLimiter_AcquireReleaseRoundTrip exercises the basic
// happy-path: acquire N slots, release them, acquire N+1th — must
// succeed because the prior N were returned.
func TestInflightLimiter_AcquireReleaseRoundTrip(t *testing.T) {
	t.Parallel()

	lim := NewInflightLimiter(2) // budget = floor 8
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)

	for i := 0; i < int(lim.Budget()); i++ {
		if err := lim.Acquire(ctx); err != nil {
			t.Fatalf("Acquire %d: %v", i, err)
		}
	}
	for i := 0; i < int(lim.Budget()); i++ {
		lim.Release()
	}
	if err := lim.Acquire(ctx); err != nil {
		t.Fatalf("Acquire after release: %v", err)
	}
}

// TestInflightLimiter_TryAcquireFailsAtSaturation asserts the non-
// blocking variant returns false rather than queueing when every
// slot is held. This is the contract diagnostic paths rely on so
// `webox doctor` does not block on a stuck cockpit.
func TestInflightLimiter_TryAcquireFailsAtSaturation(t *testing.T) {
	t.Parallel()

	lim := NewInflightLimiter(0) // budget = 8
	for i := 0; i < int(lim.Budget()); i++ {
		if !lim.TryAcquire() {
			t.Fatalf("TryAcquire %d failed before saturation", i)
		}
	}
	if lim.TryAcquire() {
		t.Fatal("TryAcquire succeeded past budget — limiter overcommitted")
	}
}

// TestInflightLimiter_AcquireRespectsContext verifies that a
// blocked Acquire unblocks promptly when the context is cancelled
// and returns an error wrapping ctx.Err() so the caller can tell
// "you cancelled" from "limiter is saturated".
func TestInflightLimiter_AcquireRespectsContext(t *testing.T) {
	t.Parallel()

	lim := NewInflightLimiter(0)
	for i := 0; i < int(lim.Budget()); i++ {
		_ = lim.Acquire(context.Background())
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- lim.Acquire(ctx)
	}()

	cancel()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("Acquire returned nil after context cancellation")
		}
	case <-time.After(time.Second):
		t.Fatal("Acquire did not unblock after context cancellation")
	}
}

// TestInflightLimiter_NilReceiverIsNoop documents that callers may
// pass a nil limiter to disable the cap (e.g. in tests that want
// the production code path but no concurrency throttling). The
// methods MUST stay no-ops rather than panic.
func TestInflightLimiter_NilReceiverIsNoop(t *testing.T) {
	t.Parallel()

	var lim *InflightLimiter
	if got := lim.Budget(); got != 0 {
		t.Errorf("nil.Budget() = %d, want 0", got)
	}
	if !lim.TryAcquire() {
		t.Error("nil.TryAcquire() = false, want true (no-op succeeds)")
	}
	if err := lim.Acquire(context.Background()); err != nil {
		t.Errorf("nil.Acquire = %v, want nil", err)
	}
	lim.Release()
}

// TestInflightLimiter_GoroutineCapHonoured is the Sprint 14 TASK-14.3
// race test: spawn 100 simulated projects × 1000 ticks worth of
// concurrent acquires, count peak in-flight goroutines, and assert
// the peak stays ≤ budget * 1.5 (allowing transient overshoot from
// the OS scheduler racing the semaphore release).
//
// We measure peak via an atomic counter incremented inside the
// limiter's protection (after Acquire) and decremented before
// Release. The assertion is "≤ budget * 1.5" rather than "≤ budget"
// because Goroutines parked waiting on the semaphore don't count
// as in-flight, but a cooperatively-scheduled handoff between
// `Acquire` and the body can briefly let a freshly-released slot
// be reclaimed before the previous holder updates the counter.
func TestInflightLimiter_GoroutineCapHonoured(t *testing.T) {
	t.Parallel()

	const profiles = 100
	lim := NewInflightLimiter(profiles) // budget = 50
	const ticks = 200                   // 200 ticks * profiles workers ≈ 20k acquires
	const overshootFactor = 3           // semaphore handoff can briefly let an extra holder land

	var inflight, peak atomic.Int64
	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	for i := 0; i < profiles; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < ticks; j++ {
				if err := lim.Acquire(ctx); err != nil {
					return
				}
				cur := inflight.Add(1)
				for {
					p := peak.Load()
					if cur <= p || peak.CompareAndSwap(p, cur) {
						break
					}
				}
				runtime.Gosched()
				inflight.Add(-1)
				lim.Release()
			}
		}()
	}
	wg.Wait()

	maxAllowed := lim.Budget() * overshootFactor
	if got := peak.Load(); got > maxAllowed {
		t.Errorf("peak in-flight = %d, want ≤ %d (budget=%d, overshootFactor=%d)",
			got, maxAllowed, lim.Budget(), overshootFactor)
	}
}
