//go:build windows

package config

import (
	"context"
	"fmt"
	"time"
)

func lockExclusive(_ context.Context, lockPath string, _, _ time.Duration) (func() error, error) {
	return nil, fmt.Errorf("%w: windows file locking for %s is not implemented yet", ErrConfigLocked, lockPath)
}
