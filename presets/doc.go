// Package presets implements Webox's provider preset registry.
//
// A preset is a host-specific configuration bundle that augments a
// hosting-panel adapter (provider_type) with concrete runtime, paths,
// restart strategy, SSL strategy, probe commands, known risks, and
// verification metadata. Presets answer the question:
//
//	"Webox knows your hosting, not just your panel."
//
// Architectural decisions (ADR-0008):
//
//   - Preset content is embedded into the binary via go:embed at
//     assets/provider-presets/*.json. There is no filesystem load
//     path, no URL load path, no plugin loader. This is a deliberate
//     supply-chain decision: every Webox release ships a known,
//     deterministic preset manifest, validated at build time by the
//     same JSON Schema enforcement chain CI runs locally.
//
//   - Validation reuses github.com/santhosh-tekuri/jsonschema/v6 with
//     the Draft 2020-12 schema in assets/provider-presets/schema.json,
//     mirroring config/schema.go. The validator additionally enforces
//     the same secret tripwire as config (no GitHub tokens, no
//     openai-style keys, no PEM private-key blocks) so a malicious PR
//     cannot smuggle credentials into a preset file.
//
//   - The runtime API is a singleton + DI seam: presets.Default()
//     returns a process-wide *Registry initialised lazily via
//     sync.Once for production call sites; presets.NewRegistry(loader)
//     constructs an isolated registry for tests. The registry is
//     read-only after initialisation.
//
// Preset capability matching (Match) returns a coarse fitness score
// based on declared capabilities; runtime probe execution (verifying
// the preset against a real account) lands with the cPanel adapter in
// Sprint 17/18 and is intentionally out of scope for v0.2 baseline.
//
// See:
//   - docs/providers/preconfiguration-vision.md (vision + format)
//   - docs/adr/0008-preset-registry.md (decision record)
//   - docs/contributing/PRESET.md (1-hour contributor walkthrough)
package presets
