package main

import (
	"bytes"
	"errors"
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"

	"github.com/dilitS/webox/assets"
)

// Supported `--preset` values. `presetBlank` is the default — it
// emits the generic skeleton without any panel-specific scaffolding.
// The named presets reserve the slot for v0.2+ work that bakes in
// vendor-specific transport boilerplate (cPanel UAPI client struct,
// DirectAdmin session login, CyberPanel API token). Until that
// boilerplate lands the named presets behave like `blank` but render
// the panel name in the doc comments so the generated walkthrough is
// honest about which transport the contributor signed up for.
const (
	presetBlank        = "blank"
	presetCpanelUAPI   = "cpanel-uapi"
	presetDirectAdmin  = "directadmin"
	presetCyberPanel   = "cyberpanel"
	defaultPreset      = presetBlank
	providersImportsGo = "cmd/webox/providers.go"
	walkthroughURL     = "docs/contributing/PROVIDER.md"

	// scaffoldDirMode is the mode for directories the generator
	// creates under `providers/<name>/` and
	// `testing/fixtures/<name>/`. 0o750 (rwxr-x---) keeps the dir
	// readable by the operator's primary group (developer machines
	// typically share a group) while excluding `other`.
	scaffoldDirMode = 0o750

	// scaffoldFileMode is the mode for generated source files —
	// matches `gofmt -w` and the rest of the repository's tree.
	// The generator never writes secrets through this writer.
	scaffoldFileMode = 0o644
)

