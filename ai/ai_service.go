package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	config "go-bank-api/config"
	models "go-bank-api/models"
	utils "go-bank-api/utils"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	pb "github.com/qdrant/go-client/qdrant"
	"google.golang.org/api/option"
)

var (
	qdrantClient   *pb.Client
	geminiEmbedder *genai.EmbeddingModel
)

type IntentResponse struct {
	Category string `json:"category"`
	Reason   string `json:"reason"`
}

func InitVectorService() error {
	if config.AppConfig == nil {
		return fmt.Errorf("konfigurasi aplikasi belum dimuat")
	}

	ctx := context.Background()

	geminiClient, err := genai.NewClient(ctx, option.WithAPIKey(config.AppConfig.GoogleAPIKey))
	if err != nil {
		return fmt.Errorf("gagal membuat client Gemini: %w", err)
	}
	geminiEmbedder = geminiClient.EmbeddingModel(config.AppConfig.EmbeddingModel)

	client, err := pb.NewClient(&pb.Config{
		Host:   config.AppConfig.QdrantGRPCHost,
		Port:   config.AppConfig.QdrantGRPCPort,
		APIKey: config.AppConfig.QdrantAPIKey,
		UseTLS: true,
	})
	if err != nil {
		return fmt.Errorf("gagal membuat Qdrant gRPC client: %w", err)
	}
	qdrantClient = client

	log.Printf("Memastikan collection cache '%s' ada via REST...", config.AppConfig.QdrantCacheCollection)
	if err := qdrantCreateCollection(ctx, config.AppConfig.QdrantURL, config.AppConfig.QdrantCacheCollection, config.AppConfig.EmbeddingVectorSize, config.AppConfig.QdrantDistanceMetric); err != nil {
		return fmt.Errorf("gagal membuat/memverifikasi cache collection: %w", err)
	}
	if err := qdrantCreateCollection(ctx, config.AppConfig.QdrantURL, config.AppConfig.QdrantCollectionName, config.AppConfig.EmbeddingVectorSize, config.AppConfig.QdrantDistanceMetric); err != nil {
		return fmt.Errorf("gagal membuat/memverifikasi RAG collection: %w", err)
	}

	log.Println("Memastikan index payload 'category' tersedia...")
	if err := qdrantCreatePayloadIndex(ctx, config.AppConfig.QdrantURL, config.AppConfig.QdrantCollectionName, "category", "keyword"); err != nil {
		log.Printf("Warning: Gagal membuat index payload: %v", err)
	}

	log.Println("✅ Berhasil terkoneksi ke Layanan Vektor (Google AI & Qdrant).")
	return nil
}

func GetSQLFromAI_Groq(userPrompt string) (models.AISqlResponse, error) {
	if utils.IsRawSQL(userPrompt) {
		log.Printf("SECURITY BLOCK: User input Raw SQL: '%s'", userPrompt)
		return models.AISqlResponse{}, &models.AppError{Code: "DANGEROUS_INTENT", Message: "DITOLAK. Silakan ganti pertanyaan Anda."}
	}
	if err := utils.ValidateSafePrompt(userPrompt); err != nil {
		log.Printf("SECURITY BLOCK: User input Raw SQL: '%s'", userPrompt)
		return models.AISqlResponse{}, &models.AppError{Code: "DANGEROUS_INTENT", Message: "DITOLAK. Silakan ganti pertanyaan Anda."}
	}
	if config.AppConfig == nil {
		return models.AISqlResponse{}, fmt.Errorf("konfigurasi aplikasi belum dimuat")
	}

	intent, _ := ClassifyIntent(userPrompt)

	if intent == "CHAT" {
		return models.AISqlResponse{}, &models.AppError{
			Code:    "CHIT_CHAT",
			Message: "Halo! Saya Asisten Data Bank Supra. Silakan tanya seputar Perbankan.",
		}
	}

	if intent == "OFF_TOPIC" {
		return models.AISqlResponse{}, &models.AppError{
			Code:    "CHIT_CHAT",
			Message: "Maaf, saya hanya bisa menjawab pertanyaan seputar Perbankan. Tidak bisa melayani topik lain.",
		}
	}

	ctx := context.Background()

	log.Println("Menerjemahkan prompt user ke vektor...")
	promptVector, err := GenerateEmbedding(userPrompt)
	if err != nil {
		return models.AISqlResponse{}, fmt.Errorf("gagal embed prompt user: %w", err)
	}

	hardHit, softCacheContext, err := checkSemanticCache(ctx, promptVector)
	if err != nil {
		log.Printf("PERINGATAN: Gagal akses cache: %v", err)
	}
	if hardHit != nil {
		return *hardHit, nil
	}

	sqlContext := getRAGContext(ctx, promptVector)

	allDDLString, err := getDDLContext(ctx, promptVector)
	if err != nil {
		return models.AISqlResponse{}, err
	}
	refDataString, _ := GetDynamicReferenceData(ctx)
	businessDict, _ := GetBusinessDictionary(ctx)

	finalPrompt := buildFinalPrompt(userPrompt, allDDLString, refDataString, businessDict, sqlContext, softCacheContext)

	rawContent, err := fetchLLMResponse(ctx, finalPrompt)
	if err != nil {
		return models.AISqlResponse{}, err
	}

	sqlQuery := extractSQLFromMarkdown(rawContent)
	log.Printf("🤖 RAW AI Response:\n%s\n", rawContent)
	log.Println("SQL dari AI (Dynamic RAG):", sqlQuery)

	sqlQuery = sanitizeSQL(sqlQuery)
	if sqlQuery == "" {
		return models.AISqlResponse{}, errors.New("SQL tidak aman atau tidak valid")
	}

	return models.AISqlResponse{
		SQL:        sqlQuery,
		Vector:     promptVector,
		PromptAsli: userPrompt,
		IsCached:   false,
	}, nil
}

