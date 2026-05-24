package sshtail_test

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/dilitS/webox/services/sshtail"
)

// TestStreamDoesNotLeakGoroutines verifies the streamer's goroutine
// cleans up within the configured leak window when the consumer
// cancels the context. The Sprint 09 perf budget allots ≤100ms for
// cancel-to-shutdown; we give it 500ms for CI jitter headroom.
func TestStreamDoesNotLeakGoroutines(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"))

	payload := strings.Repeat("line\n", 1000)
	streamer := sshtail.New(sshtail.Options{Executor: stubExecutor(payload)})

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := streamer.Stream(ctx, sshtail.Profile{Alias: "main"}, "logs/app.log")
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	// Drain a few lines, then cancel and ensure the channel is closed.
	for i := 0; i < 10; i++ {
		<-ch
	}
	cancel()

	closed := make(chan struct{})
	go func() {
		for range ch { //nolint:revive // empty body drains until channel close.
		}
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("channel did not close within 500ms of context cancel")
	}
}

// TestStreamReconnectExhaustionDoesNotLeak guards the failure path:
// even when every reconnect attempt fails, the goroutine must exit so
// the cockpit can render the "stream lost" banner without leaking.
func TestStreamReconnectExhaustionDoesNotLeak(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"))

	streamer := sshtail.New(sshtail.Options{
		Executor: sshtail.ExecutorFunc(func(_ context.Context, _ string) (io.ReadCloser, error) {
			return nil, io.ErrUnexpectedEOF
		}),
		Backoff: []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond},
		Sleep:   func(_ context.Context, _ time.Duration) error { return nil },
	})

	ch, err := streamer.Stream(context.Background(), sshtail.Profile{Alias: "main"}, "logs/app.log")
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	for range ch { //nolint:revive // empty body drains until channel close.
	}
}
