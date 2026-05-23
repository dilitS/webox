package components_test

import (
	"testing"

	"github.com/dilitS/webox/tui/components"
)

// BenchmarkRingBufferPush1000 measures the per-Push cost at the
// Sprint 09 perf budget (≤100µs for 1000 lines == ≤100ns/line on
// the M-series target hardware).
func BenchmarkRingBufferPush1000(b *testing.B) {
	rb := components.NewRingBuffer[int](1000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rb.Push(i)
	}
}

// BenchmarkRingBufferSnapshot measures the copy cost of Snapshot on a
// full ring. Snapshot is the read path used by the renderer and must
// stay sub-millisecond at the default capacity so the 60fps target
// holds even with the largest buffer.
func BenchmarkRingBufferSnapshotFull(b *testing.B) {
	rb := components.NewRingBuffer[int](1000)
	for i := 0; i < 1000; i++ {
		rb.Push(i)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = rb.Snapshot()
	}
}

// BenchmarkRingBufferTailWindow measures the cost of pulling the
// most-recent N lines — the path the cockpit calls each frame.
func BenchmarkRingBufferTailWindow(b *testing.B) {
	rb := components.NewRingBuffer[int](1000)
	for i := 0; i < 1000; i++ {
		rb.Push(i)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = rb.Tail(18)
	}
}
