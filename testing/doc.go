// Package testing aggregates the test helpers shared across Webox
// packages: golden fixtures for provider parsers, an in-process SSH
// mock, GitHub API cassettes, and small assertion utilities.
//
// The directory shadows the standard library's testing package by
// design — callers that need both alias this one (typically as
// webtesting) when importing. See docs/TESTING.md §3 and §7 for the
// catalog and the fixture provenance policy.
package testing
