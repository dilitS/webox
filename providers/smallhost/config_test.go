package smallhost_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/dilitS/webox/providers"
	"github.com/dilitS/webox/providers/smallhost"
)

// validConfig is the baseline config every constructor test starts
// from. Each subtest mutates a single field so failure messages point
// at the mutation that broke the build, not the whole struct.
func validConfig() providers.ProviderConfig {
	return providers.ProviderConfig{
		Alias: "main",
		Type:  "smallhost",
		Host:  "s1.small.pl",
		Port:  22,
		User:  "biuromody",
		Properties: map[string]string{
			"restart_method": "devil",
		},
	}
}

func TestNew_DefaultsAreApplied(t *testing.T) {
	p, err := smallhost.New(validConfig())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if p.Name() != "smallhost" {
		t.Errorf("Name() = %q, want %q", p.Name(), "smallhost")
	}
}

func TestNew_RejectsWrongType(t *testing.T) {
	cfg := validConfig()
	cfg.Type = "cpanel"
	_, err := smallhost.New(cfg)
	if !errors.Is(err, providers.ErrInvalidProviderConfig) {
		t.Fatalf("err = %v, want wrap of ErrInvalidProviderConfig", err)
	}
}

func TestNew_AcceptsAllSupportedProperties(t *testing.T) {
	cfg := validConfig()
	cfg.Properties = map[string]string{
		"restart_method":               "devil",
		"ssh_pool_max":                 "5",
		"ssh_algorithms_legacy_compat": "true",
	}
	provider, err := smallhost.New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	sp, ok := provider.(*smallhost.Provider)
	if !ok {
		t.Fatalf("New returned %T, want *smallhost.Provider", provider)
	}
	props := sp.Properties()
	if props.RestartMethod != "devil" {
		t.Errorf("RestartMethod = %q, want %q", props.RestartMethod, "devil")
	}
	if props.SSHPoolMax != 5 {
		t.Errorf("SSHPoolMax = %d, want %d", props.SSHPoolMax, 5)
	}
	if !props.LegacyAlgorithmCompat {
		t.Error("LegacyAlgorithmCompat = false, want true")
	}
}

func TestNew_DefaultsWhenPropertiesEmpty(t *testing.T) {
	cfg := validConfig()
	cfg.Properties = map[string]string{}
	provider, err := smallhost.New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	sp, ok := provider.(*smallhost.Provider)
	if !ok {
		t.Fatalf("New returned %T, want *smallhost.Provider", provider)
	}
	props := sp.Properties()
	if props.RestartMethod != "devil" {
		t.Errorf("default RestartMethod = %q, want %q", props.RestartMethod, "devil")
	}
	if props.SSHPoolMax != 3 {
		t.Errorf("default SSHPoolMax = %d, want %d", props.SSHPoolMax, 3)
	}
	if props.LegacyAlgorithmCompat {
		t.Error("default LegacyAlgorithmCompat = true, want false")
	}
}

func TestNew_RejectsBadProperties(t *testing.T) {
	tests := []struct {
		name  string
		props map[string]string
		hint  string
	}{
		{
			name:  "unsupported_restart_method",
			props: map[string]string{"restart_method": "passenger"},
			hint:  "restart_method",
		},
		{
			name:  "ssh_pool_max_not_integer",
			props: map[string]string{"ssh_pool_max": "lots"},
			hint:  "ssh_pool_max",
		},
		{
			name:  "ssh_pool_max_zero",
			props: map[string]string{"ssh_pool_max": "0"},
			hint:  "out of range",
		},
		{
			name:  "ssh_pool_max_too_high",
			props: map[string]string{"ssh_pool_max": "999"},
			hint:  "out of range",
		},
		{
			name:  "legacy_compat_not_bool",
			props: map[string]string{"ssh_algorithms_legacy_compat": "maybe"},
			hint:  "ssh_algorithms_legacy_compat",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Properties = tt.props
			_, err := smallhost.New(cfg)
			if !errors.Is(err, providers.ErrInvalidProviderConfig) {
				t.Fatalf("err = %v, want wrap of ErrInvalidProviderConfig", err)
			}
			if !strings.Contains(err.Error(), tt.hint) {
				t.Errorf("error %q should mention %q", err.Error(), tt.hint)
			}
		})
	}
}

func TestNew_IgnoresUnknownProperties(t *testing.T) {
	cfg := validConfig()
	cfg.Properties = map[string]string{
		"restart_method":     "devil",
		"future_unknown_key": "any value at all",
	}
	if _, err := smallhost.New(cfg); err != nil {
		t.Fatalf("New should ignore unknown properties, got: %v", err)
	}
}

// TestRegistry_KnowsSmallhost verifies the init()-time registration
// against the public registry surface. It is the only test in the
// adapter package that talks to the global registry — the registry's
// own tests cover the registration mechanics in detail.
func TestRegistry_KnowsSmallhost(t *testing.T) {
	names := providers.Names()
	for _, n := range names {
		if n == "smallhost" {
			return
		}
	}
	t.Fatalf("Names() = %v, missing smallhost", names)
}

// TestProvider_ConfigCopy verifies that Config() returns a stable
// snapshot — mutating the result must not affect the provider's
// internal state.
func TestProvider_ConfigCopy(t *testing.T) {
	provider, err := smallhost.New(validConfig())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	sp, ok := provider.(*smallhost.Provider)
	if !ok {
		t.Fatalf("New returned %T, want *smallhost.Provider", provider)
	}
	cfg := sp.Config()
	cfg.Host = "evil.example.com"
	if sp.Config().Host == "evil.example.com" {
		t.Fatal("Config() returned a reference, expected a value copy")
	}
}
