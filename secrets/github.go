package secrets

import (
	"fmt"
	"strings"
)

const githubPATPrefix = "github:pat:"

// GitHubPATKey returns the logical key used by the configured secrets
// backend. The account string is metadata, not a token; blank aliases map
// to "default" so first-run flows have one stable slot.
func GitHubPATKey(account string) string {
	account = strings.TrimSpace(account)
	if account == "" {
		account = "default"
	}
	return githubPATPrefix + account
}

// GetGitHubPAT returns a PAT from the selected secrets backend. The
// returned bytes are caller-owned and must never be logged or persisted
// outside keyring / secrets.enc.
func GetGitHubPAT(backend Backend, account string) ([]byte, error) {
	if backend == nil {
		return nil, fmt.Errorf("%w: github pat backend is nil", ErrSecretNotFound)
	}
	value, err := backend.Get(GitHubPATKey(account))
	if err != nil {
		return nil, err
	}
	return value, nil
}

// SetGitHubPAT stores a GitHub PAT under a logical account alias.
func SetGitHubPAT(backend Backend, account string, token []byte) error {
	if backend == nil {
		return fmt.Errorf("%w: github pat backend is nil", ErrSecretNotFound)
	}
	return backend.Set(GitHubPATKey(account), token)
}
