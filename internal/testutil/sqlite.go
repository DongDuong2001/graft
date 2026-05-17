package testutil

import (
	"database/sql"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/DongDuong2001/graft/internal/storage"
)

// MasterKey is a valid 64-hex-character AES-256 key for tests (not secret — tests only).
const MasterKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// SQLiteRepo opens a shared in-memory SQLite DB and returns a repository.
// It skips the test when CGO is disabled (go-sqlite3 stub).
func SQLiteRepo(t *testing.T) *storage.SQLiteRepo {
	t.Helper()
	db, err := sql.Open("sqlite3", "file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	repo, err := storage.NewSQLiteRepo(db, MasterKey)
	if err != nil {
		if strings.Contains(err.Error(), "CGO_ENABLED=0") {
			t.Skip("github.com/mattn/go-sqlite3 requires CGO; enable CGO for storage-backed tests")
		}
		t.Fatal(err)
	}
	return repo
}
