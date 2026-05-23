package status

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

const defaultBackgroundTimeout = 30 * time.Second

// Options configures a Cache.
type Options struct {
	Now               func() time.Time
	BackgroundTimeout time.Duration
}

// Cache stores in-memory status entries with stale-while-revalidate
// semantics. It is safe for concurrent use.
type Cache struct {
	mu      sync.RWMutex
	entries map[string]entry
	group   singleflight.Group

	now               func() time.Time
	backgroundTimeout time.Duration
}

type entry struct {
	value     any
	expiresAt time.Time
	fetchedAt time.Time
}

// Metadata describes freshness of a cached value for dashboard badges.
type Metadata struct {
	IsStale   bool
	Age       time.Duration
	FetchedAt time.Time
	ExpiresAt time.Time
}

// NewCache returns an empty in-memory status cache.
func NewCache(opts Options) *Cache {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.BackgroundTimeout <= 0 {
		opts.BackgroundTimeout = defaultBackgroundTimeout
	}
	return &Cache{
		entries:           make(map[string]entry),
		now:               opts.Now,
		backgroundTimeout: opts.BackgroundTimeout,
	}
}

// GetOrFetch returns a fresh cached value, a stale cached value while
// refreshing it in the background, or a cold value fetched synchronously.
// It is a package function, not a generic method, because Go does not
// support generic methods on non-generic types.
func GetOrFetch[T any](
	cache *Cache,
	key string,
	ttl time.Duration,
	fetch func(context.Context) (T, error),
	ctx context.Context,
) (T, bool, error) {
	if cache == nil {
		cache = NewCache(Options{})
	}
	if ctx == nil {
		ctx = context.Background()
	}

	now := cache.now()
	if cached, ok := cache.lookup(key); ok {
		value, err := cast[T](cached.value, key)
		if err != nil {
			var zero T
			return zero, false, err
		}
		if now.Before(cached.expiresAt) || now.Equal(cached.expiresAt) {
			return value, false, nil
		}
		cache.refreshBackground(key, ttl, func(ctx context.Context) (any, error) {
			return fetch(ctx)
		})
		return value, true, nil
	}

	return fetchBlocking(cache, key, ttl, fetch, ctx)
}

// GetOrFetchMeta is the metadata-rich variant used by dashboard code.
func GetOrFetchMeta[T any](
	cache *Cache,
	key string,
	ttl time.Duration,
	fetch func(context.Context) (T, error),
	ctx context.Context,
) (T, Metadata, error) {
	if cache == nil {
		cache = NewCache(Options{})
	}
	if ctx == nil {
		ctx = context.Background()
	}

	now := cache.now()
	if cached, ok := cache.lookup(key); ok {
		value, err := cast[T](cached.value, key)
		if err != nil {
			var zero T
			return zero, Metadata{}, err
		}
		meta := metadataFor(cached, now)
		if meta.IsStale {
			cache.refreshBackground(key, ttl, func(ctx context.Context) (any, error) {
				return fetch(ctx)
			})
		}
		return value, meta, nil
	}

	value, _, err := fetchBlocking(cache, key, ttl, fetch, ctx)
	if err != nil {
		var zero T
		return zero, Metadata{}, err
	}
	cached, ok := cache.lookup(key)
	if !ok {
		return value, Metadata{}, nil
	}
	return value, metadataFor(cached, cache.now()), nil
}

func (c *Cache) lookup(key string) (entry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cached, ok := c.entries[key]
	return cached, ok
}

func fetchBlocking[T any](
	cache *Cache,
	key string,
	ttl time.Duration,
	fetch func(context.Context) (T, error),
	ctx context.Context,
) (T, bool, error) {
	value, err := doFetch(cache, key, ttl, fetch, ctx)
	return value, false, err
}

func (c *Cache) refreshBackground(key string, ttl time.Duration, fetch func(context.Context) (any, error)) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), c.backgroundTimeout)
		defer cancel()
		_, _ = doFetch(c, key, ttl, fetch, ctx)
	}()
}

func doFetch[T any](
	cache *Cache,
	key string,
	ttl time.Duration,
	fetch func(context.Context) (T, error),
	ctx context.Context,
) (T, error) {
	got, err, _ := cache.group.Do(key, func() (any, error) {
		value, err := fetch(ctx)
		if err != nil {
			return nil, err
		}
		now := cache.now()
		cache.mu.Lock()
		cache.entries[key] = entry{
			value:     value,
			fetchedAt: now,
			expiresAt: now.Add(ttl),
		}
		cache.mu.Unlock()
		return value, nil
	})
	if err != nil {
		var zero T
		return zero, err
	}
	return cast[T](got, key)
}

func cast[T any](value any, key string) (T, error) {
	typed, ok := value.(T)
	if !ok {
		var zero T
		return zero, fmt.Errorf("status: cache entry %q has unexpected type %T", key, value)
	}
	return typed, nil
}

func metadataFor(cached entry, now time.Time) Metadata {
	age := now.Sub(cached.fetchedAt)
	if age < 0 {
		age = 0
	}
	return Metadata{
		IsStale:   now.After(cached.expiresAt),
		Age:       age,
		FetchedAt: cached.fetchedAt,
		ExpiresAt: cached.expiresAt,
	}
}

// Invalidate removes all entries whose key begins with prefix and
// returns the number of removed entries.
func (c *Cache) Invalidate(prefix string) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	removed := 0
	for key := range c.entries {
		if strings.HasPrefix(key, prefix) {
			delete(c.entries, key)
			removed++
		}
	}
	return removed
}

// InvalidateEvent removes all cache prefixes affected by event.
func (c *Cache) InvalidateEvent(event Event) int {
	removed := 0
	for _, prefix := range PrefixesForEvent(event) {
		removed += c.Invalidate(prefix)
	}
	return removed
}
