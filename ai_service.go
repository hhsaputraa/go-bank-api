package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	"github.com/google/uuid"
	pb "github.com/qdrant/go-client/qdrant"
	"google.golang.org/api/option"
)

var (
	qdrantClient   *pb.Client
	geminiEmbedder *genai.EmbeddingModel
)

type GroqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type GroqRequest struct {
	Model       string        `json:"model"`
	Messages    []GroqMessage `json:"messages"`
	Temperature float32       `json:"temperature"`
}

type GroqResponse struct {
	Choices []struct {
		Message GroqMessage `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type qdrantPoint struct {
	ID      string                 `json:"id"`
	Vector  []float32              `json:"vector"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

type qdrantSearchReq struct {
	Vector         []float32 `json:"vector"`
	Limit          uint64    `json:"limit"`
	WithPayload    bool      `json:"with_payload"`
	ScoreThreshold float32   `json:"score_threshold"`
}

type qdrantSearchResp struct {
	Result []qdrantSearchResult `json:"result"`
}

type qdrantSearchResult struct {
	ID      interface{}            `json:"id"`
	Score   float32                `json:"score"`
	Payload map[string]interface{} `json:"payload"`
}

type QdrantDataResponse struct {
	ID      string                 `json:"id"`
	Payload map[string]interface{} `json:"payload"`
}

type IntentResponse struct {
	Category string `json:"category"`
	Reason   string `json:"reason"`
}

func InitVectorService() error {
	if AppConfig == nil {
		return fmt.Errorf("konfigurasi aplikasi belum dimuat")
	}

	ctx := context.Background()

	geminiClient, err := genai.NewClient(ctx, option.WithAPIKey(AppConfig.GoogleAPIKey))
	if err != nil {
		return fmt.Errorf("gagal membuat client Gemini: %w", err)
	}
	geminiEmbedder = geminiClient.EmbeddingModel(AppConfig.EmbeddingModel)

	client, err := pb.NewClient(&pb.Config{
		Host:   AppConfig.QdrantGRPCHost,
		Port:   AppConfig.QdrantGRPCPort,
		APIKey: AppConfig.QdrantAPIKey,
		UseTLS: true,
	})
	if err != nil {
		return fmt.Errorf("gagal membuat Qdrant gRPC client: %w", err)
	}
	qdrantClient = client

	log.Printf("Memastikan collection cache '%s' ada via REST...", AppConfig.QdrantCacheCollection)
	if err := qdrantCreateCollection(ctx, AppConfig.QdrantURL, AppConfig.QdrantCacheCollection, AppConfig.EmbeddingVectorSize, AppConfig.QdrantDistanceMetric); err != nil {
		return fmt.Errorf("gagal membuat/memverifikasi cache collection: %w", err)
	}
	if err := qdrantCreateCollection(ctx, AppConfig.QdrantURL, AppConfig.QdrantCollectionName, AppConfig.EmbeddingVectorSize, AppConfig.QdrantDistanceMetric); err != nil {
		return fmt.Errorf("gagal membuat/memverifikasi RAG collection: %w", err)
	}

	log.Println("Memastikan index payload 'category' tersedia...")
	if err := qdrantCreatePayloadIndex(ctx, AppConfig.QdrantURL, AppConfig.QdrantCollectionName, "category", "keyword"); err != nil {
		log.Printf("Warning: Gagal membuat index payload: %v", err)
	}

	log.Println("✅ Berhasil terkoneksi ke Layanan Vektor (Google AI & Qdrant).")
	return nil
}

func getSQLFromAI_Groq(userPrompt string) (AISqlResponse, error) {
	if isDangerousSQL(userPrompt) {
		log.Printf("SECURITY BLOCK: User input Raw SQL: '%s'", userPrompt)
		return AISqlResponse{}, &AppError{Code: "DANGEROUS_INTENT", Message: "DITOLAK. Silakan ganti pertanyaan Anda."}
	}
	if AppConfig == nil {
		return AISqlResponse{}, fmt.Errorf("konfigurasi aplikasi belum dimuat")
	}

	intent, _ := ClassifyIntent(userPrompt)

	if intent == "CHAT" {
		return AISqlResponse{}, &AppError{
			Code:    "CHIT_CHAT",
			Message: "Halo! Saya Asisten Data Bank Supra. Silakan tanya seputar Perbankan.",
		}
	}

	if intent == "OFF_TOPIC" {
		return AISqlResponse{}, &AppError{
			Code:    "CHIT_CHAT",
			Message: "Maaf, saya hanya bisa menjawab pertanyaan seputar Perbankan. Tidak bisa melayani topik lain.",
		}
	}

	ctx := context.Background()

	log.Println("Menerjemahkan prompt user ke vektor...")
	promptVector, err := GenerateEmbedding(userPrompt)
	if err != nil {
		return AISqlResponse{}, fmt.Errorf("gagal embed prompt user: %w", err)
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
		return AISqlResponse{}, err
	}
	refDataString, _ := GetDynamicReferenceData(ctx)
	businessDict, _ := GetBusinessDictionary(ctx)

	finalPrompt := buildFinalPrompt(userPrompt, allDDLString, refDataString, businessDict, sqlContext, softCacheContext)

	rawContent, err := fetchLLMResponse(ctx, finalPrompt)
	if err != nil {
		return AISqlResponse{}, err
	}

	sqlQuery := extractSQLFromMarkdown(rawContent)
	log.Printf("🤖 RAW AI Response:\n%s\n", rawContent)
	log.Println("SQL dari AI (Dynamic RAG):", sqlQuery)

	sqlQuery = sanitizeSQL(sqlQuery)
	if sqlQuery == "" {
		return AISqlResponse{}, errors.New("SQL tidak aman atau tidak valid")
	}

	return AISqlResponse{
		SQL:        sqlQuery,
		Vector:     promptVector,
		PromptAsli: userPrompt,
		IsCached:   false,
	}, nil
}

func EnhanceNaturalLanguage(draft string) (string, error) {
	if AppConfig.GroqAPIKey == "" {
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
	if AppConfig == nil {
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

	rawContent, err := callGroqAPI(systemPrompt, AppConfig.GroqModel, 0.1)
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

func isDangerousSQL(input string) bool {
	sqlPattern := regexp.MustCompile(`(?i)^\s*(select|insert|update|delete|drop|alter|truncate|create|grant|revoke|with)\b`)
	return sqlPattern.MatchString(input)
}

func GenerateEmbedding(text string) ([]float32, error) {
	if geminiEmbedder == nil {
		return nil, fmt.Errorf("service embedding belum diinisialisasi")
	}
	res, err := geminiEmbedder.EmbedContent(context.Background(), genai.Text(text))
	if err != nil {
		return nil, err
	}
	return res.Embedding.Values, nil
}

func checkSemanticCache(ctx context.Context, vector []float32) (*AISqlResponse, string, error) {
	log.Println("Mencari di Semantic Cache Qdrant (REST)...")
	searchReq := qdrantSearchReq{Vector: vector, Limit: AppConfig.CacheSearchLimit, WithPayload: true}

	cacheResponse, err := qdrantSearchPoints(ctx, AppConfig.QdrantURL, AppConfig.QdrantCacheCollection, searchReq)
	if err != nil {
		return nil, "", err
	}

	if len(cacheResponse.Result) > 0 {
		cachedPoint := cacheResponse.Result[0]
		topScore := cachedPoint.Score

		cachedSql, _ := cachedPoint.Payload["sql_query"].(string)
		cachedPrompt, _ := cachedPoint.Payload["prompt_asli"].(string)

		if topScore >= AppConfig.CacheSimilarityThreshold {
			log.Printf("✅ SEMANTIC CACHE HIT! Skor: %f", topScore)
			return &AISqlResponse{SQL: cachedSql, IsCached: true}, "", nil
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
		CollectionName: AppConfig.QdrantCollectionName,
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

func fetchLLMResponse(ctx context.Context, prompt string) (string, error) {
	if AppConfig != nil && AppConfig.OllamaURL != "" {
		log.Printf("Mencoba Ollama LLM lokal...")
		ollamaReq := map[string]any{"model": AppConfig.OllamaModel, "prompt": prompt, "stream": false}

		_, respBody, err := httpDoJSON(ctx, "POST", strings.TrimRight(AppConfig.OllamaURL, "/")+"/api/generate", ollamaReq)
		if err == nil {
			var oResp map[string]any
			if json.Unmarshal(respBody, &oResp) == nil {
				if r, ok := oResp["response"].(string); ok && r != "" {
					log.Println("✅ Sukses Ollama.")
					return r, nil
				}
			}
		}
		log.Println("Ollama gagal/kosong. Beralih ke Groq.")
	}

	log.Println("Menggunakan Layanan Groq AI...")
	return callGroqAPI(prompt, AppConfig.GroqModel, 0.0)
}

func callGroqAPI(prompt string, model string, temp float32) (string, error) {
	reqBody := GroqRequest{
		Model:       model,
		Messages:    []GroqMessage{{Role: "user", Content: prompt}},
		Temperature: temp,
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", AppConfig.GroqAPIURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+AppConfig.GroqAPIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: AppConfig.GroqTimeout}
	if client.Timeout == 0 {
		client.Timeout = 30 * time.Second
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("koneksi Groq gagal: %w", err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Groq error %d: %s", resp.StatusCode, string(respBytes))
	}

	var groqResp GroqResponse
	if err := json.Unmarshal(respBytes, &groqResp); err != nil {
		return "", err
	}
	if len(groqResp.Choices) == 0 {
		return "", errors.New("Groq tidak merespon")
	}

	result := groqResp.Choices[0].Message.Content
	log.Printf("Token Usage: %d input, %d output", groqResp.Usage.PromptTokens, groqResp.Usage.CompletionTokens)

	return result, nil
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
		CollectionName: AppConfig.QdrantCollectionName,
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

func SaveToCache(promptAsli string, promptVector []float32, sqlQuery string) {
	go func() {
		if AppConfig == nil {
			return
		}
		log.Println("Menyimpan ke Semantic Cache...")
		point := qdrantPoint{
			ID: uuid.NewString(), Vector: promptVector,
			Payload: map[string]interface{}{"prompt_asli": promptAsli, "sql_query": sqlQuery},
		}
		if err := qdrantUpsertPoints(context.Background(), AppConfig.QdrantURL, AppConfig.QdrantCacheCollection, []qdrantPoint{point}); err == nil {
			log.Println("Berhasil update cache.")
		}
	}()
}

func ManualInjectCache(promptAsli string, sqlQuery string) error {
	vec, err := GenerateEmbedding(promptAsli)
	if err != nil {
		return err
	}
	point := qdrantPoint{
		ID: uuid.NewString(), Vector: vec,
		Payload: map[string]interface{}{"prompt_asli": promptAsli, "sql_query": sqlQuery},
	}
	return qdrantUpsertPoints(context.Background(), AppConfig.QdrantURL, AppConfig.QdrantCacheCollection, []qdrantPoint{point})
}

func httpDoJSON(ctx context.Context, method, url string, body any) (*http.Response, []byte, error) {
	var reqBody io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, nil, err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if AppConfig != nil && AppConfig.QdrantAPIKey != "" {
		req.Header.Set("api-key", AppConfig.QdrantAPIKey)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	return resp, respBody, nil
}

func qdrantSearchPoints(ctx context.Context, baseURL, name string, req qdrantSearchReq) (qdrantSearchResp, error) {
	url := fmt.Sprintf("%s/collections/%s/points/search", baseURL, name)
	var data qdrantSearchResp
	resp, body, err := httpDoJSON(ctx, "POST", url, req)
	if err != nil {
		return data, err
	}
	if resp.StatusCode != 200 {
		return data, fmt.Errorf("err %d: %s", resp.StatusCode, string(body))
	}
	json.Unmarshal(body, &data)
	return data, nil
}

func qdrantUpsertPoints(ctx context.Context, baseURL, name string, points []qdrantPoint) error {
	url := fmt.Sprintf("%s/collections/%s/points?wait=true", baseURL, name)
	req := map[string]any{"points": points}
	resp, body, err := httpDoJSON(ctx, "PUT", url, req)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("err %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func qdrantCreateCollection(ctx context.Context, baseURL, name string, size int, distance string) error {
	url := fmt.Sprintf("%s/collections/%s", baseURL, name)
	req := map[string]any{"vectors": map[string]any{"size": size, "distance": distance}}
	resp, body, err := httpDoJSON(ctx, "PUT", url, req)
	if err != nil {
		return err
	}
	if resp.StatusCode == 200 {
		return nil
	}
	if (resp.StatusCode == 400 || resp.StatusCode == 409) && strings.Contains(string(body), "already exists") {
		return nil
	}
	return fmt.Errorf("err %d: %s", resp.StatusCode, string(body))
}

func qdrantCreatePayloadIndex(ctx context.Context, baseURL, collectionName, fieldName, schemaType string) error {
	url := fmt.Sprintf("%s/collections/%s/index", baseURL, collectionName)
	req := map[string]string{"field_name": fieldName, "field_schema": schemaType}
	resp, body, err := httpDoJSON(ctx, "PUT", url, req)
	if err != nil {
		return err
	}
	if resp.StatusCode == 200 {
		return nil
	}
	return fmt.Errorf("err %d: %s", resp.StatusCode, string(body))
}

func DeleteQdrantPoint(ctx context.Context, collectionName string, pointID string) error {
	url := fmt.Sprintf("%s/collections/%s/points/delete?wait=true", AppConfig.QdrantURL, collectionName)
	req := map[string]any{"points": []string{pointID}}
	resp, body, err := httpDoJSON(ctx, "POST", url, req)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("err %d: %s", resp.StatusCode, string(body))
	}
	log.Printf("Berhasil hapus Point ID '%s'", pointID)
	return nil
}

func UpdateQdrantPoint(collectionName string, id string, prompt string, sqlQuery string) error {
	vector, err := GenerateEmbedding(prompt)
	if err != nil {
		return err
	}
	point := qdrantPoint{
		ID: id, Vector: vector,
		Payload: map[string]interface{}{"prompt_asli": prompt, "sql_query": sqlQuery},
	}
	return qdrantUpsertPoints(context.Background(), AppConfig.QdrantURL, collectionName, []qdrantPoint{point})
}

func GetAllQdrantPoints(collectionName string, limit uint32) ([]QdrantDataResponse, error) {
	scrollResp, err := qdrantClient.Scroll(context.Background(), &pb.ScrollPoints{
		CollectionName: collectionName, Limit: &limit, WithPayload: pb.NewWithPayload(true),
	})
	if err != nil {
		return nil, err
	}
	var results []QdrantDataResponse
	for _, item := range scrollResp {
		idStr := item.Id.GetUuid()
		if idStr == "" {
			idStr = fmt.Sprintf("%d", item.Id.GetNum())
		}
		cleanPayload := make(map[string]interface{})
		for k, v := range item.Payload {
			cleanPayload[k] = convertQdrantValue(v)
		}
		results = append(results, QdrantDataResponse{ID: idStr, Payload: cleanPayload})
	}
	return results, nil
}

func convertQdrantValue(value *pb.Value) interface{} {
	switch k := value.Kind.(type) {
	case *pb.Value_StringValue:
		return k.StringValue
	case *pb.Value_IntegerValue:
		return k.IntegerValue
	case *pb.Value_DoubleValue:
		return k.DoubleValue
	case *pb.Value_BoolValue:
		return k.BoolValue
	default:
		return nil
	}
}

func qdrantDeleteCollection(ctx context.Context, baseURL, name string) error {
	url := fmt.Sprintf("%s/collections/%s", baseURL, name)

	resp, body, err := httpDoJSON(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound {
		log.Printf("Collection '%s' berhasil dihapus (atau belum ada).", name)
		return nil
	}

	return fmt.Errorf("gagal hapus collection status %d: %s", resp.StatusCode, string(body))
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
