package auth

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	config "go-bank-api/config"
	"go-bank-api/constants"
	database "go-bank-api/database"
	utils "go-bank-api/utils"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID                 int64     `json:"id"`
	Username           string    `json:"username"`
	PasswordHash       string    `json:"-"`
	FullName           string    `json:"full_name"`
	Email              string    `json:"email"`
	IsAdmin            bool      `json:"is_admin"`
	IsActive           bool      `json:"is_active"`
	MustChangePassword bool      `json:"must_change_password"`
	LastLoginAt        time.Time `json:"last_login_at"`
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
	if database.DbInstance == nil {
		return errors.New("database belum terkoneksi")
	}

	hashedPwd, err := HashPassword(req.Password)
	if err != nil {
		return err
	}
	query := `
		INSERT INTO app_users (username, password_hash, full_name, email, is_admin, is_active)
		VALUES (:1, :2, :3, :4, :5, :6)
	`

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = database.DbInstance.ExecContext(ctx, query,
		req.Username, hashedPwd, req.FullName, req.Email,
		constants.RegularUserRole, constants.ActiveUserStatus)
	if err != nil {
		if strings.Contains(err.Error(), "ORA-00001") {
			return errors.New("username sudah digunakan")
		}
		return fmt.Errorf("gagal register: %w", err)
	}
	return nil
}

func LoginUser(req LoginRequest) (string, bool, error) {

	if req.Username == "" {
		log.Println("[AUTH] Gagal: Username kosong")
		return "", false, errors.New("username tidak boleh kosong")
	}

	if req.Password == "" {
		log.Println("[AUTH] Gagal: Password kosong")
		return "", false, errors.New("password tidak boleh kosong")
	}

	if database.DbInstance == nil {
		return "", false, errors.New("database belum terkoneksi")
	}

	log.Printf("[AUTH] Login: User='%s', IP='%s'", req.Username, req.IPAddress)

	var user User
	var isAdmin, isActive, mustChangePassword int

	query := `
		SELECT id_app_users, username, password_hash, full_name, is_admin, is_active, must_change_password
		FROM app_users WHERE username = :1
	`
	err := database.DbInstance.QueryRowContext(context.Background(), query, req.Username).Scan(
		&user.ID, &user.Username, &user.PasswordHash, &user.FullName,
		&isAdmin, &isActive, &mustChangePassword,
	)

	if err != nil {
		log.Printf("[AUTH] User not found or DB error: %v", err)
		return "", false, errors.New("username atau password salah")
	}

	/*
		// Manual conversion/debug
		log.Printf("[DEBUG] Raw MustChange: %v (Type: %T)", mustChangePasswordRaw, mustChangePasswordRaw)

		if val, ok := mustChangePasswordRaw.(int64); ok {
			mustChangePassword = int(val)
		} else if val, ok := mustChangePasswordRaw.(float64); ok {
			mustChangePassword = int(val)
		} else if val, ok := mustChangePasswordRaw.(string); ok {
			// Handle potential string return (some drivers do this for numbers)
			fmt.Sscanf(val, "%d", &mustChangePassword)
		} else {
			// If nil or unknown, default to 0
			mustChangePassword = 0
		}

		log.Printf("[DEBUG] Login: %s, DB IsAdmin: %d, IsActive: %d, MustChange: %d",
			req.Username, isAdmin, isActive, mustChangePassword)
	*/

	if !CheckPasswordHash(req.Password, user.PasswordHash) {
		log.Printf("[AUTH] Wrong password for '%s'", req.Username)
		return "", false, errors.New("username atau password salah")
	}

	if isActive == constants.InactiveUserStatus {
		return "", false, errors.New("akun dinonaktifkan silahkan hubungi administrator")
	}

	expTime := time.Now().Add(24 * time.Hour)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":  user.ID,
		"username": user.Username,
		"is_admin": isAdmin == constants.AdminRoleValue,
		"exp":      expTime.Unix(),
	})

	tokenString, err := token.SignedString([]byte(config.AppConfig.JWTSecret))
	if err != nil {
		return "", false, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = database.DbInstance.ExecContext(ctx, "UPDATE app_users SET last_login_at = CURRENT_TIMESTAMP WHERE id_app_users = :1", user.ID)
	if err != nil {
		log.Printf("[AUTH] WARNING: Gagal update last_login: %v", err)
	}

	_, err = database.DbInstance.ExecContext(ctx, "DELETE FROM user_sessions WHERE id_app_users = :1 AND device_info = :2", user.ID, req.UserAgent)
	if err != nil {
		log.Printf("[AUTH] Warning: Gagal Hapus sesi lama: %v", err)
	}
	_, err = database.DbInstance.ExecContext(ctx, `
		INSERT INTO user_sessions (id_app_users, token, device_info, ip_address, expires_at)
		VALUES (:1, :2, :3, :4, :5)
	`, user.ID, tokenString, req.UserAgent, req.IPAddress, expTime)

	if err != nil {
		log.Printf("[AUTH] Error Critical: Gagal simpan sesi: %v", err)
		return "", false, errors.New("gagal membuat sesi login")
	}

	log.Printf("[AUTH] Login Success: %s", user.Username)
	return tokenString, (mustChangePassword == constants.TrueValue), nil
}

