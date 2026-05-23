//go:build unix

package secrets

import (
	"context"
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"
)

const (
	defaultLockTimeout    = 5 * time.Second
	defaultInitialBackoff = 10 * time.Millisecond
	maxLockBackoff        = 250 * time.Millisecond
)

// ErrSecretsLocked is returned when the per-vault flock cannot be
// acquired within the deadline (another Webox instance holds it). The
// message is actionable so doctor can render it verbatim.
var ErrSecretsLocked = errors.New("secrets: another webox instance is mutating the encrypted vault; close it or wait")

// lockExclusive acquires an exclusive flock(2) lease on lockPath with
// exponential backoff. The returned closer releases and closes the
// underlying lock file. Mirrors config/lock_unix.go intentionally — the
// two layers have distinct lock files (different config / secrets
// surfaces) but identical concurrency semantics.
func lockExclusive(ctx context.Context, lockPath string, timeout, initialBackoff time.Duration) (func() error, error) {
	if timeout <= 0 {
		timeout = defaultLockTimeout
	}
	if initialBackoff <= 0 {
		initialBackoff = defaultInitialBackoff
	}

	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, ownerOnlyPerm) //nolint:gosec // G304: lock path is deterministic sibling of audited vault path.
	if err != nil {
		return nil, fmt.Errorf("secrets: open lock file %s: %w", lockPath, err)
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
					return fmt.Errorf("secrets: unlock %s: %w", lockPath, unlockErr)
				}
				if closeErr != nil {
					return fmt.Errorf("secrets: close lock file %s: %w", lockPath, closeErr)
				}
				return nil
			}, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
			_ = file.Close()
			return nil, fmt.Errorf("secrets: flock %s: %w", lockPath, err)
		}
		if time.Now().After(deadline) {
			_ = file.Close()
			return nil, ErrSecretsLocked
		}

		wait := backoff
		if remaining := time.Until(deadline); wait > remaining {
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

		if backoff < maxLockBackoff {
			backoff *= 2
		}
	}
}
