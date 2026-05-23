package doctor

import (
	"bytes"
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dilitS/webox/secrets"
)

func TestDoctor_RunAggregatesSummaryAndExitCode(t *testing.T) {
	t.Parallel()

	fixedTime := time.Date(2026, 5, 23, 0, 32, 0, 0, time.UTC)
	doctor := New([]Check{
		checkFunc(func(context.Context) Result {
			return Result{ID: "a", Category: "system", Severity: SeverityInfo, Status: StatusOK, Message: "ok"}
		}),
		checkFunc(func(context.Context) Result {
			return Result{ID: "b", Category: "security", Severity: SeverityWarn, Status: StatusWarn, Message: "warn"}
		}),
		checkFunc(func(context.Context) Result {
			return Result{ID: "c", Category: "system", Severity: SeverityFatal, Status: StatusFail, Message: "fail"}
		}),
		checkFunc(func(context.Context) Result {
			return Result{ID: "d", Category: "system", Severity: SeverityInfo, Status: StatusSkipped, Message: "skip"}
		}),
	}, Options{
		Now:          func() time.Time { return fixedTime },
		WeboxVersion: "v0.1.0",
		Platform: Platform{
			OS:        "darwin",
			Arch:      "arm64",
			GoVersion: "go1.24.0",
		},
	})

	report := doctor.Run(context.Background())
	if report.SchemaVersion != reportSchemaVersion {
		t.Fatalf("SchemaVersion = %d, want %d", report.SchemaVersion, reportSchemaVersion)
	}
	if !report.GeneratedAt.Equal(fixedTime) {
		t.Fatalf("GeneratedAt = %s, want %s", report.GeneratedAt, fixedTime)
	}
	if report.Summary.OK != 1 || report.Summary.Warn != 1 || report.Summary.Fail != 1 || report.Summary.Skipped != 1 {
		t.Fatalf("Summary = %+v, want 1/1/1/1", report.Summary)
	}
	if got := ExitCode(report); got != 2 {
		t.Fatalf("ExitCode() = %d, want 2", got)
	}
}

func TestWriteJSON(t *testing.T) {
	t.Parallel()

	report := Report{
		SchemaVersion: 1,
		GeneratedAt:   time.Date(2026, 5, 23, 0, 32, 0, 0, time.UTC),
		WeboxVersion:  "v0.1.0",
		Platform: Platform{
			OS:        "darwin",
			Arch:      "arm64",
			GoVersion: "go1.24.0",
		},
		Checks: []Result{
			{ID: "system.go_version", Category: "system", Severity: SeverityInfo, Status: StatusOK, Message: "Go runtime available: go1.24.0"},
		},
		Summary: Summary{OK: 1},
	}

	var out bytes.Buffer
	if err := WriteJSON(&out, report); err != nil {
		t.Fatalf("WriteJSON() = %v", err)
	}
	text := out.String()
	for _, needle := range []string{`"schema_version": 1`, `"webox_version": "v0.1.0"`, `"system.go_version"`} {
		if !strings.Contains(text, needle) {
			t.Fatalf("WriteJSON output missing %q\n%s", needle, text)
		}
	}
}

func TestWriteText_NoColor(t *testing.T) {
	t.Parallel()

	report := Report{
		GeneratedAt:  time.Date(2026, 5, 23, 0, 32, 0, 0, time.UTC),
		WeboxVersion: "v0.1.0",
		Platform: Platform{
			OS:        "darwin",
			Arch:      "arm64",
			GoVersion: "go1.24.0",
		},
		Checks: []Result{
			{ID: "system.go_version", Category: "system", Severity: SeverityInfo, Status: StatusOK, Message: "Go runtime available: go1.24.0"},
			{ID: "system.ssh_agent_socket", Category: "system", Severity: SeverityInfo, Status: StatusSkipped, Message: "SSH_AUTH_SOCK is not set."},
		},
		Summary: Summary{OK: 1, Skipped: 1},
	}

	var out bytes.Buffer
	if err := WriteText(&out, report, TextOptions{Color: false}); err != nil {
		t.Fatalf("WriteText() = %v", err)
	}
	text := out.String()
	for _, needle := range []string{"webox doctor", "[OK] system.go_version", "[SKIP] system.ssh_agent_socket", "summary: 1 ok, 0 warn, 0 fail, 1 skipped"} {
		if !strings.Contains(text, needle) {
			t.Fatalf("WriteText output missing %q\n%s", needle, text)
		}
	}
	if strings.Contains(text, "\x1b[") {
		t.Fatalf("WriteText(color=false) unexpectedly emitted ANSI escapes: %q", text)
	}
}

