package doctor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/fatih/color"
	"golang.org/x/term"

	"github.com/dilitS/webox/internal/version"
	"github.com/dilitS/webox/secrets"
)

const (
	reportSchemaVersion = 1
	configDirPerm       = 0o700
	ownerOnlyPerm       = 0o600
	exitWarn            = 1
	exitFail            = 2
)

// ExitCode maps doctor summary counts to the CLI contract from
// docs/DESIGN.md §15.3 and sprint TASK-01.8.
func ExitCode(report Report) int {
	switch {
	case report.Summary.Fail > 0:
		return exitFail
	case report.Summary.Warn > 0:
		return exitWarn
	default:
		return 0
	}
}

// Options control report metadata generation.
type Options struct {
	Now          func() time.Time
	WeboxVersion string
	Platform     Platform
}

// Doctor aggregates a deterministic set of checks into a report.
type Doctor struct {
	checks []Check
	opts   Options
}

// New builds a doctor runner from explicitly supplied checks. Tests use
// this to inject stub checks; production typically calls [NewDefault].
func New(checks []Check, opts Options) *Doctor {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.WeboxVersion == "" {
		opts.WeboxVersion = effectiveWeboxVersion()
	}
	if opts.Platform.OS == "" {
		opts.Platform.OS = runtime.GOOS
	}
	if opts.Platform.Arch == "" {
		opts.Platform.Arch = runtime.GOARCH
	}
	if opts.Platform.GoVersion == "" {
		opts.Platform.GoVersion = runtime.Version()
	}

	return &Doctor{
		checks: checks,
		opts:   opts,
	}
}

// Run executes all checks in order and returns the structured report.
func (d *Doctor) Run(ctx context.Context) Report {
	report := Report{
		SchemaVersion: reportSchemaVersion,
		GeneratedAt:   d.opts.Now().UTC(),
		WeboxVersion:  d.opts.WeboxVersion,
		Platform:      d.opts.Platform,
		Checks:        make([]Result, 0, len(d.checks)),
	}

	for _, check := range d.checks {
		result := check.Run(ctx)
		report.Checks = append(report.Checks, result)
		switch result.Status {
		case StatusOK:
			report.Summary.OK++
		case StatusWarn:
			report.Summary.Warn++
		case StatusFail:
			report.Summary.Fail++
		case StatusSkipped:
			report.Summary.Skipped++
		}
	}

	return report
}

