package secrets

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"github.com/zalando/go-keyring"
)

const (
	keyringService = "webox"
	probeKey       = "__webox_probe__"
	probeBytes     = 16
)

type keyringClient interface {
	Get(service, user string) (string, error)
	Set(service, user, password string) error
	Delete(service, user string) error
}

type goKeyringClient struct{}

// Get delegates to go-keyring's package-level Get function.
func (c goKeyringClient) Get(service, user string) (string, error) {
	return keyring.Get(service, user)
}

// Set delegates to go-keyring's package-level Set function.
func (c goKeyringClient) Set(service, user, password string) error {
	return keyring.Set(service, user, password)
}

// Delete delegates to go-keyring's package-level Delete function.
func (c goKeyringClient) Delete(service, user string) error {
	return keyring.Delete(service, user)
}

type osKeyringBackend struct {
	client keyringClient
}

// Detect probes the OS keyring and returns the best available backend.
func Detect() (Backend, error) {
	return detectWithClient(goKeyringClient{})
}

func detectWithClient(client keyringClient) (Backend, error) {
	token, err := probeToken()
	if err != nil {
		return nil, err
	}

	if err := client.Set(keyringService, probeKey, token); err != nil {
		if errors.Is(err, keyring.ErrUnsupportedPlatform) {
			return &FallbackBackend{}, nil
		}
		return &FallbackBackend{}, nil
	}

	cleanup := func() {
		_ = client.Delete(keyringService, probeKey)
	}
	got, err := client.Get(keyringService, probeKey)
	if err != nil {
		cleanup()
		if errors.Is(err, keyring.ErrNotFound) {
			return nil, fmt.Errorf("%w: probe read after successful write returned not found", ErrBrokenKeyring)
		}
		return &FallbackBackend{}, nil
	}
	cleanup()

	if got != token {
		return nil, fmt.Errorf("%w: probe read returned different value", ErrBrokenKeyring)
	}
	return &osKeyringBackend{client: client}, nil
}

// Get retrieves a logical secret from the OS keyring.
func (b *osKeyringBackend) Get(logicalKey string) ([]byte, error) {
	value, err := b.client.Get(keyringService, logicalKey)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return nil, ErrSecretNotFound
		}
		return nil, fmt.Errorf("keyring get %s: %w", logicalKey, err)
	}
	return []byte(value), nil
}

// Set stores a logical secret in the OS keyring.
func (b *osKeyringBackend) Set(logicalKey string, value []byte) error {
	if err := b.client.Set(keyringService, logicalKey, string(value)); err != nil {
		return fmt.Errorf("keyring set %s: %w", logicalKey, err)
	}
	return nil
}

// Delete removes a logical secret from the OS keyring. Missing keys are
// treated as already deleted.
func (b *osKeyringBackend) Delete(logicalKey string) error {
	if err := b.client.Delete(keyringService, logicalKey); err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("keyring delete %s: %w", logicalKey, err)
	}
	return nil
}

func probeToken() (string, error) {
	buf := make([]byte, probeBytes)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return "", fmt.Errorf("csprng probe token: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
