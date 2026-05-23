package httpcheck

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProbeTLS_ReturnsCertExpiryAndDaysLeft(t *testing.T) {
	t.Parallel()

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	hostPort := server.Listener.Addr().String()
	result, err := ProbeTLS(context.Background(), hostPort, TLSOptions{
		TLSConfig: &tls.Config{InsecureSkipVerify: true}, // local httptest self-signed cert
		Now:       time.Now,
	})
	if err != nil {
		t.Fatalf("ProbeTLS: %v", err)
	}
	if result.NotAfter.IsZero() {
		t.Fatal("NotAfter is zero")
	}
	if result.DaysLeft <= 0 {
		t.Fatalf("DaysLeft = %d, want positive", result.DaysLeft)
	}
}

func TestProbeTLS_ContextCancelled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := ProbeTLS(ctx, "127.0.0.1:443", TLSOptions{
		TLSConfig: &tls.Config{InsecureSkipVerify: true},
	})
	if err == nil {
		t.Fatal("ProbeTLS with cancelled context returned nil error")
	}
}

func TestProbeTLS_DefaultTimeoutIsOneSecond(t *testing.T) {
	t.Parallel()

	opts := normalizeTLSOptions(TLSOptions{})
	if opts.Timeout != time.Second {
		t.Fatalf("default TLS timeout = %v, want 1s", opts.Timeout)
	}
}

func TestProbeTLS_RejectsAddressWithoutPort(t *testing.T) {
	t.Parallel()

	_, err := ProbeTLS(context.Background(), "example.com", TLSOptions{
		Dialer: net.Dialer{Timeout: time.Millisecond},
	})
	if err == nil {
		t.Fatal("ProbeTLS address without port returned nil error")
	}
}
