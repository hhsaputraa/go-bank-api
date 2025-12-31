package utils

import (
	"encoding/json"
	"fmt"
	models "go-bank-api/models"
	"log"
	"net/http"
	"regexp"
)

func SendSuccess(w http.ResponseWriter, data interface{}) {
	resp := models.QueryResponse{
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

func SendAmbiguous(w http.ResponseWriter, message string, suggestions []string) {
	resp := models.QueryResponse{
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

func SendError(w http.ResponseWriter, statusCode int, code, message string, details ...string) {
	resp := models.QueryResponse{
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

func ValidateIdentifier(name string) error {
	validPattern := regexp.MustCompile(`^[A-Z0-9_$#]+$`)
	if !validPattern.MatchString(name) {
		return fmt.Errorf("invalid identifier detected: %s", name)
	}
	return nil
}
