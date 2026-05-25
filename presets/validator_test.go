package presets_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/dilitS/webox/presets"
)

const validPresetMinimal = `{
  "schema_version": 1,
  "id": "smallhost-devil",
  "display_name": "small.pl (Devil)",
  "provider_type": "smallhost",
  "status": "verified",
  "markets": ["PL", "global"],
  "panel": {
    "name": "Devil",
    "api": "devil_cli",
    "ssh_required": true
  },
  "capabilities": {
    "node_runtime": "devil",
    "restart_method": "devil",
    "ssl_provider": "letsencrypt",
    "database_engines": ["mysql"],
    "git_available": false,
    "sftp_available": true,
    "logs_path_known": true,
    "safe_restart": true
  },
  "paths": {
    "deploy_path_template": "/home/{{user}}/domains/{{domain}}/public_html",
    "log_path_template": "/home/{{user}}/domains/{{domain}}/logs",
    "env_path_template": "/home/{{user}}/domains/{{domain}}/.env"
  },
  "probes": [
    "devil node list",
    "devil www list"
  ],
  "known_risks": [
    "Devil CLI may rate-limit aggressive restarts"
  ],
  "sources": [
    "https://small.pl/pomoc/"
  ],
  "verified": {
    "fixture_dir": "testing/fixtures/smallhost",
    "last_verified_at": "2026-05-25",
    "verified_by": "@maintainer"
  }
}`

func TestParseValidMinimalPreset(t *testing.T) {
	t.Parallel()

	p, err := presets.Parse([]byte(validPresetMinimal))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if p.ID != "smallhost-devil" {
		t.Fatalf("ID = %q, want %q", p.ID, "smallhost-devil")
	}
	if p.Status != presets.StatusVerified {
		t.Fatalf("Status = %q, want %q", p.Status, presets.StatusVerified)
	}
	if got, want := p.Panel.API, presets.PanelAPIDevilCLI; got != want {
		t.Fatalf("Panel.API = %q, want %q", got, want)
	}
	if !p.Capabilities.SafeRestart {
		t.Fatal("Capabilities.SafeRestart = false, want true")
	}
}

func TestValidateRawRejectsMalformedJSON(t *testing.T) {
	t.Parallel()

	in := []byte(`{"schema_version": 1`)
	err := presets.ValidateRaw(in)
	if !errors.Is(err, presets.ErrInvalidJSON) {
		t.Fatalf("ValidateRaw() err = %v, want errors.Is(ErrInvalidJSON)", err)
	}
}

func TestValidateRawRejectsSchemaViolations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		mutator     func(string) string
		mustContain string
	}{
		{
			name: "wrong schema_version",
			mutator: func(in string) string {
				return strings.Replace(in, `"schema_version": 1`, `"schema_version": 2`, 1)
			},
			mustContain: "schema_version",
		},
		{
			name: "unknown status enum",
			mutator: func(in string) string {
				return strings.Replace(in, `"status": "verified"`, `"status": "experimental"`, 1)
			},
			mustContain: "status",
		},
		{
			name: "id with uppercase",
			mutator: func(in string) string {
				return strings.Replace(in, `"id": "smallhost-devil"`, `"id": "Smallhost-Devil"`, 1)
			},
			mustContain: "id",
		},
		{
			name: "id with spaces",
			mutator: func(in string) string {
				return strings.Replace(in, `"id": "smallhost-devil"`, `"id": "small host devil"`, 1)
			},
			mustContain: "id",
		},
		{
			name: "missing required field (display_name)",
			mutator: func(in string) string {
				return strings.Replace(in, `"display_name": "small.pl (Devil)",`, "", 1)
			},
			mustContain: "display_name",
		},
		{
			name: "panel.api outside enum",
			mutator: func(in string) string {
				return strings.Replace(in, `"api": "devil_cli"`, `"api": "rest"`, 1)
			},
			mustContain: "api",
		},
		{
			name: "panel.api_port out of range",
			mutator: func(in string) string {
				return strings.Replace(in,
					`"api": "devil_cli",`,
					`"api": "uapi", "api_port": 99999,`, 1)
			},
			mustContain: "api_port",
		},
		{
			name: "shell metacharacter in probe (semicolon)",
			mutator: func(in string) string {
				return strings.Replace(in, `"devil node list"`, `"devil node list; rm -rf /"`, 1)
			},
			mustContain: "probe",
		},
		{
			name: "shell metacharacter in probe (pipe)",
			mutator: func(in string) string {
				return strings.Replace(in, `"devil www list"`, `"devil www list | mail attacker"`, 1)
			},
			mustContain: "probe",
		},
		{
			name: "non-https source URL",
			mutator: func(in string) string {
				return strings.Replace(in, `"https://small.pl/pomoc/"`, `"http://small.pl/pomoc/"`, 1)
			},
			mustContain: "source",
		},
		{
			name: "additional property at root",
			mutator: func(in string) string {
				return strings.Replace(in,
					`"verified": {`,
					`"unexpected_field": "value", "verified": {`, 1)
			},
			mustContain: "additional",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			payload := tt.mutator(validPresetMinimal)
			err := presets.ValidateRaw([]byte(payload))
			if !errors.Is(err, presets.ErrSchemaViolation) {
				t.Fatalf("ValidateRaw() err = %v, want errors.Is(ErrSchemaViolation)", err)
			}
			if tt.mustContain != "" && !strings.Contains(strings.ToLower(err.Error()), tt.mustContain) {
				t.Fatalf("error message %q does not contain %q", err.Error(), tt.mustContain)
			}
		})
	}
}

