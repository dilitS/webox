package config

import "fmt"

// migrate forwards an in-memory Config from `cfg.SchemaVersion` to
// [Current]. The full migration framework (registration, ordered chain,
// idempotency, rollback backups per DESIGN §6.4) lands with TASK-01.4.
//
// Until then this stub satisfies the [Load] contract: it returns the
// config unchanged when no migration is required, and a structured
// error otherwise so [Load] can wrap it as [ErrMigrationFailed]. The
// JSON schema's `schema_version >= 1` minimum keeps this branch
// effectively unreachable from real files in v0.1, but the path exists
// so future migrators have a single seam to plug into.
func migrate(cfg *Config) (*Config, error) {
	if cfg == nil {
		return nil, errNilConfig
	}
	if cfg.SchemaVersion == Current {
		return cfg, nil
	}
	return nil, fmt.Errorf("%w: v%d → v%d (TASK-01.4)",
		errNoMigrator, cfg.SchemaVersion, Current)
}
