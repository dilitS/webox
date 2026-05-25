package main

import (
	"bytes"
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateProviderNewOpts_Table(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		opts      providerNewOpts
		wantErr   bool
		wantName  string
		wantHint  string
		wantPrset string
	}{
		{
			name:      "blank name is rejected",
			opts:      providerNewOpts{name: ""},
			wantErr:   true,
			wantHint:  "name is required",
			wantPrset: "",
		},
		{
			name:     "uppercase name is rejected",
			opts:     providerNewOpts{name: "Cpanel"},
			wantErr:  true,
			wantHint: "must match",
		},
		{
			name:     "leading digit rejected",
			opts:     providerNewOpts{name: "1panel"},
			wantErr:  true,
			wantHint: "must match",
		},
		{
			name:     "too short rejected",
			opts:     providerNewOpts{name: "ab"},
			wantErr:  true,
			wantHint: "must match",
		},
		{
			name:     "reserved name rejected",
			opts:     providerNewOpts{name: "smallhost"},
			wantErr:  true,
			wantHint: "reserved",
		},
		{
			name:     "reserved go keyword rejected",
			opts:     providerNewOpts{name: "main"},
			wantErr:  true,
			wantHint: "reserved",
		},
		{
			name:     "unknown preset rejected",
			opts:     providerNewOpts{name: "cpanel", preset: "wat"},
			wantErr:  true,
			wantHint: "preset",
		},
		{
			name:      "valid name defaults preset to blank",
			opts:      providerNewOpts{name: "cpanel", repoRoot: t.TempDir()},
			wantErr:   false,
			wantName:  "cpanel",
			wantPrset: presetBlank,
		},
		{
			name:      "valid name + cpanel-uapi preset",
			opts:      providerNewOpts{name: "cpanel", preset: presetCpanelUAPI, repoRoot: t.TempDir()},
			wantErr:   false,
			wantName:  "cpanel",
			wantPrset: presetCpanelUAPI,
		},
		{
			name:      "trims whitespace",
			opts:      providerNewOpts{name: "  cyberpanel  ", preset: "  cyberpanel  ", repoRoot: t.TempDir()},
			wantErr:   false,
			wantName:  "cyberpanel",
			wantPrset: presetCyberPanel,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			opts := tc.opts
			err := validateProviderNewOpts(&opts)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !errors.Is(err, errProviderNew) {
					t.Errorf("err = %v, want errProviderNew wrap", err)
				}
				if tc.wantHint != "" && !strings.Contains(err.Error(), tc.wantHint) {
					t.Errorf("err = %v, want substring %q", err, tc.wantHint)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if opts.name != tc.wantName {
				t.Errorf("opts.name = %q, want %q", opts.name, tc.wantName)
			}
			if opts.preset != tc.wantPrset {
				t.Errorf("opts.preset = %q, want %q", opts.preset, tc.wantPrset)
			}
		})
	}
}

func TestGenerateProviderScaffold_WritesValidGoFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	result, err := generateProviderScaffold(providerNewOpts{
		name:     "cpaneltest",
		preset:   presetCpanelUAPI,
		repoRoot: root,
	})
	if err != nil {
		t.Fatalf("generateProviderScaffold err = %v", err)
	}
	if len(result.Written) == 0 {
		t.Fatal("Written is empty, expected scaffolded files")
	}
	if !result.ImportPatched {
		t.Error("ImportPatched = false, want true for fresh generation")
	}

	// Each generated *.go file must parse as valid Go.
	fset := token.NewFileSet()
	for _, rel := range result.Written {
		if !strings.HasSuffix(rel, ".go") {
			continue
		}
		abs := filepath.Join(root, rel)
		src, err := os.ReadFile(abs)
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		if _, err := parser.ParseFile(fset, rel, src, parser.AllErrors); err != nil {
			t.Errorf("generated %s does not parse: %v\n--- source ---\n%s", rel, err, src)
		}
		// Hard guarantee: the rendered display name must appear at
		// least once in the package files so the contributor sees the
		// preset they chose reflected in the doc comments.
		if strings.HasSuffix(rel, "doc.go") && !strings.Contains(string(src), "cPanel (UAPI)") {
			t.Errorf("doc.go missing cpanel-uapi display name; got:\n%s", src)
		}
	}

	// The fixture README must land under testing/fixtures/<name>/.
	fixturePath := filepath.Join(root, "testing", "fixtures", "cpaneltest", "README.md")
	if _, err := os.Stat(fixturePath); err != nil {
		t.Errorf("fixture README not generated at %s: %v", fixturePath, err)
	}

	// cmd/webox/providers.go must exist and contain the new blank import.
	importsPath := filepath.Join(root, providersImportsGo)
	body, err := os.ReadFile(importsPath)
	if err != nil {
		t.Fatalf("imports file not generated: %v", err)
	}
	if !bytes.Contains(body, []byte(`_ "github.com/dilitS/webox/providers/cpaneltest"`)) {
		t.Errorf("imports file missing cpaneltest blank import:\n%s", body)
	}
	if !bytes.Contains(body, []byte(`_ "github.com/dilitS/webox/providers/smallhost"`)) {
		t.Errorf("seeded smallhost blank import missing:\n%s", body)
	}
	// Sorted order: cpaneltest before smallhost (c < s).
	cIdx := bytes.Index(body, []byte("cpaneltest"))
	sIdx := bytes.Index(body, []byte("smallhost"))
	if cIdx == -1 || sIdx == -1 || cIdx > sIdx {
		t.Errorf("imports not sorted alphabetically: cpaneltest=%d smallhost=%d\n%s", cIdx, sIdx, body)
	}
}

