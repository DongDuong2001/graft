package httpapi

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"Graft/internal/connectors"
	"Graft/internal/engine"
	"Graft/internal/forwarder"
	"Graft/internal/models"
	"Graft/internal/observability"
	"Graft/internal/testutil"
)

func TestWebhookHandler_MethodNotAllowed(t *testing.T) {
	observability.ResetCountersForTest()
	repo := testutil.SQLiteRepo(t)
	fwd := forwarder.NewSyncForwarder(connectors.NewHTTPForwarder(connectors.HTTPConfig{Timeout: time.Second, MaxRetries: 0}))
	eng := engine.New(repo, testutil.MasterKey, fwd)
	h := NewWebhookHandler(eng)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/hook/x", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("code %d", rec.Code)
	}
}

func TestWebhookHandler_NotFound(t *testing.T) {
	observability.ResetCountersForTest()
	repo := testutil.SQLiteRepo(t)
	fwd := forwarder.NewSyncForwarder(connectors.NewHTTPForwarder(connectors.HTTPConfig{Timeout: time.Second, MaxRetries: 0}))
	eng := engine.New(repo, testutil.MasterKey, fwd)
	h := NewWebhookHandler(eng)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/hook/missing", strings.NewReader(`{}`)))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code %d", rec.Code)
	}
}

func TestWebhookHandler_Delivers(t *testing.T) {
	observability.ResetCountersForTest()
	repo := testutil.SQLiteRepo(t)
	ctx := context.Background()

	dest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if string(b) != `{"p":1}` {
			t.Errorf("dest got %s", b)
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(dest.Close)

	if err := repo.SaveRule(ctx, models.Rule{
		ID:             "w1",
		Name:           "w",
		ListenPath:     "/hook/w",
		DestinationURL: dest.URL,
	}); err != nil {
		t.Fatal(err)
	}

	fwd := forwarder.NewSyncForwarder(connectors.NewHTTPForwarder(connectors.HTTPConfig{Timeout: 3 * time.Second, MaxRetries: 0}))
	eng := engine.New(repo, testutil.MasterKey, fwd)
	h := NewWebhookHandler(eng)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/hook/w", bytes.NewReader([]byte(`{"p":1}`))))

	if rec.Code != http.StatusOK {
		t.Fatalf("code %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "delivered") {
		t.Fatal(rec.Body.String())
	}
}
