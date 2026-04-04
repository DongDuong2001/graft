package integration

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"Graft/internal/admin"
	"Graft/internal/config"
	"Graft/internal/connectors"
	"Graft/internal/engine"
	"Graft/internal/forwarder"
	"Graft/internal/httpapi"
	"Graft/internal/router"
	"Graft/internal/storage"
	"Graft/internal/testutil"
)

func TestAdminCreateAndWebhookForward(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "rules.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	repo, err := storage.NewSQLiteRepo(db, testutil.MasterKey)
	if err != nil {
		if strings.Contains(err.Error(), "CGO_ENABLED=0") {
			t.Skip("github.com/mattn/go-sqlite3 requires CGO; run with CGO_ENABLED=1 and a C toolchain to run this test")
		}
		t.Fatalf("repo: %v", err)
	}

	const adminKey = "integration-test-admin-key-32chars!!"
	svc := admin.NewService(repo, testutil.MasterKey)

	httpFwd := connectors.NewHTTPForwarder(connectors.HTTPConfig{Timeout: 5 * time.Second, MaxRetries: 0})
	fwd := forwarder.NewSyncForwarder(httpFwd)
	eng := engine.New(repo, testutil.MasterKey, fwd, connectors.NewRegistry(nil))

	adminH := httpapi.NewAdminHandler(svc, eng)
	adminMux := http.NewServeMux()
	adminH.Register(adminMux)

	q := engine.NewMemoryQueue(100)
	wp := engine.NewWorkerPool(q, eng, 2)
	wp.Start(context.Background())
	t.Cleanup(wp.Stop)

	wh := httpapi.NewWebhookHandler(eng, q)

	mainMux := router.NewRootMux(router.Config{
		WebhookHandler: wh,
		AdminInner:     adminMux,
		AdminAPIKey:    adminKey,
		Security:       router.BuildSecurityConfig(config.Config{}),
	})

	destReceived := make(chan []byte, 1)
	dest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		destReceived <- b
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(dest.Close)

	body := map[string]any{
		"name":                "e2e",
		"listen_path":         "/hook/e2e",
		"destination_url":     dest.URL,
		"destination_method":  "POST",
		"required_signature":  false,
		"transform_template":  "",
		"destination_headers": map[string]string{},
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/rules", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+adminKey)
	rec := httptest.NewRecorder()
	mainMux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create rule: status %d body %s", rec.Code, rec.Body.String())
	}

	whPayload := []byte(`{"hello":"world"}`)
	whReq := httptest.NewRequest(http.MethodPost, "/hook/e2e", bytes.NewReader(whPayload))
	whReq.Header.Set("Content-Type", "application/json")
	whRec := httptest.NewRecorder()
	mainMux.ServeHTTP(whRec, whReq)
	if whRec.Code != http.StatusAccepted {
		t.Fatalf("webhook: status %d body %s", whRec.Code, whRec.Body.String())
	}

	select {
	case got := <-destReceived:
		if string(got) != string(whPayload) {
			t.Fatalf("destination body mismatch: %q vs %q", got, whPayload)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for destination request")
	}
}
