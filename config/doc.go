// Package config owns the on-disk Webox state stored in
// $XDG_CONFIG_HOME/webox/config.json.
//
// Reads validate the JSON Schema (Draft 2020-12) and run any pending
// migrations. Writes are atomic: an exclusive flock(2) is acquired on
// <path>.lock, the document is marshalled, written to a per-process
// temp file, fsync'd, renamed over the target, and the parent directory
// is fsync'd before the lock is released. Schema migrations live in
// migrate*.go and run forward only. See docs/DESIGN.md §6 for the
// model and §6.4 for the migration framework.
package config
