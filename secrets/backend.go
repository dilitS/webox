package secrets

// Backend stores and retrieves secret bytes by logical key.
type Backend interface {
	Get(key string) ([]byte, error)
	Set(key string, value []byte) error
	Delete(key string) error
}

// FallbackBackend is a compile-time placeholder for TASK-01.7's
// AES-GCM encrypted-file backend.
type FallbackBackend struct{}

// Get returns [ErrFallbackUnavailable] until TASK-01.7 implements the
// encrypted fallback store.
func (b *FallbackBackend) Get(_ string) ([]byte, error) {
	return nil, ErrFallbackUnavailable
}

// Set returns [ErrFallbackUnavailable] until TASK-01.7 implements the
// encrypted fallback store.
func (b *FallbackBackend) Set(_ string, _ []byte) error {
	return ErrFallbackUnavailable
}

// Delete returns [ErrFallbackUnavailable] until TASK-01.7 implements the
// encrypted fallback store.
func (b *FallbackBackend) Delete(_ string) error {
	return ErrFallbackUnavailable
}
