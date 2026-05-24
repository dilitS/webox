package components

import "sync"

// DefaultRingCapacity is the slot count used when [NewRingBuffer] gets a
// zero or negative argument. 1000 mirrors `docs/sprints/sprint-09-live-log-stream.md`
// (live log buffer per project) and amortises the cost of the circular
// wrap to once every ≈30 seconds at 30 lines/second.
const DefaultRingCapacity = 1000

// RingBuffer is a fixed-capacity FIFO with circular overwrite once full.
// It is safe for concurrent use; readers obtain an immutable snapshot
// via [Snapshot] or [Tail].
//
// Type parameter T is intentionally unconstrained because the buffer
// stores opaque rows (log lines, metric samples, audit entries). The
// generic-on-struct shape is allowed in Go 1.24+; if we ever need to
// expose Get/Find methods we will promote them to package functions per
// AGENTS §5.4.
type RingBuffer[T any] struct {
	mu     sync.RWMutex
	buf    []T
	cap    int
	head   int  // index of the next write slot
	full   bool // true after the buffer has wrapped at least once
	length int  // number of populated slots (==cap when full)
}

// NewRingBuffer constructs a buffer with capacity slots. Non-positive
// values fall back to [DefaultRingCapacity] so call sites do not have
// to defend against accidental zero arguments.
func NewRingBuffer[T any](capacity int) *RingBuffer[T] {
	if capacity <= 0 {
		capacity = DefaultRingCapacity
	}
	return &RingBuffer[T]{
		buf: make([]T, capacity),
		cap: capacity,
	}
}

// Cap returns the buffer's fixed capacity.
func (r *RingBuffer[T]) Cap() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cap
}

// Len returns the number of populated slots (≤ Cap()).
func (r *RingBuffer[T]) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.length
}

// Push appends value, evicting the oldest slot once the buffer is full.
// The operation is O(1) regardless of capacity.
func (r *RingBuffer[T]) Push(value T) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.buf[r.head] = value
	r.head = (r.head + 1) % r.cap
	if r.full {
		return
	}
	r.length++
	if r.length == r.cap {
		r.full = true
	}
}

// Snapshot returns a copy of the buffer contents in insertion order
// (oldest first). Mutating the returned slice never affects the buffer.
func (r *RingBuffer[T]) Snapshot() []T {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]T, r.length)
	if !r.full {
		copy(out, r.buf[:r.length])
		return out
	}
	// When the buffer is full, the oldest slot lives at the next write
	// index. We splice [head:] + [:head] so callers always see oldest
	// first.
	copy(out, r.buf[r.head:])
	copy(out[r.cap-r.head:], r.buf[:r.head])
	return out
}

// Tail returns the n most recent entries (newest last). Requests for
// more than Len() entries clamp to the populated size; n<=0 returns an
// empty slice. The result is an independent copy.
func (r *RingBuffer[T]) Tail(n int) []T {
	if n <= 0 {
		return nil
	}
	snap := r.Snapshot()
	if n >= len(snap) {
		return snap
	}
	tail := make([]T, n)
	copy(tail, snap[len(snap)-n:])
	return tail
}
