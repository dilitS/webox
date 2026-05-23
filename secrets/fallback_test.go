package secrets

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

const testMasterPassword = "correct-horse-battery-staple"

func newTestVaultPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "secrets.enc")
}

func TestNewFallback_RoundTripFreshFile(t *testing.T) {
	t.Parallel()

	path := newTestVaultPath(t)
	backend, err := NewFallback(path, []byte(testMasterPassword))
	if err != nil {
		t.Fatalf("NewFallback() = %v, want nil", err)
	}
	t.Cleanup(backend.Close)

	if err := backend.Set("github-token", []byte("ghp_value")); err != nil {
		t.Fatalf("Set() = %v, want nil", err)
	}
	got, err := backend.Get("github-token")
	if err != nil {
		t.Fatalf("Get() = %v, want nil", err)
	}
	if !bytes.Equal(got, []byte("ghp_value")) {
		t.Fatalf("Get() = %q, want %q", got, "ghp_value")
	}
}

func TestNewFallback_PersistsAcrossInstances(t *testing.T) {
	t.Parallel()

	path := newTestVaultPath(t)

	first, err := NewFallback(path, []byte(testMasterPassword))
	if err != nil {
		t.Fatalf("NewFallback(first) = %v", err)
	}
	if err := first.Set("db-password", []byte("hunter2")); err != nil {
		t.Fatalf("Set() = %v", err)
	}
	first.Close()

	second, err := NewFallback(path, []byte(testMasterPassword))
	if err != nil {
		t.Fatalf("NewFallback(second) = %v", err)
	}
	t.Cleanup(second.Close)

	got, err := second.Get("db-password")
	if err != nil {
		t.Fatalf("Get(second) = %v", err)
	}
	if !bytes.Equal(got, []byte("hunter2")) {
		t.Fatalf("Get(second) = %q, want hunter2", got)
	}
}

func TestNewFallback_WrongPasswordReturnsAuthFailed(t *testing.T) {
	t.Parallel()

	path := newTestVaultPath(t)
	first, err := NewFallback(path, []byte(testMasterPassword))
	if err != nil {
		t.Fatalf("NewFallback() = %v", err)
	}
	if err := first.Set("k", []byte("v")); err != nil {
		t.Fatalf("Set() = %v", err)
	}
	first.Close()

	_, err = NewFallback(path, []byte("definitely-wrong-password"))
	if !errors.Is(err, ErrAuthFailed) {
		t.Fatalf("NewFallback(wrong password) = %v, want ErrAuthFailed", err)
	}
}

func TestNewFallback_CorruptFileReturnsCorruptedSecrets(t *testing.T) {
	t.Parallel()

	path := newTestVaultPath(t)
	if err := os.WriteFile(path, []byte("garbage"), ownerOnlyPerm); err != nil {
		t.Fatalf("write corrupt file: %v", err)
	}
	_, err := NewFallback(path, []byte(testMasterPassword))
	if !errors.Is(err, ErrCorruptedSecrets) {
		t.Fatalf("NewFallback(corrupt) = %v, want ErrCorruptedSecrets", err)
	}
}

func TestNewFallback_UnknownVersionReturnsCorruptedSecrets(t *testing.T) {
	t.Parallel()

	path := newTestVaultPath(t)
	header := make([]byte, 1+saltSize+nonceSize+gcmTagSize)
	header[0] = 0xFF
	if err := os.WriteFile(path, header, ownerOnlyPerm); err != nil {
		t.Fatalf("write file: %v", err)
	}
	_, err := NewFallback(path, []byte(testMasterPassword))
	if !errors.Is(err, ErrCorruptedSecrets) {
		t.Fatalf("NewFallback(unknown version) = %v, want ErrCorruptedSecrets", err)
	}
}

func TestNewFallback_MasterPasswordTooShort(t *testing.T) {
	t.Parallel()

	path := newTestVaultPath(t)
	_, err := NewFallback(path, []byte("short"))
	if !errors.Is(err, ErrMasterPasswordTooShort) {
		t.Fatalf("NewFallback(short) = %v, want ErrMasterPasswordTooShort", err)
	}
}

