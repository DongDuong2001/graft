package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientIP(t *testing.T) {
	t.Run("remote addr", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "192.168.1.2:12345"
		if got := ClientIP(r, false); got != "192.168.1.2" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("trust x-forwarded-for", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "10.0.0.1:1"
		r.Header.Set("X-Forwarded-For", "203.0.113.1, 10.0.0.1")
		if got := ClientIP(r, true); got != "203.0.113.1" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("ignore x-forwarded-for when disabled", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "10.0.0.1:1"
		r.Header.Set("X-Forwarded-For", "203.0.113.1")
		if got := ClientIP(r, false); got != "10.0.0.1" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("non tcp remote", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "unix"
		if got := ClientIP(r, false); got != "unix" {
			t.Fatalf("got %q", got)
		}
	})
}
