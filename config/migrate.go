package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
)

// Migration transforms raw config JSON at one schema version into raw
// config JSON at the next schema version. It returns the new version so
// the registry can verify forward progress and avoid migration loops.
type Migration func(in []byte) (out []byte, newVersion int, err error)

var migrations = map[int]Migration{
	0: migrateV0toV1,
}

// Migrate forwards data to [Current]. If data is already at [Current],
// Migrate validates it and returns it unchanged, making the operation
// idempotent for callers that cannot easily know whether a file is stale.
func Migrate(data []byte) ([]byte, error) {
	version, err := schemaVersionOf(data)
	if err != nil {
		return nil, err
	}
	if version > Current {
		return nil, fmt.Errorf("%w: file is v%d, this binary supports v%d (downgrade unsupported)",
			ErrSchemaMismatch, version, Current)
	}
	if version == Current {
		if err := Validate(data); err != nil {
			return nil, err
		}
		return data, nil
	}

	current := data
	for version < Current {
		migration, ok := migrations[version]
		if !ok {
			return nil, fmt.Errorf("%w: %w: v%d → v%d",
				ErrMigrationFailed, errNoMigrator, version, Current)
		}

		next, newVersion, err := migration(current)
		if err != nil {
			return nil, fmt.Errorf("%w: v%d → v%d: %w",
				ErrMigrationFailed, version, newVersion, err)
		}
		if newVersion <= version {
			return nil, fmt.Errorf("%w: migration v%d returned non-forward version %d",
				ErrMigrationFailed, version, newVersion)
		}
		slog.Info(
			"config schema migrated",
			"migrationFrom", version,
			"migrationTo", newVersion,
		)

		current = next
		version = newVersion
	}

	if err := Validate(current); err != nil {
		return nil, fmt.Errorf("%w: migrated config invalid: %w", ErrMigrationFailed, err)
	}
	return current, nil
}

func schemaVersionOf(data []byte) (int, error) {
	var header struct {
		SchemaVersion *int `json:"schema_version"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return 0, fmt.Errorf("%w: %w", ErrInvalidJSON, err)
	}
	if header.SchemaVersion == nil {
		return 0, nil
	}
	return *header.SchemaVersion, nil
}

func wrapMigrationLoadError(err error) error {
	if errors.Is(err, ErrInvalidJSON) {
		return fmt.Errorf("%w: %w", ErrCorruptedConfig, err)
	}
	return fmt.Errorf("%w: %w", ErrMigrationFailed, err)
}
