// Package smallhost is the HostingProvider adapter for small.pl / Devil,
// the only provider shipped in Webox MVP (v0.1).
//
// All Devil interactions go through SSH using the shared connection pool
// from package ssh. Every command parser strips ANSI escapes, validates
// output size, and uses named regex groups; unknown shapes return
// providers.ErrUnknownOutputFormat. Golden fixtures used by the parser
// tests live under testing/fixtures/devil. See docs/providers/smallhost.md
// for the canonical command catalog and edge-case matrix.
package smallhost
