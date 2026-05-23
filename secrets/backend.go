package secrets

// Backend stores and retrieves secret bytes by logical key. Concrete
// implementations live in keyring.go (OS keyring) and fallback.go
// (AES-GCM-encrypted file). Implementations are expected to be safe for
// concurrent use across goroutines.
type Backend interface {
	Get(key string) ([]byte, error)
	Set(key string, value []byte) error
	Delete(key string) error
}
