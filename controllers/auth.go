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

	token, err := auth.LoginUser(req)
	if err != nil {
		utils.SendError(w, http.StatusUnauthorized, constants.ErrCodeLoginFailed, err.Error())
		return
	}
	isProduction := config.AppConfig.AppEnv == "priduction"

	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    token,
		Expires:  time.Now().Add(24 * time.Hour),
		HttpOnly: true,
		Secure:   isProduction,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
	})

	utils.SendSuccess(w, map[string]string{
		"message": "Login berhasil",
	})
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
	userIDVal := r.Context().Value(constants.ContextKeyUserID)
	if userIDVal == nil {
		utils.WriteError(w, http.StatusUnauthorized, constants.ErrCodeUnauthorized, "Token tidak valid")
		return
	}

	// Konversi UserID (JWT numeric biasanya float64)
	var userID int64
	if v, ok := userIDVal.(float64); ok {
		userID = int64(v)
	} else if v, ok := userIDVal.(int64); ok { // Jaga-jaga kalau formatnya int
		userID = v
	} else {
		utils.WriteError(w, http.StatusInternalServerError, "Format User ID salah", "User ID context salah format")
		return
	}

	// Query DB
	var user auth.User
	var isAdminInt, isActiveInt int

	// Sesuaikan nama kolom dengan tabel Anda
	query := `
		SELECT id_app_users, username, full_name, email, is_admin, is_active, last_login_at 
		FROM app_users 
		WHERE id_app_users = :1
	`
	err := database.DbInstance.QueryRowContext(r.Context(), query, userID).Scan(
		&user.ID, &user.Username, &user.FullName, &user.Email,
		&isAdminInt, &isActiveInt, &user.LastLoginAt,
	)

	if err != nil {
		log.Printf("Error get user %d: %v", userID, err)
		utils.WriteError(w, http.StatusInternalServerError, "Gagal ambil data user", "user tidak ditemukan")
		return
	}

	user.IsAdmin = (isAdminInt == 7)
	user.IsActive = (isActiveInt == 1)

	// Response Sukses
	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Data profil user",
		"data": map[string]interface{}{
			"user": user,
		},
	})
}
