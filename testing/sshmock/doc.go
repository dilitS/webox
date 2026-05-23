// Package sshmock provides a small in-process SSH server for Webox
// integration tests.
//
// It is deliberately narrower than a real OpenSSH daemon: it accepts a
// generated per-test public key, supports session channels with exec
// requests, maps command strings to deterministic stdout/stderr/exit
// codes, and can inject disconnect / delay failures. That is enough for
// the ssh pool and provider parser tests without touching a real shared
// hosting account or shelling out to the system ssh binary.
package sshmock
