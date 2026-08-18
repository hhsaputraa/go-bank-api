package middleware

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	config "go-bank-api/config"
	constants "go-bank-api/constants"
	database "go-bank-api/database"
	utils "go-bank-api/utils"

	jwt "github.com/golang-jwt/jwt/v5"
)

type sessionCacheItem struct {
	validUntil time.Time
	isValid    bool
}

var (
	sessionCache      sync.Map
	sessionCacheTTL   = 60 * time.Second
	cleanupSessionOnce sync.Once
)

func initSessionCleaner() {
	cleanupSessionOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(2 * time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				now := time.Now()
				sessionCache.Range(func(key, value interface{}) bool {
					item := value.(sessionCacheItem)
					if now.After(item.validUntil) {
						sessionCache.Delete(key)
					}
					return true
				})
			}
		}()
	})
}

// InvalidateSessionToken removes a specific token from the in-memory session cache (e.g. on logout)
func InvalidateSessionToken(tokenString string) {
	sessionCache.Delete(tokenString)
}

// InvalidateAllSessions clears all cached sessions (e.g. on mass password reset or admin actions)
func InvalidateAllSessions() {
	sessionCache.Range(func(key, value interface{}) bool {
		sessionCache.Delete(key)
		return true
	})
}

// isSessionValid checks the session validity with L1 In-Memory Caching to avoid DB roundtrips on every request
func isSessionValid(ctx context.Context, tokenString string) (bool, error) {
	initSessionCleaner()

	// 1. Check L1 Memory Cache
	if val, ok := sessionCache.Load(tokenString); ok {
		item := val.(sessionCacheItem)
		if time.Now().Before(item.validUntil) {
			return item.isValid, nil
		}
		// Expired cache item
		sessionCache.Delete(tokenString)
	}

	if database.DbInstance == nil {
		return true, nil
	}

	// 2. Query Oracle DB (L2 check)
	var exists int
	checkQuery := "SELECT COUNT(1) FROM user_sessions WHERE token = :1 AND expires_at > CURRENT_TIMESTAMP"
	err := database.DbInstance.QueryRowContext(ctx, checkQuery, tokenString).Scan(&exists)
	if err != nil {
		return false, err
	}

	isValid := exists > 0

	// 3. Store in L1 Cache
	sessionCache.Store(tokenString, sessionCacheItem{
		validUntil: time.Now().Add(sessionCacheTTL),
		isValid:    isValid,
	})

	return isValid, nil
}

// AuthMiddleware validates JWT tokens and checks session validity with L1 cache
func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var tokenString string
		cookie, err := r.Cookie("auth_token")
		if err == nil {
			tokenString = cookie.Value
		}

		if tokenString == "" {
			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				parts := strings.Split(authHeader, " ")
				if len(parts) == 2 && parts[0] == "Bearer" {
					tokenString = parts[1]
				}
			}
		}

		if tokenString == "" {
			utils.SendError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Token diperlukan")
			return
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				err := fmt.Errorf("metode signing tidak valid")
				log.Println("[middleware][AuthMiddleware] error:", err)
				return nil, err
			}
			return []byte(config.AppConfig.JWTSecret), nil
		})

		if err != nil || !token.Valid {
			if err != nil {
				log.Println("[middleware][AuthMiddleware] error:", err)
			}
			utils.SendError(w, http.StatusUnauthorized, "INVALID_TOKEN", "Token tidak valid")
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok || !token.Valid {
			err := fmt.Errorf("token claims tidak valid")
			log.Println("[middleware][AuthMiddleware] error:", err)
			utils.SendError(w, http.StatusUnauthorized, "INVALID_TOKEN", "Token claims tidak valid")
			return
		}

		// Fast session validation via L1 In-Memory Cache
		validSession, dbErr := isSessionValid(r.Context(), tokenString)
		if dbErr != nil {
			log.Printf("[middleware][AuthMiddleware] DB Session Error: %v", dbErr)
			utils.SendError(w, http.StatusUnauthorized, "SESSION_ERROR", "Gagal memvalidasi sesi")
			return
		}

		if !validSession {
			utils.SendError(w, http.StatusUnauthorized, constants.ErrCodeSessionExpired, "Sesi berakhir")
			return
		}

		ctx := context.WithValue(r.Context(), constants.ContextKeyUserID, claims["user_id"])
		if username, ok := claims["username"].(string); ok {
			ctx = context.WithValue(ctx, constants.ContextKeyUsername, username)
		}
		if isAdmin, ok := claims["is_admin"].(bool); ok {
			ctx = context.WithValue(ctx, constants.ContextKeyIsAdmin, isAdmin)
		}
		next(w, r.WithContext(ctx))
	}
}

// AdminMiddleware validates JWT token, session, and requires administrator privileges
func AdminMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		isAdmin, ok := r.Context().Value(constants.ContextKeyIsAdmin).(bool)
		if !ok || !isAdmin {
			utils.SendError(w, http.StatusForbidden, "FORBIDDEN", "Akses ditolak: Membutuhkan hak akses Administrator")
			return
		}
		next(w, r)
	})
}
