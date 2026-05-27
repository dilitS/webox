package ssh

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	cryptossh "golang.org/x/crypto/ssh"

	"github.com/dilitS/webox/testing/sshmock"
)

func TestExec_ReturnsStdoutStderrExitCodeAndDuration(t *testing.T) {
	t.Parallel()

	server := sshmock.New(t, sshmock.WithCommand("deploy", sshmock.CommandResult{
		Stdout:   "ok\n",
		Stderr:   "warn\n",
		ExitCode: 7,
	}))
	pool := NewPool(PoolOptions{
		MaxPerHost:        1,
		IdleTimeout:       time.Minute,
		CleanupInterval:   time.Hour,
		KeepaliveInterval: -1,
		Dialer:            NetDialer{},
		Config:            staticConfig(server.ClientConfig()),
	})
	defer pool.Close()

	result, err := Exec(context.Background(), pool, targetForServer(server), "deploy")
	if err == nil {
		t.Fatal("Exec non-zero command returned nil error, want ssh.ExitError")
	}
	var exitErr *cryptossh.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("Exec err = %T %v, want *ssh.ExitError", err, err)
	}
	if string(result.Stdout) != "ok\n" {
		t.Fatalf("Stdout = %q, want ok newline", result.Stdout)
	}
	if string(result.Stderr) != "warn\n" {
		t.Fatalf("Stderr = %q, want warn newline", result.Stderr)
	}
	if result.ExitCode != 7 {
		t.Fatalf("ExitCode = %d, want 7", result.ExitCode)
	}
	if result.Duration <= 0 {
		t.Fatalf("Duration = %v, want positive", result.Duration)
	}
}

func TestExec_ContextCancellationClosesSession(t *testing.T) {
	t.Parallel()

	server := sshmock.New(t, sshmock.WithCommand("slow", sshmock.CommandResult{
		Delay: 200 * time.Millisecond,
	}))
	pool := NewPool(PoolOptions{
		MaxPerHost:        1,
		IdleTimeout:       time.Minute,
		CleanupInterval:   time.Hour,
		KeepaliveInterval: -1,
		Dialer:            NetDialer{},
		Config:            staticConfig(server.ClientConfig()),
	})
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := Exec(ctx, pool, targetForServer(server), "slow")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Exec cancelled err = %v, want context deadline", err)
	}
}

func TestKeepaliveLoopStopsOnPoolClose(t *testing.T) {
	t.Parallel()

	server := sshmock.New(t)
	pool := NewPool(PoolOptions{
		MaxPerHost:        1,
		IdleTimeout:       time.Minute,
		CleanupInterval:   time.Hour,
		KeepaliveInterval: 5 * time.Millisecond,
		Dialer:            NetDialer{},
		Config:            staticConfig(server.ClientConfig()),
	})

	target := targetForServer(server)
	client, err := pool.Acquire(context.Background(), target)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	pool.Release(target, client)

	waitFor(t, time.Second, func() bool {
		return server.GlobalRequestCount(keepaliveRequest) > 0
	})

	// Sampling the count BEFORE Close would race with any in-flight
	// SendRequest that the keepalive goroutine kicked off between the
	// previous waitFor poll and the assertion: the server might bump
	// its counter after we read `before` but before Close had a chance
	// to tear down the underlying client. Instead, rely on the pool
	// contract that Close blocks until every keepalive goroutine has
	// returned (and its underlying client is closed). After Close
	// returns, the keepalive goroutine has exited, but the cryptossh
	// server-side internal goroutine may still drain one already-parsed
	// keepalive request from its requests channel into handleGlobalRequests
	// (which increments the counter under the mutex BEFORE replying).
	// That drain is async w.r.t. pool.Close and was observed bumping the
	// server-side count after Close returned on slow CI runners. To
	// remove that flake without weakening the assertion, first wait for
	// the server-side counter to stabilize (two consecutive snapshots
	// 50 ms apart that agree), then assert the count remains frozen
	// across a further window that spans many keepalive intervals.
	pool.Close()
	afterClose := waitForStableCount(t, server, keepaliveRequest, 50*time.Millisecond, 2*time.Second)
	time.Sleep(50 * time.Millisecond)
	finalCount := server.GlobalRequestCount(keepaliveRequest)
	if finalCount != afterClose {
		t.Fatalf("keepalive count changed after pool close: stable=%d final=%d", afterClose, finalCount)
	}
}

