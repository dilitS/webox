package wizard

import "errors"

// ErrInvalidStep is the sentinel returned by [Stack.Push] when a
// [CleanupStep] violates structural invariants (missing name, missing
// kind, unrecognised resource kind). It is a programmer error rather
// than user input so the wizard surfaces it loudly during development
// instead of swallowing it on disk.
var ErrInvalidStep = errors.New("wizard: invalid cleanup step")

// ErrSecretInCleanup is returned by [Stack.Push] when a [CleanupStep]
// carries a value that matches the project-wide secret-shape redactor
// list. The LIFO snapshot is persisted to `pending_cleanups.json`
// without encryption, so any plaintext secret would create a
// long-lived disclosure surface — defense in depth on top of the
// "wizard never passes secrets in Params" coding rule.
var ErrSecretInCleanup = errors.New("wizard: cleanup step carries a secret-shaped value")

// ErrUnsupportedKind is the sentinel returned by [MakeStepRunner]
// when a snapshot contains a [ResourceKind] this binary does not know
// how to handle. Resuming an upgrade-incompatible snapshot fails
// closed rather than silently skipping orphan resources.
var ErrUnsupportedKind = errors.New("wizard: unsupported cleanup kind")

// ErrInvalidPlan is the sentinel returned by [ValidatePlan] when the
// wizard-collected [ProvisionPlan] would be invalid before any
// provider call is made. Wraps a specific reason in the error message
// (missing stack, unsupported DB kind, invalid domain, …).
var ErrInvalidPlan = errors.New("wizard: invalid provision plan")

// ErrCorruptedSnapshot is returned by [LoadPending] when the on-disk
// `pending_cleanups.json` cannot be decoded. The wizard treats this
// as "abandon and prompt operator" rather than auto-resuming, to keep
// resume from acting on a half-written file.
var ErrCorruptedSnapshot = errors.New("wizard: corrupted pending_cleanups.json")

// ErrSchemaMismatch is returned by [LoadPending] when the snapshot's
// schema_version is newer than this binary supports. Mirrors the
// `config` package contract — downgrade is not supported.
var ErrSchemaMismatch = errors.New("wizard: pending_cleanups.json schema_version unsupported")

// ErrPreflightSSHDisconnected is the sentinel surfaced by [Preflight]
// when the panel returns a probe with [providers.ProviderStatus.SSHConnected]
// false. Wraps to a typed error so the wizard's failure UI can branch
// on it (suggest `webox doctor`) without parsing strings.
var ErrPreflightSSHDisconnected = errors.New("wizard: preflight: ssh disconnected")

// ErrPreflightNilStatus is the sentinel surfaced when the provider
// returns a nil [providers.ProviderStatus] without an error — a
// programmer bug at the adapter layer that the wizard catches before
// dispatching execution.
var ErrPreflightNilStatus = errors.New("wizard: preflight returned nil status")
