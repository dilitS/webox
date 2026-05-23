package ssh

import (
	"context"
	"net"
	"testing"
	"time"

	cryptossh "golang.org/x/crypto/ssh"
)

func TestTarget_AddrFormatsHostAndPort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		target Target
		want   string
	}{
		{"default port elided", Target{Host: "s1.small.pl", Port: 22, User: "u"}, "s1.small.pl:22"},
		{"explicit non-22 port", Target{Host: "10.0.0.1", Port: 2222, User: "u"}, "10.0.0.1:2222"},
		{"ipv6 bracketed", Target{Host: "::1", Port: 22, User: "u"}, "[::1]:22"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.target.Addr(); got != tt.want {
				t.Fatalf("Target%+v.Addr() = %q, want %q", tt.target, got, tt.want)
			}
		})
	}
}

func TestTarget_KeyIsStableAndPortSensitive(t *testing.T) {
	t.Parallel()

	a := Target{Host: "s1.small.pl", Port: 22, User: "u"}
	b := Target{Host: "s1.small.pl", Port: 22, User: "u"}
	c := Target{Host: "s1.small.pl", Port: 2222, User: "u"}
	d := Target{Host: "s1.small.pl", Port: 22, User: "other"}

	if a.Key() != b.Key() {
		t.Fatalf("equal targets must produce equal keys, got %q vs %q", a.Key(), b.Key())
	}
	if a.Key() == c.Key() {
		t.Fatalf("different ports must produce different keys, got %q for both", a.Key())
	}
	if a.Key() == d.Key() {
		t.Fatalf("different users must produce different keys, got %q for both", a.Key())
	}
}

func TestExecResult_ZeroValueIsSuccess(t *testing.T) {
	t.Parallel()

	var r ExecResult
	if r.ExitCode != 0 {
		t.Fatalf("zero ExecResult.ExitCode = %d, want 0", r.ExitCode)
	}
	if r.Stdout != nil || r.Stderr != nil {
		t.Fatalf("zero ExecResult should have nil Stdout/Stderr, got %v / %v", r.Stdout, r.Stderr)
	}
	if r.Duration != 0 {
		t.Fatalf("zero ExecResult.Duration = %v, want 0", r.Duration)
	}
}

func TestSeams_StubsSatisfyInterfaces(t *testing.T) {
	t.Parallel()

	var (
		_ Clock     = stubClock{}
		_ Dialer    = stubDialer{}
		_ HostKeyDB = stubHostKeyDB{}
	)
}

func TestSystemClock_NowReturnsRecentWallTime(t *testing.T) {
	t.Parallel()

	before := time.Now()
	got := SystemClock{}.Now()
	after := time.Now()

	if got.Before(before) || got.After(after) {
		t.Fatalf("SystemClock.Now() = %v, want between %v and %v", got, before, after)
	}
}

type stubClock struct{ now time.Time }

func (s stubClock) Now() time.Time { return s.now }

type stubDialer struct{}

func (stubDialer) Dial(_ context.Context, _ Target, _ *cryptossh.ClientConfig) (*cryptossh.Client, error) {
	return nil, nil //nolint:nilnil // stub satisfies the interface for compile-time check
}

type stubHostKeyDB struct{ err error }

func (s stubHostKeyDB) Check(_ string, _ net.Addr, _ cryptossh.PublicKey) error {
	return s.err
}
