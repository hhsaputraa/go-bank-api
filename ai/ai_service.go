package ai

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"


	config "go-bank-api/config"
	"go-bank-api/constants"
	database "go-bank-api/database"
	models "go-bank-api/models"
	utils "go-bank-api/utils"

	"github.com/google/generative-ai-go/genai"
	"github.com/google/uuid"
	pb "github.com/qdrant/go-client/qdrant"
	"google.golang.org/api/option"
)

var (
	qdrantClient    *pb.Client
	geminiEmbedder  *genai.EmbeddingModel
	sqlChainService *SQLChainService
)

func InitVectorService() error {

	if config.AppConfig == nil {
		err := fmt.Errorf("konfigurasi aplikasi belum dimuat")
		log.Println("[ai][ai_service][InitVectorService] error:", err)
		return err
	}

	ctx := context.Background()

	geminiClient, err := genai.NewClient(ctx, option.WithAPIKey(config.AppConfig.GoogleAPIKey))
	if err != nil {
		log.Println("[ai][ai_service][InitVectorService] error:", err)
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
		log.Println("[ai][ai_service][InitVectorService] error:", err)
		return fmt.Errorf("gagal membuat Qdrant gRPC client: %w", err)
	}
	qdrantClient = client

	log.Printf("Memastikan collection cache '%s' ada via REST...", config.AppConfig.QdrantCacheCollection)
	if err := qdrantCreateCollection(ctx, config.AppConfig.QdrantURL, config.AppConfig.QdrantCacheCollection, config.AppConfig.EmbeddingVectorSize, config.AppConfig.QdrantDistanceMetric); err != nil {
		log.Println("[ai][ai_service][InitVectorService] error:", err)
		return fmt.Errorf("gagal membuat/memverifikasi cache collection: %w", err)
	}
	if err := qdrantCreateCollection(ctx, config.AppConfig.QdrantURL, config.AppConfig.QdrantCollectionName, config.AppConfig.EmbeddingVectorSize, config.AppConfig.QdrantDistanceMetric); err != nil {
		log.Println("[ai][ai_service][InitVectorService] error:", err)
		return fmt.Errorf("gagal membuat/memverifikasi RAG collection: %w", err)
	}

	log.Println("Memastikan index payload 'category' tersedia...")
	if err := qdrantCreatePayloadIndex(ctx, config.AppConfig.QdrantURL, config.AppConfig.QdrantCollectionName, "category", "keyword"); err != nil {
		log.Println("[ai][ai_service][InitVectorService] error:", err)
		log.Printf("Warning: Gagal membuat index payload: %v", err)
	}

	log.Println("[INFO] Berhasil terkoneksi ke Layanan Vektor (Google AI & Qdrant).")

	// Inisialisasi LangChain SQL Service
	chainService, err := NewSQLChainService()
	if err != nil {
		log.Println("[ai][ai_service][InitVectorService] error:", err)
		log.Printf("Warning: Gagal inisialisasi SQLChainService: %v", err)
	} else {
		sqlChainService = chainService
		log.Println("[INFO] Berhasil menginisialisasi LangChain Modularity Chain.")
	}

	// Initialize In-Memory Cache
	InitMemoryCache()

	return nil
}

