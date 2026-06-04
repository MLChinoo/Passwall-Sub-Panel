package middleware

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
)

type proxyTrustKey struct{}

// ProxyTrust records, in the request context, whether this request's proxy
// headers (X-Forwarded-Host / -Proto) come from a TRUSTED proxy, so handler
// helpers that only hold an *http.Request (not the gin.Context) can gate on it
// via ProxyHeadersTrusted.
//
// Trust model mirrors the handler package's proxyHeadersTrustworthy: gin's
// ClientIP() resolves the client through the configured trusted_proxies list;
// if it differs from the raw TCP peer, at least one trusted hop sits in front,
// so the forwarded headers were set by infrastructure. If they're equal, no
// trusted proxy is involved and the headers are attacker-supplied — untrusted.
func ProxyTrust() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request != nil {
			raw := c.Request.RemoteAddr
			if i := strings.LastIndex(raw, ":"); i > 0 {
				raw = raw[:i]
			}
			raw = strings.Trim(raw, "[]")
			trusted := raw != c.ClientIP()
			c.Request = c.Request.WithContext(
				context.WithValue(c.Request.Context(), proxyTrustKey{}, trusted))
		}
		c.Next()
	}
}

// ProxyHeadersTrusted reports whether ProxyTrust marked this request's
// X-Forwarded-* headers as coming from a trusted proxy. Defaults to false when
// the middleware never ran (safe default: ignore the forwarded headers).
func ProxyHeadersTrusted(ctx context.Context) bool {
	v, _ := ctx.Value(proxyTrustKey{}).(bool)
	return v
}
