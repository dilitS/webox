package wizard_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dilitS/webox/wizard"
)

func TestSaveAndLoadPendingRoundtrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "pending_cleanups.json")
	snap := &wizard.PendingCleanups{
		WizardID: "wizard-42",
		Steps: []wizard.CleanupStep{
			{Name: "remove sub", Kind: wizard.ResourceSubdomain, Params: map[string]string{"domain": "app.demo.smallhost.pl"}, CreatedAt: time.Now().UTC()},
			{Name: "remove ssl", Kind: wizard.ResourceSSL, Params: map[string]string{"domain": "app.demo.smallhost.pl"}, CreatedAt: time.Now().UTC()},
		},
	}

	if err := wizard.SavePending(context.Background(), path, snap); err != nil {
		t.Fatalf("SavePending = %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat = %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("perm = %o, want 0600", info.Mode().Perm())
	}

	loaded, err := wizard.LoadPending(path)
	if err != nil {
		t.Fatalf("LoadPending = %v", err)
	}
	if loaded == nil {
		t.Fatal("loaded snapshot is nil")
	}
	if loaded.WizardID != "wizard-42" {
		t.Fatalf("WizardID = %q", loaded.WizardID)
	}
	if loaded.SchemaVersion != wizard.PendingSchemaVersion {
		t.Fatalf("SchemaVersion = %d", loaded.SchemaVersion)
	}
	if len(loaded.Steps) != 2 || loaded.Steps[0].Name != "remove sub" {
		t.Fatalf("steps = %+v", loaded.Steps)
	}
}

func TestLoadPendingMissingReturnsNilNil(t *testing.T) {
	t.Parallel()
	loaded, err := wizard.LoadPending(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("LoadPending(missing) = %v", err)
	}
	if loaded != nil {
		t.Fatalf("loaded = %+v, want nil", loaded)
	}
}

func TestLoadPendingCorrupted(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cases := []struct {
		name    string
		content []byte
	}{
		{name: "empty", content: []byte("")},
		{name: "binary garbage", content: []byte{0x00, 0xFF, 0x7F}},
		{name: "missing schema_version", content: []byte(`{"steps":[]}`)},
		{name: "invalid step", content: []byte(`{"schema_version":1,"steps":[{"name":""}]}`)},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(dir, tc.name+".json")
			if err := os.WriteFile(path, tc.content, 0o600); err != nil {
				t.Fatalf("WriteFile = %v", err)
			}
			_, err := wizard.LoadPending(path)
			if !errors.Is(err, wizard.ErrCorruptedSnapshot) && !errors.Is(err, wizard.ErrInvalidStep) {
				t.Fatalf("err = %v, want ErrCorruptedSnapshot or ErrInvalidStep", err)
			}
		})
	}
}

func TestLoadPendingSchemaTooNew(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "future.json")
	raw, _ := json.Marshal(map[string]any{"schema_version": wizard.PendingSchemaVersion + 99, "steps": []any{}})
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("WriteFile = %v", err)
	}
	_, err := wizard.LoadPending(path)
	if !errors.Is(err, wizard.ErrSchemaMismatch) {
		t.Fatalf("err = %v, want ErrSchemaMismatch", err)
	}
}

func TestSavePendingValidatesAndRejectsSecret(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "leak.json")
	snap := &wizard.PendingCleanups{
		Steps: []wizard.CleanupStep{
			{Name: "leak", Kind: wizard.ResourceSubdomain, Params: map[string]string{"domain": "x.smallhost.pl", "leak": "passwd=hunter2"}},
		},
	}
	err := wizard.SavePending(context.Background(), path, snap)
	if !errors.Is(err, wizard.ErrSecretInCleanup) {
		t.Fatalf("err = %v, want ErrSecretInCleanup", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("file should not exist when validation fails, err = %v", err)
	}
}

func TestSavePendingNilSnapshot(t *testing.T) {
	t.Parallel()
	err := wizard.SavePending(context.Background(), filepath.Join(t.TempDir(), "x.json"), nil)
	if !errors.Is(err, wizard.ErrCorruptedSnapshot) {
		t.Fatalf("err = %v, want ErrCorruptedSnapshot", err)
	}
}

func TestSavePendingHonoursContextCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := wizard.SavePending(ctx, filepath.Join(t.TempDir(), "x.json"), &wizard.PendingCleanups{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestRemovePendingIdempotent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "del.json")
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		t.Fatalf("WriteFile = %v", err)
	}
	if err := wizard.RemovePending(path); err != nil {
		t.Fatalf("RemovePending first = %v", err)
	}
	if err := wizard.RemovePending(path); err != nil {
		t.Fatalf("RemovePending second = %v (should be idempotent)", err)
	}
}

func TestFilePersisterRemovesFileOnEmptyStack(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "p.json")
	persist := wizard.NewFilePersister(path, "wizard-1")

	step := wizard.CleanupStep{Name: "sub", Kind: wizard.ResourceSubdomain, Params: map[string]string{"domain": "x.smallhost.pl"}}
	if err := persist(context.Background(), []wizard.CleanupStep{step}); err != nil {
		t.Fatalf("persist push = %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("Stat after push = %v", err)
	}
	if err := persist(context.Background(), nil); err != nil {
		t.Fatalf("persist clear = %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("file should be removed, err = %v", err)
	}
}

func TestFilePersisterDoesNotLeakSecretsInPath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "p.json")
	persist := wizard.NewFilePersister(path, "wizard-1")

	step := wizard.CleanupStep{
		Name:   "db",
		Kind:   wizard.ResourceDatabase,
		Params: map[string]string{"dbKind": "mysql", "dbName": "app_main"},
	}
	if err := persist(context.Background(), []wizard.CleanupStep{step}); err != nil {
		t.Fatalf("persist = %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile = %v", err)
	}
	for _, needle := range []string{"password", "passwd", "ghp_", "ghs_", "PRIVATE KEY"} {
		if strings.Contains(string(raw), needle) {
			t.Fatalf("snapshot contains forbidden substring %q: %s", needle, raw)
		}
	}
}

func TestDefaultPendingPath(t *testing.T) {
	t.Parallel()
	path, err := wizard.DefaultPendingPath()
	if err != nil {
		t.Fatalf("DefaultPendingPath = %v", err)
	}
	if !strings.HasSuffix(path, "pending_cleanups.json") {
		t.Fatalf("path = %s, want suffix pending_cleanups.json", path)
	}
	if !strings.Contains(path, "webox") {
		t.Fatalf("path = %s, want substring 'webox'", path)
	}
}
