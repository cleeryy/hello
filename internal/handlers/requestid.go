package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"regexp"

	"github.com/gin-gonic/gin"
)

// requestIDHeader is echoed on every response so clients can correlate
// logs, errors, and webhook deliveries with one trace.
const requestIDHeader = "X-Request-ID"

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// requestID keeps a valid incoming id and mints a random one otherwise.
func requestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(requestIDHeader)
		if !requestIDPattern.MatchString(id) {
			id = newRequestID()
		}
		c.Set(requestIDHeader, id)
		c.Header(requestIDHeader, id)
		c.Next()
	}
}

func newRequestID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(buf[:])
}
