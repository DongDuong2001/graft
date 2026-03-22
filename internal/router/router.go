package router

import (
	"net/http"

	"Graft/internal/middleware"
)

// Config wires the public HTTP route tree (health, webhooks, admin API).
type Config struct {
	WebhookHandler http.Handler
	AdminInner     http.Handler // routes under /api/v1 (already stripped prefix in child mux)
	AdminAPIKey    string
	RateLimiter    *middleware.FixedWindowLimiter
}

// NewRootMux returns the top-level ServeMux with logging and rate limits applied.
func NewRootMux(cfg Config) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz)
	mux.Handle("/hook/", cfg.RateLimiter.Middleware(cfg.WebhookHandler))
	mux.Handle("/api/v1/", middleware.AdminAuth(cfg.AdminAPIKey, http.StripPrefix("/api/v1", cfg.AdminInner)))
	return middleware.LoggingMiddleware(mux)
}

func healthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
