package main

import "net/http"

// routes.go

func RegisterRoutes() {
	// Public Routes
	http.HandleFunc("/health", HandleHealthCheck)
	http.HandleFunc("/api/auth/register", HandleRegister)
	http.HandleFunc("/api/auth/login", HandleLogin)
	http.HandleFunc("/api/auth/logout", AuthMiddleware(HandleLogout))
	http.HandleFunc("/api/auth/me", AuthMiddleware(HandleMe))

	// Protected Routes (Harus Login / Pakai Token)
	http.HandleFunc("/api/query", HandleDynamicQuery)
	http.HandleFunc("/api/enhance", HandleEnhancePrompt)

	// Kita bungkus HandleDynamicQuery dengan AuthMiddleware

	http.HandleFunc("/api/feedback/koreksi", AuthMiddleware(HandleFeedbackKoreksi))
	http.HandleFunc("/admin/retrain", HandleAdminRetrain)
	http.HandleFunc("/admin/qdrant/list", HandleAdminListQdrant)
	http.HandleFunc("/admin/qdrant/delete", HandleAdminDeleteQdrant)
	http.HandleFunc("/admin/cache/create", HandleAdminCacheCreate)
	http.HandleFunc("/admin/qdrant/update", HandleAdminQdrantUpdate)
}
