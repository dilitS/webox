package config

import "errors"

// ErrInvalidJSON wraps a syntactic JSON failure (the document never even
// reaches schema validation). Callers can distinguish "your file is broken"
// from "your file is well-formed but does not match the contract" via
// errors.Is.
var ErrInvalidJSON = errors.New("config: invalid JSON")

// ErrSchemaViolation wraps any JSON Schema (Draft 2020-12) violation
// reported by the embedded schema in schema.json. The wrapped error string
// preserves the underlying validator's pointer/path so doctor and TUI can
// render actionable messages without parsing it back.
var ErrSchemaViolation = errors.New("config: schema violation")

// ErrSecretInConfig is returned when the config document contains a
// secret-shaped string. Per AGENTS.md §2.1 this is a non-negotiable
// guardrail: plaintext secrets never belong in config.json.
var ErrSecretInConfig = errors.New("config: secret-shaped string rejected")

// ErrDanglingProfileAlias is returned when some projects[].profile_alias
// points at no profiles[].alias. DESIGN §6.1 defines profile_alias as an
// FK, so accepting dangling references would violate the in-memory model.
var ErrDanglingProfileAlias = errors.New("config: project references unknown profile alias")

// ErrCorruptedConfig wraps any failure that prevents Load from turning
// the on-disk file into a *Config: I/O failure, malformed JSON, struct
// decoding failure. The companion message points at `webox doctor` so
// users have an actionable next step (DESIGN §6.2, sprint-01 TASK-01.2).
var ErrCorruptedConfig = errors.New("config: corrupted file (run `webox doctor` to inspect)")

// ErrSchemaMismatch is returned when the file is well-formed JSON but
// either violates the embedded schema or carries a schema_version newer
// than the binary supports (DESIGN §6.4: downgrades are not supported).
var ErrSchemaMismatch = errors.New("config: schema mismatch")

// ErrMigrationFailed is returned when Load tries to forward-migrate a
// schema_version older than [Current] but the migrator chain reports an
// error. Migration framework lands with TASK-01.4; in v0.1 this path is
// reachable only via a stubbed migrator that signals "no v0 migrator
// registered".
var ErrMigrationFailed = errors.New("config: schema migration failed")

// ErrConfigLocked is returned when Save cannot acquire the per-config
// flock within the retry window. DESIGN §6 prescribes an actionable
// message so the operator knows to close the other instance or wait.
var ErrConfigLocked = errors.New("config: webox is already running; close the other instance or wait")

// errNoMigrator is returned by the migrate stub when a forward
// migration is required but no migrator is registered for the source
// version. Wrapped by [ErrMigrationFailed]; the migration framework
// (TASK-01.4) will replace this stub with a real, ordered chain.
var errNoMigrator = errors.New("no migrator registered for source schema_version")