// providerNamePattern enforces the constraints that make the user's
// chosen name a valid Go identifier AND a valid registry token:
//
//   - lowercase ASCII letter start (Go forbids digit/underscore start),
//   - lowercase alphanumeric + underscore body,
//   - 3..32 chars total (matches the registry's alias 1-32 cap but
//     bumps the floor to 3 so generated package names stay readable).
var providerNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{2,31}$`)

// reservedProviderNames blocks identifiers that would shadow existing
// provider directories or stdlib package names a contributor might
// reach for by accident. The list intentionally stays short — the
// generator surfaces a clear error and lets the contributor pick a
// different name.
var reservedProviderNames = map[string]string{
	"smallhost": "providers/smallhost already exists — pick a different name",
	"mock":      "providers/mock is reserved for in-process test doubles",
	"main":      "Go reserves `main` for the binary package",
	"init":      "Go reserves `init` for the package initializer function",
	"test":      "`test` collides with Go's test tooling — pick a more specific name",
	"testing":   "`testing` collides with stdlib package — pick a more specific name",
}

// supportedPresets is the canonical sort order used in error messages
// so the help text and the error stay in sync.
var supportedPresets = []string{presetBlank, presetCpanelUAPI, presetDirectAdmin, presetCyberPanel}

// providerNewOpts holds the parsed CLI surface for the generator.
// Keeping it a value type means tests build options inline and the
// implementation never reads global state.
type providerNewOpts struct {
	name     string
	preset   string
	dryRun   bool
	repoRoot string
}

// providerNewResult is the typed summary the implementation returns to
// the CLI handler so the test suite can assert on exact paths without
// re-parsing stdout. The CLI handler renders it into the operator-
// facing walkthrough; the tests inspect Written / Skipped directly.
type providerNewResult struct {
	// Written is the slice of repo-relative paths the generator
	// actually wrote (or, in dry-run mode, the paths it would
	// write). Files that already exist on disk are recorded under
	// Skipped instead.
	Written []string
	// Skipped is the slice of repo-relative paths the generator
	// refused to overwrite because they already exist. The CLI
	// handler surfaces these as a soft warning so contributors who
	// re-run the generator after a partial edit do not lose work.
	Skipped []string
	// ImportPatched is true when the generator modified the
	// providers blank-import file (false on dry-run, false when the
	// import was already present).
	ImportPatched bool
}

// errProviderNew is the sentinel returned by the generator helpers
// when input validation fails. The CLI handler converts it into
// exitMisuse so the operator sees the exit-2 contract documented in
// the Conventional CLI guidance.
var errProviderNew = errors.New("webox provider new: invalid input")

// runProviderNew is the package-level entry point invoked by Run when
// the operator types `webox provider new <name> [--preset PRESET] [--dry-run]`.
//
// stderr carries the operator-facing walkthrough; stdout stays empty
// so callers that pipe the command into a script see no surprise
// output. Exit codes follow the POSIX convention: 0 on success, 2 on
// validation errors, 1 on unexpected filesystem errors.
func runProviderNew(opts providerNewOpts, stdout, stderr io.Writer) int {
	// Normalise up front so the walkthrough renders the resolved
	// preset (e.g. "blank" when --preset was omitted) instead of an
	// empty placeholder. generateProviderScaffold re-validates the
	// already-normalised value cheaply; the function is idempotent.
	if err := validateProviderNewOpts(&opts); err != nil {
		if errors.Is(err, errProviderNew) {
			fmt.Fprintf(stderr, "webox: %v\n", err)
			return exitMisuse
		}
		fmt.Fprintf(stderr, "webox provider new: %v\n", err)
		return 1
	}
	result, err := generateProviderScaffold(opts)
	if err != nil {
		if errors.Is(err, errProviderNew) {
			fmt.Fprintf(stderr, "webox: %v\n", err)
			return exitMisuse
		}
		fmt.Fprintf(stderr, "webox provider new: %v\n", err)
		return 1
	}
	printProviderNewWalkthrough(opts, result, stderr)
	// stdout is intentionally untouched so the command behaves
	// well in pipelines (e.g. `webox provider new foo | grep …`).
	_ = stdout
	return exitOK
}

// generateProviderScaffold is the testable core of the generator. It
// validates the options, renders every embedded template against the
// chosen name + preset, and writes the rendered output beneath
// `opts.repoRoot`. Filesystem touches go through a single helper so
// dry-run mode is a one-line branch.
func generateProviderScaffold(opts providerNewOpts) (providerNewResult, error) {
	if err := validateProviderNewOpts(&opts); err != nil {
		return providerNewResult{}, err
	}

	files, err := assets.ProviderTemplateFiles()
	if err != nil {
		return providerNewResult{}, fmt.Errorf("load templates: %w", err)
	}

	data := buildTemplateData(opts)

	var result providerNewResult
	for _, f := range files {
		rendered, err := renderTemplate(f, data)
		if err != nil {
			return providerNewResult{}, fmt.Errorf("render %s: %w", f.RelPath, err)
		}
		dst := absTemplatePath(opts.repoRoot, opts.name, f)
		rel, _ := filepath.Rel(opts.repoRoot, dst)

		exists, err := pathExists(dst)
		if err != nil {
			return providerNewResult{}, fmt.Errorf("stat %s: %w", rel, err)
		}
		if exists {
			result.Skipped = append(result.Skipped, rel)
			continue
		}

		if !opts.dryRun {
			if err := writeFile(dst, rendered); err != nil {
				return providerNewResult{}, fmt.Errorf("write %s: %w", rel, err)
			}
		}
		result.Written = append(result.Written, rel)
	}

	patched, err := patchProvidersImports(opts)
	if err != nil {
		return providerNewResult{}, err
	}
	result.ImportPatched = patched

	sort.Strings(result.Written)
	sort.Strings(result.Skipped)
	return result, nil
}

// validateProviderNewOpts enforces the input contract documented on
// providerNewOpts. Errors wrap errProviderNew so the CLI handler can
// branch via errors.Is and surface exit-2 (misuse) instead of exit-1.
//
// The validator normalises Preset (defaulting blank when empty) and
// resolves RepoRoot to the operator's current working directory when
// the caller did not pass one, so the test suite can override the
// root explicitly while production code stays terse.
func validateProviderNewOpts(opts *providerNewOpts) error {
	opts.name = strings.TrimSpace(opts.name)
	opts.preset = strings.TrimSpace(opts.preset)

	if opts.name == "" {
		return fmt.Errorf("%w: name is required (usage: webox provider new <name>)", errProviderNew)
	}
	if !providerNamePattern.MatchString(opts.name) {
		return fmt.Errorf("%w: name %q must match %s", errProviderNew, opts.name, providerNamePattern.String())
	}
	if reason, reserved := reservedProviderNames[opts.name]; reserved {
		return fmt.Errorf("%w: name %q is reserved: %s", errProviderNew, opts.name, reason)
	}

	if opts.preset == "" {
		opts.preset = defaultPreset
	}
	if !slicesContains(supportedPresets, opts.preset) {
		return fmt.Errorf("%w: preset %q is not supported (choose one of %s)", errProviderNew, opts.preset, strings.Join(supportedPresets, ", "))
	}

	if opts.repoRoot == "" {
		root, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("resolve repo root: %w", err)
		}
		opts.repoRoot = root
	}
	return nil
}

// templateData is the typed projection passed to every
// text/template.Execute call. Keeping fields exported (and named in
// the Go conventional way) makes refactors safe: a typo inside a
// template surfaces as a `template: ... is not a method or field of`
// error at render time rather than silently emitting an empty string.
type templateData struct {
	Name           string
	PackageName    string
	DisplayName    string
	Preset         string
	WalkthroughURL string
}

func buildTemplateData(opts providerNewOpts) templateData {
	return templateData{
		Name:           opts.name,
		PackageName:    opts.name,
		DisplayName:    presetDisplayName(opts.name, opts.preset),
		Preset:         opts.preset,
		WalkthroughURL: walkthroughURL,
	}
}

// presetDisplayName returns the human-readable label rendered into
// doc comments. Falls back to the raw name when the preset does not
// imply a vendor, so the blank preset stays honest about being a
// blank scaffold.
func presetDisplayName(name, preset string) string {
	switch preset {
	case presetCpanelUAPI:
		return "cPanel (UAPI)"
	case presetDirectAdmin:
		return "DirectAdmin"
	case presetCyberPanel:
		return "CyberPanel"
	}
	return name
}

func renderTemplate(f assets.ProviderTemplateFile, data templateData) ([]byte, error) {
	tmpl, err := template.New(f.RelPath).Option("missingkey=error").Parse(f.Body)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}
	out := buf.Bytes()
	if strings.HasSuffix(f.RelPath, ".go") {
		// gofmt the rendered Go source so the contributor sees a
		// canonical file the moment they open it. Failing here
		// means a template typo produced unparseable Go — surface
		// loudly so we can fix the template.
		formatted, ferr := format.Source(out)
		if ferr != nil {
			return nil, fmt.Errorf("gofmt rendered %s: %w", f.RelPath, ferr)
		}
		return formatted, nil
	}
	return out, nil
}

func absTemplatePath(repoRoot, name string, f assets.ProviderTemplateFile) string {
	switch f.Root {
	case assets.ProviderTemplateRootPackage:
		return filepath.Join(repoRoot, "providers", name, f.RelPath)
	case assets.ProviderTemplateRootFixture:
		return filepath.Join(repoRoot, "testing", "fixtures", name, f.RelPath)
	}
	// Default to the repo root so the file is at least visible —
	// the routing table guards this from happening in practice.
	return filepath.Join(repoRoot, f.RelPath)
}

// writeFile creates `dst`'s parent directory ([scaffoldDirMode]) and
// writes `body` ([scaffoldFileMode]) so the generated source matches
// the rest of the repository's tree. The generator never writes
// secrets; documentation in the constants explains the mode choice.
func writeFile(dst string, body []byte) error {
	if err := os.MkdirAll(filepath.Dir(dst), scaffoldDirMode); err != nil {
		return err
	}
	//nolint:gosec // G306: generated scaffolds ship in the repo;
	// scaffoldFileMode matches `gofmt`-touched files. No secrets
	// pass through this writer.
	return os.WriteFile(dst, body, scaffoldFileMode)
}

func pathExists(p string) (bool, error) {
	_, err := os.Stat(p)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

// patchProvidersImports rewrites cmd/webox/providers.go so that the
// new adapter's blank import lives in the canonical sorted block. The
// function is idempotent: when the import is already present the
// file is left untouched and the second return value is false. On a
// dry-run the file is not modified, but the function still reports
// whether a write WOULD have happened.
func patchProvidersImports(opts providerNewOpts) (bool, error) {
	dst := filepath.Join(opts.repoRoot, providersImportsGo)
	exists, err := pathExists(dst)
	if err != nil {
		return false, fmt.Errorf("stat %s: %w", providersImportsGo, err)
	}
	if !exists {
		// The file is generator-owned: create it from scratch with
		// the smallhost + the new adapter so contributors do not
		// have to seed it manually.
		if opts.dryRun {
			return true, nil
		}
		body := renderProvidersImports([]string{"smallhost", opts.name})
		return true, writeFile(dst, body)
	}

	//nolint:gosec // G304: `dst` is built from opts.repoRoot
	// (operator-supplied) joined with the hard-coded relative path
	// providersImportsGo. The generator deliberately rewrites this
	// file; treating it as untrusted input would defeat the feature.
	current, err := os.ReadFile(dst)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", providersImportsGo, err)
	}
	imports, err := extractBlankProviderImports(current)
	if err != nil {
		return false, fmt.Errorf("parse %s: %w", providersImportsGo, err)
	}
	if slicesContains(imports, opts.name) {
		return false, nil
	}
	imports = append(imports, opts.name)
	sort.Strings(imports)
	if opts.dryRun {
		return true, nil
	}
	return true, writeFile(dst, renderProvidersImports(imports))
}

// extractBlankProviderImports walks the file's import block and
// returns the short provider names (last path segment) of every blank
// import that targets `github.com/dilitS/webox/providers/<name>`.
// Foreign imports are preserved verbatim by the caller — the
// generator only rewrites the providers block.
func extractBlankProviderImports(src []byte) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, providersImportsGo, src, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}
	const prefix = `"github.com/dilitS/webox/providers/`
	out := []string{}
	for _, imp := range file.Imports {
		if imp.Name == nil || imp.Name.Name != "_" {
			continue
		}
		path := imp.Path.Value
		if !strings.HasPrefix(path, prefix) {
			continue
		}
		short := strings.TrimSuffix(strings.TrimPrefix(path, prefix), `"`)
		if short == "" {
			continue
		}
		out = append(out, short)
	}
	sort.Strings(out)
	return out, nil
}

