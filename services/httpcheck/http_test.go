package httpcheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProbeHTTP_StatusClassesAndLatency(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
		wantClass  string
	}{
		{"ok", http.StatusOK, "2xx"},
		{"redirect", http.StatusFound, "3xx"},
		{"server error", http.StatusInternalServerError, "5xx"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			t.Cleanup(server.Close)

			result, err := ProbeHTTP(context.Background(), server.URL, HTTPOptions{
				Timeout: time.Second,
			})
			if err != nil {
				t.Fatalf("ProbeHTTP: %v", err)
			}
			if result.StatusCode != tt.statusCode {
				t.Fatalf("StatusCode = %d, want %d", result.StatusCode, tt.statusCode)
			}
			if result.Class != tt.wantClass {
				t.Fatalf("Class = %q, want %q", result.Class, tt.wantClass)
			}
			if result.Latency <= 0 {
				t.Fatalf("Latency = %v, want positive", result.Latency)
			}
		})
	}
}

func TestProbeHTTP_Timeout(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	_, err := ProbeHTTP(context.Background(), server.URL, HTTPOptions{Timeout: 20 * time.Millisecond})
	if err == nil {
		t.Fatal("ProbeHTTP slow server returned nil error, want timeout")
	}
}

func TestProbeHTTP_DefaultTimeoutIsOneSecond(t *testing.T) {
	t.Parallel()

	opts := normalizeHTTPOptions(HTTPOptions{})
	if opts.Timeout != time.Second {
		t.Fatalf("default HTTP timeout = %v, want 1s", opts.Timeout)
	}
}
