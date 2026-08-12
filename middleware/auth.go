package middleware

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	config "go-bank-api/config"
	constants "go-bank-api/constants"
	database "go-bank-api/database"
	utils "go-bank-api/utils"

	jwt "github.com/golang-jwt/jwt/v5"
)

// AuthMiddleware validates JWT tokens and checks session validity from cookie or Bearer header
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

		if database.DbInstance != nil {
			var exists int
			checkQuery := "SELECT COUNT(1) FROM user_sessions WHERE token = :1 AND expires_at > CURRENT_TIMESTAMP"
			err := database.DbInstance.QueryRowContext(r.Context(), checkQuery, tokenString).Scan(&exists)

			if err != nil {
				log.Printf("[middleware][AuthMiddleware] DB Session Error: %v", err)
				utils.SendError(w, http.StatusUnauthorized, "SESSION_ERROR", "Gagal memvalidasi sesi")
				return
			}

			if exists == 0 {
				utils.SendError(w, http.StatusUnauthorized, constants.ErrCodeSessionExpired, "Sesi berakhir")
				return
			}
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
