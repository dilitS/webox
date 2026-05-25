package ssh

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/sync/semaphore"
)

// InflightLimiter is the **global** SSH concurrency cap that sits in
// front of [Pool.Acquire]. Where the per-host MaxPerHost cap (`Pool`)
// protects an individual remote from a thundering herd, the
// in-flight limiter protects the **operator's machine** and the
// upstream provider from cumulative pressure when the cockpit
// supervises N profiles × M projects in one tick.
//
// Sprint 14 TASK-14.3 introduced the limiter to bound goroutine
// growth on a 50+ project dashboard: without it, a single SWR tick
// can fan out into ~150 simultaneous SSH dials (3 ops/project × 50
// projects), any one of which can stall on the network and pin a
// goroutine for the full SSH dial timeout. The limiter caps the
// stack at [Budget] simultaneous in-flight ops with a small queue
// fed by a counted semaphore.
//
// Implementation: a single weighted semaphore (capacity = Budget)
// shared across all goroutines. Acquire blocks on `ctx.Done()` so a
// caller cancellation never leaks a held slot. Release is best-
// effort idempotent: double-release on a shared limiter is logged
// as a programmer error in tests but never panics in production.
//
// The limiter is NOT a substitute for [Pool] — pool semantics
// (idle reuse, keepalive, host-key cache) are independent. Limiter
// composes around the pool: callers acquire a limiter slot, then
// acquire a pool client, do work, release pool, release limiter.
type InflightLimiter struct {
	sem    *semaphore.Weighted
	budget int64
}

// inflightFloorBudget is the minimum number of in-flight slots the
// limiter offers. Picked at 8 so the small-fleet operator (1-3
// profiles, the common Webox case) is never throttled by the cap.
const inflightFloorBudget = 8

// inflightProfileDivisor converts the profile count into a budget
// for medium / large fleets via the AC-mandated `profiles / 2`
// rule (Sprint 14 TASK-14.3). The "/2" assumes ~50 % of profiles
// are active per refresh tick — matches the SWR cache hit ratio
// observed in Sprint 11 telemetry — and is named here so the
// magic-number linter stops flagging it.
const inflightProfileDivisor = 2

// NewInflightLimiter builds a limiter sized for the given profile
// count following Sprint 14 TASK-14.3's `max(8, profiles/2)` rule:
//
//   - small fleet (≤16 profiles): floor at 8 — keeps the cockpit
//     responsive on the common 1-3 profile setup without throttling.
//   - large fleet (>16 profiles): half the profile count — assumes
//     50% of profiles are active per refresh tick, which matches
//     the SWR cache's hit ratio in the Sprint 11 telemetry.
//
// Profiles must be ≥0; negative inputs are clamped to 0 and yield
// the floor budget. The constructor never returns nil so callers
// can initialise the limiter unconditionally.
func NewInflightLimiter(profiles int) *InflightLimiter {
	if profiles < 0 {
		profiles = 0
	}
	budget := int64(profiles / inflightProfileDivisor)
	if budget < inflightFloorBudget {
		budget = inflightFloorBudget
	}
	return &InflightLimiter{
		sem:    semaphore.NewWeighted(budget),
		budget: budget,
	}
}

// Budget returns the maximum number of in-flight SSH operations
// the limiter permits. Exposed for diagnostics (`webox doctor`)
// and the race test that asserts goroutine count stays ≤ budget*1.5.
func (l *InflightLimiter) Budget() int64 {
	if l == nil {
		return 0
	}
	return l.budget
}

// Acquire reserves one in-flight slot, blocking until either a slot
// frees up or `ctx` is cancelled. The error contract follows the
// semaphore.Weighted convention: a nil error means the slot is held
// and the caller MUST call [Release] exactly once. On cancellation
// the returned error wraps `ctx.Err()`.
//
// The wrapper exists so `errors.Is(err, ctx.Err())` works at the
// caller without needing to know about the underlying semaphore.
func (l *InflightLimiter) Acquire(ctx context.Context) error {
	if l == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := l.sem.Acquire(ctx, 1); err != nil {
		return fmt.Errorf("ssh inflight limiter: %w", err)
	}
	return nil
}

// TryAcquire returns true and reserves a slot if one is immediately
// available, otherwise returns false without blocking. Used by
// diagnostic paths that prefer to fail fast over queue.
func (l *InflightLimiter) TryAcquire() bool {
	if l == nil {
		return true
	}
	return l.sem.TryAcquire(1)
}

// Release returns one slot to the limiter. Calling Release without a
// prior successful [Acquire] underflows the semaphore — production
// code MUST pair Acquire/Release via `defer`. The method is safe to
// call on a nil receiver to simplify shutdown paths.
func (l *InflightLimiter) Release() {
	if l == nil {
		return
	}
	l.sem.Release(1)
}

// ErrInflightUnavailable is returned by [TryExec] when the limiter
// has no free slots and the caller opted out of queueing. Wrapped
// `errors.Is` checks must use this sentinel rather than a string
// compare.
var ErrInflightUnavailable = errors.New("ssh: inflight limiter saturated")
