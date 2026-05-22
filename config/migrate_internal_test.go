package config

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrate_V0ToV1Golden(t *testing.T) {
	t.Parallel()

	got, err := Migrate(readConfigFixture(t, "v0.json"))
	if err != nil {
		t.Fatalf("Migrate(v0.json) = %v, want nil", err)
	}
	want := readConfigFixture(t, "v0_migrated_to_v1.json")
	if !bytes.Equal(bytes.TrimSpace(got), bytes.TrimSpace(want)) {
		t.Fatalf("Migrate(v0.json) mismatch:\nwant:\n%s\n\ngot:\n%s", want, got)
	}
	if err := Validate(got); err != nil {
		t.Fatalf("Validate(Migrate(v0.json)) = %v, want nil", err)
	}
}

func TestMigrate_Idempotent(t *testing.T) {
	t.Parallel()

	once, err := Migrate(readConfigFixture(t, "v0.json"))
	if err != nil {
		t.Fatalf("Migrate(v0.json) first pass = %v", err)
	}
	twice, err := Migrate(once)
	if err != nil {
		t.Fatalf("Migrate(Migrate(v0.json)) = %v", err)
	}
	if !bytes.Equal(once, twice) {
		t.Fatalf("Migrate is not idempotent:\nonce:\n%s\n\ntwice:\n%s", once, twice)
	}
}

func TestMigrate_CurrentVersion_ReturnsUnchanged(t *testing.T) {
	t.Parallel()

	in := readConfigFixture(t, "valid_v1.json")
	got, err := Migrate(in)
	if err != nil {
		t.Fatalf("Migrate(valid_v1.json) = %v, want nil", err)
	}
	if !bytes.Equal(in, got) {
		t.Fatalf("Migrate(valid_v1.json) changed current-version config")
	}
}

func TestMigrate_InvalidJSON(t *testing.T) {
	t.Parallel()

	_, err := Migrate([]byte("{not json"))
	if !errors.Is(err, ErrInvalidJSON) {
		t.Fatalf("Migrate(invalid JSON) = %v, want errors.Is(_, ErrInvalidJSON)", err)
	}
}

func TestMigrate_MissingMigrator(t *testing.T) {
	original := migrations[0]
	delete(migrations, 0)
	t.Cleanup(func() { migrations[0] = original })

	_, err := Migrate(readConfigFixture(t, "v0.json"))
	if !errors.Is(err, ErrMigrationFailed) {
		t.Fatalf("Migrate(v0 without registered migrator) = %v, want ErrMigrationFailed", err)
	}
	if !errors.Is(err, errNoMigrator) {
		t.Fatalf("Migrate(v0 without registered migrator) = %v, want errNoMigrator", err)
	}
}

func TestMigrate_NonForwardVersionRejected(t *testing.T) {
	original := migrations[0]
	migrations[0] = func(in []byte) ([]byte, int, error) {
		return in, 0, nil
	}
	t.Cleanup(func() { migrations[0] = original })

	_, err := Migrate(readConfigFixture(t, "v0.json"))
	if !errors.Is(err, ErrMigrationFailed) {
		t.Fatalf("Migrate(non-forward migrator) = %v, want ErrMigrationFailed", err)
	}
}

func TestMigrate_MigrationFunctionErrorWrapped(t *testing.T) {
	boom := errors.New("boom")
	original := migrations[0]
	migrations[0] = func(_ []byte) ([]byte, int, error) {
		return nil, 1, boom
	}
	t.Cleanup(func() { migrations[0] = original })

	_, err := Migrate(readConfigFixture(t, "v0.json"))
	if !errors.Is(err, ErrMigrationFailed) {
		t.Fatalf("Migrate(failing migrator) = %v, want ErrMigrationFailed", err)
	}
	if !errors.Is(err, boom) {
		t.Fatalf("Migrate(failing migrator) = %v, want wrapped boom", err)
	}
}

func TestMigrateV0ToV1_CurrentInputUnchanged(t *testing.T) {
	t.Parallel()

	in := readConfigFixture(t, "valid_v1.json")
	got, version, err := migrateV0toV1(in)
	if err != nil {
		t.Fatalf("migrateV0toV1(valid_v1.json) = %v, want nil", err)
	}
	if version != Current {
		t.Fatalf("migrateV0toV1(valid_v1.json) version = %d, want %d", version, Current)
	}
	if !bytes.Equal(in, got) {
		t.Fatal("migrateV0toV1(valid_v1.json) changed current-version input")
	}
}

func TestMigrateV0ToV1_InvalidJSON(t *testing.T) {
	t.Parallel()

	_, _, err := migrateV0toV1([]byte("{not json"))
	if !errors.Is(err, ErrInvalidJSON) {
		t.Fatalf("migrateV0toV1(invalid JSON) = %v, want ErrInvalidJSON", err)
	}
}

func TestWrapMigrationLoadError_InvalidJSONBecomesCorruptedConfig(t *testing.T) {
	t.Parallel()

	err := wrapMigrationLoadError(fmt.Errorf("%w: syntax", ErrInvalidJSON))
	if !errors.Is(err, ErrCorruptedConfig) {
		t.Fatalf("wrapMigrationLoadError(ErrInvalidJSON) = %v, want ErrCorruptedConfig", err)
	}
}

func TestMigrate_LogsVersionTransition(t *testing.T) {
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	if _, err := Migrate(readConfigFixture(t, "v0.json")); err != nil {
		t.Fatalf("Migrate(v0.json) = %v", err)
	}
	logged := buf.String()
	for _, want := range []string{`"migrationFrom":0`, `"migrationTo":1`} {
		if !strings.Contains(logged, want) {
			t.Fatalf("migration log = %s, want substring %s", logged, want)
		}
	}
}

func TestLoad_MigratesV0CreatesBackupAndSavesV1(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	original := readConfigFixture(t, "v0.json")
	if err := os.WriteFile(path, original, ownerOnlyPerm); err != nil {
		t.Fatalf("seed v0 config: %v", err)
	}

	cfg, err := Load(context.Background(), path)
	if err != nil {
		t.Fatalf("Load(v0 config) = %v, want nil", err)
	}
	if got, want := cfg.SchemaVersion, Current; got != want {
		t.Fatalf("loaded SchemaVersion = %d, want %d", got, want)
	}

	migratedOnDisk, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migrated config: %v", err)
	}
	want := readConfigFixture(t, "v0_migrated_to_v1.json")
	if !bytes.Equal(bytes.TrimSpace(migratedOnDisk), bytes.TrimSpace(want)) {
		t.Fatalf("migrated file mismatch:\nwant:\n%s\n\ngot:\n%s", want, migratedOnDisk)
	}

	backups, err := filepath.Glob(path + ".bak.v0.*")
	if err != nil {
		t.Fatalf("glob backups: %v", err)
	}
	if len(backups) != 1 {
		t.Fatalf("backup count = %d (%v), want 1", len(backups), backups)
	}
	backupRaw, err := os.ReadFile(backups[0])
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if !bytes.Equal(backupRaw, original) {
		t.Fatalf("backup content changed:\nwant:\n%s\n\ngot:\n%s", original, backupRaw)
	}
}

func readConfigFixture(t *testing.T, name string) []byte {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join("..", "testdata", "config", name)) //nolint:gosec // G304: fixed test fixture path.
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return raw
}