func EnhanceNaturalLanguage(draft string) (string, error) {
	if config.AppConfig.GroqAPIKey == "" {
		return "", fmt.Errorf("API Key Groq belum diset")
	}

	systemPrompt := `
Anda adalah editor bahasa profesional. Ubah input user yang ambigu menjadi pertanyaan baku, sopan, dan spesifik untuk query database.
Aturan:
1. JANGAN menjawab pertanyaan. HANYA perbaiki kalimatnya.
2. Jika ada angka ambigu (misal "20 juta"), tambahkan "sebesar", "minimal", atau "lebih dari".
3. Output harus langsung kalimat perbaikan tanpa tanda kutip.

Contoh:
Input: "tabungan 20 juta"
Output: Tampilkan nasabah yang memiliki saldo tabungan sebesar 20 juta rupiah atau lebih.

Input User: "%s"
Output:`
	finalPrompt := fmt.Sprintf(systemPrompt, draft)

	return callGroqAPI(finalPrompt, "llama-3.1-8b-instant", 0.1)
}

func RepairSQLFromAI(promptAsli string, sqlSalah string, pesanError string) (string, error) {
	if config.AppConfig == nil {
		return "", fmt.Errorf("konfigurasi belum dimuat")
	}
	log.Println("Memulai Self-Correction AI...")

	systemPrompt := fmt.Sprintf(`
Anda adalah ahli database Oracle 10g. Perbaiki query SQL yang error.
KONTEKS ERROR:
- Pertanyaan: "%s"
- SQL Salah: %s
- Error Oracle: %s
ATURAN:
1. Perbaiki sintaks agar kompatibel dengan Oracle 10g.
2. Langsung berikan SQL yang diperbaiki dalam blok markdown code.
`, promptAsli, sqlSalah, pesanError)

	rawContent, err := callGroqAPI(systemPrompt, config.AppConfig.GroqModel, 0.1)
	if err != nil {
		return "", err
	}

	fixedSQL := extractSQLFromMarkdown(rawContent)
	fixedSQL = sanitizeSQL(fixedSQL)
	if fixedSQL == "" {
		return "", fmt.Errorf("hasil perbaikan kosong/validasi gagal")
	}

	log.Printf("SQL berhasil diperbaiki menjadi: %s", fixedSQL)
	return fixedSQL, nil
}

func checkSemanticCache(ctx context.Context, vector []float32) (*models.AISqlResponse, string, error) {
	log.Println("Mencari di Semantic Cache Qdrant (REST)...")
	searchReq := qdrantSearchReq{Vector: vector, Limit: config.AppConfig.CacheSearchLimit, WithPayload: true}

	cacheResponse, err := qdrantSearchPoints(ctx, config.AppConfig.QdrantURL, config.AppConfig.QdrantCacheCollection, searchReq)
	if err != nil {
		return nil, "", err
	}

	if len(cacheResponse.Result) > 0 {
		cachedPoint := cacheResponse.Result[0]
		topScore := cachedPoint.Score

		cachedSql, _ := cachedPoint.Payload["sql_query"].(string)
		cachedPrompt, _ := cachedPoint.Payload["prompt_asli"].(string)

		if topScore >= config.AppConfig.CacheSimilarityThreshold {
			log.Printf("✅ SEMANTIC CACHE HIT! Skor: %f", topScore)
			return &models.AISqlResponse{SQL: cachedSql, IsCached: true}, "", nil
		}

		if topScore >= 0.80 {
			log.Printf("💡 SOFT CACHE HIT (Skor: %f).", topScore)
			softContext := fmt.Sprintf("\nCONTOH RIWAYAT SERUPA (Sangat Relevan):\nUser: \"%s\"\nSQL: %s\n", cachedPrompt, cachedSql)
			return nil, softContext, nil
		}
		log.Printf("CACHE MISS. Skor tertinggi: %f", topScore)
	} else {
		log.Println("CACHE MISS. Tidak ada item cache.")
	}
	return nil, "", nil
}

