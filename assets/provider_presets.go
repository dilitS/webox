package assets

import "embed"

// providerPresetsFS bundles the canonical Webox provider-preset
// catalog (schema + individual *.json presets) as build-time
// artefacts. Consumers in the presets/ package read from this
// embed.FS via [ProviderPresetSchema] and [ProviderPresetsFS].
//
// The embed pattern intentionally lists schema.json explicitly so
// any future addition of non-JSON sidecars (e.g. .md release notes)
// will not be picked up automatically — the loader only reads JSON
// and CI catches the drift if someone adds an unexpected file.
//
//go:embed provider-presets/schema.json
//go:embed provider-presets/*.json
var providerPresetsFS embed.FS

// ProviderPresetsFS exposes the embedded preset directory as a
// read-only embed.FS rooted at "provider-presets/". Callers walk
// it via fs.WalkDir or fs.Glob to discover *.json files.
//
// The returned filesystem includes schema.json — loaders are
// expected to skip it explicitly when listing presets.
func ProviderPresetsFS() embed.FS {
	return providerPresetsFS
}

// ProviderPresetSchema returns the embedded JSON Schema bytes for
// the v1 preset format. The validator in presets/ compiles this
// into a santhosh-tekuri/jsonschema *Schema once per process and
// reuses the result.
func ProviderPresetSchema() ([]byte, error) {
	return providerPresetsFS.ReadFile("provider-presets/schema.json")
}
