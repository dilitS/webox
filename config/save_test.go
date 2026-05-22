//go:build unix

package config

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestSave_HappyPathRoundTripAndPerms(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cfg := mustFixtureConfig(t)

	if err := Save(context.Background(), path, cfg); err != nil {
		t.Fatalf("Save(valid cfg) = %v, want nil", err)
	}

	stat, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got, want := stat.Mode().Perm(), os.FileMode(0o600); got != want {
		t.Errorf("config perms = %o, want %o", got, want)
	}

	lockStat, err := os.Stat(path + ".lock")
	if err != nil {
		t.Fatalf("stat lock file: %v", err)
	}
	if got, want := lockStat.Mode().Perm(), os.FileMode(0o600); got != want {
		t.Errorf("lock perms = %o, want %o", got, want)
	}

	loaded, err := Load(context.Background(), path)
	if err != nil {
		t.Fatalf("Load(saved cfg) = %v, want nil", err)
	}
	if !reflect.DeepEqual(cfg, loaded) {
		t.Errorf("round-trip mismatch:\nwant: %#v\ngot:  %#v", cfg, loaded)
	}
}

func TestSave_ConcurrentSaves_ConsistentState(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	base := mustFixtureConfig(t)
	cfgs := []*Config{
		withRepo(t, base, "dilitS/demo-a"),
		withRepo(t, base, "dilitS/demo-b"),
		withRepo(t, base, "dilitS/demo-c"),
		withRepo(t, base, "dilitS/demo-d"),
	}

	errCh := make(chan error, len(cfgs))
	for _, cfg := range cfgs {
		cfg := cfg
		go func() {
			errCh <- Save(context.Background(), path, cfg)
		}()
	}
	for range cfgs {
		if err := <-errCh; err != nil {
			t.Fatalf("concurrent Save() returned error: %v", err)
		}
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read final config: %v", err)
	}
	if err := Validate(raw); err != nil {
		t.Fatalf("Validate(final file) = %v, want nil", err)
	}

	loaded, err := Load(context.Background(), path)
	if err != nil {
		t.Fatalf("Load(final file) = %v, want nil", err)
	}
	gotRepo := loaded.Projects[0].Repo
	if gotRepo != "dilitS/demo-a" &&
		gotRepo != "dilitS/demo-b" &&
		gotRepo != "dilitS/demo-c" &&
		gotRepo != "dilitS/demo-d" {
		t.Fatalf("final repo = %q, want one of the concurrent writes", gotRepo)
	}

	matches, err := filepath.Glob(path + ".tmp.*")
	if err != nil {
		t.Fatalf("glob temp files: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("temporary files left behind: %v", matches)
	}
}

func TestSave_InvalidConfig_NoWrite(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cfg := mustFixtureConfig(t)
	cfg.Profiles[0].Properties["github_token"] = "gh" + "p_" + strings.Repeat("1", 36)

	err := Save(context.Background(), path, cfg)
	if !errors.Is(err, ErrSchemaMismatch) {
		t.Fatalf("Save(invalid cfg) = %v, want errors.Is(_, ErrSchemaMismatch)", err)
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("Save(invalid cfg) created %s; stat err = %v", path, statErr)
	}
}

func TestSave_BeforeRenameHook_LeavesOriginalIntact(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	originalCfg := mustFixtureConfig(t)
	if err := Save(context.Background(), path, originalCfg); err != nil {
		t.Fatalf("seed Save() = %v", err)
	}
	originalRaw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read original file: %v", err)
	}

	updatedCfg := withRepo(t, originalCfg, "dilitS/demo-updated")
	boom := errors.New("boom before rename")
	err = saveWithOptions(context.Background(), path, updatedCfg, saveOptions{
		beforeRename: func(string) error { return boom },
	})
	if !errors.Is(err, boom) {
		t.Fatalf("saveWithOptions(beforeRename boom) = %v, want errors.Is(_, boom)", err)
	}

	afterRaw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file after failed save: %v", err)
	}
	if !bytes.Equal(originalRaw, afterRaw) {
		t.Errorf("config.json changed despite pre-rename failure")
	}

	matches, err := filepath.Glob(path + ".tmp.*")
	if err != nil {
		t.Fatalf("glob temp files: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("temporary files left behind after failed save: %v", matches)
	}
}

