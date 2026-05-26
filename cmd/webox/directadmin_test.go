package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	daapi "github.com/dilitS/webox/providers/directadmin/api"
)

func TestValidateDirectadminOpts_RejectsMissingFlags(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   directadminOpts
		want error
	}{
		{"missing-host", directadminOpts{user: "u"}, errDAHostRequired},
		{"missing-user", directadminOpts{host: "h"}, errDAUserRequired},
		{"no-api-and-no-ssh", directadminOpts{host: "h", user: "u", noAPI: true, noSSH: true}, errDANoTransport},
		{"no-ssh-no-key", directadminOpts{host: "h", user: "u", noSSH: true}, errDALoginKeyRequiredNoSSH},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validateDirectadminOpts(&tc.in)
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestValidateDirectadminOpts_AppliesDefaults(t *testing.T) {
	t.Parallel()
	o := directadminOpts{host: "h", user: "u"}
	if err := validateDirectadminOpts(&o); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if o.apiPort != defaultDirectadminAPIPort {
		t.Errorf("api_port = %d, want %d", o.apiPort, defaultDirectadminAPIPort)
	}
	if o.sshPort != defaultDirectadminSSHPort {
		t.Errorf("ssh_port = %d, want %d", o.sshPort, defaultDirectadminSSHPort)
	}
	if o.timeout != defaultDirectadminTimeout {
		t.Errorf("timeout = %v, want %v", o.timeout, defaultDirectadminTimeout)
	}
}

func TestRollupDirectadminVerdict_TableDriven(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   []directadminSectionResult
		want directadminVerdict
	}{
		{"all-ok", sections("OK", "OK", "OK"), directadminOK},
		{"all-blocked", sections("FAILED", "FAILED", "AUTH_FAILED"), directadminBlocked},
		{"mixed", sections("OK", "FAILED", "OK"), directadminDegraded},
		{"disabled-counts-as-ok", sections("OK", "DISABLED", "OK"), directadminOK},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := rollupDirectadminVerdict(tc.in)
			if got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func sections(statuses ...string) []directadminSectionResult {
	out := make([]directadminSectionResult, len(statuses))
	for i, s := range statuses {
		out[i] = directadminSectionResult{Status: s}
	}
	return out
}

// fakeDARunner is a minimal SSHRunner for tests that exercise the
// SSH fallback wiring via runDoctorDirectadmin.
type fakeDARunner struct {
	stdout []byte
	stderr []byte
	exit   int
	err    error
}

func (f *fakeDARunner) Run(_ context.Context, _ string) (stdout, stderr []byte, exitCode int, err error) {
	return f.stdout, f.stderr, f.exit, f.err
}

func TestRunDoctorDirectadmin_HappyPath_EmitsTextReport(t *testing.T) {
	t.Parallel()
	// Stand up a TLS server that serves the canonical DA Live
	// API shape for the five read-only endpoints. We dispatch
	// on the URL path so we don't need per-test handlers.
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/api/whoami"):
			_, _ = w.Write([]byte(`{"username":"alice","user_type":"user"}`))
		case strings.HasSuffix(r.URL.Path, "/domains"):
			_, _ = w.Write([]byte(`{"domains":[{"domain":"a.example.com"}]}`))
		case strings.HasSuffix(r.URL.Path, "/subdomains"):
			_, _ = w.Write([]byte(`{"subdomains":[{"subdomain":"s.a.example.com","parent_domain":"a.example.com"}]}`))
		case strings.HasSuffix(r.URL.Path, "/databases"):
			_, _ = w.Write([]byte(`{"databases":[{"name":"alice_db"}]}`))
		case strings.HasSuffix(r.URL.Path, "/ssl/certificates"):
			_, _ = w.Write([]byte(`{"certificates":[{"domain":"a.example.com","letsencrypt":true}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	host, port := splitHostPort(t, srv.URL)

	opts := directadminOpts{
		host:           host,
		user:           "alice",
		loginKey:       "LOGINKEY",
		apiPort:        port,
		noSSH:          true, // SSH fallback would need a real network endpoint
		httpsTransport: srv.Client().Transport,
	}

	var stdout, stderr bytes.Buffer
	exit := runDoctorDirectadmin(opts, &stdout, &stderr)
	if exit != exitOK {
		t.Fatalf("exit = %d, stderr=%q", exit, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"Webox doctor directadmin", "verdict         OK", "[OK] Whoami", "[OK] Domains", "[OK] SSLCertificates"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output: %s", want, out)
		}
	}
}

func TestRunDoctorDirectadmin_JSONOutput_EmitsStructuredReport(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	host, port := splitHostPort(t, srv.URL)
	opts := directadminOpts{
		host:           host,
		user:           "alice",
		loginKey:       "BADKEY",
		apiPort:        port,
		noSSH:          true,
		json:           true,
		httpsTransport: srv.Client().Transport,
	}

	var stdout, stderr bytes.Buffer
	exit := runDoctorDirectadmin(opts, &stdout, &stderr)
	if exit != exitGeneric {
		t.Fatalf("expected exitGeneric (BLOCKED), got %d (stderr=%q)", exit, stderr.String())
	}
	var report directadminReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode JSON: %v\nraw=%s", err, stdout.String())
	}
	if report.Verdict != directadminBlocked {
		t.Fatalf("expected BLOCKED verdict, got %s", report.Verdict)
	}
	if len(report.Sections) != 5 {
		t.Fatalf("expected 5 sections, got %d", len(report.Sections))
	}
	for _, sec := range report.Sections {
		if sec.Status != "AUTH_FAILED" {
			t.Errorf("section %s = %s, want AUTH_FAILED", sec.Name, sec.Status)
		}
	}
}

func TestRunDoctorDirectadmin_APIDisabled_MapsToDISABLEDStatus(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("Not Found"))
	}))
	defer srv.Close()
	host, port := splitHostPort(t, srv.URL)

	opts := directadminOpts{
		host:           host,
		user:           "alice",
		loginKey:       "KEY",
		apiPort:        port,
		noSSH:          true,
		json:           true,
		httpsTransport: srv.Client().Transport,
	}
	var stdout, stderr bytes.Buffer
	exit := runDoctorDirectadmin(opts, &stdout, &stderr)
	// All 5 sections DISABLED → OK rollup per Sprint 23 policy.
	if exit != exitOK {
		t.Fatalf("expected exitOK (DISABLED rolls up OK), got %d (stderr=%q)", exit, stderr.String())
	}
	var report directadminReport
	_ = json.Unmarshal(stdout.Bytes(), &report)
	if report.Verdict != directadminOK {
		t.Fatalf("verdict = %s, want OK", report.Verdict)
	}
	for _, sec := range report.Sections {
		if sec.Status != "DISABLED" {
			t.Errorf("section %s = %s, want DISABLED", sec.Name, sec.Status)
		}
	}
}

func TestDirectadminTransportLabel_KnownTypes(t *testing.T) {
	t.Parallel()
	c, _ := daapi.NewClient("https://x:2222", "u", "k", nil)
	if got := directadminTransportLabel(c); got != "HTTPS" {
		t.Errorf("Client → %q, want HTTPS", got)
	}
	s, _ := daapi.NewSSHFallback(&fakeDARunner{}, "u", "k", 2222)
	if got := directadminTransportLabel(s); got != "SSH" {
		t.Errorf("SSHFallback → %q, want SSH", got)
	}
	comp, _ := daapi.NewComposite(c, s)
	if got := directadminTransportLabel(comp); got != "HTTPS+SSH" {
		t.Errorf("Composite → %q, want HTTPS+SSH", got)
	}
}

func TestCapDirectadminSample_TruncatesAndAnnotates(t *testing.T) {
	t.Parallel()
	in := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	got := capDirectadminSample(in)
	if len(got) != directadminPreviewLineCap+1 {
		t.Fatalf("len = %d, want %d", len(got), directadminPreviewLineCap+1)
	}
	if got[len(got)-1] != "(+2 more)" {
		t.Errorf("annotation = %q", got[len(got)-1])
	}
	if d := capDirectadminSample([]string{"x", "y"}); len(d) != 2 {
		t.Errorf("untruncated len = %d, want 2", len(d))
	}
}

// splitHostPort extracts host + port from a https://h:port test URL.
// httptest never includes a path, so a substring split is enough.
func splitHostPort(t *testing.T, u string) (host string, port int) {
	t.Helper()
	hostport := strings.TrimPrefix(u, "https://")
	idx := strings.LastIndex(hostport, ":")
	if idx < 0 {
		t.Fatalf("missing port in test URL: %s", u)
	}
	host = hostport[:idx]
	if _, err := fmtSscanf(hostport[idx+1:], &port); err != nil {
		t.Fatalf("parse port: %v", err)
	}
	return host, port
}

// fmtSscanf wraps fmt.Sscanf to keep splitHostPort tidy.
func fmtSscanf(s string, port *int) (int, error) {
	var p int
	n, err := scanInt(s, &p)
	*port = p
	return n, err
}

func scanInt(s string, out *int) (int, error) {
	v := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, errInvalidInt
		}
		v = v*10 + int(c-'0')
	}
	*out = v
	return 1, nil
}

var errInvalidInt = errors.New("invalid int")