func LearnFromCorrection(prompt string, validSQL string) {
	if config.AppConfig == nil || !config.AppConfig.EnableAutoLearning {
		return
	}

	SubmitAsyncTask(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		log.Printf("AUTO-LEARNING: Mempelajari pola baru untuk: '%s'", prompt)

		vector, err := GenerateEmbedding(prompt)
		if err != nil {
			log.Println("[ai][ai_service][LearnFromCorrection] error:", err)
			log.Printf("Auto-Learning gagal (Embedding): %v", err)
			return
		}

		pointID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(prompt)).String()
		point := &pb.PointStruct{
			Id: &pb.PointId{
				PointIdOptions: &pb.PointId_Uuid{Uuid: pointID},
			},
			Vectors: &pb.Vectors{VectorsOptions: &pb.Vectors_Vector{Vector: &pb.Vector{Data: vector}}},
			Payload: map[string]*pb.Value{
				"prompt_asli": {Kind: &pb.Value_StringValue{StringValue: prompt}},
				"sql_query":   {Kind: &pb.Value_StringValue{StringValue: validSQL}},
				"category":    {Kind: &pb.Value_StringValue{StringValue: constants.CategorySQL}},
				"source":      {Kind: &pb.Value_StringValue{StringValue: "auto_learning_v2"}},
				"created_at":  {Kind: &pb.Value_StringValue{StringValue: time.Now().Format(time.RFC3339)}},
			},
		}
		if qdrantClient == nil {
			log.Println("[ai][ai_service][LearnFromCorrection] error: qdrantClient belum terinisialisasi")
			return
		}

		_, err = qdrantClient.Upsert(ctx, &pb.UpsertPoints{
			CollectionName: config.AppConfig.QdrantCollectionName,
			Points:         []*pb.PointStruct{point},
		})

		if err != nil {
			log.Println("[ai][ai_service][LearnFromCorrection] error:", err)
			log.Printf("Auto-Learning Gagal (Qdrant Upsert): %v", err)
		} else {
			log.Println("AUTO-LEARNING BERHASIL disimpan ke Qdrant!")
		}
	})
}

func GetSQL(userPrompt string) (models.AISqlResponse, error) {
	// Default to empty model (uses config default)
	return GetSQLWithModel(userPrompt, "")
}

func GetSQLWithModel(userPrompt string, modelName string) (models.AISqlResponse, error) {
	if utils.IsRawSQL(userPrompt) {
		log.Printf("SECURITY BLOCK: User input Raw SQL: '%s'", userPrompt)
		return models.AISqlResponse{}, &models.AppError{Code: "DANGEROUS_INTENT", Message: "DITOLAK. Silakan ganti pertanyaan Anda."}
	}
	if err := utils.ValidateSafePrompt(userPrompt); err != nil {
		log.Println("[ai][ai_service][GetSQLWithModel] error:", err)
		log.Printf("SECURITY BLOCK: User input Raw SQL: '%s'", userPrompt)
		return models.AISqlResponse{}, &models.AppError{Code: "DANGEROUS_INTENT", Message: "DITOLAK. Silakan ganti pertanyaan Anda."}
	}
	if config.AppConfig == nil {
		err := fmt.Errorf("konfigurasi aplikasi belum dimuat")
		log.Println("[ai][ai_service][GetSQLWithModel] error:", err)
		return models.AISqlResponse{}, err
	}

	ctx := context.Background()

	// 1. FAST PATH: Check Semantic Cache First (Skip LLM Intent classification on cache hit)
	log.Println("Menerjemahkan prompt user ke vektor...")
	promptVector, err := GenerateEmbedding(userPrompt)
	if err != nil {
		log.Println("[ai][ai_service][GetSQLWithModel] error:", err)
		return models.AISqlResponse{}, fmt.Errorf("gagal embed prompt user: %w", err)
	}

	hardHit, softCacheContext, err := CheckSemanticCache(ctx, promptVector)
	if err != nil {
		log.Println("[ai][ai_service][GetSQLWithModel] error:", err)
		log.Printf("PERINGATAN: Gagal akses cache: %v", err)
	}
	if hardHit != nil {
		hardHit.PromptAsli = userPrompt
		hardHit.Vector = promptVector
		hardHit.IsCached = true
		return *hardHit, nil
	}

	// 2. SLOW PATH (Cache Miss): Classify Intent & Execute RAG
	intent, _ := ClassifyIntent(userPrompt)

	if intent == constants.IntentChat {
		return models.AISqlResponse{}, &models.AppError{
			Code:    constants.ErrCodeChitChat,
			Message: "Halo! Saya Asisten Data Bank Supra. Silakan tanya seputar Perbankan.",
		}
	}

	if intent == constants.IntentOffTopic {
		return models.AISqlResponse{}, &models.AppError{
			Code:    constants.ErrCodeChitChat,
			Message: "Maaf, saya hanya bisa menjawab pertanyaan seputar Perbankan. Tidak bisa melayani topik lain.",
		}
	}

	ragCtx, err := retrieveRAGContext(ctx, promptVector)
	if err != nil {
		log.Println("[ai][ai_service][GetSQLWithModel] error:", err)
		return models.AISqlResponse{}, err
	}


	finalPrompt := buildFinalPrompt(userPrompt, ragCtx.DDL, ragCtx.RefData, ragCtx.BusinessDict, ragCtx.SQLContext, softCacheContext)

	cleanContent, err := executeSQLChain(ctx, userPrompt, finalPrompt, modelName, ragCtx, softCacheContext)
	if err != nil {
		log.Println("[ai][ai_service][GetSQLWithModel] error:", err)
		return models.AISqlResponse{}, err
	}

	sqlQuery, err := extractAndSanitizeSQLResponse(cleanContent)
	if err != nil {
		return models.AISqlResponse{}, err
	}

	log.Println("SQL dari AI (Dynamic RAG via Chain):", sqlQuery)

	// Dry-Run Validation & Auto-Repair Self-Correction Loop
	sqlQuery = ValidateAndRepairSQL(ctx, sqlQuery, userPrompt)

	return models.AISqlResponse{
		SQL:        sqlQuery,
		Vector:     promptVector,
		PromptAsli: userPrompt,
		IsCached:   false,
	}, nil
}

