package ai

import (
	"testing"

	"go-bank-api/constants"
)

func TestClassifyFastPath(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantCategory string
		wantMatched  bool
	}{
		// Chat Greetings
		{"Greeting Halo", "halo", constants.IntentChat, true},
		{"Greeting Halo Bot", "halo bot", constants.IntentChat, true},
		{"Greeting Selamat Pagi", "selamat pagi", constants.IntentChat, true},
		{"Greeting Assalamualaikum", "assalamualaikum", constants.IntentChat, true},
		{"Greeting Hai", "hai", constants.IntentChat, true},

		// Chat Gratitude & Identity
		{"Gratitude Makasih", "makasih", constants.IntentChat, true},
		{"Gratitude Terima Kasih", "terima kasih", constants.IntentChat, true},
		{"Identity Siapa Kamu", "siapa kamu?", constants.IntentChat, true},
		{"Identity Kamu Bisa Apa", "kamu bisa apa aja?", constants.IntentChat, true},
		{"Misc Ping", "ping", constants.IntentChat, true},

		// Explicit SQL Queries (Verb + Noun / Aggregation + Noun)
		{"SQL Tampilkan Nasabah", "tampilkan 10 nasabah", constants.IntentSQL, true},
		{"SQL Total Saldo", "berapa total saldo tabungan", constants.IntentSQL, true},
		{"SQL Daftar Rekening", "daftar rekening aktif", constants.IntentSQL, true},
		{"SQL Rata-rata Kredit", "hitung rata-rata kredit kantor cianjur", constants.IntentSQL, true},
		{"SQL Mutasi Transaksi", "lihat mutasi transaksi", constants.IntentSQL, true},
		{"SQL Top Nasabah", "top nasabah saldo terbesar", constants.IntentSQL, true},

		// Unmatched / Non-Fast-Path
		{"Complex Recipe Off-topic", "bagaimana cara membuat kue bolu coklat", "", false},
		{"Ambiguous Banking Question", "apakah hari ini bank buka?", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCategory, gotMatched := classifyFastPath(tt.input)
			if gotMatched != tt.wantMatched {
				t.Errorf("classifyFastPath(%q) matched = %v, want %v", tt.input, gotMatched, tt.wantMatched)
			}
			if gotMatched && gotCategory != tt.wantCategory {
				t.Errorf("classifyFastPath(%q) category = %v, want %v", tt.input, gotCategory, tt.wantCategory)
			}
		})
	}
}

func TestParseRouterResponse(t *testing.T) {
	tests := []struct {
		name         string
		rawResponse  string
		wantCategory string
	}{
		{"Clean JSON SQL", `{"category": "SQL"}`, constants.IntentSQL},
		{"Clean JSON CHAT", `{"category": "CHAT"}`, constants.IntentChat},
		{"Clean JSON OFF_TOPIC", `{"category": "OFF_TOPIC"}`, constants.IntentOffTopic},
		{"Lowercase category", `{"category": "chat"}`, constants.IntentChat},
		{"Markdown codeblock", "```json\n{\"category\": \"SQL\"}\n```", constants.IntentSQL},
		{"Extra conversational text", `Tentu! Berikut kategorinya: {"category": "OFF_TOPIC"}`, constants.IntentOffTopic},
		{"Word fallback only", `Menurut saya kategorinya adalah CHAT`, constants.IntentChat},
		{"Unrecognized fallback", `Saya tidak mengerti`, constants.IntentSQL},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRouterResponse(tt.rawResponse)
			if err != nil {
				t.Fatalf("parseRouterResponse() unexpected error: %v", err)
			}
			if got != tt.wantCategory {
				t.Errorf("parseRouterResponse() = %v, want %v", got, tt.wantCategory)
			}
		})
	}
}

func TestNormalizeCategory(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"SQL", constants.IntentSQL},
		{"CHAT", constants.IntentChat},
		{"OFF_TOPIC", constants.IntentOffTopic},
		{"OFFTOPIC", constants.IntentOffTopic},
		{"OFF TOPIC", constants.IntentOffTopic},
		{"UNKNOWN", constants.IntentSQL},
	}

	for _, tt := range tests {
		got := normalizeCategory(tt.input)
		if got != tt.want {
			t.Errorf("normalizeCategory(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
