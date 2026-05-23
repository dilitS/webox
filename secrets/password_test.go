package secrets

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func envLookup(m map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) {
		v, ok := m[k]
		return v, ok
	}
}

func TestReadMasterPassword_EnvVarPathReturnsValue(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	pwd, err := ReadMasterPassword(PasswordPromptOptions{
		Stderr: &stderr,
		LookupEnv: envLookup(map[string]string{
			MasterPasswordEnv: "ci-password",
			"CI":              "true",
		}),
	})
	if err != nil {
		t.Fatalf("ReadMasterPassword() = %v", err)
	}
	if string(pwd) != "ci-password" {
		t.Fatalf("password = %q, want ci-password", pwd)
	}
	if stderr.Len() != 0 {
		t.Fatalf("CI mode unexpectedly emitted stderr: %q", stderr.String())
	}
}

func TestReadMasterPassword_WorkstationWarningEmitted(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	pwd, err := ReadMasterPassword(PasswordPromptOptions{
		Stderr: &stderr,
		LookupEnv: envLookup(map[string]string{
			MasterPasswordEnv:  "workstation-leak",
			"XDG_SESSION_TYPE": "wayland",
		}),
	})
	if err != nil {
		t.Fatalf("ReadMasterPassword() = %v", err)
	}
	if string(pwd) != "workstation-leak" {
		t.Fatalf("password = %q", pwd)
	}
	if !strings.Contains(stderr.String(), "WEBOX_MASTER_PASSWORD is set on a workstation") {
		t.Fatalf("missing workstation warning, got %q", stderr.String())
	}
}

func TestReadMasterPassword_NonTerminalStdinReturnsRead(t *testing.T) {
	t.Parallel()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer r.Close()

	go func() {
		_, _ = w.WriteString("piped-password\n")
		_ = w.Close()
	}()

	var stderr bytes.Buffer
	pwd, err := ReadMasterPassword(PasswordPromptOptions{
		Stdin:     r,
		Stderr:    &stderr,
		LookupEnv: envLookup(nil),
	})
	if err != nil {
		t.Fatalf("ReadMasterPassword() = %v", err)
	}
	if string(pwd) != "piped-password" {
		t.Fatalf("password = %q, want piped-password", pwd)
	}
	if !strings.Contains(stderr.String(), "master password") {
		t.Fatalf("expected prompt on stderr, got %q", stderr.String())
	}
}

func TestReadMasterPassword_DefaultsWiredWhenOptsZero(t *testing.T) {
	t.Setenv(MasterPasswordEnv, "default-flow")

	pwd, err := ReadMasterPassword(PasswordPromptOptions{})
	if err != nil {
		t.Fatalf("ReadMasterPassword() with zero options = %v", err)
	}
	if string(pwd) != "default-flow" {
		t.Fatalf("password = %q, want default-flow", pwd)
	}
}

func TestIsWorkstationEnv(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{name: "empty env", env: nil, want: false},
		{name: "ci truthy", env: map[string]string{"CI": "true"}, want: false},
		{name: "github actions", env: map[string]string{"GITHUB_ACTIONS": "true"}, want: false},
		{name: "ci falsy still treated as non-CI", env: map[string]string{"CI": "false"}, want: false},
		{name: "workstation ssh client", env: map[string]string{"SSH_CLIENT": "1.2.3.4 22 22"}, want: true},
		{name: "workstation display", env: map[string]string{"DISPLAY": ":0"}, want: true},
		{name: "workstation xdg session", env: map[string]string{"XDG_SESSION_TYPE": "x11"}, want: true},
		{name: "ci takes precedence over workstation", env: map[string]string{"CI": "true", "DISPLAY": ":0"}, want: false},
		{name: "empty workstation values ignored", env: map[string]string{"DISPLAY": ""}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := isWorkstationEnv(envLookup(tt.env))
			if got != tt.want {
				t.Fatalf("isWorkstationEnv(%v) = %v, want %v", tt.env, got, tt.want)
			}
		})
	}
}

func TestTrimTrailingNewline(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in, want string
	}{
		{"abc", "abc"},
		{"abc\n", "abc"},
		{"abc\r\n", "abc"},
		{"\n\n", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := string(trimTrailingNewline([]byte(tt.in)))
		if got != tt.want {
			t.Errorf("trimTrailingNewline(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