// WriteJSON renders the report in machine-readable form.
func WriteJSON(w io.Writer, report Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

// TextOptions control the human-readable doctor output.
type TextOptions struct {
	Color bool
}

// WriteText renders the report in the compact human-readable format
// used by `webox doctor`.
func WriteText(w io.Writer, report Report, opts TextOptions) error {
	if _, err := fmt.Fprintf(
		w,
		"webox doctor\nversion: %s\nplatform: %s/%s %s\ngenerated_at: %s\n\n",
		report.WeboxVersion,
		report.Platform.OS,
		report.Platform.Arch,
		report.Platform.GoVersion,
		report.GeneratedAt.Format(time.RFC3339),
	); err != nil {
		return err
	}

	for _, check := range report.Checks {
		if err := writeTextCheck(w, check, opts.Color); err != nil {
			return err
		}
	}

	_, err := fmt.Fprintf(
		w,
		"\nsummary: %d ok, %d warn, %d fail, %d skipped\n",
		report.Summary.OK,
		report.Summary.Warn,
		report.Summary.Fail,
		report.Summary.Skipped,
	)
	return err
}

func writeTextCheck(w io.Writer, check Result, useColor bool) error {
	label, attr := statusLabel(check.Status)
	if useColor {
		if _, err := color.New(attr, color.Bold).Fprintf(w, "[%s] ", label); err != nil {
			return err
		}
	} else if _, err := fmt.Fprintf(w, "[%s] ", label); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "%s: %s", check.ID, check.Message); err != nil {
		return err
	}
	if check.Hint != "" {
		if _, err := fmt.Fprintf(w, " (hint: %s)", check.Hint); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(w)
	return err
}

func statusLabel(status Status) (string, color.Attribute) {
	switch status {
	case StatusOK:
		return "OK", color.FgGreen
	case StatusWarn:
		return "WARN", color.FgYellow
	case StatusFail:
		return "FAIL", color.FgRed
	default:
		return "SKIP", color.FgBlue
	}
}

// NewDefault constructs the MVP doctor checks from the live process
// environment and local filesystem.
func NewDefault() *Doctor {
	deps := defaultDependencies()
	return newDefaultWithDeps(deps)
}

type dependencies struct {
	lookupEnv     func(string) (string, bool)
	userConfigDir func() (string, error)
	mkdirAll      func(string, os.FileMode) error
	createTemp    func(string, string) (*os.File, error)
	remove        func(string) error
	stat          func(string) (os.FileInfo, error)
	detectSecrets func() (secrets.Backend, error)
	now           func() time.Time
	goos          string
	goarch        string
	goVersion     string
	weboxVersion  string
}

func defaultDependencies() dependencies {
	return dependencies{
		lookupEnv:     os.LookupEnv,
		userConfigDir: os.UserConfigDir,
		mkdirAll:      os.MkdirAll,
		createTemp:    os.CreateTemp,
		remove:        os.Remove,
		stat:          os.Stat,
		detectSecrets: secrets.Detect,
		now:           time.Now,
		goos:          runtime.GOOS,
		goarch:        runtime.GOARCH,
		goVersion:     runtime.Version(),
		weboxVersion:  effectiveWeboxVersion(),
	}
}

func newDefaultWithDeps(deps dependencies) *Doctor {
	platform := Platform{
		OS:        deps.goos,
		Arch:      deps.goarch,
		GoVersion: deps.goVersion,
	}

	doctor := New(defaultChecks(deps), Options{
		Now:          deps.now,
		WeboxVersion: deps.weboxVersion,
		Platform:     platform,
	})
	return doctor
}

func defaultChecks(deps dependencies) []Check {
	return []Check{
		checkFunc(func(context.Context) Result {
			return Result{
				ID:       "system.go_version",
				Category: "system",
				Severity: SeverityInfo,
				Status:   StatusOK,
				Message:  fmt.Sprintf("Go runtime available: %s", deps.goVersion),
			}
		}),
		checkFunc(func(context.Context) Result {
			configDir, err := resolveConfigDir(deps)
			if err != nil {
				return Result{
					ID:       "system.config_dir_writable",
					Category: "system",
					Severity: SeverityFatal,
					Status:   StatusFail,
					Message:  fmt.Sprintf("Could not resolve Webox config directory: %v", err),
					Hint:     "Verify HOME / XDG_CONFIG_HOME and rerun `webox doctor`.",
				}
			}
			if err := deps.mkdirAll(configDir, configDirPerm); err != nil {
				return Result{
					ID:       "system.config_dir_writable",
					Category: "system",
					Severity: SeverityFatal,
					Status:   StatusFail,
					Message:  fmt.Sprintf("Config directory %s is not writable: %v", configDir, err),
					Hint:     "Fix permissions on the parent config directory.",
				}
			}

			probe, err := deps.createTemp(configDir, "doctor-*")
			if err != nil {
				return Result{
					ID:       "system.config_dir_writable",
					Category: "system",
					Severity: SeverityFatal,
					Status:   StatusFail,
					Message:  fmt.Sprintf("Config directory %s rejected a write probe: %v", configDir, err),
					Hint:     "Ensure the current user can create files inside the Webox config dir.",
				}
			}
			probePath := probe.Name()
			_ = probe.Close()
			_ = deps.remove(probePath)

			return Result{
				ID:       "system.config_dir_writable",
				Category: "system",
				Severity: SeverityFatal,
				Status:   StatusOK,
				Message:  fmt.Sprintf("Config directory %s is writable.", configDir),
			}
		}),
		checkFunc(func(context.Context) Result {
			backend, err := deps.detectSecrets()
			switch {
			case err == nil && backend != nil:
				return Result{
					ID:       "security.secrets_backend",
					Category: "security",
					Severity: SeverityInfo,
					Status:   StatusOK,
					Message:  "Secrets backend: os (system keyring available).",
				}
			case errors.Is(err, secrets.ErrKeyringUnavailable):
				return Result{
					ID:       "security.secrets_backend",
					Category: "security",
					Severity: SeverityWarn,
					Status:   StatusWarn,
					Message:  "Secrets backend: fallback (OS keyring unavailable).",
					Hint:     "This is expected on headless Linux / Docker / WSL. Webox will use secrets.enc with a master password.",
				}
			case errors.Is(err, secrets.ErrBrokenKeyring):
				return Result{
					ID:       "security.secrets_backend",
					Category: "security",
					Severity: SeverityWarn,
					Status:   StatusWarn,
					Message:  "Secrets backend: none (OS keyring probe write/read failed).",
					Hint:     "Inspect the desktop keychain service or switch to the encrypted fallback store.",
				}
			default:
				return Result{
					ID:       "security.secrets_backend",
					Category: "security",
					Severity: SeverityFatal,
					Status:   StatusFail,
					Message:  fmt.Sprintf("Secrets backend: none (unexpected detection error: %v).", err),
					Hint:     "Investigate the local keyring integration before trusting secret storage.",
				}
			}
		}),
		checkFunc(func(context.Context) Result {
			if pwd, ok := deps.lookupEnv(secrets.MasterPasswordEnv); ok && pwd != "" {
				if secretsMasterPasswordOnWorkstation(deps.lookupEnv) {
					return Result{
						ID:       "security.master_password_env",
						Category: "security",
						Severity: SeverityWarn,
						Status:   StatusWarn,
						Message:  "WEBOX_MASTER_PASSWORD is set on a workstation-like environment.",
						Hint:     "Prefer interactive unlock; this env var is only safe on ephemeral CI runners.",
					}
				}
				return Result{
					ID:       "security.master_password_env",
					Category: "security",
					Severity: SeverityInfo,
					Status:   StatusOK,
					Message:  "WEBOX_MASTER_PASSWORD is set in a CI-like environment.",
				}
			}

			return Result{
				ID:       "security.master_password_env",
				Category: "security",
				Severity: SeverityInfo,
				Status:   StatusSkipped,
				Message:  "WEBOX_MASTER_PASSWORD is not set.",
			}
		}),
		checkFunc(func(context.Context) Result {
			configDir, err := resolveConfigDir(deps)
			if err != nil {
				return Result{
					ID:       "security.secrets_file_perms",
					Category: "security",
					Severity: SeverityWarn,
					Status:   StatusWarn,
					Message:  fmt.Sprintf("Could not resolve secrets.enc path: %v", err),
					Hint:     "Verify HOME / XDG_CONFIG_HOME before using the encrypted fallback store.",
				}
			}

			secretsPath := filepath.Join(configDir, "secrets.enc")
			info, err := deps.stat(secretsPath)
			switch {
			case errors.Is(err, os.ErrNotExist):
				return Result{
					ID:       "security.secrets_file_perms",
					Category: "security",
					Severity: SeverityInfo,
					Status:   StatusSkipped,
					Message:  fmt.Sprintf("%s is not present.", secretsPath),
				}
			case err != nil:
				return Result{
					ID:       "security.secrets_file_perms",
					Category: "security",
					Severity: SeverityWarn,
					Status:   StatusWarn,
					Message:  fmt.Sprintf("Could not stat %s: %v", secretsPath, err),
					Hint:     "Check filesystem permissions before relying on the fallback secrets store.",
				}
			case info.Mode().Perm() != ownerOnlyPerm:
				return Result{
					ID:       "security.secrets_file_perms",
					Category: "security",
					Severity: SeverityWarn,
					Status:   StatusWarn,
					Message:  fmt.Sprintf("%s permissions are %04o, expected 0600.", secretsPath, info.Mode().Perm()),
					Hint:     fmt.Sprintf("Run `chmod 600 %s`.", secretsPath),
				}
			default:
				return Result{
					ID:       "security.secrets_file_perms",
					Category: "security",
					Severity: SeverityInfo,
					Status:   StatusOK,
					Message:  fmt.Sprintf("%s permissions are 0600.", secretsPath),
				}
			}
		}),
		checkFunc(func(context.Context) Result {
			socketPath, ok := deps.lookupEnv("SSH_AUTH_SOCK")
			if !ok || socketPath == "" {
				return Result{
					ID:       "system.ssh_agent_socket",
					Category: "system",
					Severity: SeverityInfo,
					Status:   StatusSkipped,
					Message:  "SSH_AUTH_SOCK is not set.",
				}
			}

			info, err := deps.stat(socketPath)
			switch {
			case errors.Is(err, os.ErrNotExist):
				return Result{
					ID:       "system.ssh_agent_socket",
					Category: "system",
					Severity: SeverityWarn,
					Status:   StatusWarn,
					Message:  fmt.Sprintf("SSH agent socket %s does not exist.", socketPath),
					Hint:     "Start an SSH agent or unset SSH_AUTH_SOCK if you rely on direct key files instead.",
				}
			case err != nil:
				return Result{
					ID:       "system.ssh_agent_socket",
					Category: "system",
					Severity: SeverityWarn,
					Status:   StatusWarn,
					Message:  fmt.Sprintf("Could not stat SSH agent socket %s: %v", socketPath, err),
					Hint:     "Verify SSH_AUTH_SOCK or your local SSH agent setup.",
				}
			case info.Mode()&os.ModeSocket == 0:
				return Result{
					ID:       "system.ssh_agent_socket",
					Category: "system",
					Severity: SeverityWarn,
					Status:   StatusWarn,
					Message:  fmt.Sprintf("SSH_AUTH_SOCK points at %s, but it is not a socket.", socketPath),
					Hint:     "Verify that your shell exports the real SSH agent socket path.",
				}
			default:
				return Result{
					ID:       "system.ssh_agent_socket",
					Category: "system",
					Severity: SeverityInfo,
					Status:   StatusOK,
					Message:  fmt.Sprintf("SSH agent socket available at %s.", socketPath),
				}
			}
		}),
	}
}

func effectiveWeboxVersion() string {
	if version.Version != "" {
		return version.Version
	}
	return version.DefaultVersion
}

func resolveConfigDir(deps dependencies) (string, error) {
	if xdg, ok := deps.lookupEnv("XDG_CONFIG_HOME"); ok && xdg != "" {
		return filepath.Join(xdg, "webox"), nil
	}
	dir, err := deps.userConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "webox"), nil
}

func secretsMasterPasswordOnWorkstation(lookupEnv func(string) (string, bool)) bool {
	for _, ciKey := range []string{"CI", "GITHUB_ACTIONS", "GITLAB_CI"} {
		if v, ok := lookupEnv(ciKey); ok && v != "" && v != "false" && v != "0" {
			return false
		}
	}
	for _, wsKey := range []string{"SSH_CLIENT", "DISPLAY", "XDG_SESSION_TYPE"} {
		if v, ok := lookupEnv(wsKey); ok && v != "" {
			return true
		}
	}
	return false
}

// ColorEnabled reports whether human-readable output should use ANSI
// styles. JSON output always bypasses this and tests usually pass false.
func ColorEnabled(w io.Writer) bool {
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	return isTerminal(int(file.Fd()))
}

var isTerminal = term.IsTerminal
