package secrets

import (
	"errors"
	"testing"
)

func TestGitHubPATHelpersRoundTripWithoutLeakingToken(t *testing.T) {
	t.Parallel()

	backend := newMemoryBackend()
	token := "github_" + "pat_" + "12345678901234567890123456789012345678901234567890123456789012345678901234567890"
	if err := SetGitHubPAT(backend, "default", []byte(token)); err != nil {
		t.Fatalf("SetGitHubPAT: %v", err)
	}
	got, err := GetGitHubPAT(backend, "default")
	if err != nil {
		t.Fatalf("GetGitHubPAT: %v", err)
	}
	if string(got) != token {
		t.Fatalf("token round-trip mismatch")
	}
}

func TestGetGitHubPATMissingUsesSentinel(t *testing.T) {
	t.Parallel()

	_, err := GetGitHubPAT(newMemoryBackend(), "default")
	if !errors.Is(err, ErrSecretNotFound) {
		t.Fatalf("err = %v, want ErrSecretNotFound", err)
	}
}

type memoryBackend struct {
	values map[string][]byte
}

func newMemoryBackend() *memoryBackend {
	return &memoryBackend{values: map[string][]byte{}}
}

func (b *memoryBackend) Get(key string) ([]byte, error) {
	value, ok := b.values[key]
	if !ok {
		return nil, ErrSecretNotFound
	}
	return append([]byte(nil), value...), nil
}

func (b *memoryBackend) Set(key string, value []byte) error {
	b.values[key] = append([]byte(nil), value...)
	return nil
}

func (b *memoryBackend) Delete(key string) error {
	delete(b.values, key)
	return nil
}