type RAGContextResult struct {
	SQLContext   string
	DDL          string
	RefData      string
	BusinessDict string
}

func retrieveRAGContext(ctx context.Context, promptVector []float32) (*RAGContextResult, error) {
	var (
		res    RAGContextResult
		ddlErr error
		wg     sync.WaitGroup
	)

	wg.Add(2)
	go func() {
		defer wg.Done()
		res.SQLContext = getRAGContext(ctx, promptVector)
	}()

	go func() {
		defer wg.Done()
		res.DDL, ddlErr = getDDLContext(ctx, promptVector)
	}()

	wg.Wait()

	if ddlErr != nil {
		return nil, ddlErr
	}

	res.RefData, _ = GetDynamicReferenceData(ctx)
	res.BusinessDict, _ = GetBusinessDictionary(ctx)
	return &res, nil
}

func executeSQLChain(ctx context.Context, userPrompt, finalPrompt, modelName string, ragCtx *RAGContextResult, softCache string) (string, error) {
	if sqlChainService != nil {
		log.Println("🔗 Mengeksekusi via LangChain SQL Chain...")
		contextData := map[string]interface{}{
			"today":        time.Now().Format("2006-01-02"),
			"ddl":          ragCtx.DDL,
			"refData":      ragCtx.RefData,
			"businessDict": ragCtx.BusinessDict,
			"ragContext":   ragCtx.SQLContext,
			"softCache":    softCache,
			"userPrompt":   userPrompt,
		}

		chainOutput, err := sqlChainService.GenerateSQL(ctx, userPrompt, contextData)
		if err == nil {
			return chainOutput, nil
		}
		log.Println("[ai][ai_service][executeSQLChain] error:", err)
		log.Printf("⚠️ LangChain Error: %v. Fallback ke direct LLM implementation.", err)
	}

	return fetchLLMResponse(ctx, finalPrompt, modelName)
}

