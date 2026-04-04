package ratelimit

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"Graft/internal/utils"
)

// TokenBucket implements a token bucket rate limiter per client IP.
// This provides smoother rate limiting than fixed-window, allowing bursts
// while maintaining a steady average rate.
type TokenBucket struct {
	mu         sync.RWMutex
	buckets    map[string]*bucket
	refillRate float64    // tokens per second
	capacity   int        // max tokens per bucket
	trustProxy bool

	// cleanupInterval controls how often we remove stale buckets
	cleanupInterval time.Duration
	lastCleanup     time.Time
}

// bucket represents a single client's token bucket.
type bucket struct {
	tokens    float64
	lastCheck time.Time
}

// NewTokenBucket creates a token bucket limiter.
//   capacity: max burst size (bucket size)
//   refillPerSecond: steady-state rate limit (tokens per second)
// Example: capacity=10, refillPerSecond=1 means burst of 10, then 1/sec.
func NewTokenBucket(capacity int, refillPerSecond float64, trustForwardedFor bool) *TokenBucket {
	if capacity < 1 {
		capacity = 1
	}
	if refillPerSecond <= 0 {
		refillPerSecond = 1
	}

	tb := &TokenBucket{
		buckets:         make(map[string]*bucket),
		capacity:        capacity,
		refillRate:      refillPerSecond,
		trustProxy:      trustForwardedFor,
		cleanupInterval: 5 * time.Minute,
		lastCleanup:     time.Now(),
	}

	// Start cleanup goroutine
	go tb.cleanupLoop()

	return tb
}

// Allow checks if a request from the given IP is allowed.
func (tb *TokenBucket) Allow(ip string) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	b, exists := tb.buckets[ip]
	if !exists {
		// New bucket with full capacity minus 1 for this request
		tb.buckets[ip] = &bucket{
			tokens:    float64(tb.capacity - 1),
			lastCheck: now,
		}
		return true
	}

	// Refill tokens based on elapsed time
	elapsed := now.Sub(b.lastCheck).Seconds()
	b.tokens += elapsed * tb.refillRate
	if b.tokens > float64(tb.capacity) {
		b.tokens = float64(tb.capacity)
	}
	b.lastCheck = now

	// Check if we can consume a token
	if b.tokens >= 1 {
		b.tokens--
		return true
	}

	return false
}

// TokensRemaining returns the current token count for an IP (for headers).
func (tb *TokenBucket) TokensRemaining(ip string) int {
	tb.mu.RLock()
	defer tb.mu.RUnlock()

	b, exists := tb.buckets[ip]
	if !exists {
		return tb.capacity
	}
	// Calculate current tokens
	elapsed := time.Since(b.lastCheck).Seconds()
	tokens := b.tokens + elapsed*tb.refillRate
	if tokens > float64(tb.capacity) {
		tokens = float64(tb.capacity)
	}
	return int(tokens)
}

// cleanupLoop periodically removes stale buckets.
func (tb *TokenBucket) cleanupLoop() {
	ticker := time.NewTicker(tb.cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		tb.cleanup()
	}
}

// cleanup removes buckets that have been idle for more than 10 minutes.
func (tb *TokenBucket) cleanup() {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	threshold := time.Now().Add(-10 * time.Minute)
	for ip, b := range tb.buckets {
		if b.lastCheck.Before(threshold) {
			delete(tb.buckets, ip)
		}
	}
	tb.lastCleanup = time.Now()
}

// Middleware returns the rate limiting middleware.
func (tb *TokenBucket) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := utils.ClientIP(r, tb.trustProxy)

		if !tb.Allow(ip) {
			// Calculate retry-after based on refill rate
			retryAfter := int(1/tb.refillRate) + 1
			if retryAfter < 1 {
				retryAfter = 1
			}
			w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}

		// Add rate limit headers
		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", tb.capacity))
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", tb.TokensRemaining(ip)))

		next.ServeHTTP(w, r)
	})
}

// RateLimiterConfig configures rate limiting for different endpoints.
type RateLimiterConfig struct {
	// Webhook capacity and refill rate (burst/avg)
	WebhookCapacity      int
	WebhookRefillPerSec  float64
	// Admin capacity and refill rate (more restrictive)
	AdminCapacity       int
	AdminRefillPerSec   float64
	TrustForwardedFor   bool
}

// DefaultRateLimiterConfig returns a sensible default configuration.
func DefaultRateLimiterConfig() RateLimiterConfig {
	return RateLimiterConfig{
		WebhookCapacity:     20,  // burst of 20
		WebhookRefillPerSec: 10,  // then 10/sec sustained
		AdminCapacity:       5,   // burst of 5
		AdminRefillPerSec:   0.5, // then 1 every 2 seconds
		TrustForwardedFor:   false,
	}
}
