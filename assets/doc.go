// Package assets exposes the static files Webox embeds into its binary
// via //go:embed: the GitHub Actions deploy workflow template and any
// other read-only resources shipped with the TUI.
//
// Embedding (rather than fetching at runtime) is mandatory per
// docs/SECURITY.md §6 and docs/.cursor/rules/70-shell-and-workflow.mdc:
// it removes a network attack surface from the wizard and makes Webox
// reproducible offline.
package assets
