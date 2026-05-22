// Package version exposes the embedded build metadata produced by the
// release toolchain (semver tag, git commit short SHA, build date).
//
// Values are populated through `go build -ldflags '-X .../version.X=...'`
// in the Makefile and GoReleaser config. They are read-only from runtime
// code; tests use Format helpers to assert the rendered string used by
// `webox --version`.
package version
