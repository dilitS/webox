package wizard

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// PendingSchemaVersion is the on-disk schema version of
// `pending_cleanups.json`. Bumped on backward-incompatible field
// changes; the wizard only accepts snapshots whose version is <=
// [PendingSchemaVersion] so a newer Webox release never crashes on
// an older snapshot. Loading a *newer* version is treated as
// [ErrSchemaMismatch] — downgrade is not supported, matching the
// `config` package contract.
const PendingSchemaVersion = 1

// pendingFilePerm is the permission bit applied to the snapshot file.
// 0600 keeps the file readable only by the operator, matching the
// `config.json` contract — the snapshot carries no secrets, but
// includes the live domain + DB metadata which is sensitive enough to
// warrant owner-only access.
const pendingFilePerm = 0o600

// pendingDirPerm matches the config-dir convention; reuse keeps the
// resume path operating in the same security envelope as `config`.
const pendingDirPerm = 0o700

// pendingTempBytes is the random suffix length for the atomic write
// temp file — `<path>.tmp.<pid>.<hex(6)>`. Generous enough to avoid
// collisions under concurrent wizards, short enough to keep `ls`
// output readable.
const pendingTempBytes = 6

// PendingCleanups is the on-disk form of a [Stack] snapshot. Fields
// MUST stay backwards-compatible across schema versions; new fields
// land in v2 and the upgrade path lives next to this type.
type PendingCleanups struct {
	SchemaVersion int           `json:"schema_version"`
	WizardID      string        `json:"wizard_id,omitempty"`
	UpdatedAt     time.Time     `json:"updated_at"`
	Steps         []CleanupStep `json:"steps"`
}

// DefaultPendingPath returns the canonical location of
// `pending_cleanups.json` next to the user's `config.json`. The
// wizard uses this when the caller does not supply an explicit
// path; tests inject their own path to keep `$XDG_CONFIG_HOME`
// untouched.
func DefaultPendingPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("wizard: resolve user config dir: %w", err)
	}
	return filepath.Join(dir, "webox", "pending_cleanups.json"), nil
}

// LoadPending reads path and returns the parsed snapshot. A missing
// file is NOT an error — the wizard treats os.ErrNotExist as "no
// pending cleanup" and returns (nil, nil). Any other read or JSON
// failure surfaces as [ErrCorruptedSnapshot].
func LoadPending(path string) (*PendingCleanups, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // G304: caller-provided path, scoped to user config dir.
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("%w: read %s: %w", ErrCorruptedSnapshot, path, err)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("%w: %s is empty", ErrCorruptedSnapshot, path)
	}

	var snap PendingCleanups
	if err := json.Unmarshal(raw, &snap); err != nil {
		return nil, fmt.Errorf("%w: parse %s: %w", ErrCorruptedSnapshot, path, err)
	}
	if snap.SchemaVersion <= 0 {
		return nil, fmt.Errorf("%w: %s missing schema_version", ErrCorruptedSnapshot, path)
	}
	if snap.SchemaVersion > PendingSchemaVersion {
		return nil, fmt.Errorf("%w: file is v%d, binary supports v%d", ErrSchemaMismatch, snap.SchemaVersion, PendingSchemaVersion)
	}
	for _, step := range snap.Steps {
		if err := validateStep(step); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrCorruptedSnapshot, err)
		}
	}
	return &snap, nil
}

// SavePending writes snap to path atomically. Mirrors the
// `config.Save` recipe: write temp file in the same directory, fsync,
// rename, fsync parent dir. No locks — concurrent writes are the
// wizard's bug, not ours; the LIFO contract assumes a single wizard
// per config dir.
func SavePending(ctx context.Context, path string, snap *PendingCleanups) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if snap == nil {
		return fmt.Errorf("%w: nil snapshot", ErrCorruptedSnapshot)
	}
	if snap.SchemaVersion == 0 {
		snap.SchemaVersion = PendingSchemaVersion
	}
	if snap.SchemaVersion > PendingSchemaVersion {
		return fmt.Errorf("%w: snapshot v%d", ErrSchemaMismatch, snap.SchemaVersion)
	}
	for _, step := range snap.Steps {
		if err := validateStep(step); err != nil {
			return err
		}
	}
	if snap.UpdatedAt.IsZero() {
		snap.UpdatedAt = time.Now().UTC()
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, pendingDirPerm); err != nil {
		return fmt.Errorf("wizard: mkdir %s: %w", dir, err)
	}

	raw, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("wizard: marshal snapshot: %w", err)
	}
	raw = append(raw, '\n')

	tmpPath, err := pendingTempPath(path)
	if err != nil {
		return err
	}
	if err := writePendingTemp(tmpPath, raw); err != nil {
		return err
	}
	defer func() {
		if tmpPath != "" {
			_ = os.Remove(tmpPath)
		}
	}()

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("wizard: rename %s -> %s: %w", tmpPath, path, err)
	}
	tmpPath = ""

	return fsyncDir(dir)
}

// RemovePending deletes the snapshot file. The wizard calls this
// after a successful execution (no rollback needed) and after a
// completed rollback (stack empty). Missing file is success — the
// LIFO contract demands idempotent cleanup.
func RemovePending(path string) error {
	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("wizard: remove %s: %w", path, err)
	}
	return nil
}

// NewFilePersister returns a [PersistFunc] that writes every snapshot
// to path. Empty step slices remove the file so the resume detector
// does not surface an empty pending.
func NewFilePersister(path, wizardID string) PersistFunc {
	return func(ctx context.Context, steps []CleanupStep) error {
		if len(steps) == 0 {
			return RemovePending(path)
		}
		snap := &PendingCleanups{
			SchemaVersion: PendingSchemaVersion,
			WizardID:      wizardID,
			UpdatedAt:     time.Now().UTC(),
			Steps:         append([]CleanupStep(nil), steps...),
		}
		return SavePending(ctx, path, snap)
	}
}

func pendingTempPath(path string) (string, error) {
	suffix := make([]byte, pendingTempBytes)
	if _, err := io.ReadFull(rand.Reader, suffix); err != nil {
		return "", fmt.Errorf("wizard: temp-path randomness: %w", err)
	}
	return fmt.Sprintf("%s.tmp.%d.%s", path, os.Getpid(), hex.EncodeToString(suffix)), nil
}

func writePendingTemp(path string, raw []byte) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, pendingFilePerm) //nolint:gosec // G304: temp path is generated inside pending dir.
	if err != nil {
		return fmt.Errorf("wizard: open temp %s: %w", path, err)
	}
	defer func() { _ = file.Close() }()
	if _, err := file.Write(raw); err != nil {
		return fmt.Errorf("wizard: write temp %s: %w", path, err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("wizard: fsync temp %s: %w", path, err)
	}
	return nil
}

func fsyncDir(dir string) error {
	handle, err := os.Open(dir) //nolint:gosec // G304: dir is the snapshot's parent, derived from caller path.
	if err != nil {
		return fmt.Errorf("wizard: open dir %s: %w", dir, err)
	}
	defer func() { _ = handle.Close() }()
	if err := handle.Sync(); err != nil {
		return fmt.Errorf("wizard: fsync dir %s: %w", dir, err)
	}
	return nil
}