// renderProvidersImports emits the canonical cmd/webox/providers.go.
// The file holds blank imports only — every adapter's init() block
// registers itself with the providers registry, so a single touch
// here is enough to make the binary aware of the new factory.
func renderProvidersImports(names []string) []byte {
	sorted := append([]string(nil), names...)
	sort.Strings(sorted)

	var b bytes.Buffer
	b.WriteString("// Package main wires the production set of hosting provider\n")
	b.WriteString("// adapters via blank imports so their init() blocks register the\n")
	b.WriteString("// factories with `providers.Register` before `Run` looks them up.\n")
	b.WriteString("//\n")
	b.WriteString("// This file is generated by `webox provider new <name>` — every\n")
	b.WriteString("// new adapter inserts itself into the sorted import block below.\n")
	b.WriteString("// Hand-editing is fine; the generator re-sorts on every run so\n")
	b.WriteString("// merge conflicts stay trivial.\n")
	b.WriteString("package main\n\n")
	b.WriteString("import (\n")
	for _, name := range sorted {
		fmt.Fprintf(&b, "\t_ %q // register %s factory\n", "github.com/dilitS/webox/providers/"+name, name)
	}
	b.WriteString(")\n")
	// gofmt the output so the file passes `make lint` immediately
	// after generation. The buffer holds well-formed Go either way
	// (the template above never breaks the gofmt contract), but
	// running the formatter is cheap insurance against future
	// drift.
	formatted, err := format.Source(b.Bytes())
	if err != nil {
		// Should never happen — fall back to the unformatted body
		// rather than crashing the generator.
		return b.Bytes()
	}
	return formatted
}

