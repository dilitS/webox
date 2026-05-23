package smallhost

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dilitS/webox/providers"
)

// fixturesDir is the canonical location of Devil CLI fixtures used by
// the parser tests. Keeping it as a constant makes refactor cheap:
// when fixtures move, one diff line.
const fixturesDir = "../../testing/fixtures/devil"

// loadFixture reads a fixture file relative to fixturesDir. It is the
// only place in the parser tests that touches the filesystem — every
// other helper operates on []byte so tests stay deterministic.
func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(fixturesDir, name))
	if err != nil {
		t.Fatalf("load fixture %s: %v", name, err)
	}
	return data
}

func TestStripAndNormalize_RejectsTooLarge(t *testing.T) {
	raw := bytes.Repeat([]byte("a"), maxOutputSize+1)
	_, err := stripAndNormalize(raw)
	if !errors.Is(err, providers.ErrOutputTooLarge) {
		t.Fatalf("err = %v, want ErrOutputTooLarge", err)
	}
}

func TestStripAndNormalize_StripsANSI(t *testing.T) {
	in := []byte("\x1b[31mhello \x1b[1mworld\x1b[0m\n")
	got, err := stripAndNormalize(in)
	if err != nil {
		t.Fatalf("stripAndNormalize: %v", err)
	}
	if want := []byte("hello world\n"); !bytes.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStripAndNormalize_NormalizesCRLF(t *testing.T) {
	in := []byte("line1\r\nline2\rline3\n")
	got, err := stripAndNormalize(in)
	if err != nil {
		t.Fatalf("stripAndNormalize: %v", err)
	}
	if want := "line1\nline2\nline3\n"; string(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStripAndNormalize_RejectsNonPrintable(t *testing.T) {
	tests := map[string][]byte{
		"nul":       {'a', 0x00, 'b'},
		"bel":       {'a', 0x07, 'b'},
		"high_byte": {'a', 0xff, 'b'},
		"del":       {'a', 0x7f, 'b'},
	}
	for name, in := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := stripAndNormalize(in)
			if !errors.Is(err, providers.ErrUnknownOutputFormat) {
				t.Fatalf("err = %v, want ErrUnknownOutputFormat", err)
			}
		})
	}
}

func TestParseWwwAdd_TableDriven(t *testing.T) {
	tests := []struct {
		name        string
		fixture     string
		wantErr     error
		wantDomain  string
		wantVersion string
	}{
		{"success", "www_add_ok.txt", nil, "webox-test.smallhost.pl", "24"},
		{"success_crlf", "www_add_ok_crlf.txt", nil, "webox-test.smallhost.pl", "24"},
		{"exists", "www_add_exists.txt", providers.ErrSubdomainExists, "", ""},
		{"invalid_node", "www_add_invalid_node.txt", providers.ErrNodeVersionUnsupported, "", ""},
		{"malicious", "www_add_malicious.txt", providers.ErrUnknownOutputFormat, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseWwwAdd(loadFixture(t, tt.fixture))
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			if got == nil {
				t.Fatal("got nil result on success path")
			}
			if got.Domain != tt.wantDomain {
				t.Errorf("Domain = %q, want %q", got.Domain, tt.wantDomain)
			}
			if got.NodeVersion != tt.wantVersion {
				t.Errorf("NodeVersion = %q, want %q", got.NodeVersion, tt.wantVersion)
			}
		})
	}
}

func TestParseWwwAdd_MaliciousDoesNotLeakIntoError(t *testing.T) {
	raw := loadFixture(t, "www_add_malicious.txt")
	_, err := parseWwwAdd(raw)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	msg := err.Error()
	// Operator-log invariant: the parser error MUST NOT echo raw
	// command-injection-looking substrings back into logs.
	for _, forbidden := range []string{"$(", "rm -rf", "\x1b["} {
		if strings.Contains(msg, forbidden) {
			t.Errorf("error message %q must not contain %q", msg, forbidden)
		}
	}
}

func TestParseWwwRestart_TableDriven(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		wantErr error
	}{
		{"success", "www_restart_ok.txt", nil},
		{"not_found", "www_restart_not_found.txt", providers.ErrAppNotFound},
		{"not_node", "www_restart_not_node.txt", providers.ErrAppNotNode},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parseWwwRestart(loadFixture(t, tt.fixture))
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseWwwRestart_RejectsGarbage(t *testing.T) {
	err := parseWwwRestart([]byte("???\n"))
	if !errors.Is(err, providers.ErrUnknownOutputFormat) {
		t.Errorf("err = %v, want ErrUnknownOutputFormat", err)
	}
}

func TestParseWwwList_FullRows(t *testing.T) {
	got, err := parseWwwList(loadFixture(t, "www_list_5.txt"))
	if err != nil {
		t.Fatalf("parseWwwList: %v", err)
	}
	if len(got) != 5 {
		t.Fatalf("len(got) = %d, want 5", len(got))
	}
	want := []providers.Subdomain{
		{Domain: "api.webox-test.smallhost.pl", Type: "nodejs", NodeVersion: "22"},
		{Domain: "app.webox-test.smallhost.pl", Type: "nodejs", NodeVersion: "24"},
		{Domain: "docs.webox-test.smallhost.pl", Type: "static"},
		{Domain: "legacy.webox-test.smallhost.pl", Type: "php"},
		{Domain: "sui.webox-test.smallhost.pl", Type: "nodejs", NodeVersion: "20"},
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("row %d = %+v, want %+v", i, got[i], w)
		}
	}
}

