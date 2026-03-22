package middleware

import (
	"net/http"
	"sync"
	"time"
)

// FixedWindowLimiter enforces maxRequests per client IP within each fixed window.
type FixedWindowLimiter struct {
	mu           sync.Mutex
	counts       map[string]int
	windowStarts map[string]time.Time
	max          int
	window       time.Duration
	trustProxy   bool
}

// NewFixedWindowLimiter creates a per-IP fixed-window limiter.
func NewFixedWindowLimiter(maxRequests int, window time.Duration, trustForwardedFor bool) *FixedWindowLimiter {
	if maxRequests < 1 {
		maxRequests = 1
	}
	if window < time.Second {
		window = time.Second
	}
	return &FixedWindowLimiter{
		counts:       make(map[string]int),
		windowStarts: make(map[string]time.Time),
		max:          maxRequests,
		window:       window,
		trustProxy:   trustForwardedFor,
	}
}

// Middleware applies rate limiting.
func (f *FixedWindowLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := ClientIP(r, f.trustProxy)
		now := time.Now()

		f.mu.Lock()
		start, ok := f.windowStarts[ip]
		if !ok || now.Sub(start) >= f.window {
			f.windowStarts[ip] = now
			f.counts[ip] = 0
		}
		f.counts[ip]++
		n := f.counts[ip]
		f.mu.Unlock()

		if n > f.max {
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
