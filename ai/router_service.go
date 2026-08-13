package ai

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	"go-bank-api/constants"
)

type intentCacheEntry struct {
	category  string
	createdAt time.Time
}

var (
	intentCache   = make(map[string]intentCacheEntry)
	intentCacheMu sync.RWMutex
	maxIntentSize = 2000
	intentTTL     = 1 * time.Hour

	// Regex for robust JSON intent extraction
	reCategoryJSON = regexp.MustCompile(`(?i)"category"\s*:\s*"([^"]+)"`)
	reCategoryWord = regexp.MustCompile(`(?i)\b(SQL|CHAT|OFF_TOPIC)\b`)

	// Fast-Path Heuristic Patterns
	chatGreetings = []string{
		"halo", "hai", "hello", "hi", "hey", "pagi", "selamat pagi",
		"siang", "selamat siang", "sore", "selamat sore", "malam", "selamat malam",
		"assalamualaikum", "assalamu'alaikum", "kulonuwun", "sampurasun",
	}

	chatGratitude = []string{
		"terima kasih", "makasih", "terimakasih", "thanks", "thank you",
		"thx", "tq", "matur nuwun", "hatur nuhun",
	}

	chatIdentity = []string{
		"siapa kamu", "kamu siapa", "siapa anda", "anda siapa", "bot apa",
		"bisa apa", "kamu bisa apa", "apa yang bisa kamu lakukan", "fitur apa",
		"bantu apa", "tolong jelaskan fungsi kamu", "kamu ai apa",
	}

	chatMisc = []string{
		"tes", "test", "ping", "pong", "bye", "dadah", "sampai jumpa",
		"ok", "oke", "sip", "siap", "mantap", "good", "keren",
	}

	sqlActionVerbs = []string{
		"tampilkan", "tunjukkan", "lihat", "cari", "daftar", "list",
		"hitung", "berapa", "rekap", "laporkan", "laporan", "ranking",
		"urutkan", "filter", "ambil", "carikan", "cek",
	}

	sqlBankingNouns = []string{
		"nasabah", "saldo", "rekening", "tabungan", "deposito", "kredit",
		"pinjaman", "angsuran", "bunga", "transaksi", "mutasi", "debet",
		"kredit", "kantor", "cabang", "user", "pengguna", "cif", "debitur",
		"kolektibilitas", "npl", "plafond", "baki debet", "pokok",
	}

	sqlAggregations = []string{
		"total", "rata-rata", "rata2", "maksimal", "minimal", "tertinggi",
		"terendah", "terbanyak", "terkecil", "paling", "jumlah", "top",
	}
)

// classifyFastPath checks local heuristics for deterministic classification (Tier 0: 0ms latency).
func classifyFastPath(cleanLower string) (string, bool) {
	// 1. Direct match for sapaan & bot identity
	for _, g := range chatGreetings {
		if cleanLower == g || strings.HasPrefix(cleanLower, g+" ") || strings.HasSuffix(cleanLower, " "+g) {
			return constants.IntentChat, true
		}
	}
	for _, g := range chatGratitude {
		if cleanLower == g || strings.HasPrefix(cleanLower, g+" ") || strings.HasSuffix(cleanLower, " "+g) {
			return constants.IntentChat, true
		}
	}
	for _, g := range chatIdentity {
		if strings.Contains(cleanLower, g) {
			return constants.IntentChat, true
		}
	}
	for _, g := range chatMisc {
		if cleanLower == g {
			return constants.IntentChat, true
		}
	}

	// 2. Explicit SQL / Banking Query check (Verb + Noun or Aggregation + Noun)
	hasNoun := false
	for _, noun := range sqlBankingNouns {
		if strings.Contains(cleanLower, noun) {
			hasNoun = true
			break
		}
	}

	if hasNoun {
		for _, verb := range sqlActionVerbs {
			if strings.Contains(cleanLower, verb) {
				return constants.IntentSQL, true
			}
		}
		for _, agg := range sqlAggregations {
			if strings.Contains(cleanLower, agg) {
				return constants.IntentSQL, true
			}
		}
	}

	return "", false
}

