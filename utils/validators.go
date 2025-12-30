package utils

import (
	"fmt"
	"regexp"
	"strings"
)

// ValidateSafePrompt checks if the user prompt acts like a dangerous command.
// It does NOT enforce "SELECT" prefix, because prompts are natural language.
func ValidateSafePrompt(prompt string) error {
	q := strings.TrimSpace(strings.ToUpper(prompt))

	// Dangerous keywords that imply data modification or schema access
	dangerousKeywords := []string{
		"DROP", "DELETE", "INSERT", "UPDATE",
		"ALTER", "TRUNCATE", "CREATE", "GRANT", "REVOKE",
		"RENAME", "MERGE", "REPLACE",
	}

	schemaKeywords := []string{
		"ALL_TABS", "ALL_TABLES", "USER_TABLES", "DBA_TABLES", "ALL_VIEWS",
		"DBA_VIEWS", "ALL_SOURCE", "USER_SOURCE", "ALL_USERS", "DBA_USERS",
		"V$", "GV$",
	}

	forbidden := append(dangerousKeywords, schemaKeywords...)

	// We use \b to ensure we match whole words, to avoid blocking "UPDATE_DATE" or typical words.
	pattern := `\b(` + strings.Join(forbidden, "|") + `)\b`
	re := regexp.MustCompile(pattern)

	if re.MatchString(q) {
		match := re.FindString(q)
		return fmt.Errorf("permintaan ditolak: terdeteksi kata kunci terlarang '%s'", match)
	}

	return nil
}

// IsRawSQL checks if the input string looks like a raw SQL command.
// This is used to block users from entering raw SQL in natural language prompts.
func IsRawSQL(input string) bool {
	// Pattern to match common SQL start keywords at the beginning of the string
	pattern := `(?i)^\s*(select|insert|update|delete|drop|alter|truncate|create|grant|revoke|with)\b`
	re := regexp.MustCompile(pattern)
	return re.MatchString(input)
}

// IsReadOnlySQL checks if the SQL query is a valid read-only statement.
// It MUST start with SELECT or WITH.
func IsReadOnlySQL(query string) error {
	q := strings.TrimSpace(strings.ToUpper(query))

	if !strings.HasPrefix(q, "SELECT") && !strings.HasPrefix(q, "WITH") {
		return fmt.Errorf("query harus dimulai dengan SELECT atau WITH")
	}

	// Re-use logic for keywords
	return ValidateSafePrompt(query)
}
