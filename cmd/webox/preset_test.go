package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/dilitS/webox/presets"
)

const stubValidPreset = `{
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
    "logs_path_known": true,
    "safe_restart": true,
    "git_available": false,
    "sftp_available": true
  },
  "paths": {
    "deploy_path_template": "/home/{{user}}/domains/{{domain}}/public_html",
    "log_path_template": "/home/{{user}}/domains/{{domain}}/logs",
    "env_path_template": "/home/{{user}}/domains/{{domain}}/.env"
  },
  "probes": ["devil node list"],
  "known_risks": ["Devil CLI requires interactive SSH shell"],
  "sources": ["https://small.pl/pomoc/"],
  "verified": {
    "fixture_dir": "testing/fixtures/devil",
    "last_verified_at": "2026-05-25",
    "verified_by": "@maintainer"
  }
}`

func stubRegistry(t *testing.T, files map[string]string) *presets.Registry {
	t.Helper()
	mfs := fstest.MapFS{}
	for name, payload := range files {
		mfs["presets/"+name] = &fstest.MapFile{Data: []byte(payload)}
	}
	res, err := presets.LoadFrom(mfs, "presets")
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}
	return presets.NewRegistryFromResult(res)
}

func stubProvider(reg *presets.Registry) presetRegistryProvider {
	return func() (*presets.Registry, error) { return reg, nil }
}

func TestPresetDoctor_List_TextHumansSorted(t *testing.T) {
	t.Parallel()

	reg := stubRegistry(t, map[string]string{"smallhost-devil.json": stubValidPreset})
	var stdout, stderr bytes.Buffer
	got := runPresetDoctor(presetOpts{}, &stdout, &stderr, stubProvider(reg))
	if got != exitOK {
		t.Fatalf("exit = %d, want 0 (stderr=%q)", got, stderr.String())
	}
	out := stdout.String()
	for _, needle := range []string{"smallhost-devil", "verified", "Poland", "1 preset"} {
		if !strings.Contains(out, needle) {
			t.Errorf("output missing %q\n--- output ---\n%s", needle, out)
		}
	}
}

func TestPresetDoctor_List_JSONShape(t *testing.T) {
	t.Parallel()

	reg := stubRegistry(t, map[string]string{"smallhost-devil.json": stubValidPreset})
	var stdout, stderr bytes.Buffer
	got := runPresetDoctor(presetOpts{json: true}, &stdout, &stderr, stubProvider(reg))
	if got != exitOK {
		t.Fatalf("exit = %d, want 0 (stderr=%q)", got, stderr.String())
	}
	var decoded presetListJSON
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("decode JSON: %v\n--- output ---\n%s", err, stdout.String())
	}
	if len(decoded.Presets) != 1 {
		t.Fatalf("decoded.Presets = %d, want 1", len(decoded.Presets))
	}
	entry := decoded.Presets[0]
	if entry.ID != "smallhost-devil" || entry.Status != "verified" {
		t.Fatalf("entry = %+v", entry)
	}
	if len(entry.Badges) == 0 {
		t.Fatal("entry.Badges should be populated for a verified preset")
	}
}

func TestPresetDoctor_Show_Text(t *testing.T) {
	t.Parallel()

	reg := stubRegistry(t, map[string]string{"smallhost-devil.json": stubValidPreset})
	var stdout, stderr bytes.Buffer
	got := runPresetDoctor(presetOpts{id: "smallhost-devil"}, &stdout, &stderr, stubProvider(reg))
	if got != exitOK {
		t.Fatalf("exit = %d (stderr=%q)", got, stderr.String())
	}
	out := stdout.String()
	for _, needle := range []string{
		"small.pl (Devil)",
		"node_runtime",
		"devil",
		"Probes",
		"Known risks",
		"Sources",
	} {
		if !strings.Contains(out, needle) {
			t.Errorf("show output missing %q\n--- output ---\n%s", needle, out)
		}
	}
}

func TestPresetDoctor_Show_JSON_RoundTrips(t *testing.T) {
	t.Parallel()

	reg := stubRegistry(t, map[string]string{"smallhost-devil.json": stubValidPreset})
	var stdout, stderr bytes.Buffer
	got := runPresetDoctor(presetOpts{id: "smallhost-devil", json: true}, &stdout, &stderr, stubProvider(reg))
	if got != exitOK {
		t.Fatalf("exit = %d (stderr=%q)", got, stderr.String())
	}
	var p presets.Preset
	if err := json.Unmarshal(stdout.Bytes(), &p); err != nil {
		t.Fatalf("unmarshal failed: %v\n--- output ---\n%s", err, stdout.String())
	}
	if p.ID != "smallhost-devil" {
		t.Fatalf("decoded ID = %q", p.ID)
	}
	if p.Status != presets.StatusVerified {
		t.Fatalf("decoded Status = %q", p.Status)
	}
}

func TestPresetDoctor_Show_NotFoundReturnsMisuse(t *testing.T) {
	t.Parallel()

	reg := stubRegistry(t, map[string]string{"smallhost-devil.json": stubValidPreset})
	var stdout, stderr bytes.Buffer
	got := runPresetDoctor(presetOpts{id: "nope"}, &stdout, &stderr, stubProvider(reg))
	if got != exitMisuse {
		t.Fatalf("exit = %d, want exitMisuse=%d", got, exitMisuse)
	}
	if !strings.Contains(stderr.String(), "preset not found") {
		t.Fatalf("stderr should mention preset not found: %q", stderr.String())
	}
}

