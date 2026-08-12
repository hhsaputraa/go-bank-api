package utils

import (
	"encoding/json"
	"net/http"
)

// APIResponse is the unified response envelope for all API endpoints.
// It covers both general responses and query-specific responses
// (which may include suggestions or structured error codes).
type APIResponse struct {
	Status      string      `json:"status"`
	Message     string      `json:"message,omitempty"`
	Data        interface{} `json:"data,omitempty"`
	Error       string      `json:"error,omitempty"`
	ErrorCode   string      `json:"error_code,omitempty"`
	ErrorDetail string      `json:"error_detail,omitempty"`
	Suggestions []string    `json:"suggestions,omitempty"`
}

func WriteJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, "Gagal encode response JSON", http.StatusInternalServerError)
	}
}

func WriteSuccess(w http.ResponseWriter, message string, data interface{}) {
	WriteJSON(w, http.StatusOK, APIResponse{
		Status:  "success",
		Message: message,
		Data:    data,
	})
}

func WriteError(w http.ResponseWriter, code int, message string, errDetail string) {
	WriteJSON(w, code, APIResponse{
		Status:  "error",
		Message: message,
		Error:   errDetail,
	})
}

