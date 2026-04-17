package security

import (
	"crypto/subtle"
	"net/http"
)

// WebhookAPIKeyAuth validates an optional per-rule API key for webhook endpoints.
// This provides authentication for cases where HMAC signature verification is not available.
// The rule's API key is compared against the header value using constant-time comparison.
type WebhookAPIKeyAuth struct {
	apiKey     string
	headerName string
}

// NewWebhookAPIKeyAuth creates webhook API key validator.
// headerName defaults to "X-API-Key" if empty.
func NewWebhookAPIKeyAuth(apiKey, headerName string) *WebhookAPIKeyAuth {
	if headerName == "" {
		headerName = "X-API-Key"
	}
	return &WebhookAPIKeyAuth{
		apiKey:     apiKey,
		headerName: headerName,
	}
}

// Middleware validates the API key from the request header.
func (wa *WebhookAPIKeyAuth) Middleware(next http.Handler) http.Handler {
	// If no API key configured, pass through
	if wa.apiKey == "" {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		providedKey := r.Header.Get(wa.headerName)
		if providedKey == "" {
			http.Error(w, "Unauthorized: missing API key", http.StatusUnauthorized)
			return
		}

		// Constant-time comparison to prevent timing attacks
		if subtle.ConstantTimeCompare([]byte(wa.apiKey), []byte(providedKey)) != 1 {
			http.Error(w, "Unauthorized: invalid API key", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// IsConfigured returns true if an API key is configured.
func (wa *WebhookAPIKeyAuth) IsConfigured() bool {
	return wa.apiKey != ""
}

// WebhookAuthConfig combines all webhook authentication options.
type WebhookAuthConfig struct {
	RequireSignature         bool
	SignatureHeader          string
	SignatureFormat          string
	SignatureSecret          string
	SignatureTimestampHeader string
	SignatureMaxSkewSeconds  int
	RequireAPIKey            bool
	APIKeyHeader             string
	APIKey                   string
}

// GetWebhookAuthMiddleware returns appropriate middleware for webhook auth.
// Priority: 1) Signature verification (highest), 2) API key (fallback)
// If neither is configured, returns a passthrough middleware.
func GetWebhookAuthMiddleware(cfg WebhookAuthConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check API key if configured
		if cfg.RequireAPIKey && cfg.APIKey != "" {
			key := r.Header.Get(cfg.APIKeyHeader)
			if key == "" {
				key = r.Header.Get("X-API-Key") // fallback
			}
			if subtle.ConstantTimeCompare([]byte(cfg.APIKey), []byte(key)) != 1 {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		}

		// Signature verification is handled separately in the engine
		// since it requires body content

		w.WriteHeader(http.StatusOK)
	}
}