func TestPresetDoctor_ProbeWithoutHostUserFallsBackToDeclarative(t *testing.T) {
	t.Parallel()

	// Sprint 21 TASK-21.4 wires live probing behind --host / --user;
	// without those flags the command must NOT attempt SSH and MUST
	// still print preset metadata so the operator can read the
	// declarative info. The notice on stderr explains how to switch
	// to live execution.
	reg := stubRegistry(t, map[string]string{"smallhost-devil.json": stubValidPreset})
	var stdout, stderr bytes.Buffer
	got := runPresetDoctor(presetOpts{id: "smallhost-devil", probe: true}, &stdout, &stderr, stubProvider(reg))
	if got != exitOK {
		t.Fatalf("exit = %d, want 0 (stderr=%q)", got, stderr.String())
	}
	if !strings.Contains(stderr.String(), "--host") || !strings.Contains(stderr.String(), "--user") {
		t.Fatalf("stderr should explain how to enable live probing: %q", stderr.String())
	}
	if !strings.Contains(stdout.String(), "small.pl") {
		t.Fatalf("stdout should still print preset details when --probe is requested without host/user: %q", stdout.String())
	}
}

func TestPresetDoctor_ListReportsLoadErrors(t *testing.T) {
	t.Parallel()

	bad := strings.Replace(stubValidPreset, `"status": "verified"`, `"status": "experimental"`, 1)
	reg := stubRegistry(t, map[string]string{
		"smallhost-devil.json": stubValidPreset,
		"broken.json":          bad,
	})
	var stdout, stderr bytes.Buffer
	got := runPresetDoctor(presetOpts{}, &stdout, &stderr, stubProvider(reg))
	if got != exitOK {
		t.Fatalf("exit = %d, want 0", got)
	}
	if !strings.Contains(stdout.String(), "Load errors") {
		t.Fatalf("text output should surface schema-violation drift: %q", stdout.String())
	}
}

func TestPresetDoctor_ProviderError(t *testing.T) {
	t.Parallel()

	provider := func() (*presets.Registry, error) { return nil, errors.New("boom") }
	var stdout, stderr bytes.Buffer
	got := runPresetDoctor(presetOpts{}, &stdout, &stderr, provider)
	if got != exitMisuse {
		t.Fatalf("exit = %d, want exitMisuse=%d", got, exitMisuse)
	}
	if !strings.Contains(stderr.String(), "boom") {
		t.Fatalf("stderr should propagate provider error: %q", stderr.String())
	}
}

func TestRun_DoctorPreset_AcceptsTokensAndDispatches(t *testing.T) {
	t.Parallel()

	stub := func(opts presetOpts, stdout, stderr io.Writer) int {
		fmt.Fprintf(stdout, "preset(id=%q,json=%t,probe=%t)", opts.id, opts.json, opts.probe)
		return exitOK
	}
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"list text", []string{"doctor", "preset"}, `preset(id="",json=false,probe=false)`},
		{"list json", []string{"doctor", "preset", "--json"}, `preset(id="",json=true,probe=false)`},
		{"show", []string{"doctor", "preset", "--id=cpanel-generic"}, `preset(id="cpanel-generic",json=false,probe=false)`},
		{"show json", []string{"doctor", "preset", "--id=cpanel-generic", "--json"}, `preset(id="cpanel-generic",json=true,probe=false)`},
		{"probe stub", []string{"doctor", "preset", "--id=cpanel-generic", "--probe"}, `preset(id="cpanel-generic",json=false,probe=true)`},
	}
	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			got := runWithFullDeps(tt.args, &stdout, &stderr, runDoctor, runDoctorGitHub, stub, brokenTUI)
			if got != exitOK {
				t.Fatalf("exit = %d (stderr=%q)", got, stderr.String())
			}
			if stdout.String() != tt.want {
				t.Fatalf("dispatcher payload = %q, want %q", stdout.String(), tt.want)
			}
		})
	}
}

func TestRun_DoctorPreset_RejectsProbeWithoutID(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	stub := func(opts presetOpts, stdout, stderr io.Writer) int { return exitOK }
	got := runWithFullDeps(
		[]string{"doctor", "preset", "--probe"},
		&stdout, &stderr,
		runDoctor, runDoctorGitHub, stub, brokenTUI,
	)
	if got != exitMisuse {
		t.Fatalf("exit = %d, want exitMisuse=%d", got, exitMisuse)
	}
	if !strings.Contains(stderr.String(), "--probe requires --id=") {
		t.Fatalf("stderr should explain --probe needs --id; got %q", stderr.String())
	}
}

func TestRun_DoctorPreset_RejectsIDWithoutPresetTarget(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	stub := func(opts presetOpts, stdout, stderr io.Writer) int { return exitOK }
	got := runWithFullDeps(
		[]string{"doctor", "--id=cpanel-generic"},
		&stdout, &stderr,
		runDoctor, runDoctorGitHub, stub, brokenTUI,
	)
	if got != exitMisuse {
		t.Fatalf("exit = %d, want exitMisuse=%d (stderr=%q)", got, exitMisuse, stderr.String())
	}
	if !strings.Contains(stderr.String(), "--id is only valid") {
		t.Fatalf("stderr should explain --id requires preset target; got %q", stderr.String())
	}
}
