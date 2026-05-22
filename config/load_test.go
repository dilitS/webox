package config_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dilitS/webox/config"
)

func TestLoad_HappyPathGoldenFixture(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load(context.Background(), "../testdata/config/valid_v1.json")
	if err != nil {
		t.Fatalf("Load(valid_v1.json) = %v, want nil", err)
	}
	if cfg == nil {
		t.Fatal("Load returned nil *Config without error")
	}
	if got, want := cfg.SchemaVersion, config.Current; got != want {
		t.Errorf("SchemaVersion = %d, want %d", got, want)
	}
	if got, want := len(cfg.Profiles), 1; got != want {
		t.Fatalf("len(Profiles) = %d, want %d", got, want)
	}
	if got, want := cfg.Profiles[0].Alias, "main"; got != want {
		t.Errorf("Profiles[0].Alias = %q, want %q", got, want)
	}
	if got, want := len(cfg.Projects), 1; got != want {
		t.Fatalf("len(Projects) = %d, want %d", got, want)
	}
	if cfg.Projects[0].ImportedAt == nil {
		t.Error("Projects[0].ImportedAt is nil; want non-nil from fixture")
	}
}

func TestLoad_MissingFile_ReturnsDefaultsNoSideEffect(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "absent.json")

	cfg, err := config.Load(context.Background(), path)
	if err != nil {
		t.Fatalf("Load(<absent>) = %v, want nil", err)
	}
	if cfg == nil {
		t.Fatal("Load(<absent>) returned nil *Config; want defaults")
	}
	if got, want := cfg.SchemaVersion, config.Current; got != want {
		t.Errorf("default SchemaVersion = %d, want %d", got, want)
	}
	if got, want := cfg.Language, "en"; got != want {
		t.Errorf("default Language = %q, want %q", got, want)
	}
	if cfg.Profiles == nil {
		t.Error("default Profiles is nil; want allocated empty slice")
	}
	if got := len(cfg.Profiles); got != 0 {
		t.Errorf("default len(Profiles) = %d, want 0", got)
	}
	if cfg.Projects == nil {
		t.Error("default Projects is nil; want allocated empty slice")
	}
	if got := len(cfg.Projects); got != 0 {
		t.Errorf("default len(Projects) = %d, want 0", got)
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("Load created %s; want no side effect (stat err = %v)", path, statErr)
	}
}

func TestLoad_TableDriven(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		body      []byte
		wantErrIs error
	}{
		{
			name:      "corrupt_json",
			body:      []byte("{not even close to json"),
			wantErrIs: config.ErrCorruptedConfig,
		},
		{
			name:      "schema_violation_uppercase_alias",
			body:      mustReadFile(t, "../testdata/config/invalid_profile_alias_uppercase.json"),
			wantErrIs: config.ErrSchemaMismatch,
		},
		{
			name:      "schema_violation_missing_profile_type",
			body:      mustReadFile(t, "../testdata/config/invalid_missing_profile_type.json"),
			wantErrIs: config.ErrSchemaMismatch,
		},
		{
			name:      "future_schema_version",
			body:      []byte(`{"schema_version":99,"profiles":[],"projects":[]}`),
			wantErrIs: config.ErrSchemaMismatch,
		},
		{
			name:      "secret_shaped_string",
			body:      secretTokenFixture(),
			wantErrIs: config.ErrSchemaMismatch,
		},
		{
			name:      "unknown_profile_alias",
			body:      mustReadFile(t, "../testdata/config/invalid_unknown_profile_alias.json"),
			wantErrIs: config.ErrSchemaMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := writeTempBytes(t, tt.body)
			_, err := config.Load(context.Background(), path)
			if !errors.Is(err, tt.wantErrIs) {
				t.Errorf("Load(%s) = %v, want errors.Is(_, %v)", tt.name, err, tt.wantErrIs)
			}
		})
	}
}

func TestLoad_ContextCancelled_ReturnsCtxErr(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := config.Load(ctx, "../testdata/config/valid_v1.json")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Load(cancelled ctx) = %v, want errors.Is(_, context.Canceled)", err)
	}
}

func TestLoad_UnreadableFile_WrapsErrCorruptedConfig(t *testing.T) {
	t.Parallel()
	if os.Getuid() == 0 {
		t.Skip("running as root; chmod 000 cannot block reads")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":1,"profiles":[],"projects":[]}`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatalf("chmod 000: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })

	_, err := config.Load(context.Background(), path)
	if !errors.Is(err, config.ErrCorruptedConfig) {
		t.Errorf("Load(<unreadable>) = %v, want errors.Is(_, ErrCorruptedConfig)", err)
	}
}

func writeTempBytes(t *testing.T, content []byte) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	return path
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()

	raw, err := os.ReadFile(path) //nolint:gosec // G304: deliberate fixture loader, path under repo testdata/.
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return raw
}
