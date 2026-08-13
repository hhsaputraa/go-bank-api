package middleware

import (
	"net/http"
)

// SecurityHeadersMiddleware sets defense-in-depth HTTP security headers on all responses
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Prevent MIME-sniffing
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// Prevent Clickjacking (framing)
		w.Header().Set("X-Frame-Options", "DENY")

		// Enable XSS filter in legacy browsers
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		// Protect referrer leakage
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Restrict permissions
		w.Header().Set("Permissions-Policy", "geolocation=(), camera=(), microphone=()")

		// Content Security Policy for API
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")

		next.ServeHTTP(w, r)
	})
}
