package utils

import (
	"testing"
)

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		isValid  bool
	}{
		{"Too short", "abc", false},
		{"Too short with symbols", "Ab1!", false},
		{"No uppercase, no digit, no symbol", "abcdef", false},
		{"No digit, no symbol", "Abcdef", false},
		{"No symbol", "Abcdef1", false},
		{"Valid with !", "Abcdef1!", true},
		{"Valid with @", "Abcdef1@", true},
		{"Valid with #", "Abcdef1#", true},
		{"Valid with $", "Abcdef1$", true},
		{"Invalid symbol %", "Abcdef1%", false},
		{"Invalid symbol ^", "Abcdef1^", false},
		{"Invalid symbol &", "Abcdef1&", false},
		{"Valid all caps", "ABCDEF1!", true},
		{"No uppercase", "abcdef1!", false},
		{"No digit", "Abcdefg!", false},
		{"Empty string", "", false},
		{"Whitespace only", "      ", false},
		{"Valid complex", "BankSupra2025#", true},
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

func TestValidateSafePrompt(t *testing.T) {
	tests := []struct {
		name    string
		prompt  string
		isValid bool
	}{
		{"Safe query nasabah", "tampilkan 10 nasabah dengan saldo terbesar", true},
		{"Safe query transaksi", "berapa total transaksi hari ini?", true},
		{"Safe query rekening aktif", "daftar rekening aktif di kantor Cianjur", true},
		
		// Dangerous DDL/DML Keywords
		{"Drop table attempt", "DROP TABLE nasabah", false},
		{"Drop table lowercase", "drop table nasabah", false},
		{"Delete from table", "DELETE FROM nasabah WHERE id = 1", false},
		{"Truncate table", "TRUNCATE nasabah", false},
		{"Alter table", "ALTER TABLE nasabah ADD COLUMN test VARCHAR(10)", false},
		{"Update statement", "UPDATE nasabah SET saldo = 999999", false},
		{"Insert statement", "INSERT INTO nasabah VALUES (1, 'Hacker')", false},
		{"Grant statement", "GRANT ALL PRIVILEGES TO user", false},
		{"Revoke statement", "REVOKE ALL PRIVILEGES FROM user", false},

		// Schema Extraction Keywords (Oracle)
		{"Query ALL_TABLES", "SELECT table_name FROM ALL_TABLES", false},
		{"Query USER_TABLES", "SELECT * FROM USER_TABLES", false},
		{"Query DBA_TABLES", "SELECT * FROM DBA_TABLES", false},
		{"Query ALL_VIEWS", "SELECT * FROM ALL_VIEWS", false},
		{"Query DBA_VIEWS", "SELECT * FROM DBA_VIEWS", false},
		{"Query ALL_USERS", "SELECT * FROM ALL_USERS", false},
		{"Query V$ system view", "SELECT * FROM V$SESSION", false},

		// Bahasa Indonesia destructive keywords
		{"Hapus keyword", "hapus semua data nasabah", false},
		{"Ubah keyword", "ubah saldo nasabah Budi jadi 100 juta", false},
		{"Tambah keyword", "tambah saldo rekening ini", false},
		{"Hilangkan keyword", "hilangkan catatan pinjaman", false},
		{"Lenyapkan keyword", "lenyapkan data transaksi", false},
		{"Bersihkan keyword", "bersihkan tabel audit", false},
		{"Kosongkan keyword", "kosongkan tabel nasabah", false},
		{"Musnahkan keyword", "musnahkan database", false},

		// Leet Speak Obfuscations
		{"Leet update", "upd4t3 saldo nasabah", false},
		{"Leet delete", "d3l3t3 nasabah", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSafePrompt(tt.prompt)
			if (err == nil) != tt.isValid {
				t.Errorf("ValidateSafePrompt(%q) = %v; want isValid=%v. Error: %v", tt.prompt, err == nil, tt.isValid, err)
			}
		})
	}
}