// waitForStableCount polls the server-side request counter until two
// consecutive samples taken `quiet` apart agree, or `timeout` elapses.
// It returns the last sample observed.
//
// Used by tests that assert "no further keepalive traffic after
// pool.Close()": the cryptossh server-side connection teardown is
// asynchronous, so even after pool.Close returns the server may still
// process one already-received request before its channel is closed.
// Waiting for stability eliminates that drain race.
func waitForStableCount(t *testing.T, s *sshmock.Server, requestType string, quiet, timeout time.Duration) int {
	t.Helper()

	deadline := time.Now().Add(timeout)
	prev := s.GlobalRequestCount(requestType)
	for time.Now().Before(deadline) {
		time.Sleep(quiet)
		cur := s.GlobalRequestCount(requestType)
		if cur == prev {
			return cur
		}
		prev = cur
	}
	return prev
}

func TestReconnect_SucceedsAfterTransientDialFailure(t *testing.T) {
	t.Parallel()

	server := sshmock.New(t)
	dialer := &flakyDialer{inner: NetDialer{}, failuresLeft: 1}
	pool := NewPool(PoolOptions{
		MaxPerHost:        1,
		IdleTimeout:       time.Minute,
		CleanupInterval:   time.Hour,
		KeepaliveInterval: -1,
		Dialer:            dialer,
		Config:            staticConfig(server.ClientConfig()),
	})
	defer pool.Close()

	client, err := Reconnect(context.Background(), pool, targetForServer(server), RetryPolicy{
		Backoff: []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond},
		Sleep:   noSleep,
	})
	if err != nil {
		t.Fatalf("Reconnect: %v", err)
	}
	pool.Release(targetForServer(server), client)
	if got := dialer.calls.Load(); got != 2 {
		t.Fatalf("dial calls = %d, want 2 (one failure, one success)", got)
	}
}

func TestReconnect_Exhausted(t *testing.T) {
	t.Parallel()

	server := sshmock.New(t)
	dialer := &flakyDialer{inner: NetDialer{}, failuresLeft: 10}
	pool := NewPool(PoolOptions{
		MaxPerHost:        1,
		IdleTimeout:       time.Minute,
		CleanupInterval:   time.Hour,
		KeepaliveInterval: -1,
		Dialer:            dialer,
		Config:            staticConfig(server.ClientConfig()),
	})
	defer pool.Close()

	_, err := Reconnect(context.Background(), pool, targetForServer(server), RetryPolicy{
		Backoff: []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond},
		Sleep:   noSleep,
	})
	if !errors.Is(err, ErrReconnectExhausted) {
		t.Fatalf("Reconnect exhausted err = %v, want ErrReconnectExhausted", err)
	}
	if got := dialer.calls.Load(); got != 3 {
		t.Fatalf("dial calls = %d, want 3 attempts", got)
	}
}

type flakyDialer struct {
	inner        Dialer
	failuresLeft int32
	calls        atomic.Int32
}

func (d *flakyDialer) Dial(ctx context.Context, target Target, config *cryptossh.ClientConfig) (*cryptossh.Client, error) {
	d.calls.Add(1)
	if atomic.AddInt32(&d.failuresLeft, -1) >= 0 {
		return nil, errors.New("temporary dial failure")
	}
	return d.inner.Dial(ctx, target, config)
}

func noSleep(context.Context, time.Duration) error { return nil }
