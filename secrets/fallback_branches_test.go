package secrets

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// TestNewFallback_ReadErrorOtherThanNotExist covers the os.ReadFile
// error path that is neither nil nor os.ErrNotExist. We trigger it by
// pointing the path at a directory: ReadFile on a directory returns a
// non-NotExist syscall error on every supported OS.
func TestNewFallback_ReadErrorOtherThanNotExist(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	_, err := NewFallback(dir, []byte(testMasterPassword))
	if err == nil {
		t.Fatalf("NewFallback(dir path) = nil, want IO error")
	}
	if errors.Is(err, ErrCorruptedSecrets) || errors.Is(err, ErrAuthFailed) {
		t.Fatalf("expected raw IO error, got typed sentinel: %v", err)
	}
}

// TestLoadFallback_PlaintextNotJSONReturnsCorrupted exercises the
// branch where AES-GCM authenticates but the decrypted plaintext is
// garbage rather than valid vault JSON. Forge a file with the right
// key but a plaintext that doesn't unmarshal.
func TestLoadFallback_PlaintextNotJSONReturnsCorrupted(t *testing.T) {
	t.Parallel()

	path := writeForgedVault(t, []byte("not-json-just-bytes"))
	_, err := NewFallback(path, []byte(testMasterPassword))
	if !errors.Is(err, ErrCorruptedSecrets) {
		t.Fatalf("NewFallback(plaintext garbage) = %v, want ErrCorruptedSecrets", err)
	}
}

// TestLoadFallback_WrongSchemaVersionReturnsCorrupted exercises the
// schema mismatch branch in loadFallback.
func TestLoadFallback_WrongSchemaVersionReturnsCorrupted(t *testing.T) {
	t.Parallel()

	plaintext, err := json.Marshal(vaultPayload{Schema: 99, Entries: map[string][]byte{}})
	if err != nil {
		t.Fatalf("marshal forged payload: %v", err)
	}
	path := writeForgedVault(t, plaintext)
	_, err = NewFallback(path, []byte(testMasterPassword))
	if !errors.Is(err, ErrCorruptedSecrets) {
		t.Fatalf("NewFallback(wrong schema) = %v, want ErrCorruptedSecrets", err)
	}
}

// TestLoadFallback_NilEntriesNormalised verifies the defensive
// allocation when a previously-saved vault decoded into a nil map (which
// only happens through hand-forged files or future-version downgrades).
func TestLoadFallback_NilEntriesNormalised(t *testing.T) {
	t.Parallel()

	plaintext, err := json.Marshal(vaultPayload{Schema: vaultSchema, Entries: nil})
	if err != nil {
		t.Fatalf("marshal forged payload: %v", err)
	}
	path := writeForgedVault(t, plaintext)
	backend, err := NewFallback(path, []byte(testMasterPassword))
	if err != nil {
		t.Fatalf("NewFallback() = %v", err)
	}
	t.Cleanup(backend.Close)

	if err := backend.Set("k", []byte("v")); err != nil {
		t.Fatalf("Set() = %v", err)
	}
}

func TestFallbackBackend_RotatePasswordOnLockedReturnsErr(t *testing.T) {
	t.Parallel()

	backend, err := NewFallback(newTestVaultPath(t), []byte(testMasterPassword))
	if err != nil {
		t.Fatalf("NewFallback() = %v", err)
	}
	backend.Close()

	if err := backend.RotatePassword([]byte("new-correct-horse-battery")); !errors.Is(err, ErrFallbackLocked) {
		t.Fatalf("RotatePassword(locked) = %v, want ErrFallbackLocked", err)
	}
}

func TestFallbackBackend_RotatePasswordTooShortRejected(t *testing.T) {
	t.Parallel()

	backend, err := NewFallback(newTestVaultPath(t), []byte(testMasterPassword))
	if err != nil {
		t.Fatalf("NewFallback() = %v", err)
	}
	t.Cleanup(backend.Close)

	if err := backend.RotatePassword([]byte("short")); !errors.Is(err, ErrMasterPasswordTooShort) {
		t.Fatalf("RotatePassword(short) = %v, want ErrMasterPasswordTooShort", err)
	}
}

