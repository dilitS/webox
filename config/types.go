package config

import "time"

// Current is the schema version of the on-disk format Webox v0.1 ships
// with. Bumped on every backward-incompatible change (see
// docs/DESIGN.md §6.4 migration framework).
const Current = 1

// Config is the in-memory representation of $XDG_CONFIG_HOME/webox/config.json.
// Field-by-field source of truth: docs/DESIGN.md §6.1.
//
// Invariants (enforced by JSON Schema in schema.json AND callers must
// preserve when constructing in memory):
//
//   - SchemaVersion >= 1.
//   - Profiles[*].Alias matches ^[a-z0-9-]{1,32}$.
//   - Each Projects[*].ProfileAlias references some Profiles[*].Alias.
//   - No secrets ever live here: only metadata (docs/SECURITY.md §10.6).
type Config struct {
	SchemaVersion int       `json:"schema_version"`
	Language      string    `json:"language,omitempty"`
	Profiles      []Profile `json:"profiles"`
	Projects      []Project `json:"projects"`
	Settings      *Settings `json:"settings,omitempty"`
}

// Profile is a hosting-provider profile (one host + one user).
type Profile struct {
	Alias      string            `json:"alias"`
	Type       string            `json:"type"`
	Host       string            `json:"host"`
	Port       int               `json:"port,omitempty"`
	User       string            `json:"user"`
	Properties map[string]string `json:"properties,omitempty"`
}

// Project is a deployable application linked to a Profile.
type Project struct {
	ID           string       `json:"id"`
	Domain       string       `json:"domain"`
	ProfileAlias string       `json:"profile_alias"`
	Repo         string       `json:"repo,omitempty"`
	LocalPath    string       `json:"local_path,omitempty"`
	Stack        string       `json:"stack,omitempty"`
	NodeVersion  string       `json:"node_version,omitempty"`
	ImportedAt   *time.Time   `json:"imported_at,omitempty"`
	SecretsMeta  []SecretMeta `json:"secrets_meta,omitempty"`
}

// SecretSource enumerates how Webox knows (or doesn't) the value of an
// application secret. See docs/SECURITY.md §10.6.
type SecretSource string

// Allowed values for SecretMeta.Source. The schema enforces this set;
// callers that build Config in memory should also use these constants
// rather than raw strings.
const (
	SecretSourceManaged    SecretSource = "managed"
	SecretSourceServerOnly SecretSource = "server_only"
	SecretSourceExternal   SecretSource = "external"
)

// SecretMeta is metadata about an application secret. Plaintext values
// NEVER appear here — Webox keeps them in the OS keyring or AES-GCM
// fallback (docs/SECURITY.md §4).
type SecretMeta struct {
	Key                  string       `json:"key"`
	CreatedAt            time.Time    `json:"created_at"`
	LastRotated          *time.Time   `json:"last_rotated,omitempty"`
	Source               SecretSource `json:"source"`
	LastSyncedGitHub     *time.Time   `json:"last_synced_github,omitempty"`
	LastSyncedServer     *time.Time   `json:"last_synced_server,omitempty"`
	RotationReminderDays int          `json:"rotation_reminder_days,omitempty"`
}

// Settings is the global preferences block. Fields default to their
// zero value, which mirrors the JSON Schema's optional-object semantics.
type Settings struct {
	ExpertMode       bool `json:"expert_mode,omitempty"`
	RefreshIntervalS int  `json:"refresh_interval_s,omitempty"`
}
