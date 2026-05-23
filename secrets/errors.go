package secrets

import "errors"

// ErrBrokenKeyring means the OS keyring accepted a probe write but could
// not read it back. That is materially different from an unsupported
// platform and should be surfaced through doctor.
var ErrBrokenKeyring = errors.New("secrets: broken keyring; run webox doctor")

// ErrSecretNotFound means the backend works, but the requested secret has
// not been stored yet.
var ErrSecretNotFound = errors.New("secrets: secret not found")

// ErrFallbackLocked is returned by any operation on a zero-value
// [FallbackBackend]. Construct a usable instance via [NewFallback] with
// a master password before calling Get/Set/Delete.
var ErrFallbackLocked = errors.New("secrets: fallback backend is locked; call NewFallback first")

// ErrAuthFailed means the master password did not produce a key that
// could authenticate the on-disk AES-GCM ciphertext. This is the
// non-information-leaking name for "wrong password OR tampered file"
// (see docs/SECURITY.md §4.2.1).
var ErrAuthFailed = errors.New("secrets: master password rejected by AES-GCM")

// ErrCorruptedSecrets means the encrypted secrets file is structurally
// invalid: truncated, unknown version byte, or otherwise unparseable
// before AES-GCM authentication is even attempted.
var ErrCorruptedSecrets = errors.New("secrets: encrypted file is corrupted")

// ErrMasterPasswordTooShort means the supplied master password is below
// the [MasterPasswordMinLen] threshold required by ADR-0004.
var ErrMasterPasswordTooShort = errors.New("secrets: master password is too short")

// ErrKeyringUnavailable signals that [Detect] could not find a usable OS
// keyring and the caller must construct an encrypted fallback via
// [NewFallback] with a master password (interactive or via
// WEBOX_MASTER_PASSWORD; see docs/SECURITY.md §4.2.2).
var ErrKeyringUnavailable = errors.New("secrets: OS keyring unavailable; use encrypted fallback")