// ClassifyIntent performs hybrid multi-tier intent classification.
// Tier 0: Local Fast-Path Pattern Matcher (0ms)
// Tier 1: In-Memory Intent Cache (0ms)
// Tier 2: Dynamic LLM Router with Schema Injection & Robust Parsing
func ClassifyIntent(userInput string) (string, error) {
	cleanInput := strings.TrimSpace(userInput)
	if cleanInput == "" {
		return constants.IntentChat, nil
	}

	cleanLower := strings.ToLower(cleanInput)

	// --- Tier 0: Fast-Path Rule Heuristics (0ms) ---
	if fastCategory, matched := classifyFastPath(cleanLower); matched {
		log.Printf("ROUTER FAST-PATH [Tier 0]: [%s] untuk '%s'", fastCategory, cleanInput)
		return fastCategory, nil
	}

	// --- Tier 1: In-Memory Intent Cache (0ms) ---
	intentCacheMu.RLock()
	if entry, found := intentCache[cleanLower]; found {
		if time.Since(entry.createdAt) < intentTTL {
			intentCacheMu.RUnlock()
			log.Printf("ROUTER RAM-CACHE [Tier 1]: [%s] untuk '%s'", entry.category, cleanInput)
			return entry.category, nil
		}
	}
	intentCacheMu.RUnlock()

	// --- Tier 2: Dynamic LLM Router (Groq Fast) ---
	category, err := classifyViaLLM(cleanInput)
	if err != nil {
		log.Printf("⚠️ Router LLM Gagal (%v), fallback ke SQL.", err)
		return constants.IntentSQL, nil
	}

	// Store in Tier 1 Cache
	intentCacheMu.Lock()
	if len(intentCache) >= maxIntentSize {
		// Evict oldest entries by clearing map
		intentCache = make(map[string]intentCacheEntry)
	}
	intentCache[cleanLower] = intentCacheEntry{
		category:  category,
		createdAt: time.Now(),
	}
	intentCacheMu.Unlock()

	log.Printf("🤖 ROUTER LLM DECISION [Tier 2]: [%s] untuk '%s'", category, cleanInput)
	return category, nil
}

func classifyViaLLM(userInput string) (string, error) {
	// Dynamically inject domain context if available from in-memory cache
	domainHint := ""
	if GlobalCache != nil {
		dict := GlobalCache.GetBusinessDict()
		if dict != "" {
			lines := strings.Split(dict, "\n")
			var preview []string
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line != "" && !strings.HasPrefix(line, "=") {
					preview = append(preview, line)
				}
				if len(preview) >= 6 {
					break
				}
			}
			if len(preview) > 0 {
				domainHint = fmt.Sprintf("\nKonteks Domain Bank:\n%s\n", strings.Join(preview, "\n"))
			}
		}
	}

	systemPrompt := `Anda adalah AI Router (Resepsionis) untuk Sistem Basis Data Perbankan Bank Supra.
Tugas Anda HANYA mengklasifikasikan input pengguna ke dalam salah satu dari 3 kategori:

1. "SQL": Jika pengguna meminta data bank, nasabah, rekening, saldo, transaksi, pinjaman, tabungan, kantor, atau laporan/statistik keuangan.
2. "CHAT": Jika pengguna menyapa (halo, pagi), mengucapkan terima kasih, bertanya identitas bot, atau percakapan ramah-tamah.
3. "OFF_TOPIC": Jika pengguna bertanya hal di luar perbankan dan data perbankan (resep masakan, politik, cuaca, puisi, curhat umum).
%s
CONTOH:
- "Tampilkan 5 nasabah dengan saldo terbesar" -> {"category": "SQL"}
- "Berapa total tabungan kantor cianjur?" -> {"category": "SQL"}
- "Halo apa kabar?" -> {"category": "CHAT"}
- "Terima kasih banyak" -> {"category": "CHAT"}
- "Bagaimana cara membuat kue bolu?" -> {"category": "OFF_TOPIC"}
- "Siapa presiden pertama?" -> {"category": "OFF_TOPIC"}

Input Pengguna: "%s"

JAWAB HANYA DENGAN FORMAT JSON VALID: {"category": "SQL" | "CHAT" | "OFF_TOPIC"}`

	finalPrompt := fmt.Sprintf(systemPrompt, domainHint, userInput)
	opts := GroqOptions{
		Temperature: 0.1,
		TopP:        0.5,
	}

	rawResponse, err := callGroqAPI(finalPrompt, constants.GroqModelFast, opts)
	if err != nil {
		log.Println("[ai][router_service][classifyViaLLM] error:", err)
		return constants.IntentSQL, err
	}

	return parseRouterResponse(rawResponse)
}

func parseRouterResponse(rawResponse string) (string, error) {
	// 1. Try regex extraction first (resilient against extra conversational text)
	if matches := reCategoryJSON.FindStringSubmatch(rawResponse); len(matches) > 1 {
		cat := strings.ToUpper(strings.TrimSpace(matches[1]))
		return normalizeCategory(cat), nil
	}

	// 2. Try JSON unmarshal
	cleanJSON := strings.TrimSpace(rawResponse)
	cleanJSON = strings.ReplaceAll(cleanJSON, "```json", "")
	cleanJSON = strings.ReplaceAll(cleanJSON, "```", "")

	var result struct {
		Category string `json:"category"`
	}
	if err := json.Unmarshal([]byte(cleanJSON), &result); err == nil && result.Category != "" {
		return normalizeCategory(strings.ToUpper(strings.TrimSpace(result.Category))), nil
	}

	// 3. Fallback word search
	if matches := reCategoryWord.FindStringSubmatch(rawResponse); len(matches) > 0 {
		return normalizeCategory(strings.ToUpper(matches[0])), nil
	}

	return constants.IntentSQL, nil
}

func normalizeCategory(cat string) string {
	switch cat {
	case "SQL":
		return constants.IntentSQL
	case "CHAT":
		return constants.IntentChat
	case "OFF_TOPIC", "OFFTOPIC", "OFF TOPIC":
		return constants.IntentOffTopic
	default:
		return constants.IntentSQL
	}
}
