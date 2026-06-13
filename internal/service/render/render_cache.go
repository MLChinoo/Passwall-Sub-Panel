package render

import (
	"context"
	"sync"
	"time"

	"github.com/KazuhaHub/passwall-sub-panel/internal/domain"
)

// subRenderCacheTTL bounds how stale a /sub response may be. The polling fleet
// re-fetches an unchanged config on a timer (Cache-Control is no-cache, forcing
// revalidation), and the body-hash ETag means every poll otherwise pays a full
// render + group/node/separator/traffic reads. Caching the rendered Output for
// this window collapses repeat polls to a map lookup; a config change (nodes,
// group, template, settings, the user's UUID/token) propagates within ≤TTL,
// which is acceptable for subscription delivery. Errors are never cached.
const subRenderCacheTTL = 60 * time.Second

// renderCacheSweepThreshold is the entry count past which put() opportunistically
// drops expired entries, bounding memory under a churn of distinct keys without
// a background sweeper.
const renderCacheSweepThreshold = 256

// renderCacheKey scopes a cached render to one user AND one client type — two
// users (or two formats for one user) never share an entry, so there is no
// cross-tenant / cross-format leakage.
type renderCacheKey struct {
	userID int64
	ct     domain.ClientType
}

type renderCacheEntry struct {
	out     *Output
	expires time.Time
}

// renderCache is a small TTL cache for rendered /sub Outputs. The cached
// *Output is shared read-only across requests (the sub handler treats Output as
// immutable — it hashes/serves the body and copies headers, never mutates), so
// no per-request copy is needed.
type renderCache struct {
	mu  sync.Mutex
	m   map[renderCacheKey]renderCacheEntry
	ttl time.Duration
}

func newRenderCache(ttl time.Duration) *renderCache {
	return &renderCache{m: make(map[renderCacheKey]renderCacheEntry), ttl: ttl}
}

func (c *renderCache) get(key renderCacheKey, now time.Time) (*Output, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[key]
	if !ok || !now.Before(e.expires) {
		return nil, false
	}
	return e.out, true
}

func (c *renderCache) put(key renderCacheKey, out *Output, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.m) > renderCacheSweepThreshold {
		for k, e := range c.m {
			if !now.Before(e.expires) {
				delete(c.m, k)
			}
		}
	}
	c.m[key] = renderCacheEntry{out: out, expires: now.Add(c.ttl)}
}

func (c *renderCache) size() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.m)
}

// RenderForUserCached is the /sub entry point: it returns a cached render for
// (user, clientType) within subRenderCacheTTL and otherwise renders fresh via
// RenderForUser and caches the result. Concurrent misses for the same key may
// each render (idempotent; last write wins) — not worth single-flighting at
// this scale. Always-fresh callers must use RenderForUser directly.
func (s *Service) RenderForUserCached(ctx context.Context, u *domain.User, ct domain.ClientType) (*Output, error) {
	if ct == "" {
		ct = domain.ClientMihomo
	}
	key := renderCacheKey{userID: u.ID, ct: ct}
	now := s.now()
	if out, ok := s.cache.get(key, now); ok {
		return out, nil
	}
	out, err := s.RenderForUser(ctx, u, ct)
	if err != nil {
		return nil, err
	}
	s.cache.put(key, out, now)
	return out, nil
}
