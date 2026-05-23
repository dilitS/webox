package ssh

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	cryptossh "golang.org/x/crypto/ssh"
)

// Target identifies a single SSH endpoint. The same provider profile
// produces one Target per (host, port, user) tuple — alias is carried
// only for logging and pool-key stability. ProxyJump / bastion support
// is intentionally absent in v0.1 (see docs/DESIGN.md §5.1 STRETCH
// note); when added, it will live alongside Target without changing
// the pool's Acquire/Release shape.
type Target struct {
	Host  string
	Port  int
	User  string
	Alias string
}

// Addr renders the target in the host:port form expected by
// [net.Dialer.DialContext] and the host_key callback. IPv6 hosts are
// bracketed per RFC 3986 so consumers can hand the result straight to
// net.SplitHostPort without manual fixup.
func (t Target) Addr() string {
	return net.JoinHostPort(t.Host, strconv.Itoa(t.Port))
}

// Key is the stable identifier used by the connection pool to bucket
// reusable clients. Two targets that differ in port or user must NOT
// collide; this is asserted by tests because pool key collisions would
// silently leak credentials across profiles.
func (t Target) Key() string {
	return fmt.Sprintf("%s@%s:%d", t.User, t.Host, t.Port)
}

// ExecResult is the verbatim outcome of a remote command execution.
// Stdout and Stderr are captured separately because providers parse
// them differently — e.g. `devil` writes structured success lines to
// stdout but error notices to stderr. ExitCode is the SSH-reported
// command exit status; Duration measures wall time as observed by the
// caller (after dial+session-open overhead).
type ExecResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
	Duration time.Duration
}

// Clock is the seam the pool, keepalive ticker, and reconnect backoff
// use for all time-sensitive logic. The production wiring lives in
// [SystemClock]; tests substitute deterministic implementations so the
// suite can advance virtual time without touching [time.Sleep].
type Clock interface {
	Now() time.Time
}

// Dialer is the seam through which the pool establishes the underlying
// TCP+SSH connection. Production wires it to the net.Dialer +
// [cryptossh.NewClientConn] combo; tests substitute the in-process
// [testing/sshmock] server (Sprint 02 TASK-02.2).
type Dialer interface {
	Dial(ctx context.Context, target Target, config *cryptossh.ClientConfig) (*cryptossh.Client, error)
}

// HostKeyDB is the seam between [BuildClientConfig] and the actual
// known_hosts implementation. The production wiring (Sprint 02
// TASK-02.3) wraps [golang.org/x/crypto/ssh/knownhosts]; tests
// substitute deterministic stubs to exercise the unknown / mismatch /
// match branches without touching the filesystem.
type HostKeyDB interface {
	Check(hostname string, remote net.Addr, key cryptossh.PublicKey) error
}

// SystemClock is the trivial production [Clock] backed by [time.Now].
// Defined here so callers do not need to write their own one-line
// adapter and tests can match its signature exactly.
type SystemClock struct{}

// Now satisfies [Clock] for the system wall clock.
func (SystemClock) Now() time.Time { return time.Now() }
