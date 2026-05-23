package wizard

import "time"

// ResourceKind enumerates the rollback-supported provider resources.
// The dispatcher in [MakeStepRunner] switches on this token; adding a
// new kind requires extending the switch AND the snapshot upgrade
// path. The string form is what lands in `pending_cleanups.json` so
// it is part of the on-disk contract.
type ResourceKind string

const (
	// ResourceSubdomain maps to [providers.HostingProvider.RemoveSubdomain].
	// Params: {"domain": "<fully-qualified subdomain>"}.
	ResourceSubdomain ResourceKind = "subdomain"

	// ResourceSSL maps to [providers.HostingProvider.RemoveSSL].
	// Params: {"domain": "<fully-qualified subdomain>"}.
	ResourceSSL ResourceKind = "ssl"

	// ResourceDatabase maps to [providers.HostingProvider.RemoveDatabase].
	// Params: {"dbKind": "mysql|postgresql", "dbName": "<identifier>"}.
	ResourceDatabase ResourceKind = "database"
)

// CleanupStep is the persisted record of a rollback action. Every
// field MUST be serialisable to JSON without information loss because
// the LIFO stack is snapshotted to `pending_cleanups.json` on every
// push so an interrupted wizard can resume on the next launch.
//
// IMPORTANT: Params is metadata-only. Secrets (passwords, tokens, SSH
// keys, generated DB credentials) NEVER appear here — they live in
// the keyring / AES-GCM fallback. [Stack.Push] enforces this by
// scanning Params for secret-shaped values before persisting.
type CleanupStep struct {
	// Name is the human-readable label rendered in the wizard's
	// rollback prompt and the resume summary. Stable across schema
	// versions because it is purely cosmetic.
	Name string `json:"name"`

	// Kind selects the [providers.HostingProvider] method invoked
	// by [MakeStepRunner]. See [ResourceKind] for the supported set.
	Kind ResourceKind `json:"kind"`

	// Params carries the metadata each Remove* method needs. Keys
	// are kind-specific; values MUST be plaintext-safe.
	Params map[string]string `json:"params,omitempty"`

	// CreatedAt timestamps the push so the resume UI can sort and
	// show "stack created N minutes ago".
	CreatedAt time.Time `json:"created_at"`
}

// ProvisionPlan is the wizard-collected snapshot of user choices the
// execution step turns into provider calls. It is intentionally
// secret-free: even DB credentials returned by [providers.HostingProvider.CreateDatabase]
// are NOT stored here — the execution step keeps them in the report
// only long enough to hand them to the keyring writer.
type ProvisionPlan struct {
	// ProfileAlias references the [config.Profile.Alias] selected by
	// the user. The wizard validates the alias resolves before it
	// constructs a [providers.HostingProvider].
	ProfileAlias string

	// Stack is one of [SupportedStacks].
	Stack string

	// Domain is the fully-qualified subdomain the wizard will create.
	// MUST satisfy [smallhost.ValidateDomain] before execution.
	Domain string

	// NodeVersion is the major version string accepted by
	// [smallhost.ValidateNodeVersion] (e.g. "20", "22"). Required for
	// node-express; empty for static.
	NodeVersion string

	// DBKind is one of [providers.DatabaseMySQL] / [providers.DatabasePostgres]
	// or empty when the wizard skipped the database step.
	DBKind string

	// DBName is the requested database identifier. MUST satisfy
	// [smallhost.ValidateDBName] when DBKind is non-empty.
	DBName string
}

// DatabaseCredentials is the panel-generated DB account returned by
// the provider after a successful database step. The wizard holds it
// in memory just long enough to persist into the keyring; it is
// NEVER serialised to `config.json` or `pending_cleanups.json`.
type DatabaseCredentials struct {
	Username string
	Password string
}

// ProvisionStatus describes the outcome of a single execution step
// (subdomain / SSL / database). Either OK is true and Err is nil, or
// Err is non-nil. The wizard renders this struct directly in the
// failure remediation view.
type ProvisionStatus struct {
	Step string
	OK   bool
	Err  error
}

// ProvisionReport aggregates the per-step outcomes for a single
// wizard execution. Empty fields (OK=false, Err=nil) mean the step
// never ran — useful in the failure UI to distinguish "failed" from
// "skipped".
type ProvisionReport struct {
	Subdomain ProvisionStatus
	SSL       ProvisionStatus
	Database  ProvisionStatus

	// Credentials carries the DB credentials returned by the
	// provider on a successful database step. Nil when the wizard
	// skipped DB provisioning or when the step failed. The wizard
	// MUST move Password into the secrets backend immediately after
	// reading it and zero this field before the report leaves the
	// wizard goroutine.
	Credentials *DatabaseCredentials
}

// CleanupResult is the per-step outcome reported by [Stack.Rollback].
// Each successful pop produces one CleanupResult regardless of Err so
// callers can render a per-step status list.
type CleanupResult struct {
	Step CleanupStep
	Err  error
}
