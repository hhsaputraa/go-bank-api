package main

import "net/http"

// routes.go

func RegisterRoutes() {
	// Public Routes (Bisa diakses siapa saja)
	http.HandleFunc("/health", HandleHealthCheck)
	http.HandleFunc("/api/auth/register", HandleRegister) // Endpoint Baru
	http.HandleFunc("/api/auth/login", HandleLogin)       // Endpoint Baru
	http.HandleFunc("/api/auth/logout", AuthMiddleware(HandleLogout))

	// Protected Routes (Harus Login / Pakai Token)
	// Kita bungkus HandleDynamicQuery dengan AuthMiddleware
	http.HandleFunc("/api/query", AuthMiddleware(HandleDynamicQuery))
	
	// Endpoint Admin juga sebaiknya diproteksi
	http.HandleFunc("/api/feedback/koreksi", AuthMiddleware(HandleFeedbackKoreksi))
	
	// Admin Routes (Harusnya punya middleware khusus admin, tapi pakai AuthMiddleware dulu gapapa)
	http.HandleFunc("/admin/retrain", AuthMiddleware(HandleAdminRetrain))
	http.HandleFunc("/admin/qdrant/list", AuthMiddleware(HandleAdminListQdrant))
	http.HandleFunc("/admin/qdrant/delete", AuthMiddleware(HandleAdminDeleteQdrant))
	http.HandleFunc("/admin/cache/create", AuthMiddleware(HandleAdminCacheCreate))
	http.HandleFunc("/admin/qdrant/update", AuthMiddleware(HandleAdminQdrantUpdate))
}
