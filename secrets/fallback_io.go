package secrets

import (
	"encoding/hex"
	"fmt"
	"os"
)

const tempSuffixBytes = 6

// tempVaultPath generates a per-process random sibling path next to the
// vault file so atomic-rename publishing never collides with another
// Webox instance (or another goroutine) writing through the same lock.
// Panics if the CSPRNG fails — see [readRandom] / AUDIT IMP-2.
func tempVaultPath(path string) string {
	suffix := make([]byte, tempSuffixBytes)
	readRandom(suffix)
	return fmt.Sprintf("%s.tmp.%d.%s", path, os.Getpid(), hex.EncodeToString(suffix))
}

// syncDir flushes parent-directory entries so the post-rename state is
// durable across power loss. Mirrors the same trick in config/save.go.
func syncDir(dir string) error {
	handle, err := os.Open(dir) //nolint:gosec // G304: dir is parent of caller-audited vault path.
	if err != nil {
		return fmt.Errorf("secrets: open parent dir %s: %w", dir, err)
	}
	defer func() { _ = handle.Close() }()
	if err := handle.Sync(); err != nil {
		return fmt.Errorf("secrets: fsync parent dir %s: %w", dir, err)
	}
	return nil
}
