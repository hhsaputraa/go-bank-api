package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strings"
	"time"

	config "go-bank-api/config"
	"go-bank-api/constants"
	database "go-bank-api/database"
	"go-bank-api/middleware"
	utils "go-bank-api/utils"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)




type User struct {
	ID            int64     `json:"id"`
	Username      string    `json:"username"`
	PasswordHash  string    `json:"-"`
	FullName      string    `json:"full_name"`
	Email         string    `json:"email"`
	IsAdmin       bool      `json:"is_admin"`
	IsActive      bool      `json:"is_active"`
	AccountStatus int       `json:"account_status"`
	LastLoginAt   time.Time `json:"last_login_at"`
	OTPCode       string    `json:"-"`
	OTPExpiredAt  time.Time `json:"-"`
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
		err := errors.New("database belum terkoneksi")
		log.Println("[auth][auth_service][RegisterUser] error:", err)
		return err
	}

	hashedPwd, err := HashPassword(req.Password)
	if err != nil {
		log.Println("[auth][auth_service][RegisterUser] error:", err)
		return err
	}
	query := `
		INSERT INTO app_users (username, password_hash, full_name, email, is_admin, is_active, account_status)
		VALUES (:1, :2, :3, :4, :5, :6, :7)
	`

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = database.DbInstance.ExecContext(ctx, query,
		req.Username, hashedPwd, req.FullName, req.Email,
		constants.RegularUserRole, constants.ActiveUserStatus, constants.AccountStatusPendingSetup)
	if err != nil {
		log.Println("[auth][auth_service][RegisterUser] error:", err)
		if strings.Contains(err.Error(), "ORA-00001") {
			return errors.New("username sudah digunakan")
		}
		return fmt.Errorf("gagal register: %w", err)
	}
	return nil
}

