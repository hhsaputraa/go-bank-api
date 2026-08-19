package utils

import "testing"

func BenchmarkValidateSafePrompt(b *testing.B) {
	prompt := "tampilkan 10 nasabah dengan saldo tabungan terbesar di kantor Cianjur"
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = ValidateSafePrompt(prompt)
	}
}

func BenchmarkValidatePassword(b *testing.B) {
	pwd := "BankSupra2025#"
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = ValidatePassword(pwd)
	}
}

func BenchmarkIsRawSQL(b *testing.B) {
	input := "SELECT * FROM app_users WHERE username = 'admin'"
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = IsRawSQL(input)
	}
}

func BenchmarkValidateIdentifier(b *testing.B) {
	ident := "APP_USERS_TBL"
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = ValidateIdentifier(ident)
	}
}
