package config

import "errors"

// ErrInvalidJSON wraps a syntactic JSON failure (the document never even
// reaches schema validation). Callers can distinguish "your file is broken"
// from "your file is well-formed but does not match the contract" via
// errors.Is.
var ErrInvalidJSON = errors.New("config: invalid JSON")

// ErrSchemaViolation wraps any JSON Schema (Draft 2020-12) violation
// reported by the embedded schema in schema.json. The wrapped error string
// preserves the underlying validator's pointer/path so doctor and TUI can
// render actionable messages without parsing it back.
var ErrSchemaViolation = errors.New("config: schema violation")
