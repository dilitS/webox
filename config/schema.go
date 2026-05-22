package config

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// firstLineSplitN is the n argument for strings.SplitN used to keep the
// first newline-delimited segment of a validator message and drop the rest.
const firstLineSplitN = 2

// SchemaJSON is the canonical JSON Schema (Draft 2020-12) describing
// config.json. It is embedded at build time so a Webox binary always
// validates against the schema it was compiled with — drift between
// binary and disk schema is impossible.
//
//go:embed schema.json
var SchemaJSON string

// schemaURI is the synthetic URI used when registering SchemaJSON with
// the jsonschema compiler. It must match the "$id" inside schema.json.
const schemaURI = "https://webox.dev/schema/config/v1.json"

type secretPattern struct {
	label string
	re    *regexp.Regexp
}

var (
	schemaOnce     sync.Once
	schemaSrc      *jsonschema.Schema
	schemaErr      error
	secretPatterns = []secretPattern{
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
	}
)

// compiledSchema lazily compiles SchemaJSON exactly once. Compilation is
// not free (the validator parses the document, resolves $defs, and links
// format assertions) so we cache the result for every Validate call.
func compiledSchema() (*jsonschema.Schema, error) {
	schemaOnce.Do(func() {
		var doc any
		if err := json.Unmarshal([]byte(SchemaJSON), &doc); err != nil {
			schemaErr = fmt.Errorf("config: parse embedded schema: %w", err)
			return
		}

		c := jsonschema.NewCompiler()
		c.AssertFormat()
		c.DefaultDraft(jsonschema.Draft2020)
		if err := c.AddResource(schemaURI, doc); err != nil {
			schemaErr = fmt.Errorf("config: register embedded schema: %w", err)
			return
		}
		s, err := c.Compile(schemaURI)
		if err != nil {
			schemaErr = fmt.Errorf("config: compile embedded schema: %w", err)
			return
		}
		schemaSrc = s
	})
	return schemaSrc, schemaErr
}

// Validate parses raw as JSON and rejects three classes of problems:
//
//   - errors.Is(err, ErrInvalidJSON) when raw is not well-formed JSON;
//   - errors.Is(err, ErrSchemaViolation) when raw is well-formed but
//     violates the schema (missing required field, wrong type, regex
//     mismatch, format failure, etc.);
//   - errors.Is(err, ErrSecretInConfig) when any string value looks like
//     a plaintext credential (`ghp_`, `ghs_`, `github_pat_`, `sk-`,
//     `BEGIN ... PRIVATE KEY`);
//   - errors.Is(err, ErrDanglingProfileAlias) when some
//     projects[].profile_alias references no profiles[].alias;
//   - nil when raw conforms to the schema.
//
// Validate intentionally works on the generic decoded JSON tree instead
// of the typed Config struct so it can enforce semantic guardrails
// before Load/Save materialise a partially trusted object.
func Validate(raw []byte) error {
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
	if err := validateNoSecrets(doc); err != nil {
		return err
	}
	if err := validateProfileAliasIntegrity(doc); err != nil {
		return err
	}
	return nil
}

// summarise extracts a flat, lowercase, human-readable digest from a
// jsonschema.ValidationError. The library's default message lists every
// nested cause with indentation; for callers that want errors.Is
// matching plus a single-line preview, a flat join is friendlier.
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

func validateNoSecrets(doc any) error {
	return walkStrings(doc, "$")
}

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
				return fmt.Errorf("%w: %s matches %s", ErrSecretInConfig, path, pattern.label)
			}
		}
	}
	return nil
}

func validateProfileAliasIntegrity(doc any) error {
	root, ok := doc.(map[string]any)
	if !ok {
		return nil
	}

	aliases := map[string]struct{}{}
	if profiles, ok := root["profiles"].([]any); ok {
		for _, rawProfile := range profiles {
			profile, ok := rawProfile.(map[string]any)
			if !ok {
				continue
			}
			alias, ok := profile["alias"].(string)
			if !ok {
				continue
			}
			aliases[alias] = struct{}{}
		}
	}

	if projects, ok := root["projects"].([]any); ok {
		for idx, rawProject := range projects {
			project, ok := rawProject.(map[string]any)
			if !ok {
				continue
			}
			alias, ok := project["profile_alias"].(string)
			if !ok {
				continue
			}
			if _, exists := aliases[alias]; !exists {
				return fmt.Errorf("%w: $.projects[%d].profile_alias=%q", ErrDanglingProfileAlias, idx, alias)
			}
		}
	}

	return nil
}
