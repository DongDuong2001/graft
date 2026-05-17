package httpapi

import (
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/DongDuong2001/graft/internal/engine"
	"github.com/DongDuong2001/graft/internal/models"
	"github.com/DongDuong2001/graft/internal/webhook"
	"time"
)

const maxBodyBytes = 5 << 20 // 5 MiB

// WebhookHandler receives POST webhooks and forwards them per rules.
type WebhookHandler struct {
	eng   *engine.Engine
	queue engine.Queue
}

// NewWebhookHandler constructs a WebhookHandler.
func NewWebhookHandler(eng *engine.Engine, q engine.Queue) *WebhookHandler {
	return &WebhookHandler{
		eng:   eng,
		queue: q,
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

	// Prepare delivery record (Pending state)
	d, _, err := h.eng.PrepareDelivery(r.Context(), wh)
	if err != nil {
		if errors.Is(err, models.ErrRuleNotFound) {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		slog.Error("Failed to prepare delivery", "path", r.URL.Path, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Enqueue for background processing
	task := &engine.Task{
		DeliveryID: d.ID,
		Webhook:    wh,
		EnqueuedAt: time.Now(),
	}

	if err := h.queue.Enqueue(r.Context(), task); err != nil {
		slog.Error("Failed to enqueue webhook", "delivery_id", d.ID, "error", err)
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"status":"accepted","delivery_id":"` + d.ID + `"}`))
}
