package secrets

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
)

// MasterPasswordEnv is the environment variable Webox honours for CI
// ephemeral runners. ANY usage outside CI is unsafe because workstation
// processes can read it via /proc/<pid>/environ, `ps eaux`, IDE
// introspection, and shell history (see docs/SECURITY.md §4.2.2 and
// AUDIT IMP-3).
const MasterPasswordEnv = "WEBOX_MASTER_PASSWORD"

// PasswordPromptOptions tweaks ReadMasterPassword's behaviour. Tests
// inject custom stdin/stderr/env hooks; production constructs the zero
// value, which reads from os.Stdin / os.Stderr / os.Getenv / os.LookupEnv.
type PasswordPromptOptions struct {
	// Prompt is rendered before reading. Defaults to a short note when empty.
	Prompt string

	// Stdin is the file descriptor read for the terminal password. When
	// nil, os.Stdin is used. The fd must be a terminal for the
	// no-echo read to engage; non-terminal stdin (pipe in CI) falls
	// back to a plain ReadAll so scripted callers still work.
	Stdin *os.File

	// Stderr receives the prompt and the workstation warning. Defaults
	// to os.Stderr.
	Stderr io.Writer

	// LookupEnv overrides env access. Used by tests to simulate CI vs
	// workstation contexts without touching the real process env.
	LookupEnv func(key string) (string, bool)
}

// ReadMasterPassword resolves the fallback master password.
//
// Priority order:
//  1. [MasterPasswordEnv] — used directly if set (CI ephemeral path).
//     When the env signals a workstation (no CI markers, presence of
//     SSH_CLIENT / DISPLAY / XDG_SESSION_TYPE), a single warning is
//     written to opts.Stderr before returning.
//  2. Interactive terminal prompt via golang.org/x/term.ReadPassword.
//
// The returned bytes are NOT wrapped in memguard — callers are
// expected to immediately pass them to [NewFallback]/[RotatePassword],
// which wrap and destroy. Callers MUST NOT retain or log the slice.
func ReadMasterPassword(opts PasswordPromptOptions) ([]byte, error) {
	lookupEnv := opts.LookupEnv
	if lookupEnv == nil {
		lookupEnv = os.LookupEnv
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}
	stdin := opts.Stdin
	if stdin == nil {
		stdin = os.Stdin
	}

	if pwd, ok := lookupEnv(MasterPasswordEnv); ok {
		if isWorkstationEnv(lookupEnv) {
			fmt.Fprintln(stderr, masterPasswordWorkstationWarning)
		}
		return []byte(pwd), nil
	}

	prompt := opts.Prompt
	if prompt == "" {
		prompt = "Webox master password: "
	}
	fmt.Fprint(stderr, prompt)

	fd := int(stdin.Fd())
	if term.IsTerminal(fd) {
		raw, err := term.ReadPassword(fd)
		fmt.Fprintln(stderr)
		if err != nil {
			return nil, fmt.Errorf("secrets: read master password from terminal: %w", err)
		}
		return raw, nil
	}

	raw, err := io.ReadAll(stdin)
	if err != nil {
		return nil, fmt.Errorf("secrets: read master password from non-terminal stdin: %w", err)
	}
	return trimTrailingNewline(raw), nil
}

// isWorkstationEnv is the CI-vs-workstation heuristic from
// docs/SECURITY.md §4.2.2: CI runners set CI / GITHUB_ACTIONS /
// GITLAB_CI; an interactive workstation typically has SSH_CLIENT,
// DISPLAY, or XDG_SESSION_TYPE. When neither marker is present we err
// on the side of *not* warning, because that's the most likely
// non-interactive batch case.
func isWorkstationEnv(lookupEnv func(string) (string, bool)) bool {
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

func trimTrailingNewline(raw []byte) []byte {
	for len(raw) > 0 {
		last := raw[len(raw)-1]
		if last != '\n' && last != '\r' {
			break
		}
		raw = raw[:len(raw)-1]
	}
	return raw
}

const masterPasswordWorkstationWarning = "warning: WEBOX_MASTER_PASSWORD is set on a workstation. " +
	"This variable is readable via /proc/<pid>/environ, `ps eaux`, IDE tools, and shell history. " +
	"Prefer the interactive prompt unless you are on an ephemeral CI runner. " +
	"See docs/SECURITY.md §4.2.2."
