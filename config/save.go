package config

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const (
	defaultLockTimeout    = 5 * time.Second
	defaultInitialBackoff = 10 * time.Millisecond
	configDirPerm         = 0o700
	ownerOnlyPerm         = 0o600
	tempSuffixBytes       = 6
)

type saveOptions struct {
	lockTimeout    time.Duration
	initialBackoff time.Duration
	beforeRename   func(string) error
}

// Save validates cfg, writes it to a temp file in the target directory,
// fsyncs the file, atomically renames it over path, fsyncs the parent
// directory, and releases the per-config flock. See docs/DESIGN.md §6.3.
func Save(ctx context.Context, path string, cfg *Config) error {
	return saveWithOptions(ctx, path, cfg, saveOptions{})
}

func saveWithOptions(ctx context.Context, path string, cfg *Config, opts saveOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if cfg == nil {
		return fmt.Errorf("%w: nil config", ErrSchemaMismatch)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, configDirPerm); err != nil {
		return fmt.Errorf("config: create parent dir %s: %w", dir, err)
	}

	unlock, err := lockExclusive(ctx, path+".lock", opts.lockTimeout, opts.initialBackoff)
	if err != nil {
		return err
	}
	defer func() { _ = unlock() }()

	raw, err := marshalConfig(cfg)
	if err != nil {
		return err
	}

	tmpPath, err := tempPath(path)
	if err != nil {
		return fmt.Errorf("config: generate temp path: %w", err)
	}
	if err := writeTempFile(tmpPath, raw); err != nil {
		return err
	}
	defer func() {
		if tmpPath != "" {
			_ = os.Remove(tmpPath)
		}
	}()

	if opts.beforeRename != nil {
		if err := opts.beforeRename(tmpPath); err != nil {
			return err
		}
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("config: rename %s -> %s: %w", tmpPath, path, err)
	}
	tmpPath = ""

	if err := syncDirectory(dir); err != nil {
		return err
	}

	return nil
}

func marshalConfig(cfg *Config) ([]byte, error) {
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("config: marshal: %w", err)
	}
	raw = append(raw, '\n')

	if err := Validate(raw); err != nil {
		if errors.Is(err, ErrSchemaViolation) ||
			errors.Is(err, ErrSecretInConfig) ||
			errors.Is(err, ErrDanglingProfileAlias) {
			return nil, fmt.Errorf("%w: %w", ErrSchemaMismatch, err)
		}
		return nil, err
	}

	return raw, nil
}

func tempPath(path string) (string, error) {
	suffix := make([]byte, tempSuffixBytes)
	if _, err := io.ReadFull(rand.Reader, suffix); err != nil {
		return "", fmt.Errorf("config: temp-path randomness: %w", err)
	}
	return fmt.Sprintf("%s.tmp.%d.%s", path, os.Getpid(), hex.EncodeToString(suffix)), nil
}

func writeTempFile(path string, raw []byte) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, ownerOnlyPerm) //nolint:gosec // G304: temp path is generated inside config dir by tempPath().
	if err != nil {
		return fmt.Errorf("config: open temp file %s: %w", path, err)
	}
	defer func() { _ = file.Close() }()

	if _, err := file.Write(raw); err != nil {
		return fmt.Errorf("config: write temp file %s: %w", path, err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("config: fsync temp file %s: %w", path, err)
	}
	return nil
}

func syncDirectory(dir string) error {
	handle, err := os.Open(dir) //nolint:gosec // G304: parent dir derived from caller path, audited at config layer.
	if err != nil {
		return fmt.Errorf("config: open parent dir %s: %w", dir, err)
	}
	defer func() { _ = handle.Close() }()

	if err := handle.Sync(); err != nil {
		return fmt.Errorf("config: fsync parent dir %s: %w", dir, err)
	}
	return nil
}
