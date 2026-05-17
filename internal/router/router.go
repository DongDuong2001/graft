package router

import (
	"net/http"

	"github.com/DongDuong2001/graft/internal/config"
	"github.com/DongDuong2001/graft/internal/middleware"
	"github.com/DongDuong2001/graft/internal/ratelimit"
	"github.com/DongDuong2001/graft/internal/security"
	"github.com/DongDuong2001/graft/internal/ui"
)

// SecurityConfig bundles all security-related middleware configuration.
type SecurityConfig struct {
	DevMode             bool
	CORSOrigins         []string
	CORSCredentials     bool
	WebhookRateLimiter  *ratelimit.TokenBucket
	AdminRateLimiter    *ratelimit.TokenBucket
	BruteForceProtector *security.BruteForceProtector
	SecurityHeaders     security.SecurityHeaders
}

// Config wires the public HTTP route tree (health, webhooks, admin API).
type Config struct {
	WebhookHandler http.Handler
	AdminInner     http.Handler // routes under /api/v1 (already stripped prefix in child mux)
	AdminAPIKey    string
	Security       SecurityConfig
}

// NewRootMux returns the top-level ServeMux with security middleware applied.
func NewRootMux(cfg Config) http.Handler {
	mux := http.NewServeMux()

	// Health check (no auth, minimal middleware)
	mux.HandleFunc("GET /healthz", healthz)

	// Webhook endpoints (rate limited, signature checked in handler)
	webhookChain := cfg.WebhookHandler
	if cfg.Security.WebhookRateLimiter != nil {
		webhookChain = cfg.Security.WebhookRateLimiter.Middleware(webhookChain)
	}
	mux.Handle("/hook/", webhookChain)

	// Web UI (CORS, security headers)
	uiChain := ui.WebHandler()
	if len(cfg.Security.CORSOrigins) > 0 {
		corsConfig := security.CORSConfig{
			AllowedOrigins:   cfg.Security.CORSOrigins,
			AllowCredentials: cfg.Security.CORSCredentials,
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-API-Key"},
		}
		uiChain = security.CORS(corsConfig)(uiChain)
	}
	mux.Handle("/", uiChain)

	// Admin API (auth + brute force protection + rate limiting)
	adminChain := cfg.AdminInner

	// Apply admin rate limiting if configured
	if cfg.Security.AdminRateLimiter != nil {
		adminChain = cfg.Security.AdminRateLimiter.Middleware(adminChain)
	}

	// Apply brute force protection if configured
	if cfg.Security.BruteForceProtector != nil {
		adminChain = cfg.Security.BruteForceProtector.Middleware(adminChain)
		adminChain = middleware.AdminAuth(cfg.AdminAPIKey, adminChain)
	} else {
		adminChain = middleware.AdminAuth(cfg.AdminAPIKey, adminChain)
	}

	adminChain = http.StripPrefix("/api/v1", adminChain)
	mux.Handle("/api/v1/", adminChain)

	// Apply global middleware
	var handler http.Handler = mux

	// Security headers (should be outermost to apply to all responses)
	handler = cfg.Security.SecurityHeaders.Middleware(handler)

	// Logging
	handler = middleware.LoggingMiddleware(handler)

	return handler
}

// BuildSecurityConfig creates a SecurityConfig from application config.
func BuildSecurityConfig(cfg config.Config) SecurityConfig {
	sec := SecurityConfig{
		DevMode:         cfg.DevMode,
		CORSOrigins:     cfg.CORSOrigins,
		CORSCredentials: cfg.CORSCredentials,
	}

	// Security headers
	if cfg.DevMode {
		sec.SecurityHeaders = security.DevSecurityHeaders()
	} else {
		sec.SecurityHeaders = security.DefaultSecurityHeaders()
	}

	// Webhook rate limiter (token bucket)
	if cfg.RateLimitBurst > 0 && cfg.RateLimitRefill > 0 {
		sec.WebhookRateLimiter = ratelimit.NewTokenBucket(
			cfg.RateLimitBurst,
			cfg.RateLimitRefill,
			cfg.TrustForwardedHeaders,
		)
	}

	// Admin rate limiter (more restrictive)
	sec.AdminRateLimiter = ratelimit.NewTokenBucket(10, 1, cfg.TrustForwardedHeaders)

	// Brute force protection for admin endpoints
	sec.BruteForceProtector = security.NewBruteForceProtector(
		cfg.BruteForceMaxFailures,
		cfg.BruteForceLockoutBase,
		cfg.BruteForceMaxLockout,
		cfg.TrustForwardedHeaders,
	)

	return sec
}

func healthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
