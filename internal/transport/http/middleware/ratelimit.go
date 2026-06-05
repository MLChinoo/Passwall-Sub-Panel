package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// PerIPLimiter is a minimal fixed-window rate limiter keyed by client IP.
// Suitable for the project's friend-circle scale; swap for a token-bucket
// or a Redis-backed limiter if traffic grows.
type PerIPLimiter struct {
	mu        sync.Mutex
	buckets   map[string]*bucket
	limit     int
	limitFn   func() int // optional dynamic override; read per request (hot-reload)
	window    time.Duration
	lastSweep time.Time
}

type bucket struct {
	count  int
	expire time.Time
}

func NewPerIPLimiter(limitPerWindow int, window time.Duration) *PerIPLimiter {
	if limitPerWindow <= 0 {
		limitPerWindow = 1
	}
	return &PerIPLimiter{
		buckets: make(map[string]*bucket),
		limit:   limitPerWindow,
		window:  window,
	}
}

// SetLimitFunc installs a dynamic per-window limit source, read on every request
// so an admin changing the rate limit in settings takes effect WITHOUT a restart.
// A returned value <= 0 falls back to the static limit (a settings-load failure
// must never open the gate). Call once during router setup, before serving.
func (l *PerIPLimiter) SetLimitFunc(fn func() int) { l.limitFn = fn }

// effectiveLimit resolves the dynamic override (if any) ahead of the lock so a
// settings read never serializes other callers behind l.mu.
func (l *PerIPLimiter) effectiveLimit() int {
	if l.limitFn != nil {
		if v := l.limitFn(); v > 0 {
			return v
		}
	}
	return l.limit
}

// Allow returns true if the request from ip is within the limit.
func (l *PerIPLimiter) Allow(ip string) bool {
	limit := l.effectiveLimit()
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	l.sweepExpiredLocked(now)
	b, ok := l.buckets[ip]
	if !ok || now.After(b.expire) {
		l.buckets[ip] = &bucket{count: 1, expire: now.Add(l.window)}
		return true
	}
	if b.count >= limit {
		return false
	}
	b.count++
	return true
}

func (l *PerIPLimiter) sweepExpiredLocked(now time.Time) {
	if l.window <= 0 || now.Sub(l.lastSweep) < l.window {
		return
	}
	for ip, b := range l.buckets {
		if now.After(b.expire) {
			delete(l.buckets, ip)
		}
	}
	l.lastSweep = now
}

// Handler returns a Gin middleware that 429s requests above the limit.
func (l *PerIPLimiter) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !l.Allow(c.ClientIP()) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "Rate limit"})
			return
		}
		c.Next()
	}
}