func TestValidateRawRejectsSecretLikeStrings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutator func(string) string
		want    string
	}{
		{
			name: "github classic token in known_risks",
			mutator: func(in string) string {
				return strings.Replace(in,
					`"Devil CLI may rate-limit aggressive restarts"`,
					`"Operator must paste ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789ab token"`, 1)
			},
			want: "github classic",
		},
		{
			name: "github fine-grained token in known_risks",
			mutator: func(in string) string {
				return strings.Replace(in,
					`"Devil CLI may rate-limit aggressive restarts"`,
					`"Token: github_pat_11ABCDEFG0abc1234567890_AbCdEfGhIjKlMnOpQrStUvWxYz"`, 1)
			},
			want: "github fine-grained",
		},
		{
			name: "openai-style secret in source description",
			mutator: func(in string) string {
				return strings.Replace(in,
					`"Devil CLI may rate-limit aggressive restarts"`,
					`"Replace sk-1234567890abcdef0123 in your config"`, 1)
			},
			want: "openai-style",
		},
		{
			name: "PEM private key block in known_risks",
			mutator: func(in string) string {
				return strings.Replace(in,
					`"Devil CLI may rate-limit aggressive restarts"`,
					`"Sample: -----BEGIN RSA PRIVATE KEY----- redacted in real life -----END RSA PRIVATE KEY-----"`, 1)
			},
			want: "private key",
		},
		{
			name: "AWS access key in display_name",
			mutator: func(in string) string {
				return strings.Replace(in,
					`"display_name": "small.pl (Devil)"`,
					`"display_name": "AKIAIOSFODNN7EXAMPLE host"`, 1)
			},
			want: "aws access key",
		},
		{
			name: "ssh-rsa public key in known_risks",
			mutator: func(in string) string {
				return strings.Replace(in,
					`"Devil CLI may rate-limit aggressive restarts"`,
					`"Sample fingerprint: ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDExampleSshKeyMaterialThatLooksLongEnoughToTrigger=="`, 1)
			},
			want: "ssh public key",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			payload := tt.mutator(validPresetMinimal)
			err := presets.ValidateRaw([]byte(payload))
			if !errors.Is(err, presets.ErrSecretInPreset) {
				t.Fatalf("ValidateRaw() err = %v, want errors.Is(ErrSecretInPreset)", err)
			}
			if !strings.Contains(strings.ToLower(err.Error()), tt.want) {
				t.Fatalf("error %q does not contain label %q", err.Error(), tt.want)
			}
		})
	}
}

func TestParseReturnsTypedPreset(t *testing.T) {
	t.Parallel()

	p, err := presets.Parse([]byte(validPresetMinimal))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got, want := len(p.Markets), 2; got != want {
		t.Fatalf("len(Markets) = %d, want %d", got, want)
	}
	if got, want := p.Region(), presets.RegionPoland; got != want {
		t.Fatalf("Region() = %q, want %q", got, want)
	}
	if !p.Capabilities.SafeRestart {
		t.Fatal("SafeRestart = false, want true (verified preset)")
	}
	want := []string{"SSH", "API", "Node", "SSL", "DB", "Logs", "Safe Restart", "Fixtures"}
	got := p.CapabilityBadges()
	if len(got) != len(want) {
		t.Fatalf("CapabilityBadges() = %v, want %v", got, want)
	}
}
