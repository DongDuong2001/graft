package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFixedWindowLimiter(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	lim := NewFixedWindowLimiter(2, time.Minute, false)

	h := lim.Middleware(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.0.2.1:1"

	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("iter %d: code %d", i, rec.Code)
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}
}

func TestFixedWindowLimiter_NormalizesMinWindow(t *testing.T) {
	lim := NewFixedWindowLimiter(0, time.Millisecond, false)
	if lim.window < time.Second || lim.max < 1 {
		t.Fatalf("window=%v max=%d", lim.window, lim.max)
	}
}
