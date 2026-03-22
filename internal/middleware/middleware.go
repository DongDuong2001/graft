package middleware

import (
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// IPAllowlistMiddleware only allows requests from specific IP addresses.
func IPAllowlistMiddleware(allowedIPs []string) func(http.Handler) http.Handler {
	allowedMap := make(map[string]bool)
	for _, ip := range allowedIPs {
		allowedMap[ip] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(allowedIPs) == 0 {
				next.ServeHTTP(w, r)
				return
			}

			clientIP := ClientIP(r, true)

			if !allowedMap[clientIP] {
				slog.Warn("Blocked request from disallowed IP", "ip", clientIP)
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// SimpleRateLimiter implements a naive token bucket per IP address.
type SimpleRateLimiter struct {
	mu           sync.Mutex
	lastRequest  map[string]time.Time
	requestCount map[string]int
	maxRequests  int
	window       time.Duration
}

func NewSimpleRateLimiter(maxRequests int, window time.Duration) *SimpleRateLimiter {
	limiter := &SimpleRateLimiter{
		lastRequest:  make(map[string]time.Time),
		requestCount: make(map[string]int),
		maxRequests:  maxRequests,
		window:       window,
	}

	// Simple cleanup routine
	go func() {
		for {
			time.Sleep(window)
			limiter.mu.Lock()
			now := time.Now()
			for ip, last := range limiter.lastRequest {
				if now.Sub(last) > window {
					delete(limiter.lastRequest, ip)
					delete(limiter.requestCount, ip)
				}
			}
			limiter.mu.Unlock()
		}
	}()

	return limiter
}

// RateLimitMiddleware applies the rate Limiter.
func (rl *SimpleRateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := ClientIP(r, true)

		rl.mu.Lock()
		rl.lastRequest[clientIP] = time.Now()
		rl.requestCount[clientIP]++
		count := rl.requestCount[clientIP]
		rl.mu.Unlock()

		if count > rl.maxRequests {
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// LoggingMiddleware logs all incoming requests.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		// Wrap ResponseWriter to capture status code? (Left as exercise, basic logging for now)
		next.ServeHTTP(w, r)

		slog.Info("Request handled",
			"method", r.Method,
			"path", r.URL.Path,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote_addr", r.RemoteAddr,
		)
	})
}
