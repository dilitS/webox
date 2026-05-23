//go:build windows

package secrets

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrSecretsLocked is the cross-platform sentinel surfaced when the
// per-vault lock cannot be acquired. The Windows stub returns it for
// every attempt until LockFileEx support lands (mirrors the
// config/lock_windows.go posture documented in sprint-01 Risk R-013).
var ErrSecretsLocked = errors.New("secrets: windows file locking is not implemented yet")

func lockExclusive(_ context.Context, lockPath string, _, _ time.Duration) (func() error, error) {
	return nil, fmt.Errorf("%w: %s", ErrSecretsLocked, lockPath)
}
