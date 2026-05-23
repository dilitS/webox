// Package httpcheck contains small network probes used by the dashboard
// status cache.
//
// The package intentionally stops at HTTP reachability and TLS
// certificate expiry. GitHub Actions status and provider-specific SSH
// probes live elsewhere so this package remains side-effect free and
// easy to exercise with httptest.
package httpcheck