func TestFallbackBackend_DeleteIsIdempotent(t *testing.T) {
	t.Parallel()

	path := newTestVaultPath(t)
	backend, err := NewFallback(path, []byte(testMasterPassword))
	if err != nil {
		t.Fatalf("NewFallback() = %v", err)
	}
	t.Cleanup(backend.Close)

	if err := backend.Set("k", []byte("v")); err != nil {
		t.Fatalf("Set() = %v", err)
	}
	if err := backend.Delete("k"); err != nil {
		t.Fatalf("Delete() = %v", err)
	}
	if err := backend.Delete("k"); err != nil {
		t.Fatalf("Delete(again) = %v, want nil idempotent", err)
	}
	if _, err := backend.Get("k"); !errors.Is(err, ErrSecretNotFound) {
		t.Fatalf("Get(deleted) = %v, want ErrSecretNotFound", err)
	}
}

func TestFallbackBackend_GetMissingReturnsNotFound(t *testing.T) {
	t.Parallel()

	backend, err := NewFallback(newTestVaultPath(t), []byte(testMasterPassword))
	if err != nil {
		t.Fatalf("NewFallback() = %v", err)
	}
	t.Cleanup(backend.Close)

	if _, err := backend.Get("never-set"); !errors.Is(err, ErrSecretNotFound) {
		t.Fatalf("Get(missing) = %v, want ErrSecretNotFound", err)
	}
}

func TestFallbackBackend_NonceUniquenessAcrossWrites(t *testing.T) {
	t.Parallel()

	path := newTestVaultPath(t)
	backend, err := NewFallback(path, []byte(testMasterPassword))
	if err != nil {
		t.Fatalf("NewFallback() = %v", err)
	}
	t.Cleanup(backend.Close)

	const writes = 1000
	seen := make(map[string]struct{}, writes)
	for i := 0; i < writes; i++ {
		if err := backend.Set("k", []byte{byte(i)}); err != nil {
			t.Fatalf("Set(%d) = %v", i, err)
		}
		nonce, err := readVaultNonce(path)
		if err != nil {
			t.Fatalf("read nonce after write %d: %v", i, err)
		}
		key := string(nonce)
		if _, dup := seen[key]; dup {
			t.Fatalf("duplicate nonce after %d writes: %x", i, nonce)
		}
		seen[key] = struct{}{}
	}
	if len(seen) != writes {
		t.Fatalf("unique nonces = %d, want %d", len(seen), writes)
	}
}

func TestFallbackBackend_RotatePasswordReencrypts(t *testing.T) {
	t.Parallel()

	path := newTestVaultPath(t)
	backend, err := NewFallback(path, []byte(testMasterPassword))
	if err != nil {
		t.Fatalf("NewFallback() = %v", err)
	}
	if err := backend.Set("k", []byte("v")); err != nil {
		t.Fatalf("Set() = %v", err)
	}
	oldSalt, err := readVaultSalt(path)
	if err != nil {
		t.Fatalf("read old salt: %v", err)
	}

	const newPassword = "another-correct-horse-battery"
	if err := backend.RotatePassword([]byte(newPassword)); err != nil {
		t.Fatalf("RotatePassword() = %v", err)
	}
	backend.Close()

	if _, err := NewFallback(path, []byte(testMasterPassword)); !errors.Is(err, ErrAuthFailed) {
		t.Fatalf("Old password should fail after rotate; got %v", err)
	}

	reopened, err := NewFallback(path, []byte(newPassword))
	if err != nil {
		t.Fatalf("NewFallback(new password) = %v", err)
	}
	t.Cleanup(reopened.Close)

	got, err := reopened.Get("k")
	if err != nil {
		t.Fatalf("Get() after rotate = %v", err)
	}
	if !bytes.Equal(got, []byte("v")) {
		t.Fatalf("Get() = %q, want v", got)
	}
	newSalt, err := readVaultSalt(path)
	if err != nil {
		t.Fatalf("read new salt: %v", err)
	}
	if bytes.Equal(oldSalt, newSalt) {
		t.Fatalf("salt did not change after rotate")
	}
}

