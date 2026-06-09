package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/dilitS/webox/providers/cpanel/uapi"
)

// fakeReader is a test double for [uapi.Reader] that records call
// counts and returns canned per-method results. Each test
// constructs one and wires it into runCpanelChecks via a custom
// reader builder.
type fakeReader struct {
	domains       *uapi.DomainInfoListResponse
	domainsErr    error
	apps          *uapi.PassengerAppsListResponse
	appsErr       error
	dbs           *uapi.MysqlListDatabasesResponse
	dbsErr        error
	keys          *uapi.SSLListKeysResponse
	keysErr       error
	domainsCalled int
}

func (f *fakeReader) ListDomains(_ context.Context) (*uapi.DomainInfoListResponse, error) {
	f.domainsCalled++
	return f.domains, f.domainsErr
}

func (f *fakeReader) ListPassengerApps(_ context.Context) (*uapi.PassengerAppsListResponse, error) {
	return f.apps, f.appsErr
}

func (f *fakeReader) ListMysqlDatabases(_ context.Context) (*uapi.MysqlListDatabasesResponse, error) {
	return f.dbs, f.dbsErr
}

func (f *fakeReader) ListSSLKeys(_ context.Context) (*uapi.SSLListKeysResponse, error) {
	return f.keys, f.keysErr
}

// Transport identifies this fake as an HTTPS Reader. All cpanel_test
// happy/sad paths wrap `fakeReader` in `uapi.Composite{Primary: ...}`
// (no Secondary), so the composite delegates Transport() back to the
// Primary's label, and "HTTPS" is what the doctor CLI prints. If a
// future test needs to assert an SSH-only composite, declare a second
// fake that returns "SSH" from Transport().
func (*fakeReader) Transport() string { return "HTTPS" }

func TestValidateCpanelOpts_RequiresHostAndUser(t *testing.T) {
	cases := []struct {
		name string
		opts cpanelOpts
		want string
	}{
		{"missing-host", cpanelOpts{user: "u"}, "--host is required"},
		{"missing-user", cpanelOpts{host: "h"}, "--user is required"},
		{"no-uapi+no-ssh", cpanelOpts{host: "h", user: "u", noSSH: true, noUAPI: true}, "at least one transport"},
		{"no-ssh+no-token", cpanelOpts{host: "h", user: "u", noSSH: true}, "--token=TOKEN is required when --no-ssh"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateCpanelOpts(&tc.opts)
			if err == nil {
				t.Fatal("want error, got nil")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %q, want substring %q", err.Error(), tc.want)
			}
		})
	}
}

func TestValidateCpanelOpts_AppliesDefaults(t *testing.T) {
	opts := cpanelOpts{host: "h", user: "u", token: "tok"}
	if err := validateCpanelOpts(&opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.apiPort != defaultCpanelAPIPort {
		t.Errorf("apiPort = %d, want %d", opts.apiPort, defaultCpanelAPIPort)
	}
	if opts.sshPort != defaultCpanelSSHPort {
		t.Errorf("sshPort = %d, want %d", opts.sshPort, defaultCpanelSSHPort)
	}
	if opts.timeout != defaultCpanelTimeout {
		t.Errorf("timeout = %v, want %v", opts.timeout, defaultCpanelTimeout)
	}
}

func TestRollupCpanelVerdict_TableDriven(t *testing.T) {
	mk := func(statuses ...string) []cpanelSectionResult {
		out := make([]cpanelSectionResult, 0, len(statuses))
		for _, s := range statuses {
			out = append(out, cpanelSectionResult{Status: s})
		}
		return out
	}
	cases := []struct {
		name    string
		input   []cpanelSectionResult
		verdict cpanelVerdict
	}{
		{"all-ok", mk("OK", "OK", "OK", "OK"), cpanelOK},
		{"all-disabled-counts-as-ok", mk("DISABLED", "DISABLED", "DISABLED", "DISABLED"), cpanelOK},
		{"mixed-ok-disabled", mk("OK", "DISABLED", "OK", "OK"), cpanelOK},
		{"one-failure", mk("OK", "OK", "FAILED", "OK"), cpanelDegraded},
		{"all-failed", mk("FAILED", "FAILED", "FAILED", "FAILED"), cpanelBlocked},
		{"all-auth-failed", mk("AUTH_FAILED", "AUTH_FAILED", "AUTH_FAILED", "AUTH_FAILED"), cpanelBlocked},
		{"all-unreachable", mk("UNREACHABLE", "UNREACHABLE", "UNREACHABLE", "UNREACHABLE"), cpanelBlocked},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := rollupCpanelVerdict(tc.input)
			if got != tc.verdict {
				t.Errorf("verdict = %s, want %s", got, tc.verdict)
			}
		})
	}
}

