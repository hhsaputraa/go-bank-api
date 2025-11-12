package main

import "net/http"

func RegisterRoutes() {
	http.HandleFunc("/health", HandleHealthCheck)
	http.HandleFunc("/api/query", HandleDynamicQuery)
	http.HandleFunc("/api/feedback/koreksi", HandleFeedbackKoreksi)
	http.HandleFunc("/admin/retrain", HandleAdminRetrain)
}
