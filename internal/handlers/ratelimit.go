package handlers

import (
	"sync"
	"time"
)

// rateLimiter is a minimal in-memory sliding-window limiter per key (IP/token).
// No external dep to keep v1.1 lean. Not distributed: mono-instance only.
type rateLimiter struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	limit  int
	window time.Duration
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{hits: make(map[string][]time.Time), limit: limit, window: window}
}

// allow reports whether key may proceed, recording the hit when allowed.
func (l *rateLimiter) allow(key string) bool {
	now := time.Now()
	cutoff := now.Add(-l.window)
	l.mu.Lock()
	defer l.mu.Unlock()
	kept := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= l.limit {
		l.hits[key] = kept
		return false
	}
	l.hits[key] = append(kept, now)
	return true
}

// retryAfter returns seconds until the oldest hit slides out, minimum 1.
func (l *rateLimiter) retryAfter(key string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	hits := l.hits[key]
	if len(hits) == 0 {
		return 1
	}
	oldest := hits[0]
	for _, t := range hits[1:] {
		if t.Before(oldest) {
			oldest = t
		}
	}
	secs := int(time.Until(oldest.Add(l.window)).Seconds()) + 1
	if secs < 1 {
		return 1
	}
	return secs
}
