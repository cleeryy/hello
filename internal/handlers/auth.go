package handlers

import (
	"crypto/sha256"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type authIdentity struct {
	ClientIP  string
	TokenHash [sha256.Size]byte
}

const authIdentityKey = "authenticatedIdentity"

// requireToken locks routes behind case-sensitive Bearer auth. WebSocket clients
// may use Sec-WebSocket-Protocol as a header-only fallback. The legacy query
// token remains temporarily available and is never written to logs.
func requireToken(expected string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if expected == "" {
			c.Next()
			return
		}

		got, ok := bearer(c.GetHeader("Authorization"))
		if !ok && c.FullPath() == "/ws" {
			if protocol, protocolOK := websocketProtocolToken(c.GetHeader("Sec-WebSocket-Protocol")); protocolOK {
				got, ok = protocol, true
			}
		}
		if !ok && c.FullPath() == "/ws" {
			if _, queryOK := queryToken(c); queryOK {
				slog.Warn("WebSocket query-token authentication is deprecated; use Authorization or Sec-WebSocket-Protocol")
				got, ok = queryToken(c)
			}
		}
		if !ok || subtle.ConstantTimeCompare([]byte(got), []byte(expected)) != 1 {
			c.Header("WWW-Authenticate", "Bearer")
			writeProblem(c, http.StatusUnauthorized, "unauthorized",
				"valid bearer token required", nil)
			c.Abort()
			return
		}

		c.Set(authIdentityKey, authIdentity{
			ClientIP:  c.ClientIP(),
			TokenHash: sha256.Sum256([]byte(expected)),
		})
		c.Next()
	}
}

func bearer(header string) (string, bool) {
	token, ok := strings.CutPrefix(header, "Bearer ")
	if !ok {
		return "", false
	}
	token = strings.TrimLeft(token, " \t")
	if token == "" || strings.ContainsAny(token, " \t") {
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

func websocketProtocolToken(header string) (string, bool) {
	for _, protocol := range strings.Split(header, ",") {
		if protocol = strings.TrimSpace(protocol); protocol != "" {
			return protocol, true
		}
	}
	return "", false
}
