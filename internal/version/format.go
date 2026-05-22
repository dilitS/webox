package version

import "fmt"

// Default values returned when the corresponding ldflags-injected
// variable is empty (e.g. `go run` without `-ldflags`, or a developer
// build without the Makefile).
const (
	DefaultVersion = "v0.0.0-dev"
	DefaultCommit  = "unknown"
	DefaultDate    = "unknown"
)

// Build metadata populated at link time via:
//
//	go build -ldflags '-X github.com/webox/webox/internal/version.Version=v0.1.0 ...'
//
// Tests treat these as plain package-level vars and restore them with
// t.Cleanup. Production code reads them through String, never directly.
var (
	Version = ""
	Commit  = ""
	Date    = ""
)

// Format renders the canonical `webox <version> (<commit>) built <date>`
// line. Empty fields are substituted with the documented defaults so the
// helper produces a sensible string both with and without `-ldflags`.
func Format(version, commit, date string) string {
	if version == "" {
		version = DefaultVersion
	}
	if commit == "" {
		commit = DefaultCommit
	}
	if date == "" {
		date = DefaultDate
	}
	return fmt.Sprintf("webox %s (%s) built %s", version, commit, date)
}

// String returns the version line built from the package-level Version,
// Commit, and Date variables. It is the canonical caller for `webox
// --version`.
func String() string {
	return Format(Version, Commit, Date)
}
