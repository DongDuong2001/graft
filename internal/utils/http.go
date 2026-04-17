// Package utils provides shared utility functions for the Graft project.
package utils

import (
	"net"
	"net/http"
	"strings"
)

// ClientIP extracts the real client IP address from the request.
// It considers X-Forwarded-For and X-Real-IP headers when trustProxy is true.
func ClientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		// Check X-Forwarded-For header (may contain multiple IPs)
		xff := r.Header.Get("X-Forwarded-For")
		if xff != "" {
			// The first IP is typically the original client
			ips := strings.Split(xff, ",")
			if len(ips) > 0 {
				ip := strings.TrimSpace(ips[0])
				if net.ParseIP(ip) != nil {
					return ip
				}
			}
		}

		// Check X-Real-IP header
		xri := r.Header.Get("X-Real-IP")
		if xri != "" {
			if net.ParseIP(xri) != nil {
				return xri
			}
		}
	}

	// Fall back to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr // Return as-is if parsing fails
	}
	return host
}

// IsPrivateIP checks if an IP address is in a private RFC1918 range.
func IsPrivateIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	// RFC1918 private ranges
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8", // Loopback
		"::1/128",     // IPv6 loopback
		"fc00::/7",    // IPv6 unique local
	}

	for _, cidr := range privateRanges {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if ipNet.Contains(ip) {
			return true
		}
	}

	return false
}
