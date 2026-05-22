package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dilitS/webox/config"
)

func TestValidate_Fixtures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		fixture     string
		raw         []byte
		wantErr     bool
		wantErrIs   error
		wantContain string
	}{
		{
			name:    "valid_v1_passes_schema",
			fixture: "valid_v1.json",
			wantErr: false,
		},
		{
			name:        "missing_schema_version_rejected",
			fixture:     "invalid_missing_schema_version.json",
			wantErr:     true,
			wantErrIs:   config.ErrSchemaViolation,
			wantContain: "schema_version",
		},
		{
			name:        "missing_profile_type_rejected",
			fixture:     "invalid_missing_profile_type.json",
			wantErr:     true,
			wantErrIs:   config.ErrSchemaViolation,
			wantContain: "type",
		},
		{
			name:        "uppercase_alias_rejected",
			fixture:     "invalid_profile_alias_uppercase.json",
			wantErr:     true,
			wantErrIs:   config.ErrSchemaViolation,
			wantContain: "alias",
		},
		{
			name:        "non_uuid_project_id_rejected",
			fixture:     "invalid_project_id_not_uuid.json",
			wantErr:     true,
			wantErrIs:   config.ErrSchemaViolation,
			wantContain: "uuid",
		},
		{
			name:        "secret_shaped_string_rejected",
			raw:         secretTokenFixture(),
			wantErr:     true,
			wantErrIs:   config.ErrSecretInConfig,
			wantContain: "github_token",
		},
		{
			name:        "unknown_profile_alias_rejected",
			fixture:     "invalid_unknown_profile_alias.json",
			wantErr:     true,
			wantErrIs:   config.ErrDanglingProfileAlias,
			wantContain: "missing-profile",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			raw := tt.raw
			if raw == nil {
				raw = loadFixture(t, tt.fixture)
			}
			err := config.Validate(raw)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("Validate(%s) = nil, want error", tt.fixture)
				}
				if tt.wantErrIs != nil && !errors.Is(err, tt.wantErrIs) {
					t.Errorf("Validate(%s) error type = %T (%v), want errors.Is(_, %v)",
						tt.fixture, err, err, tt.wantErrIs)
				}
				if tt.wantContain != "" && !strings.Contains(strings.ToLower(err.Error()), tt.wantContain) {
					t.Errorf("Validate(%s) error message = %q, want substring %q",
						tt.fixture, err.Error(), tt.wantContain)
				}
				return
			}
			if err != nil {
				t.Fatalf("Validate(%s) = %v, want nil", tt.fixture, err)
			}
		})
	}
}

func TestValidate_RejectsInvalidJSON(t *testing.T) {
	t.Parallel()

	err := config.Validate([]byte("{not valid json"))
	if err == nil {
		t.Fatal("Validate(<broken json>) = nil, want error")
	}
	if !errors.Is(err, config.ErrInvalidJSON) {
		t.Errorf("Validate(<broken json>) = %v, want errors.Is(_, ErrInvalidJSON)", err)
	}
}

func TestSchemaJSON_Embedded(t *testing.T) {
	t.Parallel()

	if got := config.SchemaJSON; got == "" {
		t.Fatal("SchemaJSON is empty; embed directive likely broken")
	}
	const want = `"$schema"`
	if !strings.Contains(config.SchemaJSON, want) {
		t.Errorf("SchemaJSON missing %q marker; got first 80 chars = %q",
			want, headN(config.SchemaJSON, 80))
	}
}

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()

	path := filepath.Join("..", "testdata", "config", name)
	raw, err := os.ReadFile(path) //nolint:gosec // G304: deliberate test fixture loader, path under repo testdata/.
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	return raw
}

func headN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func secretTokenFixture() []byte {
	return []byte(`{
  "schema_version": 1,
  "profiles": [
    {
      "alias": "main",
      "type": "smallhost",
      "host": "s1.small.pl",
      "user": "testuser",
      "properties": {
        "github_token": "` + "gh" + "p_" + strings.Repeat("1", 36) + `"
      }
    }
  ],
  "projects": []
}
`)
}