func LoginUser(ctx context.Context, req LoginRequest) (string, int, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	if req.Username == "" {
		err := errors.New("username tidak boleh kosong")
		log.Println("[auth][auth_service][LoginUser] error:", err)
		log.Println("[AUTH] Gagal: Username kosong")
		return "", 0, err
	}

	if req.Password == "" {
		err := errors.New("password tidak boleh kosong")
		log.Println("[auth][auth_service][LoginUser] error:", err)
		log.Println("[AUTH] Gagal: Password kosong")
		return "", 0, err
	}

	if database.DbInstance == nil {
		err := errors.New("database belum terkoneksi")
		log.Println("[auth][auth_service][LoginUser] error:", err)
		return "", 0, err
	}

	log.Printf("[AUTH] Login: User='%s', IP='%s'", req.Username, req.IPAddress)

	var user User
	var isAdmin, isActive, accountStatus int

	query := `
		SELECT id_app_users, username, password_hash, full_name, is_admin, is_active, account_status
		FROM app_users WHERE username = :1
	`
	err := database.DbInstance.QueryRowContext(ctx, query, req.Username).Scan(
		&user.ID, &user.Username, &user.PasswordHash, &user.FullName,
		&isAdmin, &isActive, &accountStatus,
	)

	if err != nil {
		log.Println("[auth][auth_service][LoginUser] error:", err)
		log.Printf("[AUTH] User not found or DB error: %v", err)
		return "", 0, errors.New("username atau password salah")
	}

	if !CheckPasswordHash(req.Password, user.PasswordHash) {
		err := errors.New("username atau password salah")
		log.Println("[auth][auth_service][LoginUser] error:", err)
		log.Printf("[AUTH] Wrong password for '%s'", req.Username)
		return "", 0, err
	}

	if isActive == constants.InactiveUserStatus {
		err := errors.New("akun dinonaktifkan silahkan hubungi administrator")
		log.Println("[auth][auth_service][LoginUser] error:", err)
		return "", 0, err
	}

	if accountStatus == constants.AccountStatusBlocked {
		err := errors.New("akun anda diblokir. silahkan hubungi administrator")
		log.Println("[auth][auth_service][LoginUser] error:", err)
		return "", 0, err
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
		log.Println("[auth][auth_service][LoginUser] error:", err)
		return "", 0, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = database.DbInstance.ExecContext(ctx, "UPDATE app_users SET last_login_at = CURRENT_TIMESTAMP WHERE id_app_users = :1", user.ID)
	if err != nil {
		log.Println("[auth][auth_service][LoginUser] error:", err)
		log.Printf("[AUTH] WARNING: Gagal update last_login: %v", err)
	}

	_, err = database.DbInstance.ExecContext(ctx, "DELETE FROM user_sessions WHERE id_app_users = :1 AND device_info = :2", user.ID, req.UserAgent)
	if err != nil {
		log.Println("[auth][auth_service][LoginUser] error:", err)
		log.Printf("[AUTH] Warning: Gagal Hapus sesi lama: %v", err)
	}
	_, err = database.DbInstance.ExecContext(ctx, `
		INSERT INTO user_sessions (id_app_users, token, device_info, ip_address, expires_at)
		VALUES (:1, :2, :3, :4, :5)
	`, user.ID, tokenString, req.UserAgent, req.IPAddress, expTime)

	if err != nil {
		log.Println("[auth][auth_service][LoginUser] error:", err)
		log.Printf("[AUTH] Error Critical: Gagal simpan sesi: %v", err)
		return "", 0, errors.New("gagal membuat sesi login")
	}

	log.Printf("[AUTH] Login Success: %s", user.Username)
	return tokenString, accountStatus, nil
}

func ChangePassword(userID int64, oldPassword, newPassword string) error {
	if database.DbInstance == nil {
		err := errors.New("database belum terkoneksi")
		log.Println("[auth][auth_service][ChangePassword] error:", err)
		return err
	}
	if oldPassword == "" || newPassword == "" {
		err := errors.New("password lama dan baru wajib diisi")
		log.Println("[auth][auth_service][ChangePassword] error:", err)
		return err
	}

	// 1. Ambil password lama dari DB
	var currentHash string
	queryGet := "SELECT password_hash FROM app_users WHERE id_app_users = :1"
	err := database.DbInstance.QueryRowContext(context.Background(), queryGet, userID).Scan(&currentHash)
	if err != nil {
		log.Println("[auth][auth_service][ChangePassword] error:", err)
		return fmt.Errorf("gagal mengambil data user: %w", err)
	}

	// 2. Verifikasi password lama
	if !CheckPasswordHash(oldPassword, currentHash) {
		err := errors.New("password lama salah")
		log.Println("[auth][auth_service][ChangePassword] error:", err)
		return err
	}

	// 3. Validasi password baru tidak boleh sama dengan lama
	if oldPassword == newPassword {
		err := errors.New("password baru tidak boleh sama dengan password lama")
		log.Println("[auth][auth_service][ChangePassword] error:", err)
		return err
	}

	// 4. Hash password baru
	newHash, err := HashPassword(newPassword)
	if err != nil {
		log.Println("[auth][auth_service][ChangePassword] error:", err)
		return err
	}

	// 4. Update password & reset status to PERFECT (0)
	queryUpdate := `
		UPDATE app_users 
		SET password_hash = :1, account_status = :2, last_login_at = CURRENT_TIMESTAMP 
		WHERE id_app_users = :3
	`
	_, err = database.DbInstance.ExecContext(context.Background(), queryUpdate, newHash, constants.AccountStatusPerfect, userID)
	if err != nil {
		log.Println("[auth][auth_service][ChangePassword] error:", err)
		return fmt.Errorf("gagal update password: %w", err)
	}

	// 5. Invalidate sessions across all devices for security
	middleware.InvalidateAllSessions()
	_, err = database.DbInstance.ExecContext(context.Background(), "DELETE FROM user_sessions WHERE id_app_users = :1", userID)
	if err != nil {
		log.Printf("[auth][auth_service][ChangePassword] warning: gagal hapus sesi lama: %v", err)
	}

	return nil

}

// AuthMiddleware delegates to middleware.AuthMiddleware
func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return middleware.AuthMiddleware(next)
}

// AdminMiddleware delegates to middleware.AdminMiddleware
func AdminMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return middleware.AdminMiddleware(next)
}


func LogoutUser(tokenString string) error {
	middleware.InvalidateSessionToken(tokenString)
	if database.DbInstance == nil {
		return nil
	}
	query := "DELETE FROM user_sessions WHERE token = :1"
	_, err := database.DbInstance.Exec(query, tokenString)
	if err != nil {
		log.Println("[auth][auth_service][LogoutUser] error:", err)
	}
	return err
}

// GenerateOTP generates a 6-digit OTP, sets password to defaultPassword, forces change password, and returns the OTP
func GenerateOTP(username, defaultPassword string) (string, error) {
	if database.DbInstance == nil {
		err := errors.New("database belum terkoneksi")
		log.Println("[auth][auth_service][GenerateOTP] error:", err)
		return "", err
	}

	// 1. Generate 6 digit Code using CSPRNG
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		log.Println("[auth][auth_service][GenerateOTP] error:", err)
		return "", fmt.Errorf("gagal membuat kode OTP aman: %w", err)
	}
	otpCode := fmt.Sprintf("%06d", n.Int64())

	// 2. Set Expiry (5 minutes)
	expiry := time.Now().Add(5 * time.Minute)


	// 3. Hash Default Password
	hashed, err := HashPassword(defaultPassword)
	if err != nil {
		log.Println("[auth][auth_service][GenerateOTP] error:", err)
		return "", err
	}

	// 4. Update User: Set OTP, Expiry, New Default Password, AccountStatus=ForgotPassword(2)
	query := `UPDATE app_users SET 
		otp_code = :1, 
		otp_expired_at = :2, 
		password_hash = :3, 
		account_status = :4 
		WHERE username = :5`

	result, err := database.DbInstance.ExecContext(context.Background(), query,
		otpCode, expiry, hashed, constants.AccountStatusForgotPassword, username)

	if err != nil {
		log.Println("[auth][auth_service][GenerateOTP] error:", err)
		return "", fmt.Errorf("gagal update OTP: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return "", errors.New("user tidak ditemukan")
	}

	return otpCode, nil
}

