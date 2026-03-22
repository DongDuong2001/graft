package middleware

import (
	"net"
	"net/http"
	"strings"
)

// ClientIP returns the client IP. When trustForwardedFor is false, X-Forwarded-For is ignored
// (recommended when not behind a trusted reverse proxy).
func ClientIP(r *http.Request, trustForwardedFor bool) string {
	if trustForwardedFor {
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			ips := strings.Split(forwarded, ",")
			if len(ips) > 0 {
				return strings.TrimSpace(ips[0])
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
