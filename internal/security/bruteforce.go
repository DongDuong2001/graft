package security

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"Graft/internal/utils"
)

// BruteForceProtector tracks failed authentication attempts per IP
// and applies exponential backoff to prevent credential stuffing.
type BruteForceProtector struct {
	mu           sync.RWMutex
	attempts     map[string]*authAttempt
	maxFailures  int           // failures before lockout
	lockoutBase  time.Duration // base lockout duration
	maxLockout   time.Duration // maximum lockout duration
	trustProxy   bool

	cleanupInterval time.Duration
}

// authAttempt tracks failure count and lockout state for an IP.
type authAttempt struct {
	failures   int
	lockedUntil time.Time
	lastAttempt time.Time
}

// NewBruteForceProtector creates a new brute force protection instance.
//   maxFailures: number of failures before lockout (default: 5)
//   lockoutBase: initial lockout duration (default: 5 minutes)
//   maxLockout: maximum lockout duration (default: 1 hour)
func NewBruteForceProtector(maxFailures int, lockoutBase, maxLockout time.Duration, trustForwardedFor bool) *BruteForceProtector {
	if maxFailures < 1 {
		maxFailures = 5
	}
	if lockoutBase < time.Second {
		lockoutBase = 5 * time.Minute
	}
	if maxLockout < lockoutBase {
		maxLockout = time.Hour
	}

	bf := &BruteForceProtector{
		attempts:        make(map[string]*authAttempt),
		maxFailures:     maxFailures,
		lockoutBase:     lockoutBase,
		maxLockout:      maxLockout,
		trustProxy:      trustForwardedFor,
		cleanupInterval: 10 * time.Minute,
	}

	go bf.cleanupLoop()
	return bf
}

// IsAllowed checks if the IP is allowed to attempt authentication.
func (bf *BruteForceProtector) IsAllowed(ip string) bool {
	bf.mu.RLock()
	defer bf.mu.RUnlock()

	attempt, exists := bf.attempts[ip]
	if !exists {
		return true
	}

	// Check if still locked out
	if time.Now().Before(attempt.lockedUntil) {
		return false
	}

	return true
}

// RecordFailure records a failed authentication attempt.
func (bf *BruteForceProtector) RecordFailure(ip string) {
	bf.mu.Lock()
	defer bf.mu.Unlock()

	attempt, exists := bf.attempts[ip]
	if !exists {
		bf.attempts[ip] = &authAttempt{
			failures:    1,
			lastAttempt: time.Now(),
		}
		return
	}

	attempt.failures++
	attempt.lastAttempt = time.Now()

	// Apply exponential backoff: base * 2^(failures-1)
	if attempt.failures >= bf.maxFailures {
		multiplier := 1 << uint(attempt.failures-bf.maxFailures)
		duration := bf.lockoutBase * time.Duration(multiplier)
		if duration > bf.maxLockout {
			duration = bf.maxLockout
		}
		attempt.lockedUntil = time.Now().Add(duration)
	}
}

// RecordSuccess resets the failure count for an IP.
func (bf *BruteForceProtector) RecordSuccess(ip string) {
	bf.mu.Lock()
	defer bf.mu.Unlock()

	delete(bf.attempts, ip)
}

// GetLockoutDuration returns how long the IP is locked out for.
func (bf *BruteForceProtector) GetLockoutDuration(ip string) time.Duration {
	bf.mu.RLock()
	defer bf.mu.RUnlock()

	attempt, exists := bf.attempts[ip]
	if !exists {
		return 0
	}

	remaining := time.Until(attempt.lockedUntil)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// GetRetryAfter returns the Retry-After header value in seconds.
func (bf *BruteForceProtector) GetRetryAfter(ip string) int {
	d := bf.GetLockoutDuration(ip)
	secs := int(d.Seconds())
	if secs < 1 {
		return 1
	}
	return secs
}

// cleanupLoop periodically removes stale entries.
func (bf *BruteForceProtector) cleanupLoop() {
	ticker := time.NewTicker(bf.cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		bf.cleanup()
	}
}

// cleanup removes entries that haven't had attempts in 24 hours.
func (bf *BruteForceProtector) cleanup() {
	bf.mu.Lock()
	defer bf.mu.Unlock()

	threshold := time.Now().Add(-24 * time.Hour)
	for ip, attempt := range bf.attempts {
		if attempt.lastAttempt.Before(threshold) {
			delete(bf.attempts, ip)
		}
	}
}

// Middleware wraps an http.Handler with brute force protection.
// It only protects POST/PUT/DELETE methods to admin endpoints.
func (bf *BruteForceProtector) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only protect state-changing methods
		if r.Method != http.MethodPost && r.Method != http.MethodPut &&
			r.Method != http.MethodDelete && r.Method != http.MethodPatch {
			next.ServeHTTP(w, r)
			return
		}

		ip := utils.ClientIP(r, bf.trustProxy)

		if !bf.IsAllowed(ip) {
			w.Header().Set("Retry-After", fmt.Sprintf("%d", bf.GetRetryAfter(ip)))
			http.Error(w, "Too many failed attempts. Please try again later.", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// ProtectedAuth wraps authentication and tracks failures/successes.
func (bf *BruteForceProtector) ProtectedAuth(apiKey string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := utils.ClientIP(r, bf.trustProxy)

		// Check lockout before attempting auth
		if !bf.IsAllowed(ip) {
			w.Header().Set("Retry-After", fmt.Sprintf("%d", bf.GetRetryAfter(ip)))
			http.Error(w, "Too many failed attempts. Please try again later.", http.StatusTooManyRequests)
			return
		}

		// Perform authentication
		got := utils.AdminTokenFromRequest(r)
		want := []byte(apiKey)

		if len(want) == 0 || len(got) == 0 || !utils.ConstantTimeEqual(want, []byte(got)) {
			bf.RecordFailure(ip)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Success - clear failures
		bf.RecordSuccess(ip)
		next.ServeHTTP(w, r)
	})
}

// constantTimeEqual performs constant-time comparison to prevent timing attacks.
func constantTimeEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := 0; i < len(a); i++ {
		v |= a[i] ^ b[i]
	}
	return v == 0
}

// ParseCIDR expands a list of IPs/CIDRs into individual IPs for matching.
// If s is not a valid CIDR, it's treated as a single IP.
func ParseCIDR(s string) ([]string, error) {
	_, ipNet, err := net.ParseCIDR(s)
	if err != nil {
		// Try as single IP
		ip := net.ParseIP(s)
		if ip == nil {
			return nil, fmt.Errorf("invalid IP or CIDR: %s", s)
		}
		return []string{ip.String()}, nil
	}

	var ips []string
	for ip := ipNet.IP.Mask(ipNet.Mask); ipNet.Contains(ip); incrementIP(ip) {
		ips = append(ips, ip.String())
	}
	return ips, nil
}

func incrementIP(ip net.IP) {
	for i := len(ip) - 1; i >= 0; i-- {
		ip[i]++
		if ip[i] != 0 {
			break
		}
	}
}
