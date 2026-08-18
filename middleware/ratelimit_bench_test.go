package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func BenchmarkRateLimiterConcurrent(b *testing.B) {
	limiter := NewRateLimiter(50000000, 1*time.Minute)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	var counter int64

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		w := httptest.NewRecorder()
		for pb.Next() {
			c := atomic.AddInt64(&counter, 1)
			ip := fmt.Sprintf("192.168.1.%d", c%100)
			if !limiter.allow(ip) {
				b.Fatalf("rate limiter rejected unexpectedly for IP %s", ip)
			}
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = ip + ":12345"
			handler.ServeHTTP(w, req)
		}
	})
}
