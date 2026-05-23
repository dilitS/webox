package ssh

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	cryptossh "golang.org/x/crypto/ssh"
)

// Exec runs command through a pooled SSH client and returns the captured
// stdout, stderr, exit status, and wall-clock duration. It does not
// retry the command: provider operations must first inspect remote state
// after reconnect before deciding whether replay is safe (DESIGN §9).
func Exec(ctx context.Context, pool *Pool, target Target, command string) (ExecResult, error) {
	start := time.Now()
	client, err := pool.Acquire(ctx, target)
	if err != nil {
		return ExecResult{Duration: time.Since(start)}, err
	}
	defer pool.Release(target, client)

	session, err := client.NewSession()
	if err != nil {
		return ExecResult{Duration: time.Since(start)}, err
	}
	defer func() { _ = session.Close() }()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	errCh := make(chan error, 1)
	go func() {
		errCh <- session.Run(command)
	}()

	select {
	case err := <-errCh:
		result := ExecResult{
			Stdout:   stdout.Bytes(),
			Stderr:   stderr.Bytes(),
			ExitCode: exitCode(err),
			Duration: time.Since(start),
		}
		return result, err
	case <-ctx.Done():
		_ = session.Close()
		<-errCh
		return ExecResult{
			Stdout:   stdout.Bytes(),
			Stderr:   stderr.Bytes(),
			ExitCode: -1,
			Duration: time.Since(start),
		}, ctx.Err()
	}
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *cryptossh.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitStatus()
	}
	return -1
}

// SleepFunc is the injectable sleeper used by reconnect tests.
type SleepFunc func(context.Context, time.Duration) error

// RetryPolicy controls reconnect attempts. Backoff defaults to
// 3s/6s/12s from DESIGN §9; Sleep defaults to a context-aware timer.
type RetryPolicy struct {
	Backoff []time.Duration
	Sleep   SleepFunc
}

// Reconnect repeatedly acquires a fresh client according to policy. It
// only restores connectivity; it deliberately does not replay any
// command that may have been in-flight when the connection broke.
func Reconnect(ctx context.Context, pool *Pool, target Target, policy RetryPolicy) (*cryptossh.Client, error) {
	policy = normalizeRetryPolicy(policy)
	var lastErr error
	for i, backoff := range policy.Backoff {
		client, err := pool.Acquire(ctx, target)
		if err == nil {
			return client, nil
		}
		lastErr = err
		if i == len(policy.Backoff)-1 {
			break
		}
		if sleepErr := policy.Sleep(ctx, backoff); sleepErr != nil {
			return nil, sleepErr
		}
	}
	return nil, fmt.Errorf("%w: %w", ErrReconnectExhausted, lastErr)
}

func normalizeRetryPolicy(policy RetryPolicy) RetryPolicy {
	if len(policy.Backoff) == 0 {
		policy.Backoff = []time.Duration{3 * time.Second, 6 * time.Second, 12 * time.Second}
	}
	if policy.Sleep == nil {
		policy.Sleep = sleepContext
	}
	return policy
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
