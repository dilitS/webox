package secrets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/awnumar/memguard"
)

// FallbackBackend stores secrets in an AES-GCM-encrypted file keyed by
// an Argon2id-derived master key. It is the headless-environment
// counterpart to the OS keyring (see docs/SECURITY.md §4.2 and
// docs/adr/0004-przechowywanie-sekretow-keyring.md).
//
// File layout (single-blob format chosen in TASK-01.7 acceptance
// criteria): [version(1B) | salt(16B) | nonce(12B) | ciphertext+tag].
// The plaintext is a tiny JSON object holding all entries; each Set,
// Delete or RotatePassword call re-encrypts the whole blob with a fresh
// random nonce, so AES-GCM's (key, nonce) uniqueness invariant holds.
//
// The zero value is intentionally inert and returns [ErrFallbackLocked]
// for every operation — instantiate via [NewFallback] after resolving
// the master password.
type FallbackBackend struct {
	path string

	// vaultMu guards every field below and serializes every operation
	// that touches disk, so concurrent Set/Get/Delete calls from
	// different goroutines never race. Cross-process exclusion uses
	// flock(2) on <path>.lock, taken per operation in saveLocked.
	vaultMu sync.Mutex
	key     *memguard.LockedBuffer
	salt    []byte
	entries map[string][]byte
}

type vaultPayload struct {
	Schema  int               `json:"schema"`
	Entries map[string][]byte `json:"entries"`
}

const vaultSchema = 1

// NewFallback opens — or initializes — the encrypted secrets store at
// path. The supplied password is used to derive the AES key via
// Argon2id; the bytes are not retained beyond derivation. If the file
// does not yet exist, a new salt is generated and the store starts
// empty (no file is written until the first mutation).
func NewFallback(path string, password []byte) (*FallbackBackend, error) {
	if len(password) < MasterPasswordMinLen {
		return nil, fmt.Errorf("%w: minimum %d characters", ErrMasterPasswordTooShort, MasterPasswordMinLen)
	}

	pwd := memguard.NewBufferFromBytes(append([]byte(nil), password...))
	defer pwd.Destroy()

	raw, err := os.ReadFile(path) //nolint:gosec // G304: caller-controlled config path, audited at secrets layer.
	switch {
	case err == nil:
		return loadFallback(path, pwd.Bytes(), raw)
	case errors.Is(err, os.ErrNotExist):
		return initFallback(path, pwd.Bytes()), nil
	default:
		return nil, fmt.Errorf("secrets: read %s: %w", path, err)
	}
}

func initFallback(path string, password []byte) *FallbackBackend {
	salt := make([]byte, saltSize)
	readRandom(salt)

	keyBytes := deriveKey(password, salt)
	key := memguard.NewBufferFromBytes(keyBytes)

	return &FallbackBackend{
		path:    path,
		key:     key,
		salt:    salt,
		entries: map[string][]byte{},
	}
}

func loadFallback(path string, password, raw []byte) (*FallbackBackend, error) {
	const minHeader = 1 + saltSize + nonceSize + gcmTagSize
	if len(raw) < minHeader {
		return nil, fmt.Errorf("%w: file is %d bytes, need at least %d", ErrCorruptedSecrets, len(raw), minHeader)
	}
	if raw[0] != versionV1 {
		return nil, fmt.Errorf("%w: unknown version byte 0x%02x", ErrCorruptedSecrets, raw[0])
	}
	salt := raw[1 : 1+saltSize]
	nonce := raw[1+saltSize : 1+saltSize+nonceSize]
	ciphertext := raw[1+saltSize+nonceSize:]

	keyBytes := deriveKey(password, salt)
	key := memguard.NewBufferFromBytes(keyBytes)

	plaintext, err := open(key.Bytes(), nonce, ciphertext)
	if err != nil {
		key.Destroy()
		return nil, err
	}

	var payload vaultPayload
	if err := json.Unmarshal(plaintext, &payload); err != nil {
		key.Destroy()
		return nil, fmt.Errorf("%w: plaintext is not valid vault JSON: %w", ErrCorruptedSecrets, err)
	}
	if payload.Schema != vaultSchema {
		key.Destroy()
		return nil, fmt.Errorf("%w: vault schema %d, want %d", ErrCorruptedSecrets, payload.Schema, vaultSchema)
	}
	if payload.Entries == nil {
		payload.Entries = map[string][]byte{}
	}

	return &FallbackBackend{
		path:    path,
		key:     key,
		salt:    append([]byte(nil), salt...),
		entries: payload.Entries,
	}, nil
}

// Close zeroes the in-memory key buffer. After Close, every operation
// returns [ErrFallbackLocked]. Calling Close twice is a no-op.
func (b *FallbackBackend) Close() {
	b.vaultMu.Lock()
	defer b.vaultMu.Unlock()
	if b.key != nil {
		b.key.Destroy()
		b.key = nil
	}
	b.entries = nil
}