func extractAndSanitizeSQLResponse(cleanContent string) (string, error) {
	sqlQuery := extractSQLFromMarkdown(cleanContent)

	reThinking := regexp.MustCompile("(?s)<thought>.*?</thought>")
	cleanContentText := reThinking.ReplaceAllString(cleanContent, "")
	cleanContentText = strings.TrimSpace(cleanContentText)

	if sqlQuery == "" && cleanContentText != "" {
		sqlQuery = extractSQLFromMarkdown(cleanContentText)
	}

	sqlQuery = sanitizeSQL(sqlQuery)
	if sqlQuery == "" {
		if len(cleanContentText) > 0 {
			return "", &models.AppError{
				Code:    constants.ErrCodeAiRefusal,
				Message: cleanContentText,
			}
		}
		return "", errors.New("SQL tidak aman atau tidak valid")
	}

	return sqlQuery, nil
}

// ValidateAndRepairSQL performs a dry-run check against database; if DB errors, triggers RepairSQLFromAI self-correction

func ValidateAndRepairSQL(ctx context.Context, generatedSQL string, userPrompt string) string {
	if database.DbInstance == nil {
		return generatedSQL
	}

	cleanSQL := strings.TrimSuffix(strings.TrimSpace(generatedSQL), ";")
	if cleanSQL == "" {
		return generatedSQL
	}

	explainQuery := fmt.Sprintf("EXPLAIN PLAN FOR %s", cleanSQL)
	evalCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	_, err := database.DbInstance.ExecContext(evalCtx, explainQuery)
	if err == nil {
		log.Println("[INFO] Dry-run EXPLAIN PLAN validation sukses.")
		return generatedSQL
	}

	log.Printf("[ai][ai_service] Dry-run SQL gagal (%v). Memulai perbaikan otomatis...", err)

	repairedSQL, repairErr := RepairSQLFromAI(userPrompt, cleanSQL, err.Error())
	if repairErr != nil || repairedSQL == "" {
		log.Printf("[ai][ai_service] Perbaikan otomatis gagal: %v", repairErr)
		return generatedSQL
	}

	log.Printf("🛠️ Auto-Repair Sukses! SQL Baru: %s", repairedSQL)
	return repairedSQL
}

