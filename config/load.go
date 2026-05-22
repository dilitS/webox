package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
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

	version, err := schemaVersionOf(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCorruptedConfig, err)
	}
	if version > Current {
		return nil, fmt.Errorf("%w: file is v%d, this binary supports v%d (downgrade unsupported)",
			ErrSchemaMismatch, version, Current)
	}
	if version < Current {
		migratedRaw, err := Migrate(raw)
		if err != nil {
			return nil, wrapMigrationLoadError(err)
		}
		if err := backupOriginal(path, version, raw); err != nil {
			return nil, fmt.Errorf("%w: backup original: %w", ErrMigrationFailed, err)
		}

		var migrated Config
		if err := json.Unmarshal(migratedRaw, &migrated); err != nil {
			return nil, fmt.Errorf("%w: decode migrated struct: %w", ErrMigrationFailed, err)
		}
		if err := Save(ctx, path, &migrated); err != nil {
			return nil, fmt.Errorf("%w: save migrated config: %w", ErrMigrationFailed, err)
		}
		return &migrated, nil
	}

	if vErr := Validate(raw); vErr != nil {
		if errors.Is(vErr, ErrSchemaViolation) ||
			errors.Is(vErr, ErrSecretInConfig) ||
			errors.Is(vErr, ErrDanglingProfileAlias) {
			return nil, fmt.Errorf("%w: %w", ErrSchemaMismatch, vErr)
		}
		return nil, fmt.Errorf("%w: %w", ErrCorruptedConfig, vErr)
	}

	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("%w: decode struct: %w", ErrCorruptedConfig, err)
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

func backupOriginal(path string, oldVersion int, raw []byte) error {
	name := fmt.Sprintf(
		"%s.bak.v%d.%s",
		filepath.Base(path),
		oldVersion,
		time.Now().UTC().Format("20060102T150405.000000000Z"),
	)
	backupPath := filepath.Join(filepath.Dir(path), name)
	file, err := os.OpenFile(backupPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, ownerOnlyPerm) //nolint:gosec // G304: backup path is deterministic sibling of audited config path.
	if err != nil {
		return fmt.Errorf("open backup %s: %w", backupPath, err)
	}
	defer func() { _ = file.Close() }()

	if _, err := file.Write(raw); err != nil {
		return fmt.Errorf("write backup %s: %w", backupPath, err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("fsync backup %s: %w", backupPath, err)
	}
	return nil
}
