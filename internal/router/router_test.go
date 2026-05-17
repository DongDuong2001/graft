package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DongDuong2001/graft/internal/config"
)

func TestNewRootMux_Healthz(t *testing.T) {
	dummy := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	inner := http.NewServeMux()
	inner.HandleFunc("GET /rules", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	h := NewRootMux(Config{
		WebhookHandler: dummy,
		AdminInner:     inner,
		AdminAPIKey:    "k",
		Security:       BuildSecurityConfig(config.Config{}),
	})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "ok") {
		t.Fatalf("code=%d body=%q", rec.Code, rec.Body.String())
	}
}
