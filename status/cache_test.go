package status

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestGetOrFetch_MissBlocksFetchAndStores(t *testing.T) {
	t.Parallel()

	clock := newManualClock(time.Date(2026, 5, 23, 1, 30, 0, 0, time.UTC))
	cache := NewCache(Options{Now: clock.Now})

	got, stale, err := GetOrFetch[int](cache, "http:example.com", time.Minute, func(context.Context) (int, error) {
		return 200, nil
	}, context.Background())
	if err != nil {
		t.Fatalf("GetOrFetch miss: %v", err)
	}
	if got != 200 || stale {
		t.Fatalf("miss got (%d, stale=%t), want (200, false)", got, stale)
	}

	got, stale, err = GetOrFetch[int](cache, "http:example.com", time.Minute, func(context.Context) (int, error) {
		t.Fatal("fresh hit must not call fetch")
		return 0, nil
	}, context.Background())
	if err != nil {
		t.Fatalf("GetOrFetch fresh hit: %v", err)
	}
	if got != 200 || stale {
		t.Fatalf("fresh hit got (%d, stale=%t), want (200, false)", got, stale)
	}
}

func TestGetOrFetch_StaleReturnsImmediatelyAndRefreshesInBackground(t *testing.T) {
	t.Parallel()

	clock := newManualClock(time.Date(2026, 5, 23, 1, 31, 0, 0, time.UTC))
	cache := NewCache(Options{Now: clock.Now, BackgroundTimeout: time.Second})

	_, _, err := GetOrFetch[int](cache, "ssl:example.com", time.Minute, func(context.Context) (int, error) {
		return 10, nil
	}, context.Background())
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	clock.Advance(2 * time.Minute)

	refreshStarted := make(chan struct{})
	releaseRefresh := make(chan struct{})
	got, stale, err := GetOrFetch[int](cache, "ssl:example.com", time.Minute, func(context.Context) (int, error) {
		close(refreshStarted)
		<-releaseRefresh
		return 20, nil
	}, context.Background())
	if err != nil {
		t.Fatalf("stale GetOrFetch: %v", err)
	}
	if got != 10 || !stale {
		t.Fatalf("stale hit got (%d, stale=%t), want (10, true)", got, stale)
	}

	select {
	case <-refreshStarted:
	case <-time.After(time.Second):
		t.Fatal("background refresh did not start")
	}
	close(releaseRefresh)
	waitFor(t, time.Second, func() bool {
		cached, ok := cache.lookup("ssl:example.com")
		return ok && cached.value == 20 && clock.Now().Before(cached.expiresAt)
	})
}

func TestGetOrFetch_SingleflightDedupesConcurrentMisses(t *testing.T) {
	t.Parallel()

	cache := NewCache(Options{})
	var calls atomic.Int32
	release := make(chan struct{})

	var wg sync.WaitGroup
	results := make(chan int, 8)
	errs := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, stale, err := GetOrFetch[int](cache, "ssh:node:example.com", time.Minute, func(context.Context) (int, error) {
				calls.Add(1)
				<-release
				return 24, nil
			}, context.Background())
			if stale {
				errs <- errors.New("miss should not be stale")
				return
			}
			if err != nil {
				errs <- err
				return
			}
			results <- got
		}()
	}

	waitFor(t, time.Second, func() bool { return calls.Load() == 1 })
	close(release)
	wg.Wait()
	close(results)
	close(errs)

	if calls.Load() != 1 {
		t.Fatalf("fetch calls = %d, want 1", calls.Load())
	}
	for err := range errs {
		t.Fatal(err)
	}
	for got := range results {
		if got != 24 {
			t.Fatalf("result = %d, want 24", got)
		}
	}
}

func TestGetOrFetch_CancellationPropagatesOnMiss(t *testing.T) {
	t.Parallel()

	cache := NewCache(Options{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := GetOrFetch[int](cache, "http:cancelled", time.Minute, func(ctx context.Context) (int, error) {
		return 0, ctx.Err()
	}, ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("GetOrFetch cancelled err = %v, want context.Canceled", err)
	}
}

func TestGetOrFetch_FetchErrorDoesNotPoisonCache(t *testing.T) {
	t.Parallel()

	cache := NewCache(Options{})
	boom := errors.New("boom")

	_, _, err := GetOrFetch[int](cache, "http:error", time.Minute, func(context.Context) (int, error) {
		return 0, boom
	}, context.Background())
	if !errors.Is(err, boom) {
		t.Fatalf("first fetch err = %v, want boom", err)
	}

	got, stale, err := GetOrFetch[int](cache, "http:error", time.Minute, func(context.Context) (int, error) {
		return 200, nil
	}, context.Background())
	if err != nil {
		t.Fatalf("second fetch: %v", err)
	}
	if got != 200 || stale {
		t.Fatalf("second fetch got (%d, stale=%t), want (200, false)", got, stale)
	}
}

type manualClock struct {
	mu  sync.Mutex
	now time.Time
}

func newManualClock(now time.Time) *manualClock {
	return &manualClock{now: now}
}

func (c *manualClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.now
}

func (c *manualClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.now = c.now.Add(d)
}

func waitFor(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}
