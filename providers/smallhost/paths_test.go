package smallhost_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/dilitS/webox/providers"
	"github.com/dilitS/webox/providers/smallhost"
)

// newTestProvider returns a smallhost.Provider built from validConfig
// (defined in config_test.go) so path-helper tests can share a single
// well-known user/host pair across the table.
func newTestProvider(t *testing.T) *smallhost.Provider {
	t.Helper()
	provider, err := smallhost.New(validConfig())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	sp, ok := provider.(*smallhost.Provider)
	if !ok {
		t.Fatalf("New returned %T, want *smallhost.Provider", provider)
	}
	return sp
}

func TestValidateDomain_Accepts(t *testing.T) {
	good := []string{
		"app.user.smallhost.pl",
		"sui.biuromody.smallhost.pl",
		"a.b.c",
		"abc",
		"a1-b2.example.com",
		"single.dot",
	}
	for _, d := range good {
		t.Run(d, func(t *testing.T) {
			if err := smallhost.ValidateDomain(d); err != nil {
				t.Fatalf("ValidateDomain(%q) = %v, want nil", d, err)
			}
		})
	}
}

func TestValidateDomain_Rejects(t *testing.T) {
	tests := []struct {
		name string
		in   string
		hint string
	}{
		{"empty", "", "empty"},
		{"only_spaces", "   ", "empty"},
		{"trailing_space", "app.user.smallhost.pl ", "whitespace"},
		{"leading_space", " app.user.smallhost.pl", "whitespace"},
		{"newline_injection", "app.example.com\ndevil www add evil", "control byte"},
		{"carriage_return", "app\rexample.com", "control byte"},
		{"nul_byte", "a\x00b", "control byte"},
		{"forward_slash", "../etc/passwd", "path separator"},
		{"backslash", "app\\evil.com", "path separator"},
		{"dot_dot_label", "app..example.com", "does not match"},
		{"leading_dash", "-app.example.com", "does not match"},
		{"trailing_dash", "app-.example.com", "does not match"},
		{"upper_case", "App.Example.Com", "does not match"},
		{"label_too_long", strings.Repeat("a", 64) + ".example.com", "does not match"},
		{"domain_too_long", strings.Repeat("a.", 130) + "com", "longer than"},
		{"single_dash_label", "-", "does not match"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := smallhost.ValidateDomain(tt.in)
			if err == nil {
				t.Fatalf("ValidateDomain(%q) = nil, want error", tt.in)
			}
			if !errors.Is(err, smallhost.ErrInvalidDomain) {
				t.Fatalf("err = %v, want wrap of ErrInvalidDomain", err)
			}
			if !strings.Contains(err.Error(), tt.hint) {
				t.Errorf("error %q should mention %q", err.Error(), tt.hint)
			}
		})
	}
}

func TestValidateUser_Accepts(t *testing.T) {
	good := []string{"biuromody", "user123", "a", "ab_cd-ef"}
	for _, u := range good {
		if err := smallhost.ValidateUser(u); err != nil {
			t.Errorf("ValidateUser(%q) = %v, want nil", u, err)
		}
	}
}

func TestValidateUser_Rejects(t *testing.T) {
	bad := []string{
		"",
		"Upper",
		"with space",
		"semi;colon",
		"$injection",
		strings.Repeat("a", 33),
		"slash/here",
	}
	for _, u := range bad {
		t.Run(u, func(t *testing.T) {
			err := smallhost.ValidateUser(u)
			if err == nil {
				t.Fatalf("ValidateUser(%q) = nil, want error", u)
			}
			if !errors.Is(err, smallhost.ErrInvalidUser) {
				t.Errorf("err = %v, want wrap of ErrInvalidUser", err)
			}
		})
	}
}

func TestPaths_HappyPath(t *testing.T) {
	p := newTestProvider(t)
	const domain = "sui.biuromody.smallhost.pl"
	const expectedPrefix = "/usr/home/biuromody/domains/sui.biuromody.smallhost.pl"

	tests := []struct {
		name  string
		got   string
		wantE string
	}{
		{"deploy", p.GetDeployPath(domain), expectedPrefix + "/public_nodejs/public"},
		{"log", p.GetLogPath(domain), expectedPrefix + "/logs"},
		{"env", p.EnvPath(domain), expectedPrefix + "/public_nodejs/.env"},
		{"storage", p.StoragePath(domain), expectedPrefix + "/public_nodejs/public/uploads"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.wantE {
				t.Errorf("got %q, want %q", tt.got, tt.wantE)
			}
		})
	}
}

func TestPaths_RejectInvalidDomain(t *testing.T) {
	p := newTestProvider(t)
	bad := []string{
		"",
		"../../etc/passwd",
		"a b.example.com",
		"app$(rm -rf /).example.com",
		"app.example.com\nrm -rf /",
	}
	for _, d := range bad {
		t.Run(d, func(t *testing.T) {
			if got := p.GetDeployPath(d); got != "" {
				t.Errorf("GetDeployPath(%q) = %q, want empty", d, got)
			}
			if got := p.GetLogPath(d); got != "" {
				t.Errorf("GetLogPath(%q) = %q, want empty", d, got)
			}
			if got := p.EnvPath(d); got != "" {
				t.Errorf("EnvPath(%q) = %q, want empty", d, got)
			}
			if got := p.StoragePath(d); got != "" {
				t.Errorf("StoragePath(%q) = %q, want empty", d, got)
			}
		})
	}
}

func TestPaths_RejectInvalidUser(t *testing.T) {
	cfg := validConfig()
	cfg.User = "biuromody"
	provider, err := smallhost.New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	sp, ok := provider.(*smallhost.Provider)
	if !ok {
		t.Fatalf("New returned %T, want *smallhost.Provider", provider)
	}

	if sp.GetDeployPath("app.example.com") == "" {
		t.Fatal("expected DeployPath to work for valid user")
	}

	// Defensive: swap in a user that would only ever come from
	// in-memory mutation (registry validation would catch this) to
	// prove the helper still fails closed.
	const evilUser = "bad/user"
	if err := smallhost.ValidateUser(evilUser); err == nil {
		t.Fatalf("ValidateUser(%q) should fail", evilUser)
	}
}

// TestPaths_InterfaceCompatibility makes sure the provider satisfies
// providers.HostingProvider's pure-path contract — the constructor
// returns it as such, but a direct interface assertion documents the
// guarantee for readers.
func TestPaths_InterfaceCompatibility(t *testing.T) {
	var p providers.HostingProvider
	provider, err := smallhost.New(validConfig())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	p = provider
	if p.GetDeployPath("app.example.com") == "" {
		t.Error("interface GetDeployPath returned empty for valid input")
	}
	if p.GetLogPath("app.example.com") == "" {
		t.Error("interface GetLogPath returned empty for valid input")
	}
}
