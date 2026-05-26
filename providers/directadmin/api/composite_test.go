package api

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
)

// fakeReader is a minimal Reader implementation that returns
// canned responses for every method. Tests build two of these
// to exercise the composite's primary/secondary handling.
type fakeReader struct {
	whoamiErr     error
	whoami        *WhoamiResponse
	domainsErr    error
	domains       []Domain
	subdomainsErr error
	subdomains    []Subdomain
	databasesErr  error
	databases     []Database
	sslErr        error
	ssl           []SSLCertificate
	hits          atomic.Int32
}

func (f *fakeReader) Whoami(_ context.Context) (*WhoamiResponse, error) {
	f.hits.Add(1)
	return f.whoami, f.whoamiErr
}

func (f *fakeReader) ListDomains(_ context.Context) ([]Domain, error) {
	f.hits.Add(1)
	return f.domains, f.domainsErr
}

func (f *fakeReader) ListSubdomains(_ context.Context) ([]Subdomain, error) {
	f.hits.Add(1)
	return f.subdomains, f.subdomainsErr
}

func (f *fakeReader) ListDatabases(_ context.Context) ([]Database, error) {
	f.hits.Add(1)
	return f.databases, f.databasesErr
}

func (f *fakeReader) ListSSLCertificates(_ context.Context) ([]SSLCertificate, error) {
	f.hits.Add(1)
	return f.ssl, f.sslErr
}

func TestNewComposite_RejectsNilReaders(t *testing.T) {
	t.Parallel()
	_, err := NewComposite(nil, &fakeReader{})
	if !errors.Is(err, ErrCompositeRequiresBothReaders) {
		t.Fatalf("nil primary: got %v", err)
	}
	_, err = NewComposite(&fakeReader{}, nil)
	if !errors.Is(err, ErrCompositeRequiresBothReaders) {
		t.Fatalf("nil secondary: got %v", err)
	}
}

func TestComposite_PrefersPrimaryWhenItSucceeds(t *testing.T) {
	t.Parallel()
	primary := &fakeReader{whoami: &WhoamiResponse{Username: "primary"}}
	secondary := &fakeReader{whoami: &WhoamiResponse{Username: "secondary"}}
	c, _ := NewComposite(primary, secondary)

	got, err := c.Whoami(context.Background())
	if err != nil {
		t.Fatalf("Whoami: %v", err)
	}
	if got.Username != "primary" {
		t.Fatalf("expected primary, got %q", got.Username)
	}
	if primary.hits.Load() != 1 || secondary.hits.Load() != 0 {
		t.Fatalf("hit counts: primary=%d secondary=%d", primary.hits.Load(), secondary.hits.Load())
	}
}

func TestComposite_FailsOverOnTransportUnavailable(t *testing.T) {
	t.Parallel()
	primary := &fakeReader{whoamiErr: ErrTransportUnavailable}
	secondary := &fakeReader{whoami: &WhoamiResponse{Username: "fallback"}}
	c, _ := NewComposite(primary, secondary)

	got, err := c.Whoami(context.Background())
	if err != nil {
		t.Fatalf("Whoami: %v", err)
	}
	if got.Username != "fallback" {
		t.Fatalf("expected secondary response, got %q", got.Username)
	}
	if primary.hits.Load() != 1 || secondary.hits.Load() != 1 {
		t.Fatalf("each side should be hit once")
	}
}

func TestComposite_DoesNotFailOverOnTerminalErrors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		err  error
	}{
		{"auth", ErrAuthenticationFailed},
		{"rate-limit", ErrRateLimited},
		{"malformed", ErrMalformedResponse},
		{"api-disabled", ErrAPIDisabled},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			primary := &fakeReader{whoamiErr: tc.err}
			secondary := &fakeReader{whoami: &WhoamiResponse{Username: "should-not-be-hit"}}
			c, _ := NewComposite(primary, secondary)

			_, err := c.Whoami(context.Background())
			if !errors.Is(err, tc.err) {
				t.Fatalf("expected %v to surface verbatim, got %v", tc.err, err)
			}
			if secondary.hits.Load() != 0 {
				t.Fatalf("secondary was hit for terminal error %v", tc.err)
			}
		})
	}
}

func TestComposite_AllMethodsForwardToBothReaders(t *testing.T) {
	t.Parallel()
	primary := &fakeReader{
		whoamiErr:     ErrTransportUnavailable,
		domainsErr:    ErrTransportUnavailable,
		subdomainsErr: ErrTransportUnavailable,
		databasesErr:  ErrTransportUnavailable,
		sslErr:        ErrTransportUnavailable,
	}
	secondary := &fakeReader{
		whoami:     &WhoamiResponse{Username: "fb"},
		domains:    []Domain{{Name: "ok.example.com"}},
		subdomains: []Subdomain{{Name: "sub.ok.example.com"}},
		databases:  []Database{{Name: "ok_db"}},
		ssl:        []SSLCertificate{{Domain: "ok.example.com"}},
	}
	c, _ := NewComposite(primary, secondary)
	ctx := context.Background()

	if _, err := c.Whoami(ctx); err != nil {
		t.Fatalf("Whoami: %v", err)
	}
	if got, err := c.ListDomains(ctx); err != nil || len(got) != 1 {
		t.Fatalf("ListDomains: %v / %d", err, len(got))
	}
	if got, err := c.ListSubdomains(ctx); err != nil || len(got) != 1 {
		t.Fatalf("ListSubdomains: %v / %d", err, len(got))
	}
	if got, err := c.ListDatabases(ctx); err != nil || len(got) != 1 {
		t.Fatalf("ListDatabases: %v / %d", err, len(got))
	}
	if got, err := c.ListSSLCertificates(ctx); err != nil || len(got) != 1 {
		t.Fatalf("ListSSLCertificates: %v / %d", err, len(got))
	}
	// Each method hit primary once + secondary once = 10 total.
	if got := primary.hits.Load(); got != 5 {
		t.Errorf("primary hits = %d, want 5", got)
	}
	if got := secondary.hits.Load(); got != 5 {
		t.Errorf("secondary hits = %d, want 5", got)
	}
}

func TestShouldFailover_OnlyForTransportUnavailable(t *testing.T) {
	t.Parallel()
	transport := []error{
		ErrTransportUnavailable,
		errors.New("wrapped " + ErrTransportUnavailable.Error()), // not is-wrapped
	}
	if !shouldFailover(transport[0]) {
		t.Error("expected fail-over on ErrTransportUnavailable")
	}
	// The wrapped-string-only error MUST NOT trigger fail-over;
	// only typed errors.Is matches do.
	if shouldFailover(transport[1]) {
		t.Error("string-only match should not trigger fail-over")
	}

	terminal := []error{
		ErrAuthenticationFailed, ErrRateLimited, ErrServerError,
		ErrMalformedResponse, ErrAPIDisabled, ErrUnexpectedHTTPStatus,
		ErrInvalidEndpoint, ErrMissingCredentials, nil,
	}
	for _, err := range terminal {
		if shouldFailover(err) {
			t.Errorf("expected no fail-over for %v", err)
		}
	}
}
