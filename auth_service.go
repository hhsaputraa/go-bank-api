package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"log"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	FullName     string    `json:"full_name"`
	Email        string    `json:"email"`
	IsAdmin      bool      `json:"is_admin"`
	IsActive     bool      `json:"is_active"`
	LastLoginAt  time.Time `json:"last_login_at"`
}

type LoginRequest struct {
	Username  string `json:"username"`
	Password  string `json:"password"`
	UserAgent string `json:"-"`
	IPAddress string `json:"-"`
}

type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	FullName string `json:"full_name"`
	Email    string `json:"email"`
}



func HashPassword(password string) (string, error) {
    bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    return string(bytes), err
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func RegisterUser(req RegisterRequest) error {
	if DbInstance == nil {
		return errors.New("database belum terkoneksi")
	}

	hashedPwd, err := HashPassword(req.Password)
	if err != nil {
		return err
	}
	query := `
		INSERT INTO app_users (username, password_hash, full_name, email, is_admin, is_active) 
		VALUES (:1, :2, :3, :4, 0, 1)
	`

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = DbInstance.ExecContext(ctx, query, req.Username, hashedPwd, req.FullName, req.Email)
	if err != nil {
		if strings.Contains(err.Error(), "ORA-00001") {
			return errors.New("username sudah digunakan")
		}
		return fmt.Errorf("gagal register: %w", err)
	}
	return nil
}

func LoginUser(req LoginRequest) (string, error) {

	if req.Username == "" {
		log.Println("[AUTH] Gagal: Username kosong")
		return "", errors.New("username tidak boleh kosong")
	}

	if req.Password == "" {
		log.Println("[AUTH] Gagal: Password kosong")
		return "", errors.New("password tidak boleh kosong")
	}

	if DbInstance == nil {
		return "", errors.New("database belum terkoneksi")
	}

	log.Printf("[AUTH] Login: User='%s', IP='%s'", req.Username, req.IPAddress)

	var user User
	var isAdmin, isActive int

	query := `
		SELECT id_app_users, username, password_hash, full_name, is_admin, is_active 
		FROM app_users WHERE username = :1
	`
	err := DbInstance.QueryRowContext(context.Background(), query, req.Username).Scan(
		&user.ID, &user.Username, &user.PasswordHash, &user.FullName, 
		&isAdmin, &isActive,
	)

	if err != nil {
		log.Printf("[AUTH] User not found: %v", err)
		return "", errors.New("username atau password salah")
	}

	if !CheckPasswordHash(req.Password, user.PasswordHash) {
		log.Printf("[AUTH] Wrong password for '%s'", req.Username)
		return "", errors.New("username atau password salah")
	}

	if isActive == 0 {
		return "", errors.New("akun dinonaktifkan")
	}

	expTime := time.Now().Add(24 * time.Hour)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":  user.ID,
		"username": user.Username,
		"is_admin": isAdmin == 7,
		"exp":      expTime.Unix(),
	})

	tokenString, err := token.SignedString([]byte(AppConfig.JWTSecret))
	if err != nil {
		return "", err
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		DbInstance.ExecContext(ctx, "UPDATE app_users SET last_login_at = CURRENT_TIMESTAMP WHERE id_app_users = :1", user.ID)

		DbInstance.ExecContext(ctx, "DELETE FROM user_sessions WHERE id_app_users = :1 AND device_info = :2", user.ID, req.UserAgent)

		DbInstance.ExecContext(ctx, `
			INSERT INTO user_sessions (id_app_users, token, device_info, ip_address, expires_at)
			VALUES (:1, :2, :3, :4, :5)
		`, user.ID, tokenString, req.UserAgent, req.IPAddress, expTime)
	}()

	log.Printf("[AUTH] Login Success: %s", user.Username)
	return tokenString, nil
}

func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			sendError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Token autentikasi diperlukan")
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			sendError(w, http.StatusUnauthorized, "INVALID_TOKEN", "Format token salah")
			return
		}

		tokenString := parts[1]

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("metode signing tidak valid")
			}
			return []byte(AppConfig.JWTSecret), nil
		})

		if err != nil || !token.Valid {
			sendError(w, http.StatusUnauthorized, "INVALID_TOKEN", "Token tidak valid")
			return
		}

		if DbInstance != nil {
			var exists int
			checkQuery := "SELECT COUNT(1) FROM user_sessions WHERE token = :1"
			
			err := DbInstance.QueryRowContext(r.Context(), checkQuery, tokenString).Scan(&exists)
			
			if err != nil || exists == 0 {
				sendError(w, http.StatusUnauthorized, "SESSION_EXPIRED", "Sesi anda telah berakhir atau login di perangkat lain")
				return
			}
		}

		next(w, r)
	}
}

func LogoutUser(tokenString string) error {
	if DbInstance == nil {
		return nil
	}
	query := "DELETE FROM user_sessions WHERE token = :1"
	_, err := DbInstance.Exec(query, tokenString)
	return err
}