// LoginWithOTP validates OTP and logs the user in, returning token and account status
func LoginWithOTP(username, otp, userAgent, ipAddress string) (string, int, error) {
	if database.DbInstance == nil {
		err := errors.New("database belum terkoneksi")
		log.Println("[auth][auth_service][LoginWithOTP] error:", err)
		return "", 0, err
	}

	var user User
	// 1. Get User Data
	var dbOTP string
	var otpExpiredAt time.Time
	var isAdminInt int
	var isActiveInt int
	var accountStatus int

	// Note: We use string scan for valid OTP check, avoiding NULL issues if OTP is null
	var dbOTPRaw interface{}
	var dbExpiryRaw interface{}

	query := `
		SELECT id_app_users, username, full_name, is_admin, is_active, account_status, otp_code, otp_expired_at
		FROM app_users WHERE username = :1
	`
	err := database.DbInstance.QueryRowContext(context.Background(), query, username).Scan(
		&user.ID, &user.Username, &user.FullName, &isAdminInt, &isActiveInt, &accountStatus, &dbOTPRaw, &dbExpiryRaw,
	)

	if err != nil {
		log.Println("[auth][auth_service][LoginWithOTP] error:", err)
		return "", 0, errors.New("user tidak ditemukan")
	}

	if dbOTPRaw != nil {
		dbOTP = fmt.Sprintf("%v", dbOTPRaw)
	}
	if dbExpiryRaw != nil {
		if t, ok := dbExpiryRaw.(time.Time); ok {
			otpExpiredAt = t
		}
	}

	// 2. Validate OTP with constant-time comparison to prevent timing attacks
	if dbOTP == "" || subtle.ConstantTimeCompare([]byte(dbOTP), []byte(otp)) != 1 {
		return "", 0, errors.New("kode OTP tidak sama")
	}


	// 3. Validate Expiry
	if time.Now().After(otpExpiredAt) {
		return "", 0, errors.New("kode OTP sudah kadaluarsa")
	}

	var isActive bool = (utils.InterfaceToInt(isActiveInt) == constants.ActiveUserStatus)

	if !isActive {
		return "", 0, errors.New("akun dinonaktifkan")
	}

	// 4. Consume OTP (Clear it)
	// We keep must_change_password = 1 (it was set during generation)
	clearOTPQuery := "UPDATE app_users SET otp_code = NULL, otp_expired_at = NULL, last_login_at = CURRENT_TIMESTAMP WHERE id_app_users = :1"
	_, err = database.DbInstance.ExecContext(context.Background(), clearOTPQuery, user.ID)
	if err != nil {
		log.Println("[auth][auth_service][LoginWithOTP] error:", err)
		log.Printf("[AUTH] Warning: Gagal clear OTP user %s: %v", username, err)
	}

	// 5. Generate Token
	expTime := time.Now().Add(24 * time.Hour)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":  user.ID,
		"username": user.Username,
		"is_admin": isAdminInt == constants.AdminRoleValue,
		"exp":      expTime.Unix(),
	})

	tokenString, err := token.SignedString([]byte(config.AppConfig.JWTSecret))
	if err != nil {
		log.Println("[auth][auth_service][LoginWithOTP] error:", err)
		return "", 0, err
	}

	// 6. Create Session
	_, _ = database.DbInstance.Exec("DELETE FROM user_sessions WHERE id_app_users = :1 AND device_info = :2", user.ID, userAgent)
	_, err = database.DbInstance.Exec(`
		INSERT INTO user_sessions (id_app_users, token, device_info, ip_address, expires_at)
		VALUES (:1, :2, :3, :4, :5)
	`, user.ID, tokenString, userAgent, ipAddress, expTime)

	if err != nil {
		log.Println("[auth][auth_service][LoginWithOTP] error:", err)
		return "", 0, errors.New("gagal membuat sesi")
	}

	return tokenString, accountStatus, nil
}
