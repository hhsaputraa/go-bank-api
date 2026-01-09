package utils

import (
	"testing"
)

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		password string
		isValid  bool
		name     string
	}{
		{"abc", false, "Too short"},
		{"abcdef", false, "No uppercase, no digit, no symbol"},
		{"Abcdef", false, "No digit, no symbol"},
		{"Abcdef1", false, "No symbol"},
		{"Abcdef1!", true, "Valid with !"},
		{"Abcdef1@", true, "Valid with @"},
		{"Abcdef1#", true, "Valid with #"},
		{"Abcdef1$", true, "Valid with $"},
		{"Abcdef1%", false, "Invalid symbol (only !@#$ allowed per strict interpretation)"},
		{"ABCDEF1!", true, "Valid all caps"},
		{"abcdef1!", false, "No uppercase"},
		{"Abcdefg!", false, "No digit"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.password)
			if (err == nil) != tt.isValid {
				t.Errorf("ValidatePassword(%q) = %v; want isValid=%v. Error: %v", tt.password, err == nil, tt.isValid, err)
			}
		})
	}
}