func EnhanceNaturalLanguage(draft string) (string, error) {
	if config.AppConfig.GroqAPIKey == "" {
		err := fmt.Errorf("API Key Groq belum diset")
		log.Println("[ai][ai_service][EnhanceNaturalLanguage] error:", err)
		return "", err
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
	opts := GroqOptions{
		Temperature: 0.1,
		TopP:        1.0,
	}

	return callGroqAPI(finalPrompt, "llama-3.1-8b-instant", opts)
}

func RepairSQLFromAI(promptAsli string, sqlSalah string, pesanError string) (string, error) {
	if config.AppConfig == nil {
		err := fmt.Errorf("konfigurasi belum dimuat")
		log.Println("[ai][ai_service][RepairSQLFromAI] error:", err)
		return "", err
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

	opts := GroqOptions{
		Temperature:     0.6,
		TopP:            0.95,
		ReasoningFormat: "hidden",
	}
	rawContent, err := callGroqAPI(systemPrompt, config.AppConfig.GroqModel, opts)
	if err != nil {
		log.Println("[ai][ai_service][RepairSQLFromAI] error:", err)
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

func CheckSemanticCache(ctx context.Context, vector []float32) (*models.AISqlResponse, string, error) {
	log.Println("Mencari di Semantic Cache Qdrant (gRPC)...")

	if qdrantClient == nil {
		log.Println("[ai][ai_service][CheckSemanticCache] warning: qdrantClient belum terinisialisasi")
		return nil, "", fmt.Errorf("qdrant client belum terinisialisasi")
	}

	var searchLimit uint64 = config.AppConfig.CacheSearchLimit
	searchResponse, err := qdrantClient.Query(ctx, &pb.QueryPoints{
		CollectionName: config.AppConfig.QdrantCacheCollection,
		Query:          pb.NewQuery(vector...),
		WithPayload:    pb.NewWithPayload(true),
		Limit:          &searchLimit,
	})

	if err != nil {
		log.Println("[ai][ai_service][CheckSemanticCache] error:", err)
		return nil, "", fmt.Errorf("qdrant grpc error: %w", err)
	}

	if len(searchResponse) > 0 {
		cachedPoint := searchResponse[0]
		topScore := cachedPoint.Score

		cachedSql := cachedPoint.GetPayload()["sql_query"].GetStringValue()
		cachedPrompt := cachedPoint.GetPayload()["prompt_asli"].GetStringValue()

		if topScore >= config.AppConfig.CacheSimilarityThreshold {
			log.Printf("[INFO] SEMANTIC CACHE HIT! Skor: %f", topScore)
			return &models.AISqlResponse{SQL: cachedSql, IsCached: true}, "", nil
		}

		if topScore >= constants.CacheSoftHitThreshold {
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
	if qdrantClient == nil {
		log.Println("[ai][ai_service][getRAGContext] warning: qdrantClient belum terinisialisasi")
		return "TIDAK ADA CONTOH SQL. GUNAKAN LOGIKA SENDIRI."
	}

	var searchLimit uint64 = 4
	req := &pb.QueryPoints{
		CollectionName: config.AppConfig.QdrantCollectionName,
		Query:          pb.NewQuery(vector...),
		WithPayload:    pb.NewWithPayload(true),
		Limit:          &searchLimit,
		Filter: &pb.Filter{
			Must: []*pb.Condition{{
				ConditionOneOf: &pb.Condition_Field{Field: &pb.FieldCondition{Key: "category", Match: &pb.Match{MatchValue: &pb.Match_Keyword{Keyword: constants.CategorySQL}}}},
			}},
		},
	}

	searchResponse, err := qdrantClient.Query(ctx, req)
	if err != nil && strings.Contains(err.Error(), "Index required") {
		log.Println("[ai] Index 'category' belum ada di Qdrant, membuat otomatis...")
		_ = EnsureCategoryPayloadIndex(ctx, config.AppConfig.QdrantCollectionName)
		searchResponse, err = qdrantClient.Query(ctx, req)
	}

	if err != nil || len(searchResponse) == 0 {
		if err != nil {
			log.Println("[ai][ai_service][getRAGContext] error:", err)
		}
		return "TIDAK ADA CONTOH SQL. GUNAKAN LOGIKA SENDIRI."
	}


	if searchResponse[0].Score < constants.RAGMinimumScore {
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
		if err != nil {
			log.Println("[ai][ai_service][getDDLContext] error:", err)
		}
		log.Println("Fallback ke load SEMUA tabel.")
		allDDLs, err := GetDynamicSchemaContext()
		if err != nil {
			log.Println("[ai][ai_service][getDDLContext] error:", err)
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
BERIKUT ADALAH SATU-SATUNYA SUMBER KEBENARAN UNTUK NAMA TABEL DAN KOLOM.
JANGAN pernah menggunakan nama tabel atau kolom yang tidak tercantum di bawah ini.
%s
ATURAN KERAS: Jika user meminta data atau entitas lain yang TIDAK ADA di schema di atas, MAKA ITU TIDAK ADA. Jangan mengarang tabel.

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
1. **STRICT SCHEMA ONLY**: Hanya gunakan tabel dan kolom yang TERTULIS EKSPLISIT di bagian "KAMUS DATA".
2. **NO HALLUCINATION**: Jika konsep user tidak ada tabelnya, cari tabel yang paling relevan (misal "transaksi" tipe kredit) atau kembalikan error "Data tidak ditemukan di schema".
3. **Priority Reference**: Jika user menyebut "Tabungan", "Deposito", "Aktif", atau "Tutup", WAJIB cek bagian "LIVE DATA REFERENSI" untuk mendapatkan ID yang tepat. Jangan menebak "1" atau "0".
4. **Security**: Hanya SELECT. Dilarang INSERT/UPDATE/DELETE.

== TUGAS ANDA (CHAIN OF THOUGHT) ==
Sebelum menulis kode SQL, jelaskan langkah berpikir Anda secara singkat di dalam tag <thought>.
Isi <thought> TIDAK AKAN ditampilkan ke user, jadi tulislah analisis teknis di sini.
Di LUAR tag <thought>, tuliskan pesan untuk user (jika menolak) atau Kode SQL (jika berhasil).

FORMAT OUTPUT WAJIB:
<thought>
1. Validasi Schema: ...
2. Analisis Intent: ...
</thought>
[PESAN USER SYSTEM / SQL CODE]

Aturan Pesan Penolakan (Jika data tidak ada):
1. JANGAN menyebutkan istilah teknis seperti "Schema", "DDL", "Metadata", atau nama tabel (misal "MASTER_TIPE_NASABAH") kepada user.
2. Gunakan bahasa natural dan sopan. Contoh: "Mohon maaf, data terkait pinjaman belum tersedia dalam sistem kami saat ini."

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
	if qdrantClient == nil {
		log.Println("[ai][ai_service][searchRelevantDDL] warning: qdrantClient belum terinisialisasi")
		return "", fmt.Errorf("qdrant client belum terinisialisasi")
	}

	var limit uint64 = 4
	req := &pb.QueryPoints{
		CollectionName: config.AppConfig.QdrantCollectionName,
		Query:          pb.NewQuery(promptVector...),
		WithPayload:    pb.NewWithPayload(true),
		Limit:          &limit,
		Filter: &pb.Filter{
			Must: []*pb.Condition{{
				ConditionOneOf: &pb.Condition_Field{Field: &pb.FieldCondition{Key: "category", Match: &pb.Match{MatchValue: &pb.Match_Keyword{Keyword: constants.CategoryDDL}}}},
			}},
		},
	}

	searchResponse, err := qdrantClient.Query(ctx, req)
	if err != nil && strings.Contains(err.Error(), "Index required") {
		log.Println("[ai] Index 'category' belum ada di Qdrant, membuat otomatis...")
		_ = EnsureCategoryPayloadIndex(ctx, config.AppConfig.QdrantCollectionName)
		searchResponse, err = qdrantClient.Query(ctx, req)
	}

	if err != nil {
		log.Println("[ai][ai_service][searchRelevantDDL] error:", err)
		return "", err
	}


	var sb strings.Builder
	count := 0
	for _, p := range searchResponse {
		if p.Score < constants.DDLMinimumScore {
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

func GenerateInsightStream(ctx context.Context, promptAsli string, tableData QueryResult, modelName string, chunkChan chan<- string, errChan chan<- error) {

	var sb strings.Builder

	sb.WriteString("| ")
	sb.WriteString(strings.Join(tableData.Columns, " | "))
	sb.WriteString(" |\n|")
	for range tableData.Columns {
		sb.WriteString("---|")
	}
	sb.WriteString("\n")

	// Isi Tabel
	for _, row := range tableData.Rows {
		sb.WriteString("| ")
		for i, val := range row {
			if i > 0 {
				sb.WriteString(" | ")
			}
			valStr := strings.ReplaceAll(fmt.Sprintf("%v", val), "|", "")
			valStr = strings.ReplaceAll(valStr, "\n", " ")
			valStr = strings.TrimSpace(valStr)
			if valStr == "" {
				valStr = "-"
			}
			sb.WriteString(valStr)
		}
		sb.WriteString(" |\n")

		if sb.Len() > 14000 {
			break
		}
	}

	dataStr := sb.String()

	systemPrompt := fmt.Sprintf(`Anda adalah analis keuangan senior di sebuah bank yang berbicara langsung kepada manajemen/eksekutif.

=== PERTANYAAN ===
"%s"

=== DATA ===
%s

=== INSTRUKSI UTAMA ===

Sebelum menjawab, tentukan dulu karakter data yang Anda terima:

── JIKA DATA BERISI DAFTAR / LIST (data nasabah, rekening, transaksi per entitas, dll):
   • Sebutkan berapa total record/entitas yang ditemukan
   • Sebutkan entitas-entitasnya secara natural (nama, ID, nilai)
   • Highlight yang paling menonjol: nilai tertinggi, terendah, atau yang perlu perhatian
   • Jika ada angka/saldo, bandingkan antar entitas
   • JANGAN paksa membuat total/akumulasi jika data memang tidak mengandung itu

── JIKA DATA BERISI ANGKA AGREGAT / SUMMARY (total, akumulasi, rata-rata, perbandingan):
   • Sebutkan angka utama (grand total / keseluruhan) di awal
   • Sebutkan kontributor terbesar & terkecil beserta persentasenya dari total
   • Berikan perbandingan antar entitas jika ada (kantor, produk, periode)
   • Tambahkan 1 kalimat rekomendasi/kesimpulan yang actionable

── JIKA DATA CAMPURAN (ada detail sekaligus ada angka total):
   • Mulai dari total/kesimpulan besar
   • Lalu breakdown per entitas yang signifikan
   • Highlight anomali atau yang paling menarik perhatian

=== ATURAN WAJIB ===

Format angka selalu: Rp 1.234.567 (bukan 1234567)
Persentase: hitung dari data yang ada, bulatkan 1 desimal (contoh: 42,3%%)
Jika data ada baris TOTAL/GRAND TOTAL → pakai angka itu, JANGAN hitung ulang manual
Jika data hanya 1 baris → fokus jelaskan entitas itu saja, jangan dipaksakan perbandingan
Respons: 2-4 paragraf atau poin singkat, padat, tidak bertele-tele
Gunakan bahasa natural seperti sedang presentasi ke atasan

Jangan sebut: "tabel", "baris", "kolom", "dataset", "query", "JSON", "data di atas"  
Jangan halu: HANYA gunakan angka/nama yang benar-benar ada dalam data
Jangan ulangi pertanyaan user
Jangan paksa format insight agregat jika data adalah list, dan sebaliknya
Jika data kosong → katakan terus terang bahwa tidak ada data yang ditemukan

=== CONTOH ===

Pertanyaan: "tampilkan nasabah dengan saldo >= 7.485.000"
Data: 4 nasabah (CV Sinar Terang 64jt, PT Maju Jaya 54jt, Ahmad Wijaya 24jt, Budi Santoso 7,4jt)

Output yang BENAR:
"Terdapat 4 nasabah yang memenuhi kriteria saldo minimum Rp 7.485.000. 
CV. Sinar Terang mencatat saldo tertinggi sebesar Rp 64.975.000, diikuti PT. Maju Jaya Abadi 
dengan Rp 54.975.000 — keduanya merupakan nasabah korporat yang mendominasi daftar ini. 
Di sisi individu, Ahmad Wijaya (Rp 24.000.000) dan Budi Santoso (Rp 7.485.000) melengkapi daftar. 
Perlu diperhatikan bahwa Budi Santoso berada tepat di ambang batas minimum saldo."

---

Pertanyaan: "total pendapatan bunga kredit"
Data: 36 baris per produk per kantor + grand total 5,8 miliar

Output yang BENAR:
"Total pendapatan bunga dari seluruh portofolio kredit mencapai Rp 5.813.949.691. 
Kredit Angsuran Tetap (KAT) di Cianjur menjadi kontributor terbesar dengan Rp 370.713.714 atau sekitar 6,4%% dari total. 
Kantor Cianjur secara keseluruhan mendominasi pendapatan bunga dibanding KPO dan Hogel.
Produk Kredit Fintech Pokok Tetap mencatat angka terendah di Rp 1.000.003 — perlu evaluasi relevansinya."`, promptAsli, dataStr)

	opts := GroqOptions{
		Temperature:     0.7,
		TopP:            0.9,
		ReasoningFormat: "hidden",
	}

	// Menggunakan model yang lebih pintar dan robust untuk membaca insight kompleks sesuai request user
	modelName = "openai/gpt-oss-120b"

	go CallGroqAPIStream(ctx, systemPrompt, modelName, opts, chunkChan, errChan)
}
