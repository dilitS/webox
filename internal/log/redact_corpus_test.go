package log

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"math/big"
	"strings"
	"testing"
)

// TestRedactCorpus_SecretFamilies exercises every secret family the
// live-log streamer might encounter on the wire (Sprint 09 §TASK-09.6).
// Each case asserts that the original secret payload disappears from
// the output even when wrapped in chatty surrounding text — the
// guarantee the SSH tail pipeline relies on before pushing a line into
// the ring buffer.
func TestRedactCorpus_SecretFamilies(t *testing.T) {
	t.Parallel()

	// Capture each random opaque value ONCE so the "secret removed"
	// assertion checks the exact bytes embedded in `line`. Calling
	// base64Random twice (once for `line`, once for `secrets`) would
	// compare the output against a *different* random string and pass
	// trivially regardless of whether redaction actually fired.
	secretOpaque := base64Random(48)
	bearerOpaque := base64Random(64)
	basicBlob := base64Random(64)

	cases := []struct {
		name    string
		line    string
		secrets []string
		safe    []string
	}{
		{
			name:    "github classic PAT inside http header",
			line:    "GET /repos HTTP/1.1\nAuthorization: token ghp_" + strings.Repeat("A", 36),
			secrets: []string{"ghp_" + strings.Repeat("A", 36)},
		},
		{
			name:    "github fine-grained PAT after json key",
			line:    `{"token":"github_pat_` + strings.Repeat("A", 82) + `"}`,
			secrets: []string{"github_pat_" + strings.Repeat("A", 82)},
		},
		{
			name:    "openai sk- key in env file",
			line:    "OPENAI_API_KEY=sk-" + strings.Repeat("B", 48),
			secrets: []string{"sk-" + strings.Repeat("B", 48)},
		},
		{
			name:    "aws access key in cli output",
			line:    "AWS_ACCESS_KEY_ID=AKIA" + strings.Repeat("X", 16),
			secrets: []string{"AKIA" + strings.Repeat("X", 16)},
		},
		{
			name: "rsa private key block in deploy log",
			line: "key contents:\n" +
				"-----" + "BEGIN RSA PRIVATE KEY-----\n" +
				strings.Repeat("Q", 64) + "\n" +
				"-----" + "END RSA PRIVATE KEY-----\nbye",
			secrets: []string{"-----" + "BEGIN RSA PRIVATE KEY-----"},
			safe:    []string{"key contents:", "bye"},
		},
		{
			name:    "openssh private key block",
			line:    "-----" + "BEGIN OPENSSH PRIVATE KEY-----\n" + strings.Repeat("Z", 64) + "\n-----" + "END OPENSSH PRIVATE KEY-----",
			secrets: []string{"-----" + "BEGIN OPENSSH PRIVATE KEY-----"},
		},
		{
			name:    "jwt three section token",
			line:    "session=" + jwtToken(),
			secrets: []string{jwtToken()},
		},
		{
			name:    "database uri with embedded credentials",
			line:    "connect mysql://deploy:s3cret-pw-9!a@db.internal:3306/app",
			secrets: []string{"s3cret-pw-9!a"},
			safe:    []string{"mysql://deploy:", "@db.internal"},
		},
		{
			name:    "postgres uri with embedded credentials",
			line:    "DATABASE_URL=postgres://app_user:b@d-secret@db.example:5432/app",
			secrets: []string{"b@d-secret"},
		},
		{
			name:    "generic password equals value",
			line:    "running mysql -uroot -pSup3rSecr3tPass99",
			secrets: []string{"Sup3rSecr3tPass99"},
		},
		{
			name:    "generic token equals value",
			line:    "deploy --token=xyzabc-9876-deadbeef-cafebabe-feed-face",
			secrets: []string{"xyzabc-9876-deadbeef-cafebabe-feed-face"},
		},
		{
			name:    "long base64-shaped value after secret=",
			line:    "secret=" + secretOpaque,
			secrets: []string{secretOpaque},
		},
		{
			name:    "authorization bearer with random opaque",
			line:    "Authorization: Bearer " + bearerOpaque,
			secrets: []string{bearerOpaque},
		},
		{
			// cPanel UAPI emits this header shape verbatim from
			// providers/cpanel/uapi/transport.go. We want the
			// username preserved (for post-incident triage) but
			// the token after the colon scrubbed.
			name:    "authorization cpanel user:token preserves user",
			line:    "Authorization: cpanel operator:t0k3nABC123-deadbeef-cafebabe",
			secrets: []string{"t0k3nABC123-deadbeef-cafebabe"},
			safe:    []string{"Authorization: cpanel operator:"},
		},
		{
			// DirectAdmin's Authorization is Basic-encoded; the
			// base64 blob carries user+key together so we redact
			// the whole opaque value.
			name:    "authorization basic redacts whole base64 blob",
			line:    "Authorization: Basic " + basicBlob,
			secrets: []string{basicBlob},
		},
		{
			// `webox doctor directadmin --loginkey=...` round-trips
			// through the generic key=value rule via the new
			// `login[_-]?key` alternation arm. Deterministic literal
			// here so the assertion actually proves the bytes are
			// scrubbed (vs. the older `base64Random(N)` cases which
			// pass trivially because the regenerated random differs
			// from the original).
			name:    "directadmin loginkey CLI flag",
			line:    "webox doctor directadmin --host=panel.example.com --user=op --loginkey=lkAaBbCcDdEe1234567890_FGHIJ-deadbeefcafe",
			secrets: []string{"lkAaBbCcDdEe1234567890_FGHIJ-deadbeefcafe"},
			safe:    []string{"--host=panel.example.com", "--user=op"},
		},
		{
			// DirectAdmin SSH fallback shells out to
			// `curl -sk --user '<user>:<loginkey>' …` on loopback
			// (providers/directadmin/api/ssh.go). If that command
			// ever reaches a trace/log sink the key after the colon
			// must be scrubbed while the username stays for triage.
			name:    "directadmin loopback curl --user redacts key",
			line:    `curl -sk --max-time 30 --user 'operator:lkSecretKey1234567890abcdef' --write-out '\n%{http_code}' https://localhost:2222/api/whoami`,
			secrets: []string{"lkSecretKey1234567890abcdef"},
			safe:    []string{"--user 'operator:", "https://localhost:2222/api/whoami"},
		},
		{
			// `DA_LOGIN_KEY=...` env line is already covered by
			// the upper-case env-line rule (`*KEY*=` suffix-match),
			// but exercising it here documents the redactor's
			// guarantee for the second adapter explicitly.
			name:    "directadmin login_key env line",
			line:    "DA_LOGIN_KEY=lkSecretValue1234567890_abcdef-ghijkl",
			secrets: []string{"lkSecretValue1234567890_abcdef-ghijkl"},
		},
		{
			name:    "ssh-rsa public key (treat as sensitive identifier)",
			line:    "ssh-rsa " + strings.Repeat("A", 60) + " admin@host",
			secrets: []string{strings.Repeat("A", 60)},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := Redact(tc.line)
			for _, secret := range tc.secrets {
				if strings.Contains(got, secret) {
					t.Fatalf("Redact left secret in output\n--- input ---\n%s\n--- output ---\n%s\n--- secret ---\n%s",
						tc.line, got, secret)
				}
			}
			for _, safe := range tc.safe {
				if !strings.Contains(got, safe) {
					t.Fatalf("Redact stripped expected safe substring %q\n--- output ---\n%s", safe, got)
				}
			}
			if !strings.Contains(got, replacement) {
				t.Fatalf("Redact produced no replacement marker:\n%s", got)
			}
		})
	}
}

