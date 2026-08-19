package utils

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	strictPatternRegex = regexp.MustCompile(`\b(DROP|DELETE|INSERT|UPDATE|ALTER|TRUNCATE|CREATE|GRANT|REVOKE|RENAME|MERGE|REPLACE|ALL_TABS|ALL_TABLES|USER_TABLES|DBA_TABLES|ALL_VIEWS|DBA_VIEWS|ALL_SOURCE|USER_SOURCE|ALL_USERS|DBA_USERS)\b|V\$[A-Z0-9_]*|GV\$[A-Z0-9_]*`)

	leetPatternsRegexes = []*regexp.Regexp{
		regexp.MustCompile(`UP\s*D[4@]T[3E]`),
		regexp.MustCompile(`D[3E]L[3E]T[3E]`),
	}

	looseDangerousKeywords = []string{
		"HAPUS", "UBAH", "TAMBAH", "HILANGKAN", "LENYAPKAN",
		"BERSIHKAN", "KOSONGKAN", "MUSNAHKAN",
	}

	rawSQLRegex       = regexp.MustCompile(`(?i)^\s*(select|insert|update|delete|drop|alter|truncate|create|grant|revoke|with)\b`)
	hasAlphaNumRegex  = regexp.MustCompile(`[a-zA-Z0-9]`)
)

func ValidateSafePrompt(prompt string) error {
	q := strings.TrimSpace(strings.ToUpper(prompt))
	if strictPatternRegex.MatchString(q) {
		return fmt.Errorf("permintaan ditolak: terdeteksi kata kunci berbahaya (strict match)")
	}

	for _, word := range looseDangerousKeywords {
		if strings.Contains(q, word) {
			return fmt.Errorf("permintaan ditolak: terdeteksi kata kunci berbahaya '%s'", word)
		}
	}
	for _, re := range leetPatternsRegexes {
		if re.MatchString(q) {
			return fmt.Errorf("permintaan ditolak: terdeteksi pola obfuscation/leet")
		}
	}

	return nil
}

func IsRawSQL(input string) bool {
	return rawSQLRegex.MatchString(input)
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

	if !hasAlphaNumRegex.MatchString(trimmed) {
		return fmt.Errorf("pertanyaan tidak valid")
	}
	return nil
}

func ValidatePassword(password string) error {
	if len(password) < 6 {
		return fmt.Errorf("password minimal 6 karakter")
	}

	hasUpper := false
	hasDigit := false
	hasSymbol := false

	for _, r := range password {
		switch {
		case r >= 'A' && r <= 'Z':
			hasUpper = true
		case r >= 'a' && r <= 'z':
			// valid lowercase
		case r >= '0' && r <= '9':
			hasDigit = true
		case r == '!' || r == '@' || r == '#' || r == '$':
			hasSymbol = true
		default:
			return fmt.Errorf("password hanya boleh mengandung huruf, angka, dan simbol (!@#$)")
		}
	}

	if !hasUpper {
		return fmt.Errorf("password harus memiliki minimal 1 huruf besar (A-Z)")
	}
	if !hasDigit {
		return fmt.Errorf("password harus memiliki minimal 1 angka (0-9)")
	}
	if !hasSymbol {
		return fmt.Errorf("password harus memiliki minimal 1 simbol (!@#$)")
	}
	return nil
}
