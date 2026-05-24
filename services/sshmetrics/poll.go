package sshmetrics

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dilitS/webox/status"
)

const (
	// DefaultCacheTTL is the freshness window the header tile relies
	// on. 5s matches `sprint-09-live-log-stream.md §TASK-09.5` —
	// matches Bento Ultra's heartbeat without bombarding the host
	// with SSH sessions.
	DefaultCacheTTL = 5 * time.Second

	// pingCommand is the cheapest payload we can run to measure RTT
	// — a no-op echo. Output is intentionally tiny so the round-trip
	// time approximates the SSH session establishment + a single
	// command exchange.
	pingCommand = "true"

	// uptimeCommand is the canonical uptime call. The output format
	// is documented in `parse_test.go`.
	uptimeCommand = "uptime"

	// freeCommand uses `free -m` for portability across distros.
	// FreeBSD does not ship `free`; on such hosts the runner is
	// expected to return an error which the caller surfaces as
	// "RAM: n/a" via [ErrFreeUnparseable].
	freeCommand = "free -m"
)

// CommandRunner is the seam Webox uses to execute one-shot SSH
// commands. Production wiring (`cmd/webox`) implements it against
// `ssh.Exec`; tests stub it directly.
type CommandRunner interface {
	Run(ctx context.Context, host, command string) (string, error)
}

// CommandRunnerFunc adapts a plain function to [CommandRunner].
type CommandRunnerFunc func(ctx context.Context, host, command string) (string, error)

// Run satisfies [CommandRunner].
func (f CommandRunnerFunc) Run(ctx context.Context, host, command string) (string, error) {
	return f(ctx, host, command)
}

// Metrics is the structured projection rendered by the header tile.
// All fields use safe zero values: callers can format unset metrics as
// "n/a" without nil checks.
type Metrics struct {
	ProfileAlias string
	Uptime       UptimeFacts
	Memory       MemoryFacts
	RTT          time.Duration
	FetchedAt    time.Time
}

// Profile is the minimal target descriptor (mirrors [sshtail.Profile]).
type Profile struct {
	Alias string
	Host  string
}

// Poller polls remote metrics through a [CommandRunner] and caches the
// result in the SWR `status.Cache` so the tile sees a sub-millisecond
// read path.
type Poller struct {
	runner CommandRunner
	cache  *status.Cache
	ttl    time.Duration
	now    func() time.Time
}

// New constructs a Poller. cache may be nil — the poller will create
// an in-memory one (useful for tests that don't care about persistence).
func New(runner CommandRunner, cache *status.Cache) *Poller {
	if runner == nil {
		panic("sshmetrics.New: runner is required")
	}
	if cache == nil {
		cache = status.NewCache(status.Options{})
	}
	return &Poller{
		runner: runner,
		cache:  cache,
		ttl:    DefaultCacheTTL,
		now:    time.Now,
	}
}

// WithTTL overrides [DefaultCacheTTL]. Mainly useful for tests that
// want to assert the SWR transitions deterministically.
func (p *Poller) WithTTL(ttl time.Duration) *Poller {
	if ttl > 0 {
		p.ttl = ttl
	}
	return p
}

// WithClock overrides the time source so tests can pin FetchedAt.
func (p *Poller) WithClock(now func() time.Time) *Poller {
	if now != nil {
		p.now = now
	}
	return p
}

// Poll fetches uptime + memory + RTT for profile. The result is
// cached for [DefaultCacheTTL]; in-flight duplicates are deduplicated
// by the status cache's singleflight group.
//
// Errors from individual commands degrade fields rather than failing
// the whole Poll: an unparseable `free` output keeps Uptime/RTT
// populated and zeroes Memory.
func (p *Poller) Poll(ctx context.Context, profile Profile) (Metrics, bool, error) {
	key := "ssh:metrics:" + profile.Alias
	return status.GetOrFetch(p.cache, key, p.ttl, func(ctx context.Context) (Metrics, error) {
		return p.fetch(ctx, profile)
	}, ctx)
}

func (p *Poller) fetch(ctx context.Context, profile Profile) (Metrics, error) {
	metrics := Metrics{ProfileAlias: profile.Alias, FetchedAt: p.now()}

	start := p.now()
	if _, err := p.runner.Run(ctx, profile.Host, pingCommand); err != nil {
		return metrics, fmt.Errorf("sshmetrics: ping: %w", err)
	}
	metrics.RTT = p.now().Sub(start)

	uptimeOut, err := p.runner.Run(ctx, profile.Host, uptimeCommand)
	if err != nil {
		return metrics, fmt.Errorf("sshmetrics: uptime: %w", err)
	}
	parsedUp, err := ParseUptime(uptimeOut)
	if err != nil && !errors.Is(err, ErrUptimeUnparseable) {
		return metrics, err
	}
	metrics.Uptime = parsedUp

	freeOut, err := p.runner.Run(ctx, profile.Host, freeCommand)
	if err == nil {
		if mem, perr := ParseFree(freeOut); perr == nil {
			metrics.Memory = mem
		}
	}

	return metrics, nil
}
