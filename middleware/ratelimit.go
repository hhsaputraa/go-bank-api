package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"go-bank-api/utils"
)

// RateLimiter implements a lock-free map token bucket rate limiter
type RateLimiter struct {
	visitors sync.Map
	rate     int           // requests per window
	window   time.Duration // time window
}

type visitor struct {
	lastSeen time.Time
	tokens   int
	mu       sync.Mutex
}

// NewRateLimiter creates a new rate limiter with concurrent-safe sync.Map
func NewRateLimiter(rate int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		rate:   rate,
		window: window,
	}

	// Cleanup old visitors periodically to prevent memory leaks
	go rl.cleanupVisitors()

	return rl
}

// getVisitor returns or creates the visitor for the given IP without global map lock
func (rl *RateLimiter) getVisitor(ip string) *visitor {
	if val, ok := rl.visitors.Load(ip); ok {
		return val.(*visitor)
	}

	v := &visitor{
		lastSeen: time.Now(),
		tokens:   rl.rate,
	}

	actual, _ := rl.visitors.LoadOrStore(ip, v)
	return actual.(*visitor)
}

// allow checks if the request should be allowed
func (rl *RateLimiter) allow(ip string) bool {
	v := rl.getVisitor(ip)
	v.mu.Lock()
	defer v.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(v.lastSeen)

	if elapsed >= rl.window {
		v.tokens = rl.rate
		v.lastSeen = now
	}

	if v.tokens > 0 {
		v.tokens--
		v.lastSeen = now
		return true
	}

	return false
}

// cleanupVisitors removes idle visitors without stopping concurrent requests
func (rl *RateLimiter) cleanupVisitors() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		rl.visitors.Range(func(key, value interface{}) bool {
			v := value.(*visitor)
			v.mu.Lock()
			stale := now.Sub(v.lastSeen) > 3*time.Minute
			v.mu.Unlock()

			if stale {
				rl.visitors.Delete(key)
			}
			return true
		})
	}
}

// ExtractClientIP accurately extracts the client's IP address by inspecting
// standard proxy headers (X-Forwarded-For, X-Real-IP) and stripping ephemeral ports.
func ExtractClientIP(r *http.Request) string {
	// 1. Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			ip := strings.TrimSpace(ips[0])
			if ip != "" {
				return ip
			}
		}
	}

	// 2. Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		ip := strings.TrimSpace(xri)
		if ip != "" {
			return ip
		}
	}

	// 3. Fallback to RemoteAddr (strip ephemeral port)
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}

	return r.RemoteAddr
}

// RateLimitMiddleware creates a middleware that limits requests per IP
func RateLimitMiddleware(rate int, window time.Duration) func(http.Handler) http.Handler {
	limiter := NewRateLimiter(rate, window)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := ExtractClientIP(r)

			if !limiter.allow(ip) {
				utils.SendError(w, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED",
					"Terlalu banyak permintaan. Silakan coba lagi nanti.")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
