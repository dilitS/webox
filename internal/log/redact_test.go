package log

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const redacted = "[REDACTED]"

func TestRedact_Table(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		secrets   []string
		wantSafe  []string
		wantCount int
		fixture   string
		replacers map[string]string
	}{
		{
			name:      "classic github token in sentence",
			fixture:   "github_token.txt",
			replacers: map[string]string{"{{GITHUB_TOKEN}}": githubToken("p")},
			secrets:   []string{githubToken("p")},
			wantCount: 1,
		},
		{
			name:      "github oauth token",
			input:     "token=" + githubToken("o"),
			secrets:   []string{githubToken("o")},
			wantCount: 1,
		},
		{
			name:      "github user token",
			input:     "token=" + githubToken("u"),
			secrets:   []string{githubToken("u")},
			wantCount: 1,
		},
		{
			name:      "github server token",
			input:     "token=" + githubToken("s"),
			secrets:   []string{githubToken("s")},
			wantCount: 1,
		},
		{
			name:      "github refresh token",
			input:     "token=" + githubToken("r"),
			secrets:   []string{githubToken("r")},
			wantCount: 1,
		},
		{
			name:      "github fine grained token",
			input:     "token=" + githubFineGrainedToken(),
			secrets:   []string{githubFineGrainedToken()},
			wantCount: 1,
		},
		{
			name:      "github token split across lines",
			input:     "token=" + splitToken(githubToken("p")),
			secrets:   []string{splitToken(githubToken("p"))},
			wantCount: 1,
		},
		{
			name:      "ssh private key block",
			fixture:   "private_key.txt",
			replacers: map[string]string{"{{PRIVATE_KEY}}": privateKeyBlock()},
			secrets:   []string{privateKeyBlock()},
			wantCount: 1,
		},
		{
			name:      "aws access key placeholder",
			fixture:   "aws_key.txt",
			replacers: map[string]string{"{{AWS_KEY}}": awsKey()},
			secrets:   []string{awsKey()},
			wantCount: 1,
		},
		{
			name:      "authorization bearer",
			fixture:   "bearer.txt",
			replacers: map[string]string{"{{TOKEN}}": bearerToken()},
			secrets:   []string{bearerToken()},
			wantCount: 1,
		},
		{
			name:      "url password",
			fixture:   "url_password.txt",
			replacers: map[string]string{"{{PASSWORD}}": password()},
			secrets:   []string{password()},
			wantSafe:  []string{"https://deploy:", "@example.test/repo.git"},
			wantCount: 1,
		},
		{
			name:    "env sensitive lines",
			fixture: "env_lines.txt",
			replacers: map[string]string{
				"{{PASSWORD}}": password(),
				"{{TOKEN}}":    bearerToken(),
			},
			secrets:   []string{password(), bearerToken()},
			wantSafe:  []string{"PUBLIC_URL=https://example.test"},
			wantCount: 2,
		},
		{
			name:    "json sensitive fields",
			fixture: "json_fields.txt",
			replacers: map[string]string{
				"{{PASSWORD}}": password(),
				"{{TOKEN}}":    bearerToken(),
			},
			secrets:   []string{password(), bearerToken()},
			wantSafe:  []string{`"safe":"visible"`},
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			input := tt.input
			if tt.fixture != "" {
				input = loadRedactFixture(t, tt.fixture)
			}
			for from, to := range tt.replacers {
				input = strings.ReplaceAll(input, from, to)
			}

			got := Redact(input)
			for _, secret := range tt.secrets {
				if strings.Contains(got, secret) {
					t.Fatal("Redact output still contains original secret")
				}
			}
			for _, safe := range tt.wantSafe {
				if !strings.Contains(got, safe) {
					t.Fatalf("Redact output = %q, want safe substring %q preserved", got, safe)
				}
			}
			if count := strings.Count(got, redacted); count != tt.wantCount {
				t.Fatalf("Redact output = %q, redaction count = %d, want %d", got, count, tt.wantCount)
			}
		})
	}
}

func TestRedact_NoSecretNoChange(t *testing.T) {
	t.Parallel()

	input := "status=ok project=demo public_url=https://example.test"
	if got := Redact(input); got != input {
		t.Fatalf("Redact(no secret) = %q, want unchanged %q", got, input)
	}
}

func TestRedact_LargeInput(t *testing.T) {
	t.Parallel()

	secret := githubToken("p")
	input := strings.Repeat("safe line\n", 10_000) + secret
	got := Redact(input)
	if strings.Contains(got, secret) {
		t.Fatal("Redact(large input) leaked secret")
	}
}

func loadRedactFixture(t *testing.T, name string) string {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "redact", name)) //nolint:gosec // G304: fixed test fixture path.
	if err != nil {
		t.Fatalf("read redact fixture %s: %v", name, err)
	}
	return string(raw)
}

func githubToken(kind string) string {
	return "gh" + kind + "_" + strings.Repeat("A", 36)
}

func githubFineGrainedToken() string {
	return "github" + "_pat_" + strings.Repeat("A", 82)
}

func splitToken(token string) string {
	return token[:22] + "\n" + token[22:]
}

func privateKeyBlock() string {
	return "-----" + "BEGIN OPENSSH PRIVATE KEY-----\n" +
		strings.Repeat("A", 64) + "\n" +
		"-----" + "END OPENSSH PRIVATE KEY-----"
}

func awsKey() string {
	return "AKIA" + strings.Repeat("A", 16)
}

func bearerToken() string {
	return "bearer_" + strings.Repeat("B", 48)
}

func password() string {
	return "pw-" + strings.Repeat("C", 24)
}
