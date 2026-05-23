package status

import (
	"context"
	"testing"
	"time"
)

func TestGetOrFetchMeta_ReturnsAgeAndStaleMetadata(t *testing.T) {
	t.Parallel()

	clock := newManualClock(time.Date(2026, 5, 23, 2, 0, 0, 0, time.UTC))
	cache := NewCache(Options{Now: clock.Now})

	got, meta, err := GetOrFetchMeta[int](cache, "http:example.com", HTTPStatusTTL, func(context.Context) (int, error) {
		return 200, nil
	}, context.Background())
	if err != nil {
		t.Fatalf("GetOrFetchMeta initial: %v", err)
	}
	if got != 200 {
		t.Fatalf("got = %d, want 200", got)
	}
	if meta.IsStale {
		t.Fatalf("initial metadata IsStale = true, want false")
	}
	if meta.Age != 0 {
		t.Fatalf("initial Age = %v, want 0", meta.Age)
	}

	clock.Advance(HTTPStatusTTL + 5*time.Second)
	got, meta, err = GetOrFetchMeta[int](cache, "http:example.com", HTTPStatusTTL, func(context.Context) (int, error) {
		return 201, nil
	}, context.Background())
	if err != nil {
		t.Fatalf("GetOrFetchMeta stale: %v", err)
	}
	if got != 200 {
		t.Fatalf("stale got = %d, want old value 200", got)
	}
	if !meta.IsStale {
		t.Fatal("stale metadata IsStale = false, want true")
	}
	if meta.Age != HTTPStatusTTL+5*time.Second {
		t.Fatalf("stale Age = %v, want TTL+5s", meta.Age)
	}
}

func TestInvalidate_RemovesMatchingPrefixOnly(t *testing.T) {
	t.Parallel()

	cache := NewCache(Options{})
	seedCache(t, cache, "http:one", 200)
	seedCache(t, cache, "http:two", 201)
	seedCache(t, cache, "ssl:one", 30)

	removed := cache.Invalidate(PrefixHTTP)
	if removed != 2 {
		t.Fatalf("Invalidate(http:) removed %d, want 2", removed)
	}
	assertCacheMissThenFetch(t, cache, "http:one", 202)
	assertCacheHit(t, cache, "ssl:one", 30)
}

func TestTTLConstantsMatchADR0005(t *testing.T) {
	t.Parallel()

	if HTTPStatusTTL != 30*time.Second {
		t.Fatalf("HTTPStatusTTL = %v, want 30s", HTTPStatusTTL)
	}
	if SSHNodeTTL != time.Minute {
		t.Fatalf("SSHNodeTTL = %v, want 60s", SSHNodeTTL)
	}
	if SSLCertTTL != 5*time.Minute {
		t.Fatalf("SSLCertTTL = %v, want 300s", SSLCertTTL)
	}
	if GitHubLastDeployTTL != time.Minute {
		t.Fatalf("GitHubLastDeployTTL = %v, want 60s", GitHubLastDeployTTL)
	}
}

func TestInvalidateEventPrefixes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		event Event
		want  []string
	}{
		{EventRestart, []string{PrefixHTTP}},
		{EventDeploy, []string{PrefixHTTP, PrefixGitHubLastDeploy}},
		{EventSetupSSL, []string{PrefixSSL}},
		{EventRenewSSL, []string{PrefixSSL}},
		{EventChangeNodeVersion, []string{PrefixSSHNode}},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(string(tt.event), func(t *testing.T) {
			t.Parallel()
			got := PrefixesForEvent(tt.event)
			if len(got) != len(tt.want) {
				t.Fatalf("PrefixesForEvent(%q) = %v, want %v", tt.event, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("PrefixesForEvent(%q) = %v, want %v", tt.event, got, tt.want)
				}
			}
		})
	}
}

func TestInvalidateEvent_RemovesExpectedKeys(t *testing.T) {
	t.Parallel()

	cache := NewCache(Options{})
	seedCache(t, cache, "http:example.com", 200)
	seedCache(t, cache, "gh:lastDeploy:dilitS/webox", 1)
	seedCache(t, cache, "ssl:example.com", 30)

	removed := cache.InvalidateEvent(EventDeploy)
	if removed != 2 {
		t.Fatalf("InvalidateEvent(Deploy) removed %d, want 2", removed)
	}
	assertCacheMissThenFetch(t, cache, "http:example.com", 201)
	assertCacheMissThenFetch(t, cache, "gh:lastDeploy:dilitS/webox", 2)
	assertCacheHit(t, cache, "ssl:example.com", 30)
}

func seedCache[T comparable](t *testing.T, cache *Cache, key string, value T) {
	t.Helper()
	_, _, err := GetOrFetch[T](cache, key, time.Minute, func(context.Context) (T, error) {
		return value, nil
	}, context.Background())
	if err != nil {
		t.Fatalf("seed %s: %v", key, err)
	}
}

func assertCacheHit[T comparable](t *testing.T, cache *Cache, key string, want T) {
	t.Helper()
	got, stale, err := GetOrFetch[T](cache, key, time.Minute, func(context.Context) (T, error) {
		t.Fatalf("fresh key %s should not call fetch", key)
		var zero T
		return zero, nil
	}, context.Background())
	if err != nil {
		t.Fatalf("GetOrFetch(%s): %v", key, err)
	}
	if stale || got != want {
		t.Fatalf("GetOrFetch(%s) = (%v, stale=%t), want (%v, false)", key, got, stale, want)
	}
}

func assertCacheMissThenFetch[T comparable](t *testing.T, cache *Cache, key string, want T) {
	t.Helper()
	got, stale, err := GetOrFetch[T](cache, key, time.Minute, func(context.Context) (T, error) {
		return want, nil
	}, context.Background())
	if err != nil {
		t.Fatalf("GetOrFetch(%s): %v", key, err)
	}
	if stale || got != want {
		t.Fatalf("GetOrFetch(%s) = (%v, stale=%t), want (%v, false)", key, got, stale, want)
	}
}
