package main

import "net/http"

func RegisterRoutes() {
	http.HandleFunc("/health", HandleHealthCheck)
	http.HandleFunc("/api/query", HandleDynamicQuery)
	http.HandleFunc("/api/feedback/koreksi", HandleFeedbackKoreksi)
	http.HandleFunc("/admin/retrain", HandleAdminRetrain)
	http.HandleFunc("/admin/qdrant/list", HandleAdminListQdrant)
	// http.HandleFunc("/admin/qdrant/delete", HandleAdminDeleteQdrant)
	http.HandleFunc("/admin/cache/create", HandleAdminCacheCreate)
	http.HandleFunc("/admin/qdrant/update", HandleAdminQdrantUpdate)
	http.HandleFunc("/api/cache/forget", HandleForgetCache)
}
