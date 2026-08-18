package controllers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	auth "go-bank-api/auth"
	config "go-bank-api/config"
	"go-bank-api/constants"
	database "go-bank-api/database"
	utils "go-bank-api/utils"
)

func HandleRegister(w http.ResponseWriter, r *http.Request) {
	// CORS is handled by global middleware

	if r.Method != http.MethodPost {
		utils.SendError(w, http.StatusMethodNotAllowed, constants.ErrCodeMethodNotAllowed, "Hanya POST yang diizinkan")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB body limit
	var req auth.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.SendError(w, http.StatusBadRequest, "INVALID_BODY", "Format JSON salah")
		return
	}

	if req.Username == "" || req.Password == "" {
		utils.SendError(w, http.StatusBadRequest, "INVALID_DATA", "Username dan Password wajib diisi")
		return
	}

	if err := utils.ValidatePassword(req.Password); err != nil {
		utils.SendError(w, http.StatusBadRequest, "INVALID_PASSWORD", err.Error())
		return
	}

	if err := auth.RegisterUser(req); err != nil {
		utils.SendError(w, http.StatusConflict, "REGISTER_FAILED", err.Error())
		return
	}

	utils.SendSuccess(w, map[string]string{"message": "Registrasi berhasil. Silakan login."})
}

func HandleLogin(w http.ResponseWriter, r *http.Request) {
	// CORS is handled by global middleware

	if r.Method != http.MethodPost {
		utils.SendError(w, http.StatusMethodNotAllowed, constants.ErrCodeMethodNotAllowed, "Hanya POST yang diizinkan")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB body limit
	var req auth.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.SendError(w, http.StatusBadRequest, "INVALID_BODY", "Format JSON salah")
		return
	}

	plainUsername, err := utils.DecryptField(req.Username)
	if err != nil {
		log.Printf("[Security] Gagal dekripsi username: %v\n", err)
		utils.SendError(w, http.StatusBadRequest, constants.ErrCodeDecryptFail, "Gagal membaca data rahasia (Username)")
		return
	}
	req.Username = plainUsername

	plainPassword, err := utils.DecryptField(req.Password)
	if err != nil {
		log.Printf("[Security] Gagal dekripsi password: %v\n", err)
		utils.SendError(w, http.StatusBadRequest, constants.ErrCodeDecryptFail, "Gagal membaca data rahasia (Password)")
		return
	}
	req.Password = plainPassword
	req.UserAgent = r.UserAgent()
	req.IPAddress = r.RemoteAddr

	token, mustChangePwd, err := auth.LoginUser(r.Context(), req)
	if err != nil {
		utils.SendError(w, http.StatusUnauthorized, constants.ErrCodeLoginFailed, err.Error())
		return
	}
	isProduction := config.AppConfig.AppEnv == "production"

	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    token,
		Expires:  time.Now().Add(24 * time.Hour),
		HttpOnly: true,
		Secure:   isProduction,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
	})

	utils.SendSuccess(w, map[string]interface{}{
		"message":              "Login berhasil",
		"must_change_password": mustChangePwd,
		"token":                token,
	})
}

func HandleChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.SendError(w, http.StatusMethodNotAllowed, constants.ErrCodeMethodNotAllowed, "Hanya POST yang diizinkan")
		return
	}

	userID, err := utils.GetUserIDFromContext(r.Context().Value(constants.ContextKeyUserID))
	if err != nil {
		utils.SendError(w, http.StatusUnauthorized, constants.ErrCodeUnauthorized, "Token tidak valid")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB body limit
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.SendError(w, http.StatusBadRequest, "INVALID_BODY", "Format JSON salah")
		return
	}


	plainOld, err := utils.DecryptField(req.OldPassword)
	if err != nil {
		utils.SendError(w, http.StatusBadRequest, constants.ErrCodeDecryptFail, "Gagal decrypt password lama")
		return
	}
	plainNew, err := utils.DecryptField(req.NewPassword)
	if err != nil {
		utils.SendError(w, http.StatusBadRequest, constants.ErrCodeDecryptFail, "Gagal decrypt password baru")
		return
	}

	if err := utils.ValidatePassword(plainNew); err != nil {
		utils.SendError(w, http.StatusBadRequest, "WEAK_PASSWORD", err.Error())
		return
	}

	if err := auth.ChangePassword(userID, plainOld, plainNew); err != nil {
		utils.SendError(w, http.StatusBadRequest, "CHANGE_PASSWORD_FAILED", err.Error())
		return
	}

	utils.SendSuccess(w, map[string]string{"message": "Password berhasil diubah. Silakan login kembali."})
}

