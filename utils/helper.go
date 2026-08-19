package utils

import (
	"fmt"
	"net/http"
	"regexp"
)

// SendSuccess writes a success response for query endpoints.
func SendSuccess(w http.ResponseWriter, data interface{}) {
	WriteJSON(w, http.StatusOK, APIResponse{
		Status:  "success",
		Message: "Query berhasil dieksekusi",
		Data:    data,
	})
}

// SendAmbiguous writes an ambiguous-intent response with suggestions.
func SendAmbiguous(w http.ResponseWriter, message string, suggestions []string) {
	WriteJSON(w, http.StatusOK, APIResponse{
		Status:      "ambiguous",
		Message:     message,
		Suggestions: suggestions,
	})
}

// SendError writes a structured error response with an error code.
func SendError(w http.ResponseWriter, statusCode int, code, message string, details ...string) {
	resp := APIResponse{
		Status:    "error",
		Message:   message,
		ErrorCode: code,
	}
	if len(details) > 0 {
		resp.ErrorDetail = details[0]
	}
	WriteJSON(w, statusCode, resp)
}

var validIdentifierPattern = regexp.MustCompile(`^[A-Z0-9_$#]+$`)

func ValidateIdentifier(name string) error {
	if !validIdentifierPattern.MatchString(name) {
		return fmt.Errorf("invalid identifier detected: %s", name)
	}
	return nil
}

// GetUserIDFromContext safely extracts and casts the UserID stored in context by AuthMiddleware.
func GetUserIDFromContext(userIDVal interface{}) (int64, error) {
	if userIDVal == nil {
		return 0, fmt.Errorf("user ID not found in context")
	}
	if v, ok := userIDVal.(float64); ok {
		return int64(v), nil
	}
	if v, ok := userIDVal.(int64); ok {
		return v, nil
	}
	if v, ok := userIDVal.(int); ok {
		return int64(v), nil
	}
	return 0, fmt.Errorf("user ID context is invalid type: %T", userIDVal)
}