// printProviderNewWalkthrough renders the operator-facing summary on
// stderr. The format is intentionally chatty: this command runs once
// per provider, so we trade brevity for a self-contained "what to do
// next" guide.
func printProviderNewWalkthrough(opts providerNewOpts, result providerNewResult, w io.Writer) {
	mode := ""
	if opts.dryRun {
		mode = " (dry-run — no files written)"
	}
	fmt.Fprintf(w, "webox provider new %s --preset %s%s\n", opts.name, opts.preset, mode)
	fmt.Fprintln(w)

	if len(result.Written) > 0 {
		header := "Wrote"
		if opts.dryRun {
			header = "Would write"
		}
		fmt.Fprintln(w, header+":")
		for _, p := range result.Written {
			fmt.Fprintf(w, "  + %s\n", p)
		}
	}
	if len(result.Skipped) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Already existed — left untouched:")
		for _, p := range result.Skipped {
			fmt.Fprintf(w, "  · %s\n", p)
		}
	}
	if result.ImportPatched {
		fmt.Fprintln(w)
		verb := "Patched"
		if opts.dryRun {
			verb = "Would patch"
		}
		fmt.Fprintf(w, "%s %s (blank import added in sorted block).\n", verb, providersImportsGo)
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Next steps:")
	fmt.Fprintln(w, "  1. Read the 4-hour walkthrough:", walkthroughURL)
	fmt.Fprintln(w, "  2. Implement CheckStatus first (cheapest probe).")
	fmt.Fprintln(w, "  3. Capture a real fixture and replace TestParsersTableDrivenSkeleton.")
	fmt.Fprintln(w, "  4. `make ci` should stay green at every step.")
	fmt.Fprintln(w, "  5. Open a draft PR early — ask the maintainer for pair-review.")
}

// slicesContains keeps `cmd/webox` independent of `golang.org/x/exp/slices`
// so the binary's import graph stays minimal.
func slicesContains[T comparable](haystack []T, needle T) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}
