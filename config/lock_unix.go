//go:build unix

package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"
)

func lockExclusive(ctx context.Context, lockPath string, timeout, initialBackoff time.Duration) (func() error, error) {
	if timeout <= 0 {
		timeout = defaultLockTimeout
	}
	if initialBackoff <= 0 {
		initialBackoff = defaultInitialBackoff
	}

	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, ownerOnlyPerm) //nolint:gosec // G304: lock path is deterministic sibling of audited config path.
	if err != nil {
		return nil, fmt.Errorf("config: open lock file %s: %w", lockPath, err)
	}

	deadline := time.Now().Add(timeout)
	backoff := initialBackoff

	for {
		if err := ctx.Err(); err != nil {
			_ = file.Close()
			return nil, err
		}

		err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return func() error {
				unlockErr := syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
				closeErr := file.Close()
				if unlockErr != nil {
					return fmt.Errorf("config: unlock %s: %w", lockPath, unlockErr)
				}
				if closeErr != nil {
					return fmt.Errorf("config: close lock file %s: %w", lockPath, closeErr)
				}
				return nil
			}, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
			_ = file.Close()
			return nil, fmt.Errorf("config: flock %s: %w", lockPath, err)
		}
		if time.Now().After(deadline) {
			_ = file.Close()
			return nil, ErrConfigLocked
		}

		wait := backoff
		remaining := time.Until(deadline)
		if wait > remaining {
			wait = remaining
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			_ = file.Close()
			return nil, ctx.Err()
		case <-timer.C:
		}

		if backoff < 250*time.Millisecond {
			backoff *= 2
		}
	}
}