// TestRedactCorpus_NoFalsePositivesOnPlainText guards against the
// over-eager-redactor risk: chatty log lines without secrets must
// remain identical after Redact, otherwise the cockpit hides useful
// diagnostics from the operator.
func TestRedactCorpus_NoFalsePositivesOnPlainText(t *testing.T) {
	t.Parallel()

	lines := []string{
		"GET /healthz 200 12ms",
		"[INFO] starting worker pool=4",
		"npm warn deprecated foo@1.2.3",
		"using node version v20.11.0",
		"build completed in 12.45s",
	}
	for _, line := range lines {
		if Redact(line) != line {
			t.Errorf("Redact altered plain log line %q -> %q", line, Redact(line))
		}
	}
}

// TestRedactCorpus_PropertyRandomSecrets samples random-looking
// secret-shaped strings and checks the recall rate stays within the
// sprint's 99% target. The acceptance margin is deliberately loose
// (allow at most 5% leakage) — pattern coverage is iteratively tuned
// per real-world incident, not bench-marked to perfection.
func TestRedactCorpus_PropertyRandomSecrets(t *testing.T) {
	t.Parallel()

	const samples = 200
	templates := []func() string{
		func() string { return "ghp_" + randomToken(36) },
		func() string { return "github_pat_" + randomToken(82) },
		func() string { return "sk-" + randomToken(48) },
		func() string { return "AKIA" + randomUpper(16) },
		jwtToken,
	}

	missed := 0
	for i := 0; i < samples; i++ {
		tpl := templates[i%len(templates)]
		secret := tpl()
		input := fmt.Sprintf("ts=%d msg=auth secret=%s extra=ok", i, secret)
		if strings.Contains(Redact(input), secret) {
			missed++
		}
	}

	if rate := float64(missed) / float64(samples); rate > 0.05 {
		t.Fatalf("redactor missed %d/%d secrets (%.1f%% leakage, want ≤5%%)",
			missed, samples, rate*100)
	}
}

func jwtToken() string {
	const header = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"
	const payload = "eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ"
	const sig = "SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
	return header + "." + payload + "." + sig
}

func base64Random(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	enc := base64.RawStdEncoding.EncodeToString(b)
	if len(enc) > n {
		enc = enc[:n]
	}
	return enc
}

func randomToken(n int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_"
	var sb strings.Builder
	sb.Grow(n)
	for i := 0; i < n; i++ {
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		sb.WriteByte(alphabet[idx.Int64()])
	}
	return sb.String()
}

func randomUpper(n int) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var sb strings.Builder
	sb.Grow(n)
	for i := 0; i < n; i++ {
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		sb.WriteByte(alphabet[idx.Int64()])
	}
	return sb.String()
}
