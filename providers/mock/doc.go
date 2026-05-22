// Package mock is an in-memory HostingProvider used by tests, the
// `--mock` debug flag, and the dry-run wizard mode.
//
// The mock records every call, lets tests inject errors, and never
// touches the network. It is the default provider in unit tests for
// tui, wizard, and status — see docs/TESTING.md §3 for usage patterns.
package mock