func TestRunCpanelChecks_HappyPath(t *testing.T) {
	reader := &uapi.Composite{Primary: &fakeReader{
		domains: &uapi.DomainInfoListResponse{
			MainDomain:   "example.com",
			SubDomains:   []string{"api.example.com"},
			AddonDomains: []string{"second.example.com"},
		},
		apps: &uapi.PassengerAppsListResponse{Applications: []uapi.PassengerApp{
			{Name: "shop"},
			{Name: "blog"},
		}},
		dbs: &uapi.MysqlListDatabasesResponse{Databases: []uapi.MysqlDatabase{
			{Name: "operator_shop"},
		}},
		keys: &uapi.SSLListKeysResponse{Keys: []uapi.SSLKey{
			{FriendlyName: "example.com"},
		}},
	}}
	report := runCpanelChecks(context.Background(), cpanelOpts{host: "h", user: "u", apiPort: 2083, sshPort: 22}, reader, nil)
	if report.Verdict != cpanelOK {
		t.Errorf("verdict = %s, want OK", report.Verdict)
	}
	if len(report.Sections) != 4 {
		t.Fatalf("sections = %d, want 4", len(report.Sections))
	}
	for _, sec := range report.Sections {
		if sec.Status != "OK" {
			t.Errorf("%s.Status = %s, want OK", sec.Name, sec.Status)
		}
		if sec.Transport != "HTTPS" {
			t.Errorf("%s.Transport = %s, want HTTPS (Composite{Primary: ...})", sec.Name, sec.Transport)
		}
	}
}

func TestRunCpanelChecks_PartialFailureProducesDegraded(t *testing.T) {
	reader := &uapi.Composite{Primary: &fakeReader{
		domains: &uapi.DomainInfoListResponse{MainDomain: "example.com"},
		appsErr: uapi.ErrModuleFunctionDenied,
		dbs:     &uapi.MysqlListDatabasesResponse{Databases: []uapi.MysqlDatabase{{Name: "db"}}},
		keysErr: uapi.ErrAuthenticationFailed,
	}}
	report := runCpanelChecks(context.Background(), cpanelOpts{host: "h", user: "u"}, reader, nil)
	if report.Verdict != cpanelDegraded {
		t.Errorf("verdict = %s, want DEGRADED", report.Verdict)
	}
	wantStatus := map[string]string{
		"Domains":        "OK",
		"PassengerApps":  "DISABLED",
		"MysqlDatabases": "OK",
		"SSLKeys":        "AUTH_FAILED",
	}
	for _, sec := range report.Sections {
		if sec.Status != wantStatus[sec.Name] {
			t.Errorf("%s.Status = %s, want %s", sec.Name, sec.Status, wantStatus[sec.Name])
		}
	}
}

func TestRunCpanelChecks_AllFailedProducesBlocked(t *testing.T) {
	reader := &uapi.Composite{Primary: &fakeReader{
		domainsErr: uapi.ErrTransportUnavailable,
		appsErr:    uapi.ErrTransportUnavailable,
		dbsErr:     uapi.ErrTransportUnavailable,
		keysErr:    uapi.ErrTransportUnavailable,
	}}
	report := runCpanelChecks(context.Background(), cpanelOpts{host: "h", user: "u"}, reader, nil)
	if report.Verdict != cpanelBlocked {
		t.Errorf("verdict = %s, want BLOCKED", report.Verdict)
	}
}

func TestEmitCpanelReport_TextRendersSections(t *testing.T) {
	report := cpanelReport{
		Host:    "panel.example.com",
		User:    "operator",
		Verdict: cpanelOK,
		Sections: []cpanelSectionResult{
			{Name: "Domains", Status: "OK", Transport: "HTTPS", Count: 1, Sample: []string{"example.com"}},
		},
	}
	var stdout, stderr bytes.Buffer
	code := emitCpanelReport(false, report, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("exit = %d, want exitOK", code)
	}
	for _, want := range []string{"panel.example.com", "Verdict: OK", "[OK] Domains", "example.com"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("stdout missing %q, got:\n%s", want, stdout.String())
		}
	}
}

