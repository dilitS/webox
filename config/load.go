package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// Load reads path, validates it against the embedded JSON Schema, and
// forward-migrates the schema_version if the file is older than
// [Current]. It deliberately does NOT create the file when it is missing
// — that is [Save]'s job (DESIGN §6, sprint-01 TASK-01.2 note).
//
// Returned errors are wrapped sentinels so callers can react via
// errors.Is:
//
//   - context.Canceled / context.DeadlineExceeded — propagated as-is.
//   - [ErrCorruptedConfig] — file unreadable or not parseable as JSON.
//   - [ErrSchemaMismatch]  — file conflicts with the schema, or its
//     schema_version is newer than this binary supports (downgrade is
//     not supported per DESIGN §6.4).
//   - [ErrMigrationFailed] — the migrator chain refused to forward the
//     legacy file to [Current]. Migration framework lands with TASK-01.4;
//     in v0.1 this only fires for synthetic stubs.
//
// On a missing file Load returns the value of [DefaultConfig] and a nil
// error. The caller decides whether to persist that default via [Save].
func Load(ctx context.Context, path string) (*Config, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	raw, err := os.ReadFile(path) //nolint:gosec // G304: caller-provided path, audited at config layer.
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("%w: read %s: %w", ErrCorruptedConfig, path, err)
	}

	if vErr := Validate(raw); vErr != nil {
		if errors.Is(vErr, ErrSchemaViolation) {
			return nil, fmt.Errorf("%w: %w", ErrSchemaMismatch, vErr)
		}
		return nil, fmt.Errorf("%w: %w", ErrCorruptedConfig, vErr)
	}

	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("%w: decode struct: %w", ErrCorruptedConfig, err)
	}

	switch {
	case cfg.SchemaVersion > Current:
		return nil, fmt.Errorf("%w: file is v%d, this binary supports v%d (downgrade unsupported)",
			ErrSchemaMismatch, cfg.SchemaVersion, Current)
	case cfg.SchemaVersion < Current:
		migrated, mErr := migrate(&cfg)
		if mErr != nil {
			return nil, fmt.Errorf("%w: %w", ErrMigrationFailed, mErr)
		}
		cfg = *migrated
	}

	return &cfg, nil
}

// DefaultConfig returns the in-memory defaults used when no file exists.
// [Load] returns this when the path doesn't exist; [Save] is responsible
// for materialising it on disk on the next write.
func DefaultConfig() *Config {
	return &Config{
		SchemaVersion: Current,
		Language:      "en",
		Profiles:      []Profile{},
		Projects:      []Project{},
	}
}
