package github

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/blake2b"
	"golang.org/x/crypto/nacl/box"
)

const (
	publicKeySize = 32
	nonceSize     = 24
)

// EncryptSecretForGitHub implements libsodium's crypto_box_seal format
// expected by GitHub Actions secrets: ephemeral public key followed by a
// NaCl box encrypted with a BLAKE2b(ephemeral_pk || recipient_pk) nonce.
func EncryptSecretForGitHub(publicKeyBase64 string, plaintext []byte) (string, error) {
	recipientRaw, err := base64.StdEncoding.DecodeString(publicKeyBase64)
	if err != nil {
		return "", fmt.Errorf("github: decode actions public key: %w", err)
	}
	if len(recipientRaw) != publicKeySize {
		return "", fmt.Errorf("%w: got %d bytes, want %d", ErrInvalidPublicKey, len(recipientRaw), publicKeySize)
	}
	var recipient [publicKeySize]byte
	copy(recipient[:], recipientRaw)

	ephemeralPublic, ephemeralPrivate, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return "", fmt.Errorf("github: generate sealed-box key: %w", err)
	}
	nonce := sealedBoxNonce(ephemeralPublic, &recipient)
	sealed := box.Seal(ephemeralPublic[:], plaintext, &nonce, &recipient, ephemeralPrivate)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

func sealedBoxNonce(ephemeralPublic, recipient *[publicKeySize]byte) [nonceSize]byte {
	hash, err := blake2b.New(nonceSize, nil)
	if err != nil {
		panic(fmt.Sprintf("github: initialize blake2b nonce hash: %v", err))
	}
	_, _ = hash.Write(ephemeralPublic[:])
	_, _ = hash.Write(recipient[:])
	sum := hash.Sum(nil)
	var nonce [nonceSize]byte
	copy(nonce[:], sum)
	return nonce
}
