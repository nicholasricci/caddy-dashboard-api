package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type limiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type LimiterStore struct {
	mu      sync.Mutex
	entries map[string]*limiterEntry
	r       rate.Limit
	burst   int
}

func NewLimiterStore(r rate.Limit, burst int) *LimiterStore {
	return &LimiterStore{
		entries: make(map[string]*limiterEntry),
		r:       r,
		burst:   burst,
	}
}

func (s *LimiterStore) limiterFor(key string) *rate.Limiter {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e, ok := s.entries[key]; ok {
		e.lastSeen = time.Now()
		return e.limiter
	}
	l := rate.NewLimiter(s.r, s.burst)
	s.entries[key] = &limiterEntry{limiter: l, lastSeen: time.Now()}
	return l
}

func (s *LimiterStore) CleanupOlderThan(maxAge time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := time.Now().Add(-maxAge)
	for k, v := range s.entries {
		if v.lastSeen.Before(cutoff) {
			delete(s.entries, k)
		}
	}
}

func RateLimitByIP(store *LimiterStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.ClientIP() + ":" + c.FullPath()
		if !store.limiterFor(key).Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}
