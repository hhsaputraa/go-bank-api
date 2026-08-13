package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestExtractClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		wantIP     string
	}{
		{
			name:       "Standard RemoteAddr with port",
			remoteAddr: "192.168.1.100:54321",
			headers:    nil,
			wantIP:     "192.168.1.100",
		},
		{
			name:       "IPv6 RemoteAddr with port",
			remoteAddr: "[::1]:54321",
			headers:    nil,
			wantIP:     "::1",
		},
		{
			name:       "X-Forwarded-For single IP",
			remoteAddr: "127.0.0.1:8080",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.195"},
			wantIP:     "203.0.113.195",
		},
		{
			name:       "X-Forwarded-For multiple IPs (first is client)",
			remoteAddr: "127.0.0.1:8080",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.195, 70.41.3.18, 150.172.238.178"},
			wantIP:     "203.0.113.195",
		},
		{
			name:       "X-Real-IP header",
			remoteAddr: "127.0.0.1:8080",
			headers:    map[string]string{"X-Real-IP": "198.51.100.1"},
			wantIP:     "198.51.100.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/test", nil)
			req.RemoteAddr = tt.remoteAddr
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			got := ExtractClientIP(req)
			if got != tt.wantIP {
				t.Errorf("ExtractClientIP() = %v, want %v", got, tt.wantIP)
			}
		})
	}
}

func TestSecurityHeadersMiddleware(t *testing.T) {
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := SecurityHeadersMiddleware(dummyHandler)
	req := httptest.NewRequest("GET", "/api/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	expectedHeaders := map[string]string{
		"X-Content-Type-Options":    "nosniff",
		"X-Frame-Options":           "DENY",
		"X-XSS-Protection":          "1; mode=block",
		"Referrer-Policy":           "strict-origin-when-cross-origin",
		"Content-Security-Policy":   "default-src 'none'; frame-ancestors 'none'",
		"Permissions-Policy":        "geolocation=(), camera=(), microphone=()",
	}

	for header, expectedVal := range expectedHeaders {
		gotVal := rr.Header().Get(header)
		if gotVal != expectedVal {
			t.Errorf("Header %q = %q, want %q", header, gotVal, expectedVal)
		}
	}
}

func TestRateLimitMiddlewareWithPortStripping(t *testing.T) {
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Rate limit: 2 requests per 100ms
	rl := RateLimitMiddleware(2, 100*time.Millisecond)
	handler := rl(dummyHandler)

	// 1st request from port 5001
	req1 := httptest.NewRequest("GET", "/api/test", nil)
	req1.RemoteAddr = "10.0.0.1:5001"
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)
	if rr1.Code != http.StatusOK {
		t.Fatalf("Req 1 failed: code %d", rr1.Code)
	}

	// 2nd request from port 5002 (different port, same IP)
	req2 := httptest.NewRequest("GET", "/api/test", nil)
	req2.RemoteAddr = "10.0.0.1:5002"
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("Req 2 failed: code %d", rr2.Code)
	}

	// 3rd request from port 5003 (exceeds limit!)
	req3 := httptest.NewRequest("GET", "/api/test", nil)
	req3.RemoteAddr = "10.0.0.1:5003"
	rr3 := httptest.NewRecorder()
	handler.ServeHTTP(rr3, req3)
	if rr3.Code != http.StatusTooManyRequests {
		t.Fatalf("Req 3 should be rate-limited, got code %d", rr3.Code)
	}
}