func TestNewDefaultWithDeps(t *testing.T) {
	t.Parallel()

	t.Run("healthy os backend", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		socketDir, err := os.MkdirTemp("", "wbx-agent-*")
		if err != nil {
			t.Fatalf("MkdirTemp(socket dir) = %v", err)
		}
		t.Cleanup(func() { _ = os.RemoveAll(socketDir) })
		socketPath := filepath.Join(socketDir, "a.sock")
		listener, err := net.Listen("unix", socketPath)
		if err != nil {
			t.Fatalf("listen unix: %v", err)
		}
		t.Cleanup(func() { _ = listener.Close() })

		report := newDefaultWithDeps(testDependencies(t, root, map[string]string{
			"SSH_AUTH_SOCK": socketPath,
		}, func() (secrets.Backend, error) {
			return stubBackend{}, nil
		})).Run(context.Background())

		assertStatus(t, report, "system.config_dir_writable", StatusOK)
		assertStatus(t, report, "security.secrets_backend", StatusOK)
		assertStatus(t, report, "security.master_password_env", StatusSkipped)
		assertStatus(t, report, "security.secrets_file_perms", StatusSkipped)
		assertStatus(t, report, "system.ssh_agent_socket", StatusOK)
		if got := ExitCode(report); got != 0 {
			t.Fatalf("ExitCode() = %d, want 0", got)
		}
	})

	t.Run("fallback and workstation env warn", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		configDir := filepath.Join(root, "webox")
		if err := os.MkdirAll(configDir, configDirPerm); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		secretsPath := filepath.Join(configDir, "secrets.enc")
		if err := os.WriteFile(secretsPath, []byte("ciphertext"), 0o644); err != nil {
			t.Fatalf("write secrets.enc: %v", err)
		}

		report := newDefaultWithDeps(testDependencies(t, root, map[string]string{
			secrets.MasterPasswordEnv: "dev-only",
			"DISPLAY":                 ":0",
		}, func() (secrets.Backend, error) {
			return nil, secrets.ErrKeyringUnavailable
		})).Run(context.Background())

		assertStatus(t, report, "security.secrets_backend", StatusWarn)
		assertStatus(t, report, "security.master_password_env", StatusWarn)
		assertStatus(t, report, "security.secrets_file_perms", StatusWarn)
		assertStatus(t, report, "system.ssh_agent_socket", StatusSkipped)
		if got := ExitCode(report); got != 1 {
			t.Fatalf("ExitCode() = %d, want 1", got)
		}
	})

	t.Run("broken config dir and unexpected secrets error fail", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		report := newDefaultWithDeps(func() dependencies {
			deps := testDependencies(t, root, nil, func() (secrets.Backend, error) {
				return nil, errors.New("boom")
			})
			deps.userConfigDir = func() (string, error) {
				return "", errors.New("no home")
			}
			return deps
		}()).Run(context.Background())

		assertStatus(t, report, "system.config_dir_writable", StatusFail)
		assertStatus(t, report, "security.secrets_backend", StatusFail)
		if got := ExitCode(report); got != 2 {
			t.Fatalf("ExitCode() = %d, want 2", got)
		}
	})

	t.Run("master password env in ci is ok", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		report := newDefaultWithDeps(testDependencies(t, root, map[string]string{
			secrets.MasterPasswordEnv: "ci-secret",
			"CI":                      "true",
		}, func() (secrets.Backend, error) {
			return nil, secrets.ErrKeyringUnavailable
		})).Run(context.Background())

		assertStatus(t, report, "security.master_password_env", StatusOK)
	})
}

func testDependencies(
	t *testing.T,
	root string,
	env map[string]string,
	detect func() (secrets.Backend, error),
) dependencies {
	t.Helper()

	if env == nil {
		env = map[string]string{}
	}

	return dependencies{
		lookupEnv: func(key string) (string, bool) {
			value, ok := env[key]
			return value, ok
		},
		userConfigDir: func() (string, error) { return root, nil },
		mkdirAll:      os.MkdirAll,
		createTemp:    os.CreateTemp,
		remove:        os.Remove,
		stat:          os.Stat,
		detectSecrets: detect,
		now: func() time.Time {
			return time.Date(2026, 5, 23, 0, 32, 0, 0, time.UTC)
		},
		goos:         "darwin",
		goarch:       "arm64",
		goVersion:    "go1.24.0",
		weboxVersion: "v0.1.0",
	}
}

func assertStatus(t *testing.T, report Report, id string, want Status) {
	t.Helper()

	for _, check := range report.Checks {
		if check.ID == id {
			if check.Status != want {
				t.Fatalf("check %s status = %s, want %s (message: %s)", id, check.Status, want, check.Message)
			}
			return
		}
	}
	t.Fatalf("check %s not found in report", id)
}

type stubBackend struct{}

func (stubBackend) Get(string) ([]byte, error) { return nil, nil }
func (stubBackend) Set(string, []byte) error   { return nil }
func (stubBackend) Delete(string) error        { return nil }