func HandleLogout(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	// Handle Preflight Request
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		utils.SendError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Hanya POST yang diizinkan")
		return
	}
	var tokenString string

	cookie, err := r.Cookie("auth_token")
	if err == nil {
		tokenString = cookie.Value
	} else {
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 && parts[0] == "Bearer" {
				tokenString = parts[1]
			}
		}
	}

	if tokenString != "" {
		if err := auth.LogoutUser(tokenString); err != nil {

			log.Printf("Warning: Gagal menghapus sesi dari DB: %v", err)
		}
	}

	isProduction := config.AppConfig.AppEnv == "production"

	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    "",              // Kosongkan isi
		Path:     "/",             // Harus sama dengan path saat Login
		Expires:  time.Unix(0, 0), // Set waktu ke masa lalu (Jan 1 1970)
		MaxAge:   -1,              // Instruksi ke browser untuk segera hapus
		HttpOnly: true,
		Secure:   isProduction,
	})
	utils.SendSuccess(w, map[string]string{
		"message": "Logout berhasil. Sesi telah dihapus.",
	})
}

func HandleMe(w http.ResponseWriter, r *http.Request) {
	// CORS is handled by global middleware

	if r.Method != http.MethodGet {
		utils.WriteError(w, http.StatusMethodNotAllowed, constants.ErrCodeMethodNotAllowed, "Hanya GET yang diizinkan")
		return
	}

	// Ambil UserID dari Context (hasil dari AuthMiddleware)
	userID, err := utils.GetUserIDFromContext(r.Context().Value(constants.ContextKeyUserID))
	if err != nil {
		utils.WriteError(w, http.StatusUnauthorized, constants.ErrCodeUnauthorized, "Token tidak valid")
		return
	}

	// Query DB
	var user auth.User
	var isAdminInt, isActiveInt, accountStatusInt interface{}

	// Sesuaikan nama kolom dengan tabel Anda
	query := `
		SELECT id_app_users, username, full_name, email, is_admin, is_active, last_login_at, account_status 
		FROM app_users 
		WHERE id_app_users = :1
	`
	err = database.DbInstance.QueryRowContext(r.Context(), query, userID).Scan(
		&user.ID, &user.Username, &user.FullName, &user.Email,
		&isAdminInt, &isActiveInt, &user.LastLoginAt, &accountStatusInt,
	)


	if err != nil {
		log.Printf("Error get user %d: %v", userID, err)
		utils.WriteError(w, http.StatusInternalServerError, "Gagal ambil data user", "user tidak ditemukan")
		return
	}

	user.IsAdmin = (utils.InterfaceToInt(isAdminInt) == 7)
	user.IsActive = (utils.InterfaceToInt(isActiveInt) == 1)
	user.AccountStatus = utils.InterfaceToInt(accountStatusInt)

	// Response Sukses
	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Data profil user",
		"data": map[string]interface{}{
			"user": user,
		},
	})
}

func HandleLoginOTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.SendError(w, http.StatusMethodNotAllowed, constants.ErrCodeMethodNotAllowed, "Hanya POST yang diizinkan")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB body limit
	var req struct {
		Username string `json:"username"`
		OTP      string `json:"otp"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.SendError(w, http.StatusBadRequest, "INVALID_BODY", "Format JSON salah")
		return
	}


	plainUsername, err := utils.DecryptField(req.Username)
	if err != nil {
		// Just in case username is not encrypted, try raw
		plainUsername = req.Username
		if req.Username == "" {
			utils.SendError(w, http.StatusBadRequest, "INVALID_DATA", "Username wajib diisi")
			return
		}
	}

	// OTP usually is not encrypted, but if it is, decrypt it. Assuming plain for now or match Frontend.
	// Based on request "username dan otp", usually OTP is just 6 digits.

	req.Username = plainUsername
	userAgent := r.UserAgent()
	ipAddress := r.RemoteAddr

	token, accountStatus, err := auth.LoginWithOTP(req.Username, req.OTP, userAgent, ipAddress)
	if err != nil {
		utils.SendError(w, http.StatusUnauthorized, "LOGIN_FAILED", err.Error())
		return
	}

	isProduction := config.AppConfig.AppEnv == "production"
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    token,
		Expires:  time.Now().Add(24 * time.Hour),
		HttpOnly: true,
		Secure:   isProduction,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
	})

	utils.SendSuccess(w, map[string]interface{}{
		"message":        "Login berhasil via OTP",
		"account_status": accountStatus,
		"token":          token,
	})
}
