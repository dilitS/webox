// Package telemetry defines the local-only observability seam for
// Webox.
//
// MVP policy is strict: no remote telemetry, no auto-upload, no hidden
// network side effects. The package therefore ships only a disabled
// no-op sink that callers can depend on without introducing conditional
// logic. Future local metrics or opt-in bundle export flows extend this
// surface without changing the zero-remote-telemetry guarantee.
package telemetry
