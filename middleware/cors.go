package middleware

import (
	"go-bank-api/config"
	"net/http"
	"strings"
)

// CORSMiddleware handles Cross-Origin Resource Sharing (CORS) headers
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowedOrigins := getAllowedOrigins()

		// Always handle origin if provided
		if origin != "" {
			if isOriginAllowed(origin, allowedOrigins) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			} else {
				// Fallback: If in development mode, reflect origin
				if config.AppConfig == nil || config.AppConfig.AppEnv == "development" || len(allowedOrigins) == 0 {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
			}
		} else {
			// Default fallback for requests without Origin header
			if len(allowedOrigins) > 0 && allowedOrigins[0] != "*" {
				w.Header().Set("Access-Control-Allow-Origin", allowedOrigins[0])
			} else {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			}
		}

		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")

		// Dynamically allow requested headers or fallback to standard defaults
		reqHeaders := r.Header.Get("Access-Control-Request-Headers")
		if reqHeaders != "" {
			w.Header().Set("Access-Control-Allow-Headers", reqHeaders)
		} else {
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, Accept, Origin")
		}

		w.Header().Set("Access-Control-Max-Age", "3600")

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// getAllowedOrigins returns the list of allowed origins from config
func getAllowedOrigins() []string {
	if config.AppConfig == nil || config.AppConfig.FrontendURL == "" {
		return []string{"http://localhost:5173", "http://192.168.55.192:5173", "http://localhost:3084", "http://localhost:3000"}
	}

	origins := strings.Split(config.AppConfig.FrontendURL, ",")
	var result []string
	for _, origin := range origins {
		trimmed := strings.TrimSpace(origin)
		trimmed = strings.TrimSuffix(trimmed, "/")
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// isOriginAllowed checks if the given origin is in the allowed list
func isOriginAllowed(origin string, allowedOrigins []string) bool {
	cleanOrigin := strings.TrimSuffix(strings.TrimSpace(origin), "/")
	for _, allowed := range allowedOrigins {
		cleanAllowed := strings.TrimSuffix(strings.TrimSpace(allowed), "/")
		if cleanAllowed == "*" || cleanAllowed == cleanOrigin {
			return true
		}
		// Allow any local development origin
		if strings.HasPrefix(cleanOrigin, "http://localhost:") || strings.HasPrefix(cleanOrigin, "http://127.0.0.1:") {
			return true
		}
	}
	return false
}
