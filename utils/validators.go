package utils

import (
	"fmt"
	"regexp"
	"strings"
)

func ValidateSafePrompt(prompt string) error {
	q := strings.TrimSpace(strings.ToUpper(prompt))
	strictKeywords := []string{
		"DROP", "DELETE", "INSERT", "UPDATE",
		"ALTER", "TRUNCATE", "CREATE", "GRANT", "REVOKE",
		"RENAME", "MERGE", "REPLACE",
	}
	looseKeywords := []string{
		"HAPUS", "UBAH", "TAMBAH", "HILANGKAN", "LENYAPKAN",
		"BERSIHKAN", "KOSONGKAN", "MUSNAHKAN",
	}
	leetPatterns := []string{
		`UP\s*D[4@]T[3E]`,
		`D[3E]L[3E]T[3E]`,
	}
	schemaKeywords := []string{
		"ALL_TABS", "ALL_TABLES", "USER_TABLES", "DBA_TABLES", "ALL_VIEWS",
		"DBA_VIEWS", "ALL_SOURCE", "USER_SOURCE", "ALL_USERS", "DBA_USERS",
		"V$", "GV$",
	}
	strictPattern := `\b(` + strings.Join(append(strictKeywords, schemaKeywords...), "|") + `)\b`
	if regexp.MustCompile(strictPattern).MatchString(q) {
		return fmt.Errorf("permintaan ditolak: terdeteksi kata kunci berbahaya (strict match)")
	}
	for _, word := range looseKeywords {
		if strings.Contains(q, word) {
			return fmt.Errorf("permintaan ditolak: terdeteksi kata kunci berbahaya '%s'", word)
		}
	}
	for _, pat := range leetPatterns {
		if regexp.MustCompile(pat).MatchString(q) {
			return fmt.Errorf("permintaan ditolak: terdeteksi pola obfuscation/leet")
		}
	}

	return nil
}
func IsRawSQL(input string) bool {
	pattern := `(?i)^\s*(select|insert|update|delete|drop|alter|truncate|create|grant|revoke|with)\b`
	re := regexp.MustCompile(pattern)
	return re.MatchString(input)
}
func IsReadOnlySQL(query string) error {
	q := strings.TrimSpace(strings.ToUpper(query))

	if !strings.HasPrefix(q, "SELECT") && !strings.HasPrefix(q, "WITH") {
		return fmt.Errorf("query harus dimulai dengan SELECT atau WITH")
	}
	return ValidateSafePrompt(query)
}

func ValidatePromptSanity(prompt string) error {
	trimmed := strings.TrimSpace(prompt)
	if len(trimmed) < 4 {
		return fmt.Errorf("pertanyaan terlalu pendek (minimal 4 karakter)")
	}

	hasAlpha := regexp.MustCompile(`[a-zA-Z0-9]`).MatchString(trimmed)
	if !hasAlpha {
		return fmt.Errorf("pertanyaan tidak valid")
	}
	return nil
}
