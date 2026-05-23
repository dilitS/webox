package sshtail_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dilitS/webox/services/sshtail"
	"github.com/dilitS/webox/tui/components"
)

func TestStreamRejectsInvalidLogPath(t *testing.T) {
	t.Parallel()

	cases := []string{
		"",
		"../../etc/passwd",
		"logs/app.log\nrm -rf",
		"logs/\x00app.log",
	}
	for _, path := range cases {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			streamer := sshtail.New(sshtail.Options{
				Executor: sshtail.ExecutorFunc(func(_ context.Context, _ string) (io.ReadCloser, error) {
					t.Fatal("executor should not be called for invalid path")
					return nil, nil
				}),
			})
			_, err := streamer.Stream(context.Background(), sshtail.Profile{Alias: "main"}, path)
			if !errors.Is(err, sshtail.ErrLogPathInvalid) {
				t.Fatalf("Stream(%q) err = %v, want ErrLogPathInvalid", path, err)
			}
		})
	}
}

func TestStreamRedactsSecretsBeforeEmission(t *testing.T) {
	t.Parallel()

	const secret = "ghp_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	payload := strings.Join([]string{
		"normal line",
		"token=" + secret,
		"another normal line",
	}, "\n") + "\n"

	streamer := sshtail.New(sshtail.Options{
		Executor: stubExecutor(payload),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch, err := streamer.Stream(ctx, sshtail.Profile{Alias: "main"}, "logs/node.log")
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	var lines []sshtail.Line
	for line := range ch {
		lines = append(lines, line)
		if len(lines) == 3 {
			cancel()
		}
	}

	if len(lines) < 3 {
		t.Fatalf("got %d lines, want >=3: %#v", len(lines), lines)
	}
	for _, line := range lines {
		if strings.Contains(line.Raw, secret) {
			t.Fatalf("secret leaked into channel: %q", line.Raw)
		}
	}
	if !lines[1].Redacted {
		t.Fatalf("second line should be marked Redacted=true, got %#v", lines[1])
	}
	if lines[0].Redacted || lines[2].Redacted {
		t.Fatalf("non-secret lines should be Redacted=false: %#v / %#v", lines[0], lines[2])
	}
}

func TestStreamCancellationClosesChannel(t *testing.T) {
	t.Parallel()

	infinite := newBlockingReader()
	streamer := sshtail.New(sshtail.Options{
		Executor: sshtail.ExecutorFunc(func(_ context.Context, _ string) (io.ReadCloser, error) {
			return infinite, nil
		}),
	})

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := streamer.Stream(ctx, sshtail.Profile{Alias: "main"}, "logs/app.log")
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	cancel()
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("channel should be drained/closed after ctx cancel")
		}
	case <-time.After(time.Second):
		t.Fatal("channel did not close within 1s of cancel")
	}
}

func TestStreamReconnectsExhaustsBackoff(t *testing.T) {
	t.Parallel()

	var attempts int
	executor := sshtail.ExecutorFunc(func(_ context.Context, _ string) (io.ReadCloser, error) {
		attempts++
		return nil, errors.New("transport: connection refused")
	})

	sleeps := []time.Duration{}
	streamer := sshtail.New(sshtail.Options{
		Executor: executor,
		Backoff:  []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond},
		Sleep: func(_ context.Context, d time.Duration) error {
			sleeps = append(sleeps, d)
			return nil
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	ch, err := streamer.Stream(ctx, sshtail.Profile{Alias: "main"}, "logs/app.log")
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	for range ch { //nolint:revive // empty body drains the channel until close.
	}

	if attempts < 4 {
		t.Fatalf("want >=4 open attempts (initial + 3 retries), got %d", attempts)
	}
	if len(sleeps) != 3 {
		t.Fatalf("backoff slept %d times, want 3", len(sleeps))
	}
}

func TestStreamClassifiesLogLevel(t *testing.T) {
	t.Parallel()

	payload := strings.Join([]string{
		"[INFO] starting",
		"WARN: slow query",
		"[ERROR] connection refused",
	}, "\n") + "\n"

	streamer := sshtail.New(sshtail.Options{Executor: stubExecutor(payload)})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	ch, err := streamer.Stream(ctx, sshtail.Profile{Alias: "main"}, "logs/node.log")
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	want := []components.LogLevel{components.LevelInfo, components.LevelWarn, components.LevelError}
	var idx int
	for line := range ch {
		if idx >= len(want) {
			break
		}
		if line.Level != want[idx] {
			t.Fatalf("line %d level = %s, want %s (raw=%q)", idx, line.Level, want[idx], line.Raw)
		}
		idx++
		if idx == len(want) {
			cancel()
		}
	}
	if idx != len(want) {
		t.Fatalf("classified %d/%d lines", idx, len(want))
	}
}

func TestStreamPanicsWithoutExecutor(t *testing.T) {
	t.Parallel()

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic when Executor is nil")
		}
	}()
	_ = sshtail.New(sshtail.Options{})
}

func TestIsTransportErrorRecognisesEOFAndClosedSession(t *testing.T) {
	t.Parallel()

	if !sshtail.IsTransportError(io.EOF) {
		t.Error("io.EOF should be transport error")
	}
	if !sshtail.IsTransportError(sshtail.ErrSessionClosed) {
		t.Error("ErrSessionClosed should be transport error")
	}
	if sshtail.IsTransportError(nil) {
		t.Error("nil should not be transport error")
	}
	if sshtail.IsTransportError(errors.New("syntax error")) {
		t.Error("unrelated error should not be transport error")
	}
}

// --- helpers ---

type stubReader struct {
	r io.Reader
}

func (s *stubReader) Read(p []byte) (int, error) { return s.r.Read(p) }
func (*stubReader) Close() error                 { return nil }

func stubExecutor(payload string) sshtail.Executor {
	return sshtail.ExecutorFunc(func(_ context.Context, _ string) (io.ReadCloser, error) {
		return &stubReader{r: strings.NewReader(payload)}, nil
	})
}

// blockingReader returns no data and blocks until Close is invoked.
// It mirrors the behaviour of `tail -f` against an empty log file.
type blockingReader struct {
	mu     sync.Mutex
	closed bool
	wake   chan struct{}
}

func newBlockingReader() *blockingReader {
	return &blockingReader{wake: make(chan struct{})}
}

func (b *blockingReader) Read(p []byte) (int, error) {
	<-b.wake
	return 0, io.EOF
}

func (b *blockingReader) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.closed {
		b.closed = true
		close(b.wake)
	}
	return nil
}
