package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"Graft/internal/admin"
	"Graft/internal/middleware"
	"Graft/internal/models"
	"Graft/internal/testutil"
)

func adminMux(t *testing.T) (http.Handler, string) {
	t.Helper()
	repo := testutil.SQLiteRepo(t)
	const adminKey = "unit-test-admin-key-string!!"
	svc := admin.NewService(repo, testutil.MasterKey)
	h := NewAdminHandler(svc)
	mux := http.NewServeMux()
	h.Register(mux)
	return middleware.AdminAuth(adminKey, mux), adminKey
}

func TestAdminHandler_CreateRule_Validation(t *testing.T) {
	h, key := adminMux(t)

	body := map[string]any{
		"name":            "",
		"listen_path":     "/hook/x",
		"destination_url": "https://x",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/rules", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+key)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code %d body %s", rec.Code, rec.Body.String())
	}
}

func TestAdminHandler_CreateRule_List_Get_Delete(t *testing.T) {
	h, key := adminMux(t)

	create := map[string]any{
		"name":               "t",
		"listen_path":        "/hook/t",
		"destination_url":    "https://example.com",
		"required_signature": false,
		"destination_headers": map[string]string{
			"X-A": "b",
		},
	}
	b, _ := json.Marshal(create)
	req := httptest.NewRequest(http.MethodPost, "/rules", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+key)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %d: %s", rec.Code, rec.Body.String())
	}
	var created models.Rule
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || created.ListenPath != "/hook/t" {
		t.Fatalf("%+v", created)
	}

	req = httptest.NewRequest(http.MethodGet, "/rules", nil)
	req.Header.Set("Authorization", "Bearer "+key)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatal(rec.Code)
	}
	var list []models.Rule
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list) != 1 {
		t.Fatalf("%v %s", err, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/rules/"+created.ID, nil)
	req.Header.Set("Authorization", "Bearer "+key)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatal(rec.Code)
	}

	req = httptest.NewRequest(http.MethodDelete, "/rules/"+created.ID, nil)
	req.Header.Set("Authorization", "Bearer "+key)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/rules/"+created.ID, nil)
	req.Header.Set("Authorization", "Bearer "+key)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("get after delete: %d", rec.Code)
	}
}

func TestAdminHandler_GetMetrics(t *testing.T) {
	h, key := adminMux(t)
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Bearer "+key)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatal(rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatal(rec.Header().Get("Content-Type"))
	}
}
