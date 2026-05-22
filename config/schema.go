package config

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
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

var (
	schemaOnce sync.Once
	schemaSrc  *jsonschema.Schema
	schemaErr  error
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

// Validate parses raw as JSON and asserts it against the embedded
// config.json schema. It returns:
//
//   - errors.Is(err, ErrInvalidJSON) when raw is not well-formed JSON;
//   - errors.Is(err, ErrSchemaViolation) when raw is well-formed but
//     violates the schema (missing required field, wrong type, regex
//     mismatch, format failure, etc.);
//   - nil when raw conforms to the schema.
//
// Validate does NOT decode raw into a Go struct — that's [Decode]'s job.
// Splitting the two surfaces keeps schema feedback decoupled from any
// strict-JSON struct decoder we layer on top later.
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
