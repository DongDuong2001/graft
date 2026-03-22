package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// AdminAuth requires a Bearer token or X-API-Key matching apiKey (constant-time).
func AdminAuth(apiKey string, next http.Handler) http.Handler {
	want := []byte(apiKey)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(want) == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		got := []byte(adminTokenFromRequest(r))
		if len(got) == 0 || len(got) != len(want) || subtle.ConstantTimeCompare(want, got) != 1 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func adminTokenFromRequest(r *http.Request) string {
	if v := r.Header.Get("X-API-Key"); v != "" {
		return strings.TrimSpace(v)
	}
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return strings.TrimSpace(h[len(prefix):])
	}
	return ""
}
