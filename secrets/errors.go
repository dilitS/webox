package secrets

import "errors"

// ErrBrokenKeyring means the OS keyring accepted a probe write but could
// not read it back. That is materially different from an unsupported
// platform and should be surfaced through doctor.
var ErrBrokenKeyring = errors.New("secrets: broken keyring; run webox doctor")

// ErrSecretNotFound means the backend works, but the requested secret has
// not been stored yet.
var ErrSecretNotFound = errors.New("secrets: secret not found")

// ErrFallbackUnavailable is returned by the TASK-01.6 placeholder
// fallback backend. TASK-01.7 replaces it with the AES-GCM backend.
var ErrFallbackUnavailable = errors.New("secrets: fallback backend unavailable")