// Get returns the secret bytes associated with key, or
// [ErrSecretNotFound] if no such entry exists. The returned slice is a
// defensive copy.
func (b *FallbackBackend) Get(key string) ([]byte, error) {
	b.vaultMu.Lock()
	defer b.vaultMu.Unlock()
	if b.key == nil {
		return nil, ErrFallbackLocked
	}
	value, ok := b.entries[key]
	if !ok {
		return nil, ErrSecretNotFound
	}
	return append([]byte(nil), value...), nil
}

// Set stores value under key and persists the re-encrypted vault to
// disk atomically. CSPRNG failure during nonce generation panics — see
// [readRandom] and AUDIT IMP-2.
func (b *FallbackBackend) Set(key string, value []byte) error {
	b.vaultMu.Lock()
	defer b.vaultMu.Unlock()
	if b.key == nil {
		return ErrFallbackLocked
	}
	b.entries[key] = append([]byte(nil), value...)
	return b.persistLocked()
}

// Delete removes key from the vault if present. Calling Delete on a
// missing key is a no-op (matches osKeyringBackend's idempotent
// contract). Persistence is skipped when nothing changed.
func (b *FallbackBackend) Delete(key string) error {
	b.vaultMu.Lock()
	defer b.vaultMu.Unlock()
	if b.key == nil {
		return ErrFallbackLocked
	}
	if _, ok := b.entries[key]; !ok {
		return nil
	}
	delete(b.entries, key)
	return b.persistLocked()
}

// RotatePassword re-encrypts the entire vault under a freshly derived
// key (new random salt, new random nonce). The old key buffer is
// destroyed atomically with the swap. Concurrent Get/Set/Delete calls
// observe either the pre-rotate or post-rotate state, never a torn one.
func (b *FallbackBackend) RotatePassword(newPassword []byte) error {
	if len(newPassword) < MasterPasswordMinLen {
		return fmt.Errorf("%w: minimum %d characters", ErrMasterPasswordTooShort, MasterPasswordMinLen)
	}

	b.vaultMu.Lock()
	defer b.vaultMu.Unlock()
	if b.key == nil {
		return ErrFallbackLocked
	}

	pwd := memguard.NewBufferFromBytes(append([]byte(nil), newPassword...))
	defer pwd.Destroy()

	newSalt := make([]byte, saltSize)
	readRandom(newSalt)
	newKey := memguard.NewBufferFromBytes(deriveKey(pwd.Bytes(), newSalt))

	oldKey := b.key
	oldSalt := b.salt
	b.key = newKey
	b.salt = newSalt

	if err := b.persistLocked(); err != nil {
		newKey.Destroy()
		b.key = oldKey
		b.salt = oldSalt
		return err
	}
	oldKey.Destroy()
	return nil
}

// persistLocked marshals entries, re-encrypts the blob with a fresh
// nonce, and atomically writes the result to disk under flock(2).
// Caller MUST hold b.vaultMu.
func (b *FallbackBackend) persistLocked() error {
	payload := vaultPayload{Schema: vaultSchema, Entries: b.entries}
	plaintext, err := json.Marshal(&payload)
	if err != nil {
		return fmt.Errorf("secrets: marshal vault: %w", err)
	}

	nonce, ciphertext, err := seal(b.key.Bytes(), plaintext)
	if err != nil {
		return err
	}

	blob := make([]byte, 0, 1+len(b.salt)+len(nonce)+len(ciphertext))
	blob = append(blob, versionV1)
	blob = append(blob, b.salt...)
	blob = append(blob, nonce...)
	blob = append(blob, ciphertext...)

	return writeVaultFile(b.path, blob)
}

func writeVaultFile(path string, blob []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, configDirPerm); err != nil {
		return fmt.Errorf("secrets: create parent dir %s: %w", dir, err)
	}

	unlock, err := lockExclusive(context.Background(), path+".lock", 0, 0)
	if err != nil {
		return err
	}
	defer func() { _ = unlock() }()

	tmpPath := tempVaultPath(path)
	tmp, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, ownerOnlyPerm) //nolint:gosec // G304: temp path is generated under audited config dir.
	if err != nil {
		return fmt.Errorf("secrets: open temp file %s: %w", tmpPath, err)
	}
	cleanup := func() {
		_ = os.Remove(tmpPath)
	}
	if _, err := tmp.Write(blob); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("secrets: write %s: %w", tmpPath, err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("secrets: fsync %s: %w", tmpPath, err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("secrets: close %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		cleanup()
		return fmt.Errorf("secrets: rename %s -> %s: %w", tmpPath, path, err)
	}
	return syncDir(dir)
}
