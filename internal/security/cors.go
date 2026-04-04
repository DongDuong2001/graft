package security

import (
	"net/http"
	"strings"
)

// CORSConfig configures Cross-Origin Resource Sharing policy.
type CORSConfig struct {
	// AllowedOrigins is a list of allowed origins. Use ["*"] for any origin.
	// Empty means CORS is disabled.
	AllowedOrigins []string
	// AllowedMethods is a list of allowed HTTP methods.
	AllowedMethods []string
	// AllowedHeaders is a list of allowed headers.
	AllowedHeaders []string
	// AllowCredentials indicates whether credentials are allowed.
	AllowCredentials bool
	// MaxAge is the max age for preflight cache in seconds.
	MaxAge int
}

// DefaultCORSConfig returns a restrictive default configuration.
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins:   []string{}, // Deny all by default
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-API-Key"},
		AllowCredentials: false,
		MaxAge:           86400, // 24 hours
	}
}

// CORS returns a middleware that handles CORS headers.
func CORS(config CORSConfig) func(http.Handler) http.Handler {
	// Pre-compute string representations
	allowMethods := strings.Join(config.AllowedMethods, ", ")
	allowHeaders := strings.Join(config.AllowedHeaders, ", ")
	maxAge := "86400"
	if config.MaxAge > 0 {
		maxAge = "86400"
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Check if origin is allowed
			if !isOriginAllowed(config.AllowedOrigins, origin) {
				// Not an allowed origin, continue without CORS headers
				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusForbidden)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			// Set CORS headers
			if len(config.AllowedOrigins) == 1 && config.AllowedOrigins[0] == "*" {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
			}

			w.Header().Set("Access-Control-Allow-Methods", allowMethods)
			w.Header().Set("Access-Control-Allow-Headers", allowHeaders)
			w.Header().Set("Access-Control-Max-Age", maxAge)

			if config.AllowCredentials {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			// Handle preflight request
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isOriginAllowed checks if the origin is in the allowed list.
func isOriginAllowed(allowed []string, origin string) bool {
	if len(allowed) == 0 {
		return false
	}
	for _, a := range allowed {
		if a == "*" || strings.EqualFold(a, origin) {
			return true
		}
	}
	return false
}

// CORSForUI returns CORS config suitable for the admin UI (same-origin + dev).
func CORSForUI(devMode bool) CORSConfig {
	cfg := DefaultCORSConfig()
	if devMode {
		cfg.AllowedOrigins = []string{"http://localhost:*", "http://127.0.0.1:*"}
	} else {
		// Same-origin only in production
		cfg.AllowedOrigins = []string{}
	}
	cfg.AllowCredentials = true
	return cfg
}