func TestFallbackBackend_LockedGetSetDelete(t *testing.T) {
	t.Parallel()

	backend, err := NewFallback(newTestVaultPath(t), []byte(testMasterPassword))
	if err != nil {
		t.Fatalf("NewFallback() = %v", err)
	}
	backend.Close()

	if _, err := backend.Get("k"); !errors.Is(err, ErrFallbackLocked) {
		t.Fatalf("Get(closed) = %v, want ErrFallbackLocked", err)
	}
	if err := backend.Set("k", []byte("v")); !errors.Is(err, ErrFallbackLocked) {
		t.Fatalf("Set(closed) = %v, want ErrFallbackLocked", err)
	}
	if err := backend.Delete("k"); !errors.Is(err, ErrFallbackLocked) {
		t.Fatalf("Delete(closed) = %v, want ErrFallbackLocked", err)
	}
}

func TestFallbackBackend_DeleteMissingIsSilentNoOp(t *testing.T) {
	t.Parallel()

	backend, err := NewFallback(newTestVaultPath(t), []byte(testMasterPassword))
	if err != nil {
		t.Fatalf("NewFallback() = %v", err)
	}
	t.Cleanup(backend.Close)

	// Set then Delete writes; the second Delete should NOT touch disk.
	if err := backend.Set("k", []byte("v")); err != nil {
		t.Fatalf("Set() = %v", err)
	}
	before, err := os.ReadFile(backend.path)
	if err != nil {
		t.Fatalf("read before: %v", err)
	}
	if err := backend.Delete("missing"); err != nil {
		t.Fatalf("Delete(missing) = %v", err)
	}
	after, err := os.ReadFile(backend.path)
	if err != nil {
		t.Fatalf("read after: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("Delete(missing) modified the file: before=%x after=%x", before, after)
	}
}

func TestWriteVaultFile_ReadOnlyParentReturnsError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission semantics required")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permission checks")
	}
	t.Parallel()

	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("chmod 0500: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	err := writeVaultFile(filepath.Join(dir, "secrets.enc"), []byte("blob"))
	if err == nil {
		t.Fatalf("writeVaultFile(read-only dir) = nil, want error")
	}
}

func TestRotatePassword_PersistFailureRestoresOldKey(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission semantics required")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permission checks")
	}
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

	dir := filepath.Dir(path)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("chmod 0500: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	rotateErr := backend.RotatePassword([]byte("another-correct-horse"))
	if rotateErr == nil {
		t.Fatalf("RotatePassword on read-only dir = nil, want error")
	}

	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatalf("restore perms: %v", err)
	}
	if got, err := backend.Get("k"); err != nil || string(got) != "v" {
		t.Fatalf("after failed rotate Get(k) = %q, %v; want v, nil", got, err)
	}
}

func TestLockExclusive_TimeoutReturnsErrSecretsLocked(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	lockPath := filepath.Join(dir, "lock")

	release, err := lockExclusive(context.Background(), lockPath, time.Second, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("first lockExclusive() = %v", err)
	}
	t.Cleanup(func() { _ = release() })

	_, err = lockExclusive(context.Background(), lockPath, 50*time.Millisecond, 5*time.Millisecond)
	if !errors.Is(err, ErrSecretsLocked) {
		t.Fatalf("contested lockExclusive() = %v, want ErrSecretsLocked", err)
	}
}

func TestLockExclusive_CancelledContextReturnsCtxErr(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	lockPath := filepath.Join(dir, "lock")

	release, err := lockExclusive(context.Background(), lockPath, time.Second, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("first lockExclusive() = %v", err)
	}
	t.Cleanup(func() { _ = release() })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = lockExclusive(ctx, lockPath, time.Second, 10*time.Millisecond)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("lockExclusive(cancelled) = %v, want context.Canceled", err)
	}
}

// writeForgedVault writes an AES-GCM blob with the documented file
// format, using the same Argon2id derivation as the production code so
// loadFallback will authenticate it and surface only the
// post-decryption (corrupted plaintext / wrong schema) branches.
func writeForgedVault(t *testing.T, plaintext []byte) string {
	t.Helper()

	path := newTestVaultPath(t)
	salt := make([]byte, saltSize)
	for i := range salt {
		salt[i] = byte(i + 1) // deterministic, doesn't matter for plaintext branches
	}
	key := deriveKey([]byte(testMasterPassword), salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("aes.NewCipher: %v", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("cipher.NewGCM: %v", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	for i := range nonce {
		nonce[i] = byte(0x80 + i)
	}
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	blob := make([]byte, 0, 1+saltSize+nonceSize+len(ciphertext))
	blob = append(blob, versionV1)
	blob = append(blob, salt...)
	blob = append(blob, nonce...)
	blob = append(blob, ciphertext...)
	if err := os.WriteFile(path, blob, ownerOnlyPerm); err != nil {
		t.Fatalf("write forged vault: %v", err)
	}
	return path
}
