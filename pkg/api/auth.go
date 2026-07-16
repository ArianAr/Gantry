package api

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// AuthConfig controls optional operator authentication.
// When Token is empty, authentication is disabled (open access).
type AuthConfig struct {
	// Token is the shared API token. Empty = auth disabled.
	Token string
	// TrustProxyHeaders when true accepts a non-empty Remote-User or
	// X-Remote-User header as authenticated (for reverse proxies).
	// Only use when Gantry is not directly exposed to untrusted clients.
	TrustProxyHeaders bool
}

// AuthEnabled reports whether any auth gate is active.
func (a AuthConfig) AuthEnabled() bool {
	return strings.TrimSpace(a.Token) != "" || a.TrustProxyHeaders
}

// Middleware returns a Gin handler that enforces AuthConfig.
// Always allows /healthz. When auth is disabled, all requests pass.
func (a AuthConfig) Middleware() gin.HandlerFunc {
	token := strings.TrimSpace(a.Token)
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		// Health and metrics stay open for probes / Prometheus scrapers.
		if path == "/healthz" || path == "/metrics" {
			c.Next()
			return
		}
		// Auth completely off
		if token == "" && !a.TrustProxyHeaders {
			c.Next()
			return
		}

		// Proxy identity (optional)
		if a.TrustProxyHeaders {
			if user := proxyUser(c); user != "" {
				c.Set("auth_user", user)
				c.Set("auth_method", "proxy")
				c.Next()
				return
			}
			// If only proxy mode (no token), reject missing identity
			if token == "" {
				abortUnauthorized(c, "missing reverse-proxy identity header")
				return
			}
		}

		// Shared token
		if token != "" {
			got := extractBearerOrAPIKey(c)
			if got != "" && subtle.ConstantTimeCompare([]byte(got), []byte(token)) == 1 {
				c.Set("auth_user", "token")
				c.Set("auth_method", "token")
				c.Next()
				return
			}
			abortUnauthorized(c, "invalid or missing credentials")
			return
		}

		c.Next()
	}
}

func proxyUser(c *gin.Context) string {
	for _, h := range []string{"Remote-User", "X-Remote-User", "X-Forwarded-User"} {
		if v := strings.TrimSpace(c.GetHeader(h)); v != "" {
			return v
		}
	}
	return ""
}

func extractBearerOrAPIKey(c *gin.Context) string {
	if h := c.GetHeader("Authorization"); h != "" {
		const p = "Bearer "
		if strings.HasPrefix(h, p) {
			return strings.TrimSpace(strings.TrimPrefix(h, p))
		}
		// Also accept raw token in Authorization for simple clients
		if !strings.Contains(h, " ") {
			return strings.TrimSpace(h)
		}
	}
	if k := c.GetHeader("X-API-Key"); k != "" {
		return strings.TrimSpace(k)
	}
	// Query param for EventSource (cannot set headers in browser EventSource)
	if q := c.Query("access_token"); q != "" {
		return strings.TrimSpace(q)
	}
	return ""
}

func abortUnauthorized(c *gin.Context, msg string) {
	c.Header("WWW-Authenticate", `Bearer realm="gantry"`)
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": msg})
}
