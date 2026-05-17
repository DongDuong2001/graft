package server

import (
	"net/http"
	"testing"

	"github.com/DongDuong2001/graft/internal/config"
)

func TestNewHTTPServer_Addr(t *testing.T) {
	t.Setenv("MASTER_KEY", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	t.Setenv("ADMIN_API_KEY", "k")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Port = "9999"

	s := NewHTTPServer(cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), nil)
	if s.Addr() != ":9999" {
		t.Fatalf("addr %q", s.Addr())
	}
}