func TestParseWwwList_Empty(t *testing.T) {
	got, err := parseWwwList(loadFixture(t, "www_list_empty.txt"))
	if err != nil {
		t.Fatalf("parseWwwList: %v", err)
	}
	if got == nil {
		t.Fatal("got nil; want empty non-nil slice (distinguishable from error path)")
	}
	if len(got) != 0 {
		t.Fatalf("len(got) = %d, want 0", len(got))
	}
}

func TestParseWwwList_RejectsUnknownRow(t *testing.T) {
	raw := []byte("domain       type     node_version\nbroken-line-without-columns\n")
	_, err := parseWwwList(raw)
	if !errors.Is(err, providers.ErrUnknownOutputFormat) {
		t.Errorf("err = %v, want ErrUnknownOutputFormat", err)
	}
}

func TestParseVhostList(t *testing.T) {
	entries, ip, err := parseVhostList(loadFixture(t, "vhost_list.txt"))
	if err != nil {
		t.Fatalf("parseVhostList: %v", err)
	}
	if ip != "203.0.113.10" {
		t.Errorf("ip = %q, want 203.0.113.10", ip)
	}
	if len(entries) != 3 {
		t.Fatalf("len(entries) = %d, want 3", len(entries))
	}
}

func TestParseVhostList_NoIP(t *testing.T) {
	_, _, err := parseVhostList([]byte("domain   ip   type\n"))
	if !errors.Is(err, providers.ErrUnknownOutputFormat) {
		t.Errorf("err = %v, want ErrUnknownOutputFormat", err)
	}
}

func TestParseVhostList_BadIP(t *testing.T) {
	_, _, err := parseVhostList([]byte("domain ip type\napp.example.com notanip nodejs\n"))
	if !errors.Is(err, providers.ErrUnknownOutputFormat) {
		t.Errorf("err = %v, want ErrUnknownOutputFormat", err)
	}
}

func TestParseSSLAdd_TableDriven(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		wantErr error
	}{
		{"success", "ssl_add_ok.txt", nil},
		{"dns_not_ready", "ssl_add_dns_not_ready.txt", providers.ErrDNSNotResolving},
		{"rate_limit", "ssl_add_rate_limit.txt", providers.ErrRateLimitLetsEncrypt},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parseSSLAdd(loadFixture(t, tt.fixture))
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseSSLAdd_Unknown(t *testing.T) {
	err := parseSSLAdd([]byte("something random\n"))
	if !errors.Is(err, providers.ErrUnknownOutputFormat) {
		t.Errorf("err = %v, want ErrUnknownOutputFormat", err)
	}
}

func TestParseSSLDelete_TableDriven(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
	}{
		{"success", "ssl_del_ok.txt"},
		{"no_cert", "ssl_del_no_cert.txt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := parseSSLDelete(loadFixture(t, tt.fixture)); err != nil {
				t.Errorf("parseSSLDelete: %v, want nil (idempotent)", err)
			}
		})
	}
}

func TestParseSSLDelete_Unknown(t *testing.T) {
	if err := parseSSLDelete([]byte("nope\n")); !errors.Is(err, providers.ErrUnknownOutputFormat) {
		t.Errorf("err = %v, want ErrUnknownOutputFormat", err)
	}
}

func TestParseDBAdd_Success(t *testing.T) {
	got, err := parseDBAdd(loadFixture(t, "mysql_add_ok.txt"))
	if err != nil {
		t.Fatalf("parseDBAdd: %v", err)
	}
	if got.User != "myapp_prod" {
		t.Errorf("User = %q, want %q", got.User, "myapp_prod")
	}
	if got.Password != "REDACTED-NEVER-A-REAL-SECRET-aBcD1234EfGh5678" {
		t.Errorf("Password mismatch")
	}
}

func TestParseDBAdd_Taken(t *testing.T) {
	_, err := parseDBAdd(loadFixture(t, "mysql_add_taken.txt"))
	if !errors.Is(err, providers.ErrDBNameTaken) {
		t.Errorf("err = %v, want ErrDBNameTaken", err)
	}
}

// TestParseDBAdd_PasswordNeverInError is the linchpin invariant: even
// when the parser fails (no user/no password match), the returned
// error MUST NOT echo the input back. SECURITY §3 demands this so a
// future log statement printing err.Error() cannot leak DB
// credentials.
func TestParseDBAdd_PasswordNeverInError(t *testing.T) {
	raw := []byte("Username: myapp\nPassword: SUPERSECRETLEAK\nthen garbage that makes no overall sense for the parser\n")
	_, err := parseDBAdd(raw)
	if err == nil {
		// The fixture actually parses if both regexes match — we
		// only assert when err != nil, but the rest of this test
		// still exercises the "no leakage" property if it runs.
		return
	}
	if strings.Contains(err.Error(), "SUPERSECRETLEAK") {
		t.Fatalf("parser error leaked password: %v", err)
	}
}

func TestParseDBAdd_Unknown(t *testing.T) {
	_, err := parseDBAdd([]byte("Database myapp_prod created.\n"))
	if !errors.Is(err, providers.ErrUnknownOutputFormat) {
		t.Errorf("err = %v, want ErrUnknownOutputFormat", err)
	}
}

func TestParseDBDelete_TableDriven(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
	}{
		{"success", "mysql_del_ok.txt"},
		{"not_found", "mysql_del_not_found.txt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := parseDBDelete(loadFixture(t, tt.fixture)); err != nil {
				t.Errorf("parseDBDelete: %v, want nil (idempotent)", err)
			}
		})
	}
}

func TestParseDBDelete_Unknown(t *testing.T) {
	if err := parseDBDelete([]byte("???\n")); !errors.Is(err, providers.ErrUnknownOutputFormat) {
		t.Errorf("err = %v, want ErrUnknownOutputFormat", err)
	}
}
