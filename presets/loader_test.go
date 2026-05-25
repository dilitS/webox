package presets_test

import (
	"errors"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/dilitS/webox/presets"
)

func cpanelGenericValid() string {
	return `{
  "schema_version": 1,
  "id": "cpanel-generic",
  "display_name": "Generic cPanel UAPI",
  "provider_type": "cpanel",
  "status": "research",
  "markets": ["global"],
  "panel": {
    "name": "cPanel",
    "api": "uapi",
    "api_port": 2083,
    "ssh_required": true
  },
  "capabilities": {
    "node_runtime": "passenger",
    "restart_method": "passenger",
    "ssl_provider": "autossl",
    "database_engines": ["mysql"],
    "git_available": true,
    "sftp_available": true,
    "logs_path_known": true,
    "safe_restart": true
  },
  "paths": {
    "deploy_path_template": "/home/{{user}}/{{app_root}}/public",
    "log_path_template": "/home/{{user}}/{{app_root}}/logs",
    "env_path_template": "/home/{{user}}/{{app_root}}/.env"
  },
  "probes": [
    "uapi --output=json Version get_version",
    "uapi --output=json PassengerApps list_applications"
  ],
  "known_risks": [
    "Application Manager may be disabled in WHM Feature Manager"
  ],
  "sources": [
    "https://docs.cpanel.net/cpanel/software/application-manager/"
  ],
  "verified": {}
}`
}

func TestLoadFromAcceptsValidCatalog(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"presets/schema.json":           {Data: []byte(`{"$schema":"x"}`)},
		"presets/smallhost-devil.json":  {Data: []byte(validPresetMinimal)},
		"presets/cpanel-generic.json":   {Data: []byte(cpanelGenericValid())},
		"presets/_draft-something.json": {Data: []byte("nope")},
		"presets/README.md":             {Data: []byte("# preset notes")},
		"presets/notes.txt":             {Data: []byte("ignored")},
		"presets/sub/another.json":      {Data: []byte(validPresetMinimal)},
	}
	res, err := presets.LoadFrom(fsys, "presets")
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}
	if got, want := len(res.Presets), 2; got != want {
		t.Fatalf("len(Presets) = %d, want %d (got: %v)", got, want, presetIDs(res.Presets))
	}
	if got := res.Presets[0].ID; got != "cpanel-generic" {
		t.Fatalf("Presets[0].ID = %q, want %q (sorted)", got, "cpanel-generic")
	}
	if got := res.Presets[1].ID; got != "smallhost-devil" {
		t.Fatalf("Presets[1].ID = %q, want %q (sorted)", got, "smallhost-devil")
	}
	if len(res.Errors) != 0 {
		t.Fatalf("len(Errors) = %d, want 0 (got: %v)", len(res.Errors), res.Errors)
	}
}

func TestLoadFromSkipsInvalidPresetButKeepsOthers(t *testing.T) {
	t.Parallel()

	bad := strings.Replace(validPresetMinimal, `"status": "verified"`, `"status": "experimental"`, 1)
	fsys := fstest.MapFS{
		"presets/smallhost-devil.json": {Data: []byte(validPresetMinimal)},
		"presets/cpanel-generic.json":  {Data: []byte(bad)},
	}
	res, err := presets.LoadFrom(fsys, "presets")
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}
	if got, want := len(res.Presets), 1; got != want {
		t.Fatalf("len(Presets) = %d, want %d (got %v)", got, want, presetIDs(res.Presets))
	}
	if res.Presets[0].ID != "smallhost-devil" {
		t.Fatalf("survivor = %q, want smallhost-devil", res.Presets[0].ID)
	}
	if got, want := len(res.Errors), 1; got != want {
		t.Fatalf("len(Errors) = %d, want %d", got, want)
	}
	if !errors.Is(res.Errors["cpanel-generic.json"], presets.ErrSchemaViolation) {
		t.Fatalf("Errors[cpanel-generic.json] = %v, want errors.Is(ErrSchemaViolation)", res.Errors["cpanel-generic.json"])
	}
}

func TestLoadFromRejectsFilenameMismatch(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"presets/smallhost-typo.json": {Data: []byte(validPresetMinimal)},
	}
	res, err := presets.LoadFrom(fsys, "presets")
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}
	if len(res.Presets) != 0 {
		t.Fatalf("len(Presets) = %d, want 0", len(res.Presets))
	}
	if !errors.Is(res.Errors["smallhost-typo.json"], presets.ErrInvalidPreset) {
		t.Fatalf("expected ErrInvalidPreset, got %v", res.Errors["smallhost-typo.json"])
	}
}

func TestLoadFromAcceptsTwoUnrelatedPresets(t *testing.T) {
	t.Parallel()

	matching := strings.Replace(validPresetMinimal,
		`"id": "smallhost-devil"`,
		`"id": "smallhost-devil-copy"`, 1)
	fsys := fstest.MapFS{
		"presets/smallhost-devil.json":      {Data: []byte(validPresetMinimal)},
		"presets/smallhost-devil-copy.json": {Data: []byte(matching)},
	}
	res, err := presets.LoadFrom(fsys, "presets")
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}
	if len(res.Presets) != 2 {
		t.Fatalf("expected 2 presets, got %d (errors: %v)", len(res.Presets), res.Errors)
	}
}

func TestLoadFromMissingDirectoryReturnsError(t *testing.T) {
	t.Parallel()

	_, err := presets.LoadFrom(fstest.MapFS{}, "nonexistent")
	if err == nil {
		t.Fatal("LoadFrom() error = nil, want error")
	}
}

func TestFormatLoadErrorsIsDeterministic(t *testing.T) {
	t.Parallel()

	errs := map[string]error{
		"zzz.json": presets.ErrInvalidPreset,
		"aaa.json": presets.ErrSchemaViolation,
		"mmm.json": presets.ErrSecretInPreset,
	}
	first := presets.FormatLoadErrors(errs)
	for i := 0; i < 10; i++ {
		got := presets.FormatLoadErrors(errs)
		if got != first {
			t.Fatalf("FormatLoadErrors(): non-deterministic (iter %d):\n got %q\nfirst %q", i, got, first)
		}
	}
	if !strings.HasPrefix(first, "aaa.json:") {
		t.Fatalf("FormatLoadErrors() should start with sorted-first filename, got %q", first)
	}
}

func TestLoadEmbeddedSucceeds(t *testing.T) {
	t.Parallel()

	res, err := presets.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v (catastrophic; embedded catalog must be loadable)", err)
	}
	if len(res.Errors) > 0 {
		t.Fatalf("LoadEmbedded() reported per-file errors: %v\n%s", res.Errors, presets.FormatLoadErrors(res.Errors))
	}
	if len(res.Presets) == 0 {
		t.Fatal("LoadEmbedded() returned 0 presets; expected at least the smallhost canonical preset")
	}
}

func presetIDs(in []*presets.Preset) []string {
	out := make([]string, 0, len(in))
	for _, p := range in {
		out = append(out, p.ID)
	}
	return out
}
