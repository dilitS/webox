// Package ssh wraps golang.org/x/crypto/ssh and pkg/sftp into a
// connection pool tuned for Webox dashboards.
//
// Connections are keyed by ProviderConfig (host+port+user+key
// fingerprint), reused across exec calls, and refreshed by a background
// keepalive ticker. SFTP transfers use the atomic put pattern (write
// to <file>.tmp, fsync, rename). Host-key verification is strict — a
// mismatch blocks until the user confirms the new fingerprint with an
// out-of-band phrase per docs/SECURITY.md §5. See docs/DESIGN.md §5
// for the pool contract.
package ssh