func TestFallbackBackend_FilePermissionsAreOwnerOnly(t *testing.T) {
	t.Parallel()

	path := newTestVaultPath(t)
	backend, err := NewFallback(path, []byte(testMasterPassword))
	if err != nil {
		t.Fatalf("NewFallback() = %v", err)
	}
	t.Cleanup(backend.Close)
	if err := backend.Set("k", []byte("v")); err != nil {
		t.Fatalf("Set() = %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if mode := info.Mode().Perm(); mode != ownerOnlyPerm {
		t.Fatalf("perms = %o, want %o", mode, ownerOnlyPerm)
	}
}

func TestFallbackBackend_ConcurrentSetSafe(t *testing.T) {
	t.Parallel()

	path := newTestVaultPath(t)
	backend, err := NewFallback(path, []byte(testMasterPassword))
	if err != nil {
		t.Fatalf("NewFallback() = %v", err)
	}
	t.Cleanup(backend.Close)

	var wg sync.WaitGroup
	const workers = 16
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(id int) {
			defer wg.Done()
			key := string([]byte{byte('a' + id)})
			value := []byte{byte(id)}
			if err := backend.Set(key, value); err != nil {
				t.Errorf("Set(%s) = %v", key, err)
			}
		}(i)
	}
	wg.Wait()

	for i := 0; i < workers; i++ {
		key := string([]byte{byte('a' + i)})
		got, err := backend.Get(key)
		if err != nil {
			t.Fatalf("Get(%s) = %v", key, err)
		}
		if len(got) != 1 || got[0] != byte(i) {
			t.Fatalf("Get(%s) = %x, want %x", key, got, []byte{byte(i)})
		}
	}
}

func TestFallbackBackend_CSPRNGFailurePanics(t *testing.T) {
	original := randReader
	t.Cleanup(func() { randReader = original })
	randReader = errReader{}

	path := newTestVaultPath(t)
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected panic on CSPRNG failure, got nil")
		}
	}()
	// NewFallback on a fresh file generates a salt via crypto/rand; this
	// must panic when the CSPRNG fails (per SECURITY §4.2.1 / AUDIT IMP-2).
	_, _ = NewFallback(path, []byte(testMasterPassword))
}

func TestFallbackBackend_LockedBackendReturnsErrFallbackLocked(t *testing.T) {
	t.Parallel()

	var locked Backend = &FallbackBackend{}
	if _, err := locked.Get("k"); !errors.Is(err, ErrFallbackLocked) {
		t.Fatalf("Get() = %v, want ErrFallbackLocked", err)
	}
	if err := locked.Set("k", []byte("v")); !errors.Is(err, ErrFallbackLocked) {
		t.Fatalf("Set() = %v, want ErrFallbackLocked", err)
	}
	if err := locked.Delete("k"); !errors.Is(err, ErrFallbackLocked) {
		t.Fatalf("Delete() = %v, want ErrFallbackLocked", err)
	}
}

type errReader struct{}

func (errReader) Read(_ []byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}

func readVaultNonce(path string) ([]byte, error) {
	data, err := os.ReadFile(path) //nolint:gosec // G304: test helper reads test-scoped temp path.
	if err != nil {
		return nil, err
	}
	if len(data) < 1+saltSize+nonceSize {
		return nil, errors.New("file too small for header")
	}
	return data[1+saltSize : 1+saltSize+nonceSize], nil
}

func readVaultSalt(path string) ([]byte, error) {
	data, err := os.ReadFile(path) //nolint:gosec // G304: test helper reads test-scoped temp path.
	if err != nil {
		return nil, err
	}
	if len(data) < 1+saltSize {
		return nil, errors.New("file too small for header")
	}
	return data[1 : 1+saltSize], nil
}
