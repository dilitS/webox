package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"os"

	"golang.org/x/crypto/argon2"
)

// Argon2id parameters per docs/SECURITY.md §4.2 and ADR-0004.
// Conservative envelope: ~250 ms derivation on a modern CPU, which
// hardens offline brute-force without making interactive unlock annoying.
const (
	argonMemory      uint32 = 64 * 1024
	argonIterations  uint32 = 3
	argonParallelism uint8  = 2
	argonKeyLen      uint32 = 32
)

// File format sizes — see docs/sprints/sprint-01-foundations.md
// (TASK-01.7 acceptance criteria) and ADR-0004 parameter table.
const (
	versionV1  byte = 0x01
	saltSize   int  = 16
	nonceSize  int  = 12
	gcmTagSize int  = 16
)

// ownerOnlyPerm matches config/save.go (0600). The fallback file is at
// least as sensitive as the keyring's escape hatch, so we never widen
// this without an explicit ADR.
const ownerOnlyPerm os.FileMode = 0o600

// configDirPerm is the upper bound for the parent directory of the
// vault file — owner-only traversal, never group/world readable.
const configDirPerm os.FileMode = 0o700

// MasterPasswordMinLen is the minimum length Webox accepts for the
// fallback master password. Stricter rules (entropy heuristics) may be
// layered on by higher-level UX, but this is the hard floor declared
// by ADR-0004.
const MasterPasswordMinLen = 12

// randReader is the package-level CSPRNG seam. Tests swap it to assert
// the panic-on-CSPRNG-failure invariant (AUDIT IMP-2). Production code
// MUST keep this pointed at crypto/rand.Reader.
var randReader io.Reader = rand.Reader

// readRandom fills buf with cryptographically random bytes. Per
// docs/SECURITY.md §4.2.1, any CSPRNG failure aborts the calling
// operation via panic — Webox MUST NOT retry in a loop on bad entropy.
func readRandom(buf []byte) {
	if _, err := io.ReadFull(randReader, buf); err != nil {
		panic(fmt.Errorf("secrets: CSPRNG failure: %w", err))
	}
}

// deriveKey runs Argon2id with the documented parameters. Returns a
// 32-byte AES-256 key. The output is a fresh allocation; callers wrap
// it in memguard.LockedBuffer to bound exposure.
func deriveKey(password, salt []byte) []byte {
	return argon2.IDKey(password, salt, argonIterations, argonMemory, argonParallelism, argonKeyLen)
}

// seal encrypts plaintext under key using a freshly generated 96-bit
// nonce. Returns the nonce alongside ciphertext+tag so the caller can
// frame the file header. CSPRNG failure panics — see [readRandom].
func seal(key, plaintext []byte) (nonce, ciphertext []byte, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, fmt.Errorf("secrets: aes.NewCipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("secrets: cipher.NewGCM: %w", err)
	}
	nonce = make([]byte, gcm.NonceSize())
	readRandom(nonce)
	ciphertext = gcm.Seal(nil, nonce, plaintext, nil)
	return nonce, ciphertext, nil
}

// open verifies and decrypts ciphertext under key with nonce. Returns
// [ErrAuthFailed] on any AEAD authentication failure to avoid leaking
// whether the failure was a wrong key or a tampered ciphertext.
func open(key, nonce, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("secrets: aes.NewCipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("secrets: cipher.NewGCM: %w", err)
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrAuthFailed
	}
	return plaintext, nil
}
