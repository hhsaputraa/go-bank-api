package main

import (
	"encoding/json"
	"net/http"
)

func sendSuccess(w http.ResponseWriter, data interface{}) {
	resp := QueryResponse{
		Status:  "success",
		Message: "Query berhasil dieksekusi",
		Data:    data,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func sendAmbiguous(w http.ResponseWriter, message string, suggestions []string) {
	resp := QueryResponse{
		Status:      "ambiguous",
		Message:     message,
		Suggestions: suggestions,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK) // tetap 200 supaya frontend gampang handle
	json.NewEncoder(w).Encode(resp)
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
	json.NewEncoder(w).Encode(resp)
}
