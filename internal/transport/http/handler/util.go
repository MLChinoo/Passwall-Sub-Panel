package handler

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// isHTTPS detects whether the request is over HTTPS, considering
// common proxy headers like X-Forwarded-Proto.
func isHTTPS(c *gin.Context) bool {
	// Check X-Forwarded-Proto header (common with reverse proxies)
	if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
		return strings.EqualFold(proto, "https")
	}
	// Check X-Forwarded-Ssl header
	if ssl := c.GetHeader("X-Forwarded-Ssl"); ssl != "" {
		return strings.EqualFold(ssl, "on")
	}
	// Check the request's TLS state
	return c.Request.TLS != nil
}

// sanitizeReturnTo validates and sanitizes the return_to parameter
// to prevent open redirect attacks. Only allows relative paths.
func sanitizeReturnTo(returnTo string, fallback string) string {
	if returnTo == "" {
		return fallback
	}
	// Must start with /
	if !strings.HasPrefix(returnTo, "/") {
		return fallback
	}
	// Must not contain protocol:// (prevents redirects to external sites)
	if strings.Contains(returnTo, "://") {
		return fallback
	}
	// Must not contain double slashes (prevents protocol-relative URLs)
	if strings.HasPrefix(returnTo, "//") {
		return fallback
	}
	return returnTo
}
