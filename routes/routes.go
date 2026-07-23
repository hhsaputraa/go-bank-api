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
	// Rate Limiters
	authRateLimiter := middleware.RateLimitMiddleware(10, 1*time.Minute)
	queryRateLimiter := middleware.RateLimitMiddleware(30, 1*time.Minute)
	adminRateLimiter := middleware.RateLimitMiddleware(10, 1*time.Minute)

	// Public Routes
	mux.HandleFunc("/health", controllers.HandleHealthCheck)
	mux.Handle("/api/auth/register", authRateLimiter(http.HandlerFunc(controllers.HandleRegister)))
	mux.Handle("/api/auth/login", authRateLimiter(http.HandlerFunc(controllers.HandleLogin)))
	mux.Handle("/api/auth/login-otp", authRateLimiter(http.HandlerFunc(controllers.HandleLoginOTP)))
	mux.Handle("/api/auth/logout", auth.AuthMiddleware(controllers.HandleLogout))
	mux.Handle("/api/auth/me", auth.AuthMiddleware(controllers.HandleMe))
	mux.Handle("/api/auth/change-password", auth.AuthMiddleware(controllers.HandleChangePassword))

	// Protected Routes (Harus Login / Pakai Token + Rate Limit)
	mux.Handle("/api/query", queryRateLimiter(auth.AuthMiddleware(controllers.HandleDynamicQuery)))
	mux.Handle("/api/enhance", queryRateLimiter(auth.AuthMiddleware(controllers.HandleEnhancePrompt)))
	mux.Handle("/api/upload-session", queryRateLimiter(auth.AuthMiddleware(controllers.HandleUploadSession)))
	mux.Handle("/api/chat-session", queryRateLimiter(auth.AuthMiddleware(controllers.HandleChatSession)))

	// Feedback Routes (Protected)
	mux.Handle("/api/feedback/koreksi", auth.AuthMiddleware(controllers.HandleFeedbackKoreksi))

	// Admin Routes (Protected: Harus Login + Role Admin + Rate Limit)
	mux.Handle("/admin/retrain", adminRateLimiter(auth.AdminMiddleware(controllers.HandleAdminRetrain)))
	mux.Handle("/admin/otp/generate", adminRateLimiter(auth.AdminMiddleware(controllers.HandleAdminGenerateOTP)))
	mux.Handle("/admin/users", adminRateLimiter(auth.AdminMiddleware(users.HandleGetAllUsersOracle)))
	mux.Handle("/admin/qdrant/list", adminRateLimiter(auth.AdminMiddleware(controllers.HandleAdminListQdrant)))
	mux.Handle("/admin/qdrant/delete", adminRateLimiter(auth.AdminMiddleware(controllers.HandleAdminDeleteQdrant)))
	mux.Handle("/admin/cache/create", adminRateLimiter(auth.AdminMiddleware(controllers.HandleAdminCacheCreate)))
	mux.Handle("/admin/qdrant/update", adminRateLimiter(auth.AdminMiddleware(controllers.HandleAdminQdrantUpdate)))
}
