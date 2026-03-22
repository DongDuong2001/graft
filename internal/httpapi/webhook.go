package httpapi

import (
	"errors"
	"io"
	"log/slog"
	"net/http"

	"Graft/internal/engine"
	"Graft/internal/models"
	"Graft/internal/webhook"
)

const maxBodyBytes = 5 << 20 // 5 MiB

// WebhookHandler receives POST webhooks and forwards them per rules.
type WebhookHandler struct {
	eng *engine.Engine
}

// NewWebhookHandler constructs a WebhookHandler.
func NewWebhookHandler(eng *engine.Engine) *WebhookHandler {
	return &WebhookHandler{
		eng: eng,
	}
}

// ServeHTTP implements http.Handler.
func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		slog.Warn("Failed to read webhook body", "error", err, "remote_addr", r.RemoteAddr)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	wh := webhook.NewFromRequest(r, body)

	_, err = h.eng.Process(r.Context(), wh)
	if err != nil {
		if errors.Is(err, models.ErrRuleNotFound) {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, models.ErrUnauthorized) {
			slog.Warn("Unauthorized webhook attempt", "path", r.URL.Path, "error", err)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		
		slog.Error("Webhook processing failed", "path", r.URL.Path, "error", err)
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"delivered"}`))
}