func TestGenerateProviderScaffold_IsIdempotent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	opts := providerNewOpts{name: "ideopanel", repoRoot: root}

	first, err := generateProviderScaffold(opts)
	if err != nil {
		t.Fatalf("first run err = %v", err)
	}
	if !first.ImportPatched {
		t.Fatal("first run should patch imports")
	}

	second, err := generateProviderScaffold(opts)
	if err != nil {
		t.Fatalf("second run err = %v", err)
	}
	if len(second.Written) != 0 {
		t.Errorf("second run wrote %v, want nothing", second.Written)
	}
	if len(second.Skipped) == 0 {
		t.Error("second run skipped nothing — every file should be reported as already existing")
	}
	if second.ImportPatched {
		t.Error("second run patched imports; blank import should already be present")
	}
}

func TestGenerateProviderScaffold_DryRunDoesNotTouchDisk(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	result, err := generateProviderScaffold(providerNewOpts{
		name:     "drypanel",
		repoRoot: root,
		dryRun:   true,
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(result.Written) == 0 {
		t.Fatal("dry-run should still report would-write files")
	}
	for _, rel := range result.Written {
		if _, err := os.Stat(filepath.Join(root, rel)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("dry-run leaked file %s (stat err = %v)", rel, err)
		}
	}
	if !result.ImportPatched {
		t.Error("dry-run should report ImportPatched=true so the operator sees the planned change")
	}
	if _, err := os.Stat(filepath.Join(root, providersImportsGo)); !errors.Is(err, os.ErrNotExist) {
		t.Error("dry-run leaked imports file")
	}
}

func TestPatchProvidersImports_IdempotentAndSorted(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	importsDir := filepath.Join(root, filepath.Dir(providersImportsGo))
	if err := os.MkdirAll(importsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	seeded := []byte(`package main

import (
	_ "github.com/dilitS/webox/providers/smallhost" // register smallhost factory
	_ "github.com/dilitS/webox/providers/zeta"      // register zeta factory
)
`)
	if err := os.WriteFile(filepath.Join(root, providersImportsGo), seeded, 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	first, err := patchProvidersImports(providerNewOpts{name: "cpanel", repoRoot: root})
	if err != nil {
		t.Fatalf("first patch err = %v", err)
	}
	if !first {
		t.Fatal("first patch should report a change")
	}

	body, err := os.ReadFile(filepath.Join(root, providersImportsGo))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	// All three imports present, sorted (cpanel < smallhost < zeta).
	got := string(body)
	want := []string{"cpanel", "smallhost", "zeta"}
	last := -1
	for _, name := range want {
		idx := strings.Index(got, name)
		if idx == -1 {
			t.Fatalf("missing %q in patched body:\n%s", name, got)
		}
		if idx < last {
			t.Errorf("%q out of order in:\n%s", name, got)
		}
		last = idx
	}

	second, err := patchProvidersImports(providerNewOpts{name: "cpanel", repoRoot: root})
	if err != nil {
		t.Fatalf("second patch err = %v", err)
	}
	if second {
		t.Error("second patch should be no-op when import already exists")
	}
}

func TestExtractBlankProviderImports_IgnoresForeignImports(t *testing.T) {
	t.Parallel()

	src := []byte(`package main

import (
	"context"
	_ "embed"
	_ "github.com/dilitS/webox/providers/smallhost"
	_ "github.com/dilitS/webox/providers/cpanel"
	_ "github.com/some/other/lib"
)

var _ = context.Background
`)
	got, err := extractBlankProviderImports(src)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	want := []string{"cpanel", "smallhost"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestRenderProvidersImports_OutputIsGofmtClean(t *testing.T) {
	t.Parallel()

	body := renderProvidersImports([]string{"zeta", "alpha", "cpanel"})

	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, providersImportsGo, body, parser.AllErrors); err != nil {
		t.Fatalf("rendered file does not parse: %v\n%s", err, body)
	}

	got := string(body)
	// Sorted alphabetically.
	for _, name := range []string{"alpha", "cpanel", "zeta"} {
		if !strings.Contains(got, name) {
			t.Errorf("rendered output missing %q:\n%s", name, got)
		}
	}
	alphaIdx := strings.Index(got, "alpha")
	cpanelIdx := strings.Index(got, "cpanel")
	zetaIdx := strings.Index(got, "zeta")
	if alphaIdx >= cpanelIdx || cpanelIdx >= zetaIdx {
		t.Errorf("imports not sorted (alpha=%d cpanel=%d zeta=%d):\n%s", alphaIdx, cpanelIdx, zetaIdx, got)
	}
}

func TestRunProviderNew_HappyPathExitsZero(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	var stdout, stderr bytes.Buffer
	got := runProviderNew(providerNewOpts{
		name:     "happypanel",
		repoRoot: root,
	}, &stdout, &stderr)
	if got != exitOK {
		t.Fatalf("exit = %d, want %d; stderr=%q", got, exitOK, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout should stay empty for scripts; got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Next steps") {
		t.Errorf("walkthrough missing Next steps section: %q", stderr.String())
	}
}

func TestRunProviderNew_InvalidNameReturnsMisuse(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	var stdout, stderr bytes.Buffer
	got := runProviderNew(providerNewOpts{
		name:     "Bad-Name!",
		repoRoot: root,
	}, &stdout, &stderr)
	if got != exitMisuse {
		t.Fatalf("exit = %d, want %d", got, exitMisuse)
	}
	if !strings.Contains(stderr.String(), "must match") {
		t.Errorf("stderr missing 'must match' hint: %q", stderr.String())
	}
}

func TestParseArgs_ProviderNew_Table(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		args       []string
		wantErr    string
		wantName   string
		wantPreset string
		wantDryRun bool
	}{
		{
			name:     "basic provider new",
			args:     []string{"provider", "new", "cpanel"},
			wantName: "cpanel",
		},
		{
			name:       "with preset",
			args:       []string{"provider", "new", "cpanel", "--preset=cpanel-uapi"},
			wantName:   "cpanel",
			wantPreset: "cpanel-uapi",
		},
		{
			name:       "dry-run after name",
			args:       []string{"provider", "new", "cpanel", "--dry-run"},
			wantName:   "cpanel",
			wantDryRun: true,
		},
		{
			name:    "missing name",
			args:    []string{"provider", "new"},
			wantErr: "requires a name",
		},
		{
			name:    "new before provider is misuse",
			args:    []string{"new", "cpanel"},
			wantErr: "only valid after `provider`",
		},
		{
			name:    "preset without provider is misuse",
			args:    []string{"--preset=blank"},
			wantErr: "only valid with",
		},
		{
			name:    "dry-run without provider is misuse",
			args:    []string{"--dry-run"},
			wantErr: "only valid with",
		},
		{
			name:    "empty preset value is misuse",
			args:    []string{"provider", "new", "cpanel", "--preset="},
			wantErr: "--preset requires a value",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			parsed, errMsg := parseArgs(tc.args)
			if tc.wantErr != "" {
				if !strings.Contains(errMsg, tc.wantErr) {
					t.Fatalf("errMsg = %q, want substring %q", errMsg, tc.wantErr)
				}
				return
			}
			if errMsg != "" {
				t.Fatalf("unexpected errMsg = %q", errMsg)
			}
			if !parsed.providerNew {
				t.Error("providerNew should be true")
			}
			if parsed.providerNewName != tc.wantName {
				t.Errorf("name = %q, want %q", parsed.providerNewName, tc.wantName)
			}
			if parsed.providerPreset != tc.wantPreset {
				t.Errorf("preset = %q, want %q", parsed.providerPreset, tc.wantPreset)
			}
			if parsed.providerDryRun != tc.wantDryRun {
				t.Errorf("dryRun = %t, want %t", parsed.providerDryRun, tc.wantDryRun)
			}
		})
	}
}
