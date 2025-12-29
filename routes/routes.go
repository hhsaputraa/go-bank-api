package routes

import (
	"net/http"

	auth "go-bank-api/auth"
	controllers "go-bank-api/controllers"
)

func RegisterRoutes() {
	// Public Routes
	http.HandleFunc("/health", controllers.HandleHealthCheck)
	http.HandleFunc("/api/auth/register", controllers.HandleRegister)
	http.HandleFunc("/api/auth/login", controllers.HandleLogin)
	http.HandleFunc("/api/auth/logout", auth.AuthMiddleware(controllers.HandleLogout))
	http.HandleFunc("/api/auth/me", auth.AuthMiddleware(controllers.HandleMe))

	// Protected Routes (Harus Login / Pakai Token)
	http.HandleFunc("/api/query", controllers.HandleDynamicQuery)
	http.HandleFunc("/api/enhance", controllers.HandleEnhancePrompt)

	// Kita bungkus HandleDynamicQuery dengan AuthMiddleware

	http.HandleFunc("/api/feedback/koreksi", auth.AuthMiddleware(controllers.HandleFeedbackKoreksi))
	http.HandleFunc("/admin/retrain", controllers.HandleAdminRetrain)
	http.HandleFunc("/admin/qdrant/list", controllers.HandleAdminListQdrant)
	http.HandleFunc("/admin/qdrant/delete", controllers.HandleAdminDeleteQdrant)
	http.HandleFunc("/admin/cache/create", controllers.HandleAdminCacheCreate)
	http.HandleFunc("/admin/qdrant/update", controllers.HandleAdminQdrantUpdate)
}
