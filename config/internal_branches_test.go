package config

import (
	cryptorand "crypto/rand"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
)

func TestTempPath_RandomnessFailure(t *testing.T) {
	origReader := cryptorand.Reader
	cryptorand.Reader = failingReader{}
	t.Cleanup(func() { cryptorand.Reader = origReader })

	_, err := tempPath("/tmp/config.json")
	if err == nil {
		t.Fatal("tempPath(randomness failure) = nil, want error")
	}
	if !strings.Contains(err.Error(), "temp-path randomness") {
		t.Fatalf("tempPath(randomness failure) = %v, want temp-path randomness context", err)
	}
}

func TestCompiledSchema_ParseFailure(t *testing.T) {
	withSchemaOverride(t, "{")

	_, err := compiledSchema()
	if err == nil {
		t.Fatal("compiledSchema(parse failure) = nil, want error")
	}
	if !strings.Contains(err.Error(), "parse embedded schema") {
		t.Fatalf("compiledSchema(parse failure) = %v, want parse context", err)
	}
}

func TestCompiledSchema_CompileFailure(t *testing.T) {
	withSchemaOverride(t, `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://webox.dev/schema/config/v1.json",
  "type": "object",
  "properties": {
    "profiles": {
      "type": "array",
      "items": {
        "type": "not-a-real-json-schema-type"
      }
    }
  }
}`)

	_, err := compiledSchema()
	if err == nil {
		t.Fatal("compiledSchema(compile failure) = nil, want error")
	}
	if !strings.Contains(err.Error(), "compile embedded schema") {
		t.Fatalf("compiledSchema(compile failure) = %v, want compile context", err)
	}
}

func TestSummarise_GenericErrorLowercases(t *testing.T) {
	got := summarise(errors.New("BOOM"))
	if got != "boom" {
		t.Fatalf("summarise(generic error) = %q, want %q", got, "boom")
	}
}

func TestValidateProfileAliasIntegrity_NonObjectRoot(t *testing.T) {
	if err := validateProfileAliasIntegrity([]any{"not", "an", "object"}); err != nil {
		t.Fatalf("validateProfileAliasIntegrity(non-object root) = %v, want nil", err)
	}
}

func withSchemaOverride(t *testing.T, raw string) {
	t.Helper()

	origJSON := SchemaJSON
	SchemaJSON = raw
	schemaOnce = sync.Once{}
	schemaSrc = nil
	schemaErr = nil

	t.Cleanup(func() {
		SchemaJSON = origJSON
		schemaOnce = sync.Once{}
		schemaSrc = nil
		schemaErr = nil
	})
}

type failingReader struct{}

func (failingReader) Read(_ []byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}