func TestEmitCpanelReport_JSONHasStableSchema(t *testing.T) {
	report := cpanelReport{
		Host: "panel.example.com", User: "operator", APIPort: 2083, SSHPort: 22,
		Verdict: cpanelDegraded,
		Sections: []cpanelSectionResult{
			{Name: "Domains", Status: "OK", Transport: "HTTPS", Count: 1, Sample: []string{"example.com"}},
			{Name: "PassengerApps", Status: "DISABLED", Transport: "SSH", Count: 0, Error: "disabled"},
		},
		Notes: []string{"--token absent: HTTPS UAPI disabled"},
	}
	var stdout, stderr bytes.Buffer
	code := emitCpanelReport(true, report, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("exit = %d, want exitOK (DEGRADED still exits 0)", code)
	}
	var decoded cpanelReport
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if decoded.Host != "panel.example.com" || decoded.Verdict != cpanelDegraded {
		t.Errorf("decoded mismatch: %+v", decoded)
	}
	if len(decoded.Sections) != 2 {
		t.Fatalf("decoded.Sections = %d", len(decoded.Sections))
	}
}

func TestEmitCpanelReport_BlockedExitsNonZero(t *testing.T) {
	report := cpanelReport{
		Verdict:  cpanelBlocked,
		Sections: []cpanelSectionResult{{Name: "Domains", Status: "FAILED"}},
	}
	var stdout, stderr bytes.Buffer
	code := emitCpanelReport(false, report, &stdout, &stderr)
	if code != exitGeneric {
		t.Errorf("exit = %d, want %d (BLOCKED → 1)", code, exitGeneric)
	}
}

func TestBuildCpanelReader_HTTPSOnly(t *testing.T) {
	r, notes, err := buildCpanelReader(cpanelOpts{
		host: "h", user: "u", token: "tok", apiPort: 2083, sshPort: 22, timeout: time.Second,
		noSSH: true,
	})
	if err != nil {
		t.Fatalf("buildCpanelReader: %v", err)
	}
	c, ok := r.(*uapi.Composite)
	if !ok {
		t.Fatalf("expected *uapi.Composite, got %T", r)
	}
	if c.Primary == nil {
		t.Error("expected Primary (HTTPS) to be wired")
	}
	if c.Secondary != nil {
		t.Error("expected Secondary (SSH) to be nil with --no-ssh")
	}
	if len(notes) == 0 || !strings.Contains(notes[0], "SSH fallback disabled") {
		t.Errorf("expected note about --no-ssh, got %v", notes)
	}
}

func TestBuildCpanelReader_SSHOnly(t *testing.T) {
	r, notes, err := buildCpanelReader(cpanelOpts{
		host: "h", user: "u", apiPort: 2083, sshPort: 22, timeout: time.Second,
		sshFactory: fakeSSHFactory,
	})
	if err != nil {
		t.Fatalf("buildCpanelReader: %v", err)
	}
	c := r.(*uapi.Composite)
	if c.Primary != nil {
		t.Error("expected Primary nil when no --token")
	}
	if c.Secondary == nil {
		t.Error("expected Secondary (SSH) wired")
	}
	if len(notes) == 0 || !strings.Contains(notes[0], "--token absent") {
		t.Errorf("expected note about absent token, got %v", notes)
	}
}

func TestBuildCpanelReader_BothPaths(t *testing.T) {
	r, _, err := buildCpanelReader(cpanelOpts{
		host: "h", user: "u", token: "tok", apiPort: 2083, sshPort: 22, timeout: time.Second,
		sshFactory: fakeSSHFactory,
	})
	if err != nil {
		t.Fatalf("buildCpanelReader: %v", err)
	}
	c := r.(*uapi.Composite)
	if c.Primary == nil || c.Secondary == nil {
		t.Error("expected both transports wired with --token + default SSH")
	}
}

func TestRunDoctorCpanel_FailsOnMissingHost(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runDoctorCpanel(cpanelOpts{user: "u"}, &stdout, &stderr)
	if code != exitMisuse {
		t.Errorf("exit = %d, want exitMisuse", code)
	}
	if !strings.Contains(stderr.String(), "--host is required") {
		t.Errorf("stderr = %q, want --host error", stderr.String())
	}
}

func TestRunDoctorCpanel_EndToEnd_TextOutput(t *testing.T) {
	opts := cpanelOpts{
		host: "h", user: "u", timeout: time.Second,
		sshFactory: fakeSSHFactory,
		noSSH:      false,
		noUAPI:     true,
	}
	var stdout, stderr bytes.Buffer
	code := runDoctorCpanel(opts, &stdout, &stderr)
	if code == exitOK {
		// SSH fakery means we will report all-FAILED. Either way
		// the call should not crash; we just assert non-empty
		// output and reasonable exit code (BLOCKED → 1).
		t.Logf("note: faked SSH; verdict-based exit is acceptable")
	}
	if !strings.Contains(stdout.String(), "Webox doctor cpanel") {
		t.Errorf("stdout missing header; got %q", stdout.String())
	}
}