func ChangePassword(userID int64, oldPassword, newPassword string) error {
	if database.DbInstance == nil {
		return errors.New("database belum terkoneksi")
	}
	if oldPassword == "" || newPassword == "" {
		return errors.New("password lama dan baru wajib diisi")
	}

	// 1. Ambil password lama dari DB
	var currentHash string
	queryGet := "SELECT password_hash FROM app_users WHERE id_app_users = :1"
	err := database.DbInstance.QueryRowContext(context.Background(), queryGet, userID).Scan(&currentHash)
	if err != nil {
		return fmt.Errorf("gagal mengambil data user: %w", err)
	}

	// 2. Verifikasi password lama
	if !CheckPasswordHash(oldPassword, currentHash) {
		return errors.New("password lama salah")
	}

	// 3. Validasi password baru tidak boleh sama dengan lama
	if oldPassword == newPassword {
		return errors.New("password baru tidak boleh sama dengan password lama")
	}

	// 4. Hash password baru
	newHash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}

	// 4. Update password & reset flag must_change_password
	queryUpdate := `
		UPDATE app_users 
		SET password_hash = :1, must_change_password = :2, last_login_at = CURRENT_TIMESTAMP 
		WHERE id_app_users = :3
	`
	_, err = database.DbInstance.ExecContext(context.Background(), queryUpdate, newHash, constants.FalseValue, userID)
	if err != nil {
		return fmt.Errorf("gagal update password: %w", err)
	}

	// 5. Invalidate sessions (Optional: Force logout other devices)
	// _, _ = database.DbInstance.Exec("DELETE FROM user_sessions WHERE id_app_users = :1", userID)

	return nil
}

// AuthMiddleware validates JWT tokens and checks session validity
func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// CORS is handled by global CORS middleware, not here

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
				return nil, fmt.Errorf("metode signing tidak valid")
			}
			return []byte(config.AppConfig.JWTSecret), nil
		})

		if err != nil || !token.Valid {
			utils.SendError(w, http.StatusUnauthorized, "INVALID_TOKEN", "Token tidak valid")
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok || !token.Valid {
			utils.SendError(w, http.StatusUnauthorized, "INVALID_TOKEN", "Token claims tidak valid")
			return
		}

		if database.DbInstance != nil {
			var exists int

			checkQuery := "SELECT COUNT(1) FROM user_sessions WHERE token = :1 AND expires_at > CURRENT_TIMESTAMP"

			err := database.DbInstance.QueryRowContext(r.Context(), checkQuery, tokenString).Scan(&exists)

			if err != nil {
				log.Printf("DB Session Error: %v", err)
				utils.SendError(w, http.StatusUnauthorized, "SESSION_ERROR", "Gagal memvalidasi sesi")
				return
			}

			if exists == 0 {
				utils.SendError(w, http.StatusUnauthorized, constants.ErrCodeSessionExpired, "Sesi berakhir")
				return
			}
		}
		ctx := context.WithValue(r.Context(), constants.ContextKeyUserID, claims["user_id"])
		next(w, r.WithContext(ctx))
	}
}

func LogoutUser(tokenString string) error {
	if database.DbInstance == nil {
		return nil
	}
	query := "DELETE FROM user_sessions WHERE token = :1"
	_, err := database.DbInstance.Exec(query, tokenString)
	return err
}