func getRAGContext(ctx context.Context, vector []float32) string {
	var searchLimit uint64 = 10
	searchResponse, err := qdrantClient.Query(ctx, &pb.QueryPoints{
		CollectionName: config.AppConfig.QdrantCollectionName,
		Query:          pb.NewQuery(vector...),
		WithPayload:    pb.NewWithPayload(true),
		Limit:          &searchLimit,
		Filter: &pb.Filter{
			Must: []*pb.Condition{{
				ConditionOneOf: &pb.Condition_Field{Field: &pb.FieldCondition{Key: "category", Match: &pb.Match{MatchValue: &pb.Match_Keyword{Keyword: "sql"}}}},
			}},
		},
	})

	if err != nil || len(searchResponse) == 0 {
		return "TIDAK ADA CONTOH SQL. GUNAKAN LOGIKA SENDIRI."
	}

	if searchResponse[0].Score < 0.45 {
		log.Println("Score RAG rendah. Mengabaikan contoh RAG.")
		return "TIDAK ADA CONTOH SQL YANG RELEVAN."
	}

	var sb strings.Builder
	sb.WriteString("Berikut adalah CONTOH DDL/SQL relevan:\n")
	seen := make(map[string]bool)
	for _, p := range searchResponse {
		if val := p.GetPayload()["content"].GetStringValue(); val != "" && !seen[val] {
			seen[val] = true
			sb.WriteString(val + "\n---\n")
		}
	}
	return sb.String()
}

func getDDLContext(ctx context.Context, vector []float32) (string, error) {
	relevantDDL, err := searchRelevantDDL(ctx, vector)
	if err != nil || strings.TrimSpace(relevantDDL) == "" {
		log.Println("Fallback ke load SEMUA tabel.")
		allDDLs, err := GetDynamicSchemaContext()
		if err != nil {
			return "", fmt.Errorf("gagal mengambil DDL dinamis: %w", err)
		}
		return strings.Join(allDDLs, "\n---\n"), nil
	}
	return relevantDDL, nil
}

func buildFinalPrompt(userPrompt, ddl, refData, dict, ragContext, softCache string) string {
	return fmt.Sprintf(`
Anda adalah ahli SQL Oracle 10g senior. Tanggal hari ini: %s.

== 1. KAMUS DATA (DDL & STRUKTUR) ==
Baca DDL ini dengan teliti. Perhatikan KOMENTAR (-- ...) di setiap kolom untuk memahami artinya.
%s

== 2. LIVE DATA REFERENSI (PENTING: JANGAN MENEBAK ID) ==
Gunakan ID yang tertera di sini jika query membutuhkan filter berdasarkan Status, Tipe, atau Kategori.
JANGAN MENGARANG ID SENDIRI.
%s

== 3. KAMUS ISTILAH BISNIS ==
%s

== 4. CONTOH SQL (RAG CONTEXT) ==
%s

== 5. SOFT CACHE HIT (REFERENSI) ==
Gunakan contoh di bawah ini sebagai referensi utama pola query jika relevan.
%s

== ATURAN PENULISAN SQL (ZERO-SHOT & RAG) ==
1. **Priority Reference**: Jika user menyebut "Tabungan", "Deposito", "Aktif", atau "Tutup", WAJIB cek bagian "LIVE DATA REFERENSI" untuk mendapatkan ID yang tepat. Jangan menebak "1" atau "0".
2. **Column Validation**: Hanya gunakan kolom yang ADA di DDL di atas.
3. **Security**: Hanya SELECT. Dilarang INSERT/UPDATE/DELETE.

== TUGAS ANDA (CHAIN OF THOUGHT) ==
Sebelum menulis kode SQL, jelaskan langkah berpikir Anda secara singkat:
1. **Analisis Intent**: Apa data yang dicari user?
2. **Mapping Referensi**: Apakah ada kata kunci (misal: "blokir") yang perlu dicari ID-nya di "LIVE DATA REFERENSI"? Jika ada, sebutkan nilai-nya.
3. **Strategi Query**: Table mana yang di-JOIN? Apa kondisi WHERE-nya?
4. **SQL Final**: Tulis query dalam blok markdown code.

Pertanyaan Pengguna: "%s"
`, time.Now().Format("2006-01-02"), ddl, refData, dict, ragContext, softCache, userPrompt)
}

