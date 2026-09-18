package handlers

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// requireToken locks routes behind Bearer auth. An empty expected token
// keeps open mode for local development. Browsers cannot set headers on a
// WebSocket handshake, so /ws additionally accepts ?token=.
func requireToken(expected string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if expected == "" {
			c.Next()
			return
		}
		got, ok := bearer(c.GetHeader("Authorization"))
		if !ok && c.FullPath() == "/ws" {
			got, ok = queryToken(c)
		}
		if !ok || subtle.ConstantTimeCompare([]byte(got), []byte(expected)) != 1 {
			c.Header("WWW-Authenticate", "Bearer")
			writeProblem(c, http.StatusUnauthorized, "unauthorized",
				"valid bearer token required", nil)
			c.Abort()
			return
		}
		c.Next()
	}
}

func bearer(header string) (string, bool) {
	token, ok := strings.CutPrefix(header, "Bearer ")
	if !ok || token == "" {
		return "", false
	}
	return token, true
}

func queryToken(c *gin.Context) (string, bool) {
	token := c.Query("token")
	if token == "" {
		return "", false
	}
	return token, true
}