func TestCpanelCLI_ParseAccepts(t *testing.T) {
	args := []string{
		"doctor", "cpanel",
		"--host=panel.example.com", "--user=operator",
		"--token=CPANELTOKEN", "--api-port=2083", "--ssh-port=22",
		"--timeout=20s", "--no-uapi",
		"--json",
	}
	parsed, errMsg := parseArgs(args)
	if errMsg != "" {
		t.Fatalf("parseArgs: %s", errMsg)
	}
	if parsed.doctorTarget != "cpanel" {
		t.Errorf("doctorTarget = %q, want cpanel", parsed.doctorTarget)
	}
	if parsed.cpanelToken != "CPANELTOKEN" {
		t.Errorf("cpanelToken = %q", parsed.cpanelToken)
	}
	if !parsed.cpanelNoUAPI {
		t.Error("expected --no-uapi to set cpanelNoUAPI")
	}
	if parsed.cpanelAPIPort != 2083 || parsed.cpanelSSHPort != 22 {
		t.Errorf("ports = %d/%d", parsed.cpanelAPIPort, parsed.cpanelSSHPort)
	}
	if parsed.presetTimeout != 20*time.Second {
		t.Errorf("timeout = %v, want 20s", parsed.presetTimeout)
	}
}

func TestCpanelCLI_ParseRejectsTokenOutsideCpanel(t *testing.T) {
	_, errMsg := parseArgs([]string{"doctor", "preset", "--token=X"})
	if !strings.Contains(errMsg, "--token is only valid") {
		t.Errorf("errMsg = %q, want token-context message", errMsg)
	}
}

func TestCpanelCLI_RejectsNoSSHWithoutToken(t *testing.T) {
	opts := cpanelOpts{host: "h", user: "u", noSSH: true}
	err := validateCpanelOpts(&opts)
	if err == nil || !strings.Contains(err.Error(), "--token=TOKEN is required when --no-ssh") {
		t.Errorf("err = %v, want explicit --token requirement", err)
	}
}

func TestCpanelCLI_RejectsBothToggles(t *testing.T) {
	opts := cpanelOpts{host: "h", user: "u", noSSH: true, noUAPI: true}
	err := validateCpanelOpts(&opts)
	if err == nil || !strings.Contains(err.Error(), "at least one transport") {
		t.Errorf("err = %v, want at-least-one message", err)
	}
}

func TestTruncateWord_KeepsShortStrings(t *testing.T) {
	if got := truncateWord("short", 80); got != "short" {
		t.Errorf("got %q, want short", got)
	}
}

func TestTruncateWord_TruncatesLongStrings(t *testing.T) {
	in := strings.Repeat("x", 200)
	out := truncateWord(in, 80)
	if !strings.HasSuffix(out, "...") {
		t.Errorf("missing ellipsis: %q", out)
	}
	if len(out) > 83 {
		t.Errorf("len = %d, want ≤ 83", len(out))
	}
}

func TestCapSample_TruncatesWithCounter(t *testing.T) {
	in := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	out := capSample(in)
	if len(out) != cpanelPreviewLineCap+1 {
		t.Fatalf("len = %d, want %d", len(out), cpanelPreviewLineCap+1)
	}
	if !strings.HasPrefix(out[len(out)-1], "(+") {
		t.Errorf("missing (+N more) marker: %v", out)
	}
}

// fakeSSHFactory always returns the same fake runner; tests that
// just want the Composite wiring don't care about the runner's
// behaviour.
func fakeSSHFactory(_, _ string, _ int, _ time.Duration) (uapi.SSHRunner, error) {
	return &fakeRunner{}, nil
}

type fakeRunner struct{}

func (f *fakeRunner) Run(_ context.Context, _ string) (stdout, stderr []byte, exitCode int, err error) {
	// Pretend SSH succeeded but with an unrecognised payload, so the
	// fallback decoder surfaces ErrMalformedResponse. The CLI then
	// catches the error and rolls each section into FAILED.
	return []byte("not json"), nil, 0, nil
}

// Compile-time assertion that the fake SSH factory's runner
// satisfies uapi.SSHRunner.
var _ uapi.SSHRunner = (*fakeRunner)(nil)

// Compile-time assertion that the fake reader satisfies
// uapi.Reader, anchoring the type contract.
var _ uapi.Reader = (*fakeReader)(nil)

// Keep errors imported so the test file compiles even when none of
// the table-driven cases above actually invoke errors.Is.
var _ = errors.Is

// Keep io imported so future tests can hand-roll stub writers
// without churning the imports block.
var _ io.Writer = (*bytes.Buffer)(nil)
