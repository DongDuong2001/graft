package storage

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/DongDuong2001/graft/internal/models"
)

const testMasterKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", "file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func newTestRepo(t *testing.T) *SQLiteRepo {
	t.Helper()
	repo, err := NewSQLiteRepo(openTestDB(t), testMasterKey)
	if err != nil {
		if strings.Contains(err.Error(), "CGO_ENABLED=0") {
			t.Skip("sqlite3 requires CGO")
		}
		t.Fatal(err)
	}
	return repo
}

func TestSQLiteRepo_SaveAndGetRuleByPath(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	rule := models.Rule{
		ID:                "r1",
		Name:              "n",
		ListenPath:        "/hook/x",
		DestinationURL:    "https://example.com",
		DestinationMethod: "POST",
		DestinationHeaders: map[string]string{
			"X-Custom": "v",
		},
	}
	if err := repo.SaveRule(ctx, rule); err != nil {
		t.Fatal(err)
	}

	got, err := repo.GetRuleByPath(ctx, "/hook/x")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.ID != "r1" || got.Name != "n" {
		t.Fatalf("GetRuleByPath: %+v", got)
	}
	if got.DestinationHeaders["X-Custom"] != "v" {
		t.Fatalf("headers: %+v", got.DestinationHeaders)
	}
}

func TestSQLiteRepo_GetRuleByID_List_Delete(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	rule := models.Rule{
		ID:             "a",
		Name:           "a",
		ListenPath:     "/hook/a",
		DestinationURL: "https://a",
	}
	if err := repo.SaveRule(ctx, rule); err != nil {
		t.Fatal(err)
	}

	got, err := repo.GetRuleByID(ctx, "a")
	if err != nil || got == nil || got.ListenPath != "/hook/a" {
		t.Fatalf("GetRuleByID: %v %+v", err, got)
	}

	list, err := repo.ListRules(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListRules: %v %#v", err, list)
	}

	if err := repo.DeleteRule(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteRule(ctx, "a"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestSQLiteRepo_Deliveries(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	if err := repo.SaveRule(ctx, models.Rule{
		ID:             "r",
		Name:           "r",
		ListenPath:     "/hook/r",
		DestinationURL: "https://x",
	}); err != nil {
		t.Fatal(err)
	}
	d := models.Delivery{
		ID:         "d1",
		RuleID:     "r",
		CreatedAt:  1,
		Success:    true,
		StatusCode: 200,
		DurationMS: 5,
		RetryCount: 0,
	}
	if err := repo.SaveDelivery(ctx, d); err != nil {
		t.Fatal(err)
	}
	out, err := repo.ListDeliveriesByRule(ctx, "r", 10)
	if err != nil || len(out) != 1 || out[0].ID != "d1" {
		t.Fatalf("deliveries: %v %#v", err, out)
	}
}

func TestSQLiteRepo_UniqueListenPath(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	path := "/hook/only"
	if err := repo.SaveRule(ctx, models.Rule{ID: "1", Name: "1", ListenPath: path, DestinationURL: "https://a"}); err != nil {
		t.Fatal(err)
	}
	err := repo.SaveRule(ctx, models.Rule{ID: "2", Name: "2", ListenPath: path, DestinationURL: "https://b"})
	if err == nil {
		t.Fatal("expected unique constraint on listen_path")
	}
}
