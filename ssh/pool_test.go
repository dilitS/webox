package ssh

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	cryptossh "golang.org/x/crypto/ssh"

	"github.com/dilitS/webox/testing/sshmock"
)

func TestPool_AcquireReleaseReusesClient(t *testing.T) {
	t.Parallel()

	server := sshmock.New(t, sshmock.WithCommand("true", sshmock.CommandResult{}))
	dialer := &countingDialer{inner: NetDialer{}}
	pool := NewPool(PoolOptions{
		MaxPerHost:      3,
		IdleTimeout:     time.Minute,
		CleanupInterval: time.Hour,
		Dialer:          dialer,
		Config:          staticConfig(server.ClientConfig()),
	})
	defer pool.Close()

	target := targetForServer(server)
	first, err := pool.Acquire(context.Background(), target)
	if err != nil {
		t.Fatalf("Acquire first: %v", err)
	}
	pool.Release(target, first)

	second, err := pool.Acquire(context.Background(), target)
	if err != nil {
		t.Fatalf("Acquire second: %v", err)
	}
	defer pool.Release(target, second)

	if first != second {
		t.Fatalf("expected released client to be reused, first=%p second=%p", first, second)
	}
	if got := dialer.calls.Load(); got != 1 {
		t.Fatalf("dial calls = %d, want 1", got)
	}
}

func TestPool_AcquireRespectsLimitAndReturnsPoolBusy(t *testing.T) {
	t.Parallel()

	server := sshmock.New(t)
	pool := NewPool(PoolOptions{
		MaxPerHost:      1,
		IdleTimeout:     time.Minute,
		CleanupInterval: time.Hour,
		Dialer:          NetDialer{},
		Config:          staticConfig(server.ClientConfig()),
	})
	defer pool.Close()

	target := targetForServer(server)
	held, err := pool.Acquire(context.Background(), target)
	if err != nil {
		t.Fatalf("Acquire held: %v", err)
	}
	defer pool.Release(target, held)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err = pool.Acquire(ctx, target)
	if !errors.Is(err, ErrPoolBusy) {
		t.Fatalf("Acquire while pool full err = %v, want errors.Is(_, ErrPoolBusy)", err)
	}
}

func TestPool_AcquireCancelledContext(t *testing.T) {
	t.Parallel()

	server := sshmock.New(t)
	pool := NewPool(PoolOptions{
		MaxPerHost:      1,
		IdleTimeout:     time.Minute,
		CleanupInterval: time.Hour,
		Dialer:          NetDialer{},
		Config:          staticConfig(server.ClientConfig()),
	})
	defer pool.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := pool.Acquire(ctx, targetForServer(server))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Acquire with cancelled ctx err = %v, want context.Canceled", err)
	}
}

func TestPool_DoubleReleaseDoesNotCorruptState(t *testing.T) {
	t.Parallel()

	server := sshmock.New(t)
	dialer := &countingDialer{inner: NetDialer{}}
	pool := NewPool(PoolOptions{
		MaxPerHost:      1,
		IdleTimeout:     time.Minute,
		CleanupInterval: time.Hour,
		Dialer:          dialer,
		Config:          staticConfig(server.ClientConfig()),
	})
	defer pool.Close()

	target := targetForServer(server)
	client, err := pool.Acquire(context.Background(), target)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	pool.Release(target, client)
	pool.Release(target, client)

	reused, err := pool.Acquire(context.Background(), target)
	if err != nil {
		t.Fatalf("Acquire after double release: %v", err)
	}
	pool.Release(target, reused)

	if got := dialer.calls.Load(); got != 1 {
		t.Fatalf("double release corrupted pool; dial calls = %d, want 1", got)
	}
}

func TestPool_IdleCleanupClosesExpiredClients(t *testing.T) {
	t.Parallel()

	server := sshmock.New(t)
	dialer := &countingDialer{inner: NetDialer{}}
	pool := NewPool(PoolOptions{
		MaxPerHost:      1,
		IdleTimeout:     10 * time.Millisecond,
		CleanupInterval: 5 * time.Millisecond,
		Dialer:          dialer,
		Config:          staticConfig(server.ClientConfig()),
	})
	defer pool.Close()

	target := targetForServer(server)
	first, err := pool.Acquire(context.Background(), target)
	if err != nil {
		t.Fatalf("Acquire first: %v", err)
	}
	pool.Release(target, first)

	waitFor(t, time.Second, func() bool { return dialer.calls.Load() == 1 && pool.IdleCount(target) == 0 })

	second, err := pool.Acquire(context.Background(), target)
	if err != nil {
		t.Fatalf("Acquire second: %v", err)
	}
	pool.Release(target, second)

	if got := dialer.calls.Load(); got != 2 {
		t.Fatalf("dial calls after idle cleanup = %d, want 2", got)
	}
	if first == second {
		t.Fatal("idle cleanup should close expired client and force a fresh dial")
	}
}

func TestPool_ConcurrentAcquireReleaseRace(t *testing.T) {
	t.Parallel()

	server := sshmock.New(t, sshmock.WithCommand("true", sshmock.CommandResult{}))
	pool := NewPool(PoolOptions{
		MaxPerHost:      3,
		IdleTimeout:     time.Minute,
		CleanupInterval: time.Hour,
		Dialer:          NetDialer{},
		Config:          staticConfig(server.ClientConfig()),
	})
	defer pool.Close()

	target := targetForServer(server)
	var wg sync.WaitGroup
	for range 24 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client, err := pool.Acquire(context.Background(), target)
			if err != nil {
				t.Errorf("Acquire: %v", err)
				return
			}
			pool.Release(target, client)
		}()
	}
	wg.Wait()
}

type countingDialer struct {
	inner Dialer
	calls atomic.Int32
}

func (d *countingDialer) Dial(ctx context.Context, target Target, config *cryptossh.ClientConfig) (*cryptossh.Client, error) {
	d.calls.Add(1)
	return d.inner.Dial(ctx, target, config)
}

func staticConfig(config *cryptossh.ClientConfig) func(Target) (*cryptossh.ClientConfig, error) {
	return func(Target) (*cryptossh.ClientConfig, error) {
		return config, nil
	}
}

func targetForServer(server *sshmock.Server) Target {
	host, port := splitHostPortForTest(server.Addr())
	return Target{Host: host, Port: port, User: "webox-test"}
}

func splitHostPortForTest(addr string) (string, int) {
	host, port, err := splitHostPort(addr)
	if err != nil {
		panic(err)
	}
	return host, port
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