func extractSQLFromMarkdown(content string) string {
	re := regexp.MustCompile("(?s)```sql(.*?)```")
	matches := re.FindAllStringSubmatch(content, -1)

	var sqlQuery string
	for _, match := range matches {
		c := strings.TrimSpace(match[1])
		if strings.HasPrefix(strings.ToUpper(c), "SELECT") || strings.HasPrefix(strings.ToUpper(c), "WITH") {
			sqlQuery = c
		}
	}
	if sqlQuery == "" && len(matches) > 0 {
		sqlQuery = strings.TrimSpace(matches[len(matches)-1][1])
	}
	if sqlQuery == "" {
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(content)), "SELECT") {
			sqlQuery = content
		}
	}
	return sqlQuery
}

func sanitizeSQL(sql string) string {
	lines := strings.Split(sql, "\n")
	var cleanLines []string
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "--") {
			continue
		}
		cleanLines = append(cleanLines, line)
	}
	cleanSql := strings.Join(cleanLines, "\n")
	cleanSql = strings.TrimSpace(strings.TrimRight(strings.TrimSpace(cleanSql), ";"))

	lower := strings.ToLower(cleanSql)
	forbidden := []string{"insert", "update", "delete", "drop", "alter", "create", "truncate", "grant", "revoke"}
	for _, f := range forbidden {
		if strings.Contains(lower, f) {
			return ""
		}
	}

	if !strings.HasPrefix(lower, "select") && !strings.HasPrefix(lower, "with") {
		return ""
	}
	return cleanSql
}

func searchRelevantDDL(ctx context.Context, promptVector []float32) (string, error) {
	var limit uint64 = 5
	searchResponse, err := qdrantClient.Query(ctx, &pb.QueryPoints{
		CollectionName: config.AppConfig.QdrantCollectionName,
		Query:          pb.NewQuery(promptVector...),
		WithPayload:    pb.NewWithPayload(true),
		Limit:          &limit,
		Filter: &pb.Filter{
			Must: []*pb.Condition{{
				ConditionOneOf: &pb.Condition_Field{Field: &pb.FieldCondition{Key: "category", Match: &pb.Match{MatchValue: &pb.Match_Keyword{Keyword: "ddl"}}}},
			}},
		},
	})
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	count := 0
	for _, p := range searchResponse {
		if p.Score < 0.3 {
			continue
		}
		if val := p.GetPayload()["content"].GetStringValue(); val != "" {
			sb.WriteString(val + "\n\n")
			count++
		}
	}
	log.Printf("Dynamic Context: %d tabel relevan.", count)
	return sb.String(), nil
}

func ClassifyIntent(userInput string) (string, error) {
	systemPrompt := `
Anda adalah AI Router (Resepsionis) untuk Sistem Database Bank. 
Tugas Anda HANYA mengklasifikasikan input user ke dalam 3 kategori:

1. "SQL": Jika user meminta data bank, nasabah, rekening, transaksi, saldo,umur nasabah atau laporan.
2. "CHAT": Jika user hanya menyapa (halo, selamat pagi), bertanya identitas bot, atau berterima kasih.
3. "OFF_TOPIC": Jika user bertanya hal di luar perbankan (misal: resep masakan, politik, coding, curhat).

CONTOH:
- "Tampilkan nasabah saldo tertinggi" -> {"category": "SQL"}
- "Halo apa kabar" -> {"category": "CHAT"}
- "Cara masak rendang" -> {"category": "OFF_TOPIC"}
- "Siapa kamu?" -> {"category": "CHAT"}
- "Total tabungan Budi" -> {"category": "SQL"}

Input User: "%s"

JAWAB HANYA DENGAN FORMAT JSON VALID: {"category": "..."}
`
	finalPrompt := fmt.Sprintf(systemPrompt, userInput)
	rawResponse, err := callGroqAPI(finalPrompt, "llama-3.1-8b-instant", 0.0)
	if err != nil {
		return "SQL", nil
	}
	var result IntentResponse
	cleanJSON := strings.TrimSpace(rawResponse)
	cleanJSON = strings.ReplaceAll(cleanJSON, "```json", "")
	cleanJSON = strings.ReplaceAll(cleanJSON, "```", "")

	if err := json.Unmarshal([]byte(cleanJSON), &result); err != nil {
		log.Printf("⚠️ Gagal parse intent JSON: %v. Raw: %s", err, rawResponse)
		return "SQL", nil
	}

	log.Printf("ROUTER DECISION: [%s] untuk input '%s'", result.Category, userInput)
	return result.Category, nil
}
