package routes

import (
	"net/http"
	"time"

	auth "go-bank-api/auth"
	controllers "go-bank-api/controllers"
	"go-bank-api/middleware"
)

func RegisterRoutes(mux *http.ServeMux) {
	// Public Routes
	mux.HandleFunc("/health", controllers.HandleHealthCheck)
	mux.HandleFunc("/api/auth/register", controllers.HandleRegister)
	mux.HandleFunc("/api/auth/login", controllers.HandleLogin)
	mux.HandleFunc("/api/auth/logout", auth.AuthMiddleware(controllers.HandleLogout))
	mux.HandleFunc("/api/auth/me", auth.AuthMiddleware(controllers.HandleMe))

	// Protected Routes (Harus Login / Pakai Token)
	mux.HandleFunc("/api/query", controllers.HandleDynamicQuery)
	mux.HandleFunc("/api/enhance", controllers.HandleEnhancePrompt)

	// Feedback & Admin Routes (Protected)
	mux.HandleFunc("/api/feedback/koreksi", auth.AuthMiddleware(controllers.HandleFeedbackKoreksi))

	// Admin Routes with rate limiting (10 requests per minute)
	rateLimitedAdmin := middleware.RateLimitMiddleware(10, 1*time.Minute)
	mux.Handle("/admin/retrain", rateLimitedAdmin(http.HandlerFunc(controllers.HandleAdminRetrain)))
	mux.HandleFunc("/admin/qdrant/list", controllers.HandleAdminListQdrant)
	mux.HandleFunc("/admin/qdrant/delete", controllers.HandleAdminDeleteQdrant)
	mux.HandleFunc("/admin/cache/create", controllers.HandleAdminCacheCreate)
	mux.HandleFunc("/admin/qdrant/update", controllers.HandleAdminQdrantUpdate)
}
