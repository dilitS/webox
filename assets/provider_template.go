package assets

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"strings"
)

// ErrProviderTemplateRouting is the sentinel returned when the
// embedded provider-template directory contains a `.tmpl` file that
// is not listed in [providerTemplateRouting]. Surfacing it as a typed
// error (rather than panicking) gives the caller a deterministic
// branch in tests and lets the linter keep err113 green.
var ErrProviderTemplateRouting = errors.New("assets: provider template has no routing entry — extend providerTemplateRouting")

// providerTemplateFS embeds the canonical provider-adapter scaffold
// the `webox provider new` generator writes into providers/<name>/
// (and testing/fixtures/<name>/). The templates live as `.tmpl` files
// in repo so a contributor reading assets/provider-template/ sees
// readable, lint-friendly Go source instead of escaped string
// literals.
//
//go:embed provider-template/*.tmpl
var providerTemplateFS embed.FS

// ProviderTemplateFile is one rendered output of the scaffold. The
// generator iterates [ProviderTemplateFiles] (in the canonical sort
// order so output is deterministic across platforms) and writes each
// rendered body to RelPath underneath the package directory the
// caller chose.
//
// RelPath is the file path RELATIVE to providers/<name>/ for Go
// sources (`provider.go`, `doc.go`, `provider_test.go`, …) and
// RELATIVE to testing/fixtures/<name>/ for the fixture README
// (`README.md`). The two roots are kept separate so contributors
// can grep the fixture conventions next to other panel directories
// instead of inside the adapter package.
type ProviderTemplateFile struct {
	// RelPath is the on-disk path the rendered template lives at.
	// It excludes the .tmpl suffix so callers do not have to strip
	// it themselves.
	RelPath string

	// Root identifies which top-level directory the file lands in.
	// Two roots exist today:
	//
	//   - ProviderTemplateRootPackage → providers/<name>/
	//   - ProviderTemplateRootFixture → testing/fixtures/<name>/
	Root ProviderTemplateRoot

	// Body is the raw template source (Go text/template) the
	// generator passes through `text/template.Execute`. Caller
	// retains the raw bytes (no normalisation) so review diffs
	// stay grep-friendly.
	Body string
}

// ProviderTemplateRoot identifies which top-level directory the
// generator writes a rendered template to.
type ProviderTemplateRoot int

const (
	// ProviderTemplateRootPackage maps to providers/<name>/
	// (Go source files: doc.go, provider.go, parsers.go, …).
	ProviderTemplateRootPackage ProviderTemplateRoot = iota

	// ProviderTemplateRootFixture maps to testing/fixtures/<name>/
	// (currently just the per-directory README that documents the
	// fixture provenance + sanitisation policy).
	ProviderTemplateRootFixture
)

// String renders the root as a short token used in walkthrough
// messages so the generator can print "wrote providers/cpanel/doc.go"
// vs "wrote testing/fixtures/cpanel/README.md" without the caller
// open-coding the prefix.
func (r ProviderTemplateRoot) String() string {
	switch r {
	case ProviderTemplateRootPackage:
		return "providers"
	case ProviderTemplateRootFixture:
		return "testing/fixtures"
	default:
		return fmt.Sprintf("root(%d)", int(r))
	}
}

// providerTemplateRouting maps each embedded `<name>.tmpl` file to
// the (Root, RelPath) pair the generator writes it to. Keeping the
// routing in a single explicit table — instead of "fixture.* goes to
// fixtures/" pattern matching — makes the generator behaviour
// reviewable in one place and prevents accidental cross-routing when
// new templates land.
var providerTemplateRouting = map[string]struct {
	Root    ProviderTemplateRoot
	RelPath string
}{
	"provider-template/doc.go.tmpl":           {ProviderTemplateRootPackage, "doc.go"},
	"provider-template/provider.go.tmpl":      {ProviderTemplateRootPackage, "provider.go"},
	"provider-template/provider_test.go.tmpl": {ProviderTemplateRootPackage, "provider_test.go"},
	"provider-template/parsers.go.tmpl":       {ProviderTemplateRootPackage, "parsers.go"},
	"provider-template/parsers_test.go.tmpl":  {ProviderTemplateRootPackage, "parsers_test.go"},
	"provider-template/fixture.md.tmpl":       {ProviderTemplateRootFixture, "README.md"},
}

// ProviderTemplateFiles returns the embedded scaffold files in a
// stable order so the generator's output (and its diff in code
// review) is reproducible across runs and platforms. The returned
// slice is a fresh allocation — callers may sort, filter, or mutate
// it without affecting future calls.
func ProviderTemplateFiles() ([]ProviderTemplateFile, error) {
	entries, err := fs.ReadDir(providerTemplateFS, "provider-template")
	if err != nil {
		return nil, fmt.Errorf("assets: read provider template dir: %w", err)
	}
	out := make([]ProviderTemplateFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".tmpl") {
			continue
		}
		key := "provider-template/" + name
		route, ok := providerTemplateRouting[key]
		if !ok {
			return nil, fmt.Errorf("%w: %q", ErrProviderTemplateRouting, key)
		}
		body, err := providerTemplateFS.ReadFile(key)
		if err != nil {
			return nil, fmt.Errorf("assets: read provider template %q: %w", key, err)
		}
		out = append(out, ProviderTemplateFile{
			RelPath: route.RelPath,
			Root:    route.Root,
			Body:    string(body),
		})
	}
	// fs.ReadDir returns lexical order; explicitly enforce the
	// invariant via a second sort so future template additions do
	// not silently reorder existing output.
	sortProviderTemplateFiles(out)
	return out, nil
}

// sortProviderTemplateFiles orders package files before fixture files,
// then alphabetically by RelPath inside each bucket. The generator
// prints filenames in the same order so the walkthrough output reads
// top-to-bottom in the same direction a contributor browses
// `providers/<name>/` in their editor.
func sortProviderTemplateFiles(files []ProviderTemplateFile) {
	// Simple in-place insertion sort: N ≤ 10 so the algorithmic
	// choice does not matter — readability does.
	for i := 1; i < len(files); i++ {
		j := i
		for j > 0 && providerTemplateFileLess(files[j], files[j-1]) {
			files[j], files[j-1] = files[j-1], files[j]
			j--
		}
	}
}

func providerTemplateFileLess(a, b ProviderTemplateFile) bool {
	if a.Root != b.Root {
		return a.Root < b.Root
	}
	return a.RelPath < b.RelPath
}
