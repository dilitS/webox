package presets

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/dilitS/webox/assets"
)

// schemaSource resolves the canonical JSON Schema (Draft 2020-12)
// describing a preset file. The schema is embedded at build time
// in the assets package so a Webox binary always validates against
// the schema it was compiled with — drift between binary and disk
// schema is impossible. Tests override this seam to point at
// fixture schemas.
var schemaSource = assets.ProviderPresetSchema

// schemaURI is the synthetic URI used when registering SchemaJSON
// with the jsonschema compiler. It must match the "$id" inside
// schema.json.
const schemaURI = "https://webox.dev/schema/provider-presets/v1.json"

// firstLineSplitN is the n argument for strings.SplitN used to keep
// the first newline-delimited segment of a validator message and
// drop the rest.
const firstLineSplitN = 2

// secretPattern pairs a regex with a human-readable label so error
// messages stay actionable when a malicious or accidental token
// slips into a preset.
type secretPattern struct {
	label string
	re    *regexp.Regexp
}

// secretPatterns mirrors config/schema.go's tripwire: any preset
// string matching one of these is rejected at load time. We keep
// the list locally instead of importing config/ to keep package
// dependencies one-way (presets/ depends on no Webox packages).
var secretPatterns = []secretPattern{
	{
		label: "github classic token",
		re:    regexp.MustCompile(`\bgh[ps]_[A-Za-z0-9]{36,255}\b`),
	},
	{
		label: "github fine-grained token",
		re:    regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{20,}\b`),
	},
	{
		label: "openai-style secret",
		re:    regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{16,}\b`),
	},
	{
		label: "private key block",
		re:    regexp.MustCompile(`(?s)-{5}BEGIN [A-Z ]+PRIVATE KEY-{5}`),
	},
	{
		label: "aws access key id",
		re:    regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),
	},
	{
		label: "ssh public key fingerprint",
		re:    regexp.MustCompile(`\bssh-(?:rsa|ed25519|dss|ecdsa)\s+[A-Za-z0-9+/=]{40,}`),
	},
}

// schemaInit caches the compiled schema. Compilation is not free
// (the validator parses the document, resolves $defs, and links
// format assertions) so we cache the result for every Validate
// call.
var (
	schemaOnce sync.Once
	schemaSrc  *jsonschema.Schema
	schemaErr  error
)

func compiledSchema() (*jsonschema.Schema, error) {
	schemaOnce.Do(func() {
		raw, err := schemaSource()
		if err != nil {
			schemaErr = fmt.Errorf("presets: read embedded schema: %w", err)
			return
		}
		var doc any
		if err := json.Unmarshal(raw, &doc); err != nil {
			schemaErr = fmt.Errorf("presets: parse embedded schema: %w", err)
			return
		}
		c := jsonschema.NewCompiler()
		c.AssertFormat()
		c.DefaultDraft(jsonschema.Draft2020)
		if err := c.AddResource(schemaURI, doc); err != nil {
			schemaErr = fmt.Errorf("presets: register embedded schema: %w", err)
			return
		}
		s, err := c.Compile(schemaURI)
		if err != nil {
			schemaErr = fmt.Errorf("presets: compile embedded schema: %w", err)
			return
		}
		schemaSrc = s
	})
	return schemaSrc, schemaErr
}

// ValidateRaw checks raw against the embedded JSON Schema and the
// secret tripwire. It returns one of: ErrInvalidJSON,
// ErrSchemaViolation, ErrSecretInPreset, or nil. Use [Parse] when
// the typed Preset value is also needed.
func ValidateRaw(raw []byte) error {
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidJSON, err)
	}

	s, err := compiledSchema()
	if err != nil {
		return err
	}
	if vErr := s.Validate(doc); vErr != nil {
		return fmt.Errorf("%w: %s", ErrSchemaViolation, summarise(vErr))
	}
	if err := walkStrings(doc, "$"); err != nil {
		return err
	}
	return nil
}

// Parse validates raw and, on success, returns a typed *Preset
// constructed from it. The Preset is decoded after schema
// validation so callers never observe a partially-trusted struct.
func Parse(raw []byte) (*Preset, error) {
	if err := ValidateRaw(raw); err != nil {
		return nil, err
	}
	var p Preset
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("%w: decode parsed payload: %w", ErrInvalidPreset, err)
	}
	if !p.Status.Valid() {
		// Defensive: should be unreachable because schema enum
		// already enforces this, but a typed Status is part of
		// the package contract.
		return nil, fmt.Errorf("%w: status %q", ErrSchemaViolation, p.Status)
	}
	return &p, nil
}

// summarise extracts a flat, lowercase, human-readable digest from
// a jsonschema.ValidationError. The library's default message
// lists every nested cause with indentation; for callers that want
// errors.Is matching plus a single-line preview, a flat join is
// friendlier.
func summarise(err error) string {
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		return strings.ToLower(err.Error())
	}
	parts := []string{}
	collect(ve, &parts)
	return strings.ToLower(strings.Join(parts, "; "))
}

func collect(ve *jsonschema.ValidationError, out *[]string) {
	if ve == nil {
		return
	}
	if msg := ve.Error(); msg != "" {
		*out = append(*out, strings.SplitN(msg, "\n", firstLineSplitN)[0])
	}
	for _, child := range ve.Causes {
		collect(child, out)
	}
}

// walkStrings descends through the parsed JSON tree and rejects any
// string value that matches a secretPattern. The path argument
// gives the JSON pointer of the offending field for actionable
// error messages.
func walkStrings(node any, path string) error {
	switch v := node.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if err := walkStrings(v[key], path+"."+key); err != nil {
				return err
			}
		}
	case []any:
		for idx, item := range v {
			if err := walkStrings(item, fmt.Sprintf("%s[%d]", path, idx)); err != nil {
				return err
			}
		}
	case string:
		for _, pattern := range secretPatterns {
			if pattern.re.MatchString(v) {
				return fmt.Errorf("%w: %s matches %s", ErrSecretInPreset, path, pattern.label)
			}
		}
	}
	return nil
}
