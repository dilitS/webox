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
