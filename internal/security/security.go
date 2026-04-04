package security

import "net/http"

// SecurityHeaders adds security-related HTTP headers to all responses.
// These headers help mitigate common web attacks like XSS, clickjacking,
// and MIME-type sniffing attacks.
type SecurityHeaders struct {
	// CSPolicy allows custom Content-Security-Policy (default is restrictive)
	CSPolicy string
	// DisableHSTS disables Strict-Transport-Security header (useful for HTTP-only dev)
	DisableHSTS bool
	// FrameOptions allows custom X-Frame-Options (default DENY)
	FrameOptions string
}

// DefaultSecurityHeaders returns a SecurityHeaders with safe defaults.
func DefaultSecurityHeaders() SecurityHeaders {
	return SecurityHeaders{
		CSPolicy: "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; " +
			"img-src 'self' data:; font-src 'self'; connect-src 'self'; " +
			"frame-ancestors 'none'; base-uri 'self'; form-action 'self'",
		DisableHSTS:  false,
		FrameOptions: "DENY",
	}
}

// Middleware returns an HTTP middleware that adds security headers.
func (s SecurityHeaders) Middleware(next http.Handler) http.Handler {
	csp := s.CSPolicy
	if csp == "" {
		csp = "default-src 'self'"
	}
	frameOpt := s.FrameOptions
	if frameOpt == "" {
		frameOpt = "DENY"
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Prevent MIME type sniffing
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// Prevent clickjacking
		w.Header().Set("X-Frame-Options", frameOpt)

		// XSS protection (legacy, but defense in depth)
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		// Referrer policy
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Permissions policy (restrict browser features)
		w.Header().Set("Permissions-Policy",
			"accelerometer=(), camera=(), geolocation=(), gyroscope=(), "+
				"magnetometer=(), microphone=(), payment=(), usb=()")

		// Content Security Policy
		w.Header().Set("Content-Security-Policy", csp)

		// HSTS - only over HTTPS
		if !s.DisableHSTS && r.TLS != nil {
			w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
		}

		next.ServeHTTP(w, r)
	})
}

// DevSecurityHeaders returns relaxed headers for development (allows CDN Tailwind).
func DevSecurityHeaders() SecurityHeaders {
	return SecurityHeaders{
		CSPolicy: "default-src 'self'; script-src 'self' 'unsafe-inline' cdn.tailwindcss.com; " +
			"style-src 'self' 'unsafe-inline' cdn.tailwindcss.com; " +
			"img-src 'self' data:; font-src 'self'; connect-src 'self' *; " +
			"frame-ancestors 'none'; base-uri 'self'; form-action 'self'",
		DisableHSTS:  true, // HTTP in dev
		FrameOptions: "DENY",
	}
}
