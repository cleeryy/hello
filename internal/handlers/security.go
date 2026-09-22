package handlers

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	maxRequestBodyBytes = 1 << 20
	hstsHeader          = "max-age=31536000; includeSubDomains"
)

// securityHeaders sets baseline response headers on every request. HSTS is
// effective only when the public endpoint terminates TLS at the reverse proxy.
func securityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline' https://cdn.tailwindcss.com https://unpkg.com; script-src 'self' 'unsafe-inline' https://cdn.tailwindcss.com https://unpkg.com; img-src 'self' data:; connect-src 'self'")
		c.Header("Strict-Transport-Security", hstsHeader)
		c.Next()
	}
}

func requestBodyLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > maxRequestBodyBytes {
			writeProblem(c, http.StatusRequestEntityTooLarge, "payload too large",
				"request body exceeds the configured limit", nil)
			c.Abort()
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRequestBodyBytes)
		c.Next()
	}
}

// corsMiddleware enforces an exact origin whitelist. With no configured origins,
// only same-origin browser requests are allowed; cross-origin requests get 403.
func corsMiddleware(allowed []string) gin.HandlerFunc {
	whitelist := make(map[string]struct{}, len(allowed))
	for _, origin := range allowed {
		whitelist[origin] = struct{}{}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			c.Next()
			return
		}
		_, allowedOrigin := whitelist[origin]
		allowedOrigin = allowedOrigin || sameOrigin(c.Request, origin)
		c.Header("Vary", "Origin")
		if !allowedOrigin {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Credentials", "true")
		if c.Request.Method == http.MethodOptions {
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, Sec-WebSocket-Protocol")
			c.Header("Access-Control-Max-Age", "86400")
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func sameOrigin(r *http.Request, rawOrigin string) bool {
	origin, err := url.Parse(rawOrigin)
	if err != nil || origin.Host == "" {
		return false
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	} else if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); forwarded != "" {
		scheme = strings.TrimSpace(strings.Split(forwarded, ",")[0])
	}
	return origin.Scheme == scheme && origin.Host == r.Host
}
