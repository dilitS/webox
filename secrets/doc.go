// Package secrets is the only place in Webox that handles plaintext
// credentials.
//
// The default backend is the OS keyring (zalando/go-keyring), detected
// via a probe that distinguishes ErrUnsupportedPlatform from ErrNotFound.
// On headless systems Webox falls back to an AES-GCM-encrypted file
// guarded by an Argon2id-derived key; nonces come exclusively from
// crypto/rand and the package panics if the CSPRNG fails. In-memory
// secrets are kept in memguard.LockedBuffer values and zeroed via
// Destroy. See docs/SECURITY.md §4 and docs/adr/0004 for the rationale
// and AUDIT §8 IMP-2/IMP-9 for the hardening history.
package secrets
