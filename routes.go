package main

import "net/http"

func RegisterRoutes() {
	http.HandleFunc("/health", HandleHealthCheck)
	http.HandleFunc("/api/query", HandleDynamicQuery)
}
