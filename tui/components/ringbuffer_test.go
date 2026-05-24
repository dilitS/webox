package components_test

import (
	"sync"
	"testing"

	"github.com/dilitS/webox/tui/components"
)

func TestRingBufferStartsEmpty(t *testing.T) {
	t.Parallel()

	rb := components.NewRingBuffer[int](8)
	if rb.Len() != 0 {
		t.Fatalf("Len = %d, want 0", rb.Len())
	}
	if rb.Cap() != 8 {
		t.Fatalf("Cap = %d, want 8", rb.Cap())
	}
	if snap := rb.Snapshot(); len(snap) != 0 {
		t.Fatalf("Snapshot = %v, want empty", snap)
	}
}

func TestRingBufferZeroOrNegativeCapacityFallsBackToDefault(t *testing.T) {
	t.Parallel()

	rb := components.NewRingBuffer[int](0)
	if rb.Cap() != components.DefaultRingCapacity {
		t.Fatalf("Cap(0) = %d, want %d", rb.Cap(), components.DefaultRingCapacity)
	}
	rb2 := components.NewRingBuffer[int](-3)
	if rb2.Cap() != components.DefaultRingCapacity {
		t.Fatalf("Cap(-3) = %d, want %d", rb2.Cap(), components.DefaultRingCapacity)
	}
}

func TestRingBufferPushPreservesInsertionOrderUntilFull(t *testing.T) {
	t.Parallel()

	rb := components.NewRingBuffer[string](4)
	rb.Push("a")
	rb.Push("b")
	rb.Push("c")

	got := rb.Snapshot()
	want := []string{"a", "b", "c"}
	if !equal(got, want) {
		t.Fatalf("Snapshot = %v, want %v", got, want)
	}
}

func TestRingBufferCircularOverwriteWhenFull(t *testing.T) {
	t.Parallel()

	rb := components.NewRingBuffer[int](3)
	for i := 1; i <= 6; i++ {
		rb.Push(i)
	}

	if rb.Len() != 3 {
		t.Fatalf("Len after overflow = %d, want 3", rb.Len())
	}
	got := rb.Snapshot()
	want := []int{4, 5, 6}
	if !equal(got, want) {
		t.Fatalf("Snapshot after overflow = %v, want %v (oldest evicted)", got, want)
	}
}

func TestRingBufferSnapshotIsIndependentCopy(t *testing.T) {
	t.Parallel()

	rb := components.NewRingBuffer[int](4)
	rb.Push(1)
	rb.Push(2)
	snap := rb.Snapshot()
	snap[0] = 999
	if again := rb.Snapshot(); again[0] != 1 {
		t.Fatalf("Snapshot mutated underlying buffer, got first=%d want 1", again[0])
	}
}

func TestRingBufferTailReturnsMostRecentN(t *testing.T) {
	t.Parallel()

	rb := components.NewRingBuffer[int](6)
	for i := 1; i <= 6; i++ {
		rb.Push(i)
	}
	if got, want := rb.Tail(3), []int{4, 5, 6}; !equal(got, want) {
		t.Fatalf("Tail(3) = %v, want %v", got, want)
	}
	if got, want := rb.Tail(10), []int{1, 2, 3, 4, 5, 6}; !equal(got, want) {
		t.Fatalf("Tail(10) on Len=6 = %v, want %v", got, want)
	}
	if got := rb.Tail(0); len(got) != 0 {
		t.Fatalf("Tail(0) should be empty, got %v", got)
	}
}

func TestRingBufferConcurrentPushIsThreadSafe(t *testing.T) {
	t.Parallel()

	rb := components.NewRingBuffer[int](2048)
	const writers = 8
	const perWriter = 256

	var wg sync.WaitGroup
	wg.Add(writers)
	for w := 0; w < writers; w++ {
		go func() {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				rb.Push(i)
			}
		}()
	}
	wg.Wait()

	if rb.Len() != writers*perWriter {
		t.Fatalf("Len = %d, want %d", rb.Len(), writers*perWriter)
	}
}

func equal[T comparable](a, b []T) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
