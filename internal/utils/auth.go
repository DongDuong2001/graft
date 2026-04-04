// Package utils provides shared utility functions for the Graft project.
package utils

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// AdminTokenFromRequest extracts the admin API key from the request.
// It checks the Authorization header (Bearer token) and X-API-Key header.
func AdminTokenFromRequest(r *http.Request) string {
	// Check Authorization header (Bearer token)
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}

	// Check X-API-Key header
	return r.Header.Get("X-API-Key")
}

// ConstantTimeEqual performs constant-time comparison of two byte slices.
// This prevents timing attacks when comparing sensitive values like API keys.
func ConstantTimeEqual(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}
