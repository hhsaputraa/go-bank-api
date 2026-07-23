package routes

import (
	"net/http"
	"time"

	auth "go-bank-api/auth"
	controllers "go-bank-api/controllers"
	"go-bank-api/middleware"
	users "go-bank-api/modules/users"
)

func RegisterRoutes(mux *http.ServeMux) {
	// Public Routes
	mux.HandleFunc("/health", controllers.HandleHealthCheck)
	mux.HandleFunc("/api/auth/register", controllers.HandleRegister)
	mux.HandleFunc("/api/auth/login", controllers.HandleLogin)
	mux.HandleFunc("/api/auth/logout", auth.AuthMiddleware(controllers.HandleLogout))
	mux.HandleFunc("/api/auth/me", auth.AuthMiddleware(controllers.HandleMe))
	mux.HandleFunc("/api/auth/change-password", auth.AuthMiddleware(controllers.HandleChangePassword))
	mux.HandleFunc("/api/auth/login-otp", controllers.HandleLoginOTP)

	// Protected Routes (Harus Login / Pakai Token)
	mux.HandleFunc("/api/query", controllers.HandleDynamicQuery)
	mux.HandleFunc("/api/enhance", controllers.HandleEnhancePrompt)
	mux.HandleFunc("/api/upload-session", controllers.HandleUploadSession)
	mux.HandleFunc("/api/chat-session", controllers.HandleChatSession)

	// Feedback & Admin Routes (Protected)
	mux.HandleFunc("/api/feedback/koreksi", auth.AuthMiddleware(controllers.HandleFeedbackKoreksi))

	// Admin Routes with rate limiting (10 requests per minute)
	rateLimitedAdmin := middleware.RateLimitMiddleware(10, 1*time.Minute)
	mux.Handle("/admin/retrain", rateLimitedAdmin(http.HandlerFunc(controllers.HandleAdminRetrain)))
	mux.Handle("/admin/otp/generate", rateLimitedAdmin(auth.AuthMiddleware(controllers.HandleAdminGenerateOTP)))
	mux.Handle("/admin/users", (http.HandlerFunc(users.HandleGetAllUsersOracle)))
	mux.HandleFunc("/admin/qdrant/list", controllers.HandleAdminListQdrant)
	mux.HandleFunc("/admin/qdrant/delete", controllers.HandleAdminDeleteQdrant)
	mux.HandleFunc("/admin/cache/create", controllers.HandleAdminCacheCreate)
	mux.HandleFunc("/admin/qdrant/update", controllers.HandleAdminQdrantUpdate)
}
