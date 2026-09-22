package handlers

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const wakeDiscoverCooldown = 30 * time.Second

type requestLimiter struct {
	mu       sync.Mutex
	last     map[rateKey]time.Time
	cooldown time.Duration
}

type rateKey struct {
	clientIP  string
	tokenHash [32]byte
}

func newRequestLimiter(cooldown time.Duration) *requestLimiter {
	return &requestLimiter{last: make(map[rateKey]time.Time), cooldown: cooldown}
}

func (l *requestLimiter) allow(identity authIdentity) (bool, time.Duration) {
	key := rateKey{clientIP: identity.ClientIP, tokenHash: identity.TokenHash}
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	if last, ok := l.last[key]; ok {
		if remaining := l.cooldown - now.Sub(last); remaining > 0 {
			return false, remaining
		}
		delete(l.last, key)
	}
	l.last[key] = now
	return true, 0
}

func throttle(limiter *requestLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		identity, _ := c.Get(authIdentityKey)
		authed, ok := identity.(authIdentity)
		if !ok {
			authed.ClientIP = c.ClientIP()
		}
		allowed, remaining := limiter.allow(authed)
		if !allowed {
			retryAfter := int((remaining + time.Second - 1) / time.Second)
			if retryAfter < 1 {
				retryAfter = 1
			}
			c.Header("Retry-After", fmt.Sprint(retryAfter))
			writeProblem(c, http.StatusTooManyRequests, "too many requests",
				"request cooldown is active", nil)
			c.Abort()
			return
		}
		c.Next()
	}
}