func TestIsRawSQL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Standard SELECT", "SELECT * FROM nasabah", true},
		{"SELECT with spaces", "   SELECT nama, saldo FROM nasabah", true},
		{"Lowercase select", "select count(*) from nasabah", true},
		{"WITH CTE statement", "WITH active_users AS (SELECT * FROM users) SELECT * FROM active_users", true},
		{"INSERT statement", "INSERT INTO nasabah (id) VALUES (1)", true},
		{"Natural language query", "tampilkan nasabah saldo terbesar", false},
		{"Natural language question", "berapa total pinjaman bulan ini?", false},
		{"Empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRawSQL(tt.input)
			if result != tt.expected {
				t.Errorf("IsRawSQL(%q) = %v; want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsReadOnlySQL(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		isValid bool
	}{
		{"Valid SELECT", "SELECT * FROM nasabah ORDER BY saldo DESC", true},
		{"Valid WITH CTE", "WITH summary AS (SELECT COUNT(*) AS total FROM nasabah) SELECT * FROM summary", true},
		{"Invalid INSERT", "INSERT INTO nasabah VALUES (1, 'Test')", false},
		{"Invalid UPDATE", "UPDATE nasabah SET saldo = 0", false},
		{"Invalid DROP", "DROP TABLE nasabah", false},
		{"Invalid empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := IsReadOnlySQL(tt.query)
			if (err == nil) != tt.isValid {
				t.Errorf("IsReadOnlySQL(%q) = %v; want isValid=%v. Error: %v", tt.query, err == nil, tt.isValid, err)
			}
		})
	}
}

func TestValidatePromptSanity(t *testing.T) {
	tests := []struct {
		name    string
		prompt  string
		isValid bool
	}{
		{"Valid prompt", "tampilkan data", true},
		{"Valid short query", "saldo", true},
		{"Too short 3 chars", "abc", false},
		{"Too short 2 chars", "hi", false},
		{"Empty prompt", "", false},
		{"Whitespace only", "   ", false},
		{"Symbols only no alphanumeric", "???!!!", false},
		{"Valid alphanumeric with symbols", "saldo > 10jt?", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePromptSanity(tt.prompt)
			if (err == nil) != tt.isValid {
				t.Errorf("ValidatePromptSanity(%q) = %v; want isValid=%v. Error: %v", tt.prompt, err == nil, tt.isValid, err)
			}
		})
	}
}

func TestValidateIdentifier(t *testing.T) {
	tests := []struct {
		name    string
		ident   string
		isValid bool
	}{
		{"Standard uppercase identifier", "TABEL_NASABAH", true},
		{"Identifier with numbers", "NASABAH_2025", true},
		{"Identifier with dollar sign", "V$SESSION", true},
		{"Identifier with hash", "TABEL#TEMP", true},
		{"Invalid lowercase", "tabel_nasabah", false},
		{"Invalid space", "TABEL NASABAH", false},
		{"Invalid SQL injection quote", "NASABAH'; DROP TABLE--", false},
		{"Invalid semicolon", "NASABAH;--", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateIdentifier(tt.ident)
			if (err == nil) != tt.isValid {
				t.Errorf("ValidateIdentifier(%q) = %v; want isValid=%v. Error: %v", tt.ident, err == nil, tt.isValid, err)
			}
		})
	}
}

func TestGetUserIDFromContext(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected int64
		hasError bool
	}{
		{"From float64 (JWT standard numeric)", float64(42), 42, false},
		{"From int64", int64(100), 100, false},
		{"From int", int(7), 7, false},
		{"From nil", nil, 0, true},
		{"From invalid string", "user_42", 0, true},
		{"From struct", struct{}{}, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := GetUserIDFromContext(tt.input)
			if (err != nil) != tt.hasError {
				t.Errorf("GetUserIDFromContext(%v) error = %v; want hasError=%v", tt.input, err, tt.hasError)
			}
			if id != tt.expected {
				t.Errorf("GetUserIDFromContext(%v) id = %d; want %d", tt.input, id, tt.expected)
			}
		})
	}
}
