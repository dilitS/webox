package main

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

// stubDirectadminDispatcher captures the directadminOpts passed
// to the dispatcher so tests can assert the parser routed every
// flag into the right struct field.
func stubDirectadminDispatcher(captured *directadminOpts) directadminDispatcher {
	return func(opts directadminOpts, _, _ io.Writer) int {
		*captured = opts
		return exitOK
	}
}

func TestRun_DoctorDirectadmin_RoutesEveryFlag(t *testing.T) {
	t.Parallel()
	var captured directadminOpts
	args := []string{
		"doctor", "directadmin",
		"--host=panel.example.com",
		"--user=alice",
		"--loginkey=LK",
		"--api-port=2233",
		"--ssh-port=2222",
		"--timeout=15s",
		"--json",
	}
	var stdout, stderr bytes.Buffer
	got := runWithFullDeps(args, &stdout, &stderr, runDoctor, runDoctorGitHub, defaultPresetDispatcher, brokenCpanelDispatcher, stubDirectadminDispatcher(&captured), brokenTUI)
	if got != exitOK {
		t.Fatalf("exit = %d, stderr=%q", got, stderr.String())
	}
	if captured.host != "panel.example.com" {
		t.Errorf("host = %q", captured.host)
	}
	if captured.user != "alice" {
		t.Errorf("user = %q", captured.user)
	}
	if captured.loginKey != "LK" {
		t.Errorf("loginKey = %q", captured.loginKey)
	}
	if captured.apiPort != 2233 {
		t.Errorf("apiPort = %d", captured.apiPort)
	}
	if captured.sshPort != 2222 {
		t.Errorf("sshPort = %d", captured.sshPort)
	}
	if captured.timeout.Seconds() != 15 {
		t.Errorf("timeout = %v", captured.timeout)
	}
	if !captured.json {
		t.Error("json should be true")
	}
}

func TestRun_DoctorDirectadmin_NoSSHAndNoAPIToggleIntoStruct(t *testing.T) {
	t.Parallel()
	var captured directadminOpts
	args := []string{
		"doctor", "directadmin",
		"--host=h", "--user=u", "--loginkey=k",
		"--no-ssh",
	}
	var stdout, stderr bytes.Buffer
	if got := runWithFullDeps(args, &stdout, &stderr, runDoctor, runDoctorGitHub, defaultPresetDispatcher, brokenCpanelDispatcher, stubDirectadminDispatcher(&captured), brokenTUI); got != exitOK {
		t.Fatalf("--no-ssh: exit = %d, stderr=%q", got, stderr.String())
	}
	if !captured.noSSH {
		t.Errorf("noSSH should be true")
	}

	captured = directadminOpts{}
	args2 := []string{
		"doctor", "directadmin",
		"--host=h", "--user=u",
		"--no-api",
	}
	var s2, e2 bytes.Buffer
	if got := runWithFullDeps(args2, &s2, &e2, runDoctor, runDoctorGitHub, defaultPresetDispatcher, brokenCpanelDispatcher, stubDirectadminDispatcher(&captured), brokenTUI); got != exitOK {
		t.Fatalf("--no-api: exit = %d, stderr=%q", got, e2.String())
	}
	if !captured.noAPI {
		t.Errorf("noAPI should be true")
	}
}

func TestRun_DoctorDirectadmin_LoginkeyOnlyValidWithDirectadminTarget(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			"loginkey-with-cpanel",
			[]string{"doctor", "cpanel", "--host=h", "--user=u", "--loginkey=LK"},
			"--loginkey is only valid with `webox doctor directadmin`",
		},
		{
			"token-with-directadmin",
			[]string{"doctor", "directadmin", "--host=h", "--user=u", "--token=TOK"},
			"--token is only valid with `webox doctor cpanel`",
		},
		{
			"no-api-with-cpanel",
			[]string{"doctor", "cpanel", "--host=h", "--user=u", "--token=T", "--no-api"},
			"--no-api is only valid with `webox doctor directadmin`",
		},
		{
			"no-uapi-with-directadmin",
			[]string{"doctor", "directadmin", "--host=h", "--user=u", "--loginkey=K", "--no-uapi"},
			"--no-uapi is only valid with `webox doctor cpanel`",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			got := runWithFullDeps(tc.args, &stdout, &stderr, runDoctor, runDoctorGitHub, defaultPresetDispatcher, brokenCpanelDispatcher, brokenDirectadminDispatcher, brokenTUI)
			if got != exitMisuse {
				t.Fatalf("expected exitMisuse, got %d", got)
			}
			if !strings.Contains(stderr.String(), tc.wantErr) {
				t.Fatalf("stderr %q missing %q", stderr.String(), tc.wantErr)
			}
		})
	}
}

func TestRun_DoctorDirectadmin_RejectsBareDirectadminToken(t *testing.T) {
	t.Parallel()
	// `directadmin` outside `doctor` context must not silently
	// activate the doctor target.
	var stdout, stderr bytes.Buffer
	got := runWithFullDeps([]string{"directadmin", "--host=h"}, &stdout, &stderr, runDoctor, runDoctorGitHub, defaultPresetDispatcher, brokenCpanelDispatcher, brokenDirectadminDispatcher, brokenTUI)
	if got != exitMisuse {
		t.Fatalf("expected exitMisuse, got %d", got)
	}
	if !strings.Contains(stderr.String(), "`directadmin` is only valid after `doctor`") {
		t.Fatalf("stderr should explain `directadmin` requires `doctor` prefix; got %q", stderr.String())
	}
}

func TestRun_DoctorDirectadmin_ProviderNewClaimsDirectadminName(t *testing.T) {
	t.Parallel()
	// `webox provider new directadmin` must reach the provider-
	// new path; the `directadmin` token belongs to it, not to
	// `doctor`. Sprint 22 already proved this works for `cpanel`;
	// this test extends the coverage to the new token.
	var stdout, stderr bytes.Buffer
	got := runWithFullDeps([]string{"provider", "new", "directadmin", "--dry-run"}, &stdout, &stderr, runDoctor, runDoctorGitHub, defaultPresetDispatcher, brokenCpanelDispatcher, brokenDirectadminDispatcher, brokenTUI)
	if got == exitMisuse {
		t.Fatalf("provider new directadmin should not be exitMisuse; stderr=%q", stderr.String())
	}
}

func TestApplyAPIPortFlag_PortValidation(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, port string
		target     string
		wantErr    string
	}{
		{"negative", "-1", "directadmin", "must be a positive integer"},
		{"zero", "0", "directadmin", "must be a positive integer"},
		{"too-big", "70000", "directadmin", "must be a positive integer"},
		{"not-numeric", "abc", "directadmin", "must be a positive integer"},
		{"wrong-context", "2222", "preset", "--api-port is only valid"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			parsed := &opts{doctorTarget: tc.target}
			msg, handled := applyAPIPortFlag(parsed, tc.port)
			if !handled {
				t.Fatal("flag should be handled")
			}
			if !strings.Contains(msg, tc.wantErr) {
				t.Errorf("expected %q in %q", tc.wantErr, msg)
			}
		})
	}
}
