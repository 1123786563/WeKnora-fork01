package skillhub

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// Cache semantics (M4 market plan): TTL 300s by default, singleflight so a
// burst of callers triggers exactly one upstream fetch, and stale-fallback
// so a registry outage degrades to the last good value instead of an error.
var (
	// ErrUnreachable: the upstream fetch failed and no cached value exists.
	ErrUnreachable = errors.New("skillhub: upstream unreachable and no cached value available")
	// ErrStaleOnly: the upstream refresh failed and the last good value is
	// being served alongside this marker error. Both return values are
	// meaningful: callers that can tolerate staleness use the value.
	ErrStaleOnly = errors.New("skillhub: upstream failed, serving stale cached value")
)

// DefaultCacheTTL is the recommended freshness window for marketplace
// listings (300 seconds).
const DefaultCacheTTL = 300 * time.Second

// cacheEntry is one cached value with its fetch time.
type cacheEntry struct {
	value     any
	fetchedAt time.Time
}

// CachedClient wraps an inner Client with a per-key TTL cache, request
// deduplication and stale-fallback. It implements Client and, when the inner
// client also implements SkillsetClient, the skillset index surface with the
// same semantics.
//
// Download deliberately bypasses the cache (M4 hand-off ruling): package
// payloads are 32 MiB zips, so caching them would pin hundreds of megabytes
// in the entries map and stale-fallback would resurrect outdated packages
// after an outage. Every Download fetches fresh from the inner client.
type CachedClient struct {
	inner Client
	ttl   time.Duration
	group singleflight.Group

	// skillsets is inner narrowed to SkillsetClient when it implements it;
	// nil means the skillset operations are unavailable (ErrNoSkillsetClient).
	skillsets SkillsetClient

	mu      sync.Mutex
	entries map[string]*cacheEntry

	// now is injectable for tests.
	now func() time.Time
}

// NewCached wraps inner with the given freshness TTL. A non-positive TTL
// disables fresh hits (every call goes upstream, still deduplicated).
func NewCached(inner Client, ttl time.Duration) *CachedClient {
	return newCached(inner, ttl, time.Now)
}

func newCached(inner Client, ttl time.Duration, now func() time.Time) *CachedClient {
	c := &CachedClient{
		inner:   inner,
		ttl:     ttl,
		entries: make(map[string]*cacheEntry),
		now:     now,
	}
	if skillsets, ok := inner.(SkillsetClient); ok {
		c.skillsets = skillsets
	}
	return c
}

// Search implements Client.
func (c *CachedClient) Search(ctx context.Context, query string, limit int) ([]SkillSummary, error) {
	key := cacheKey("search", query, fmt.Sprintf("%d", limit))
	value, err := c.do(ctx, key, func() (any, error) {
		return c.inner.Search(ctx, query, limit)
	})
	if value == nil {
		return nil, err
	}
	results, _ := value.([]SkillSummary)
	return results, err
}

// Rankings implements Client. The kind enum is validated before the cache
// so an invalid kind never populates an entry.
func (c *CachedClient) Rankings(ctx context.Context, kind string) ([]SkillSummary, error) {
	if _, ok := rankingEndpoints[kind]; !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedRanking, kind)
	}
	key := cacheKey("rankings", kind)
	value, err := c.do(ctx, key, func() (any, error) {
		return c.inner.Rankings(ctx, kind)
	})
	if value == nil {
		return nil, err
	}
	results, _ := value.([]SkillSummary)
	return results, err
}

// Download implements Client. It bypasses the cache entirely: every call
// fetches fresh from the inner client, and errors propagate unwrapped (no
// ErrStaleOnly / ErrUnreachable markers — see the CachedClient doc comment).
func (c *CachedClient) Download(ctx context.Context, slug string) ([]byte, error) {
	return c.inner.Download(ctx, slug)
}

// do is the shared cache flow: fresh hit, deduplicated upstream fetch, then
// stale-fallback on failure. The returned error wraps ErrStaleOnly or
// ErrUnreachable (with the underlying error chain intact) whenever the
// upstream fetch failed.
func (c *CachedClient) do(ctx context.Context, key string, fetch func() (any, error)) (any, error) {
	if value, ok := c.freshValue(key); ok {
		return value, nil
	}

	value, err, _ := c.group.Do(key, func() (any, error) {
		// Re-check under the flight: a predecessor may have refreshed the
		// entry while this caller was queueing.
		if value, ok := c.freshValue(key); ok {
			return value, nil
		}
		value, fetchErr := fetch()
		if fetchErr == nil {
			c.mu.Lock()
			c.entries[key] = &cacheEntry{value: value, fetchedAt: c.now()}
			c.mu.Unlock()
		}
		return value, fetchErr
	})
	if err == nil {
		return value, nil
	}

	// Upstream failed. Serve the last good value with the stale marker when
	// one exists, otherwise surface the typed unreachable error.
	c.mu.Lock()
	entry := c.entries[key]
	c.mu.Unlock()
	if entry != nil {
		return entry.value, fmt.Errorf("%w: %w", ErrStaleOnly, err)
	}
	return nil, fmt.Errorf("%w: %w", ErrUnreachable, err)
}

// freshValue returns the cached value when it is still inside the TTL.
func (c *CachedClient) freshValue(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok || c.ttl <= 0 || c.now().Sub(entry.fetchedAt) >= c.ttl {
		return nil, false
	}
	return entry.value, true
}

// cacheKey builds a collision-free cache key from the operation and its
// arguments.
func cacheKey(parts ...string) string {
	key := ""
	for i, part := range parts {
		if i > 0 {
			key += "\x00"
		}
		key += part
	}
	return key
}
