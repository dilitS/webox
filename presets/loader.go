package presets

import (
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/dilitS/webox/assets"
)

// LoadResult bundles the outcome of loading a directory of preset
// files. Successfully-parsed presets land in Presets (sorted by id
// for deterministic iteration); files that failed validation are
// reported in Errors keyed by their relative filename. The loader
// never panics on a single bad preset — schema drift in one file
// must not take down the whole catalog.
type LoadResult struct {
	Presets []*Preset
	Errors  map[string]error
}

// Source abstracts the read-only filesystem the loader walks. The
// production source is assets.ProviderPresetsFS(); tests substitute
// fstest.MapFS instances to exercise edge cases (duplicate ids,
// invalid JSON, schema violations) without touching the real
// embedded catalog.
type Source interface {
	fs.FS
}

// loadDir reads all *.json files under dir on src (non-recursive),
// skipping schema.json and any files starting with `_`. Each file
// is validated through ValidateRaw + Parse; failures are
// accumulated in LoadResult.Errors keyed by their basename so
// callers can surface them in `webox doctor preset` without
// blocking the rest of the catalog.
//
// Two integrity invariants are enforced eagerly across the
// successful set:
//
//  1. No duplicate preset id. If two files declare the same id,
//     both are dropped from Presets and ErrDuplicateID is recorded
//     for the second-seen filename.
//  2. Filename must match the preset id (`<id>.json`). The
//     loader is strict here so a preset's id is always
//     discoverable from its filename — important for `webox
//     provider new` future tooling and for the Provider Catalog
//     UI.
func loadDir(src Source, dir string) (LoadResult, error) {
	entries, err := fs.ReadDir(src, dir)
	if err != nil {
		return LoadResult{}, fmt.Errorf("presets: read directory %q: %w", dir, err)
	}

	res := LoadResult{Errors: map[string]error{}}
	byID := map[string]string{}

	for _, entry := range entries {
		if !shouldLoadEntry(entry) {
			continue
		}
		full := path.Join(dir, entry.Name())
		raw, readErr := fs.ReadFile(src, full)
		if readErr != nil {
			res.Errors[entry.Name()] = fmt.Errorf("read: %w", readErr)
			continue
		}
		preset, parseErr := Parse(raw)
		if parseErr != nil {
			res.Errors[entry.Name()] = parseErr
			continue
		}
		expectedName := preset.ID + ".json"
		if entry.Name() != expectedName {
			res.Errors[entry.Name()] = fmt.Errorf(
				"%w: filename %q does not match preset id %q (expected %q)",
				ErrInvalidPreset, entry.Name(), preset.ID, expectedName,
			)
			continue
		}
		if existing, dup := byID[preset.ID]; dup {
			res.Errors[entry.Name()] = fmt.Errorf(
				"%w: id %q already declared by %q",
				ErrDuplicateID, preset.ID, existing,
			)
			continue
		}
		byID[preset.ID] = entry.Name()
		res.Presets = append(res.Presets, preset)
	}

	sort.Slice(res.Presets, func(i, j int) bool {
		return res.Presets[i].ID < res.Presets[j].ID
	})

	return res, nil
}

// shouldLoadEntry filters directory entries the way the loader
// expects: regular files only, *.json extension, skip schema.json,
// skip leading-underscore files (reserved for future _meta.json /
// _draft- files).
func shouldLoadEntry(entry fs.DirEntry) bool {
	if entry.IsDir() {
		return false
	}
	name := entry.Name()
	if name == "schema.json" {
		return false
	}
	if strings.HasPrefix(name, "_") {
		return false
	}
	return strings.HasSuffix(name, ".json")
}

// LoadEmbedded reads the canonical preset catalog shipped with the
// Webox binary. The catalog lives in
// assets/provider-presets/*.json and is embedded via
// [assets.ProviderPresetsFS]. Production code paths (Registry
// initialisation, `webox doctor preset`, TUI Provider Catalog) all
// route through this function.
//
// LoadEmbedded never returns an error directly — schema drift in
// a single preset is reported via the per-file Errors map of
// LoadResult so the rest of the catalog stays usable. A non-nil
// returned error is reserved for catastrophic failures (the
// embedded directory is unreadable).
func LoadEmbedded() (LoadResult, error) {
	fsys := assets.ProviderPresetsFS()
	return loadDir(fsys, "provider-presets")
}

// LoadFrom is the test seam for loadDir. Tests pass a custom
// fs.FS (e.g. fstest.MapFS) and the dir to read from. Production
// callers should use [LoadEmbedded].
func LoadFrom(src Source, dir string) (LoadResult, error) {
	return loadDir(src, dir)
}

// FormatLoadErrors renders Errors as a flat, lowercased list
// suitable for inclusion in a doctor report. The output is
// deterministic (sorted by filename) so snapshot tests stay
// stable.
func FormatLoadErrors(errs map[string]error) string {
	if len(errs) == 0 {
		return ""
	}
	names := make([]string, 0, len(errs))
	for name := range errs {
		names = append(names, name)
	}
	sort.Strings(names)
	var b strings.Builder
	for i, name := range names {
		if i > 0 {
			b.WriteString("; ")
		}
		fmt.Fprintf(&b, "%s: %s", name, strings.ToLower(errs[name].Error()))
	}
	return b.String()
}
