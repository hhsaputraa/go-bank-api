package main

import (
	"encoding/json"
	"net/http"
	"log"
)

func sendSuccess(w http.ResponseWriter, data interface{}) {
	resp := QueryResponse{
		Status:  "success",
		Message: "Query berhasil dieksekusi",
		Data:    data,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("Error encoding success response: %v", err)
	}
}

func sendAmbiguous(w http.ResponseWriter, message string, suggestions []string) {
	resp := QueryResponse{
		Status:      "ambiguous",
		Message:     message,
		Suggestions: suggestions,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("Error encoding ambiguous response: %v", err)
	}
}

func sendError(w http.ResponseWriter, statusCode int, code, message string, details ...string) {
	resp := QueryResponse{
		Status:      "error",
		Message:     message,
		ErrorCode:   code,
		ErrorDetail: "",
	}
	if len(details) > 0 {
		resp.ErrorDetail = details[0]
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("Error encoding error response: %v", err)
	}
}