func TestSave_LockTimeout_ReturnsErrConfigLocked(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	lockPath := path + ".lock"

	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("open lock file: %v", err)
	}
	defer func() {
		_ = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
		_ = lockFile.Close()
	}()
	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatalf("seed lock: %v", err)
	}

	err = saveWithOptions(context.Background(), path, mustFixtureConfig(t), saveOptions{
		lockTimeout:    30 * time.Millisecond,
		initialBackoff: 5 * time.Millisecond,
	})
	if !errors.Is(err, ErrConfigLocked) {
		t.Fatalf("saveWithOptions(locked) = %v, want errors.Is(_, ErrConfigLocked)", err)
	}
}

func TestSave_ContextCancelled_ReturnsCtxErr(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Save(ctx, filepath.Join(t.TempDir(), "config.json"), mustFixtureConfig(t))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Save(cancelled ctx) = %v, want errors.Is(_, context.Canceled)", err)
	}
}

func TestSave_NilConfig_ReturnsErrSchemaMismatch(t *testing.T) {
	t.Parallel()

	err := Save(context.Background(), filepath.Join(t.TempDir(), "config.json"), nil)
	if !errors.Is(err, ErrSchemaMismatch) {
		t.Fatalf("Save(nil cfg) = %v, want errors.Is(_, ErrSchemaMismatch)", err)
	}
}

func TestSave_CreatesParentDir(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "nested", "config.json")
	if err := Save(context.Background(), path, mustFixtureConfig(t)); err != nil {
		t.Fatalf("Save(create parent dir) = %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("saved config missing at %s: %v", path, err)
	}
}

func TestLockExclusive_ContextCancelledWhileWaiting(t *testing.T) {
	t.Parallel()

	lockPath := filepath.Join(t.TempDir(), "config.json.lock")
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("open lock file: %v", err)
	}
	defer func() {
		_ = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
		_ = lockFile.Close()
	}()
	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatalf("seed lock: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	_, err = lockExclusive(ctx, lockPath, 200*time.Millisecond, 5*time.Millisecond)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("lockExclusive(cancelled wait) = %v, want errors.Is(_, context.Canceled)", err)
	}
}

func TestLockExclusive_UnlockReleasesLock(t *testing.T) {
	t.Parallel()

	lockPath := filepath.Join(t.TempDir(), "config.json.lock")
	unlock, err := lockExclusive(context.Background(), lockPath, 100*time.Millisecond, 5*time.Millisecond)
	if err != nil {
		t.Fatalf("first lockExclusive = %v", err)
	}
	if err := unlock(); err != nil {
		t.Fatalf("unlock() = %v", err)
	}

	unlock2, err := lockExclusive(context.Background(), lockPath, 100*time.Millisecond, 5*time.Millisecond)
	if err != nil {
		t.Fatalf("second lockExclusive after unlock = %v", err)
	}
	if err := unlock2(); err != nil {
		t.Fatalf("second unlock() = %v", err)
	}
}

func TestWriteTempFile_OpenError(t *testing.T) {
	t.Parallel()

	err := writeTempFile(t.TempDir(), []byte("not important"))
	if err == nil {
		t.Fatal("writeTempFile(dir path) = nil, want error")
	}
}

func TestSyncDirectory_OpenError(t *testing.T) {
	t.Parallel()

	err := syncDirectory(filepath.Join(t.TempDir(), "missing"))
	if err == nil {
		t.Fatal("syncDirectory(missing dir) = nil, want error")
	}
}

func TestMarshalConfig_RejectsDanglingAlias(t *testing.T) {
	t.Parallel()

	cfg := mustFixtureConfig(t)
	cfg.Projects[0].ProfileAlias = "missing-profile"

	_, err := marshalConfig(cfg)
	if !errors.Is(err, ErrSchemaMismatch) {
		t.Fatalf("marshalConfig(dangling alias) = %v, want errors.Is(_, ErrSchemaMismatch)", err)
	}
}

func mustFixtureConfig(t *testing.T) *Config {
	t.Helper()

	cfg, err := Load(context.Background(), "../testdata/config/valid_v1.json")
	if err != nil {
		t.Fatalf("Load(valid_v1.json): %v", err)
	}
	return cfg
}

func withRepo(t *testing.T, in *Config, repo string) *Config {
	t.Helper()

	cloned := cloneConfig(t, in)
	cloned.Projects[0].Repo = repo
	return cloned
}

func cloneConfig(t *testing.T, in *Config) *Config {
	t.Helper()

	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal clone source: %v", err)
	}
	var out Config
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal clone target: %v", err)
	}
	return &out
}
