package middleware

import "github.com/gin-gonic/gin"

// NoStore prevents browsers and intermediary proxies from reusing authenticated
// API responses. Management pages commonly reload a resource immediately after
// a mutation; serving a cached pre-mutation GET there makes a successful save
// appear to have been ignored and can also expose user-specific data from a
// shared cache.
func NoStore() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		c.Next()
	}
}
