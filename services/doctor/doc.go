// Package doctor runs the non-invasive local diagnostics exposed through
// `webox doctor` and `webox doctor --json`.
//
// The package collects small, deterministic health checks for the local
// workstation or CI runner: Go runtime info, config-dir writability,
// secrets backend availability, encrypted-fallback file permissions, and
// the presence of an SSH agent socket. It produces both a machine-
// readable JSON report and a human-focused text rendering with stable
// exit codes (0 ok, 1 warn-only, 2 fail).
package doctor
