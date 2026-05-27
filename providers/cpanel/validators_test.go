package cpanel_test

import (
	"errors"
	"testing"

	"github.com/dilitS/webox/providers/cpanel"
)

func TestValidateDomain(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		domain  string
		wantErr bool
	}{
		{"empty", "", true},
		{"simple_addon", "client.example.com", false},
		{"deep_subdomain", "api.staging.example.com", false},
		{"hyphenated_label", "shop-1.example.com", false},
		{"uppercase_rejected", "Shop.Example.com", true},
		{"trailing_dot_rejected", "example.com.", true},
		{"single_label_rejected", "localhost", true},
		{"underscore_rejected", "in_valid.example.com", true},
		{"space_rejected", "client .example.com", true},
		{"shell_injection_rejected", "client.example.com;rm -rf /", true},
		{"newline_rejected", "client.example.com\n", true},
		{"too_long_rejected", longString(254) + ".com", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := cpanel.ValidateDomain(tc.domain)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for %q, got nil", tc.domain)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.domain, err)
			}
			if err != nil && !errors.Is(err, cpanel.ErrInvalidDomain) {
				t.Fatalf("expected ErrInvalidDomain, got %T (%v)", err, err)
			}
		})
	}
}

func TestValidateNodeVersion(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		version string
		wantErr bool
	}{
		{"empty", "", true},
		{"major_only", "22", false},
		{"major_minor_patch", "22.4.1", false},
		{"with_label", "lts-iron", false},
		{"space_rejected", "22 4", true},
		{"shell_injection_rejected", "22;true", true},
		{"too_long_rejected", "1234567890abcdefghi", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := cpanel.ValidateNodeVersion(tc.version)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for %q, got nil", tc.version)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.version, err)
			}
			if err != nil && !errors.Is(err, cpanel.ErrInvalidNodeVersion) {
				t.Fatalf("expected ErrInvalidNodeVersion, got %T (%v)", err, err)
			}
		})
	}
}

func TestValidateDBName(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty", "", true},
		{"simple", "shop", false},
		{"with_underscore", "shop_prod", false},
		{"numeric", "shop2026", false},
		{"uppercase_rejected", "Shop", true},
		{"hyphen_rejected", "shop-prod", true},
		{"too_long_rejected", longString(33), true},
		{"shell_injection_rejected", "shop;DROP TABLE", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := cpanel.ValidateDBName(tc.input)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for %q, got nil", tc.input)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.input, err)
			}
			if err != nil && !errors.Is(err, cpanel.ErrInvalidDBName) {
				t.Fatalf("expected ErrInvalidDBName, got %T (%v)", err, err)
			}
		})
	}
}

func longString(n int) string {
	out := make([]byte, n)
	for i := range out {
		out[i] = 'a'
	}
	return string(out)
}
