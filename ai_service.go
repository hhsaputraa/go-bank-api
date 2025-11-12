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
	"os"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	"github.com/google/uuid" // Pastikan import ini ada
	pb "github.com/qdrant/go-client/qdrant"
	"google.golang.org/api/option"
)

var qdrantClient *pb.Client
var geminiEmbedder *genai.EmbeddingModel

// === KONSTANTA BARU PINDAH KE BAWAH BERSAMA HELPER LAINNYA ===

func InitVectorService() error {
	ctx := context.Background()

	googleApiKey := os.Getenv("GOOGLE_API_KEY")
	if googleApiKey == "" {
		return errors.New("GOOGLE_API_KEY tidak ditemukan di .env")
	}

	geminiClient, err := genai.NewClient(ctx, option.WithAPIKey(googleApiKey))
	if err != nil {
		return fmt.Errorf("gagal membuat client Gemini: %w", err)
	}
	geminiEmbedder = geminiClient.EmbeddingModel(EMBEDDING_MODEL) // EMBEDDING_MODEL dari train.go

	// 1. Inisialisasi gRPC client (HANYA untuk RAG .Query())
	client, err := pb.NewClient(&pb.Config{
		Host: "localhost",
		Port: 6334,
	})
	if err != nil {
		return fmt.Errorf("gagal membuat Qdrant gRPC client: %w", err)
	}
	qdrantClient = client

	// 2. Inisialisasi REST client (Untuk Cache: Search, Upsert, Create)
	qdrantBase := getQdrantBaseURL()
	log.Printf("Memastikan collection cache '%s' ada via REST...", QDRANT_CACHE_COLLECTION)
	const vectorSize = 768 // Harus sama dengan model embedding

	// Gunakan helper qdrantCreateCollection (dari train.go)
	if err := qdrantCreateCollection(ctx, qdrantBase, QDRANT_CACHE_COLLECTION, vectorSize, "Cosine"); err != nil {
		// qdrantCreateCollection helper sudah di-update di bawah untuk menangani "already exists"
		return fmt.Errorf("gagal membuat/memverifikasi cache collection: %w", err)
	}

	log.Println("✅ Berhasil terkoneksi ke Layanan Vektor (Google AI & Qdrant).")
	return nil
}

type GroqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type GroqRequest struct {
	Model    string        `json:"model"`
	Messages []GroqMessage `json:"messages"`
}
type GroqResponse struct {
	Choices []struct {
		Message GroqMessage `json:"message"`
	} `json:"choices"`
}

func getSQLFromAI_Groq(userPrompt string) (string, error) {

	apiKey := os.Getenv("GROQ_API_KEY")
	if apiKey == "" {
		return "", errors.New("GROQ_API_KEY tidak ditemukan di environment")
	}
	ctx := context.Background()
	qdrantBase := getQdrantBaseURL()

	log.Println("Menerjemahkan prompt user ke vektor...")
	res, err := geminiEmbedder.EmbedContent(ctx, genai.Text(userPrompt))
	if err != nil {
		return "", fmt.Errorf("gagal embed prompt user: %w", err)
	}
	promptVector := res.Embedding.Values

	log.Println("Mencari di Semantic Cache Qdrant (REST)...")
	var cacheSearchLimit uint64 = 1
	var cacheThreshold float32 = 0.95

	searchReq := qdrantSearchReq{
		Vector:      promptVector,
		Limit:       cacheSearchLimit,
		WithPayload: true,
	}

	cacheResponse, err := qdrantSearchPoints(ctx, qdrantBase, QDRANT_CACHE_COLLECTION, searchReq)
	if err != nil {
		log.Printf("PERINGATAN: Gagal mencari di cache Qdrant: %v", err)
	}

	// Periksa apakah ada hasil yang lolos threshold
	if len(cacheResponse.Result) > 0 {
		// Kita dapat hasil teratas, sekarang cek skornya
		cachedPoint := cacheResponse.Result[0]
		topScore := cachedPoint.Score // Ambil skornya

		if topScore >= cacheThreshold {
			// Skor LULUS threshold!
			if cachedSql, ok := cachedPoint.Payload["sql_query"]; ok {
				log.Printf("✅ SEMANTIC CACHE HIT! Skor: %f (Melebihi Threshold: %f)", topScore, cacheThreshold)
				return cachedSql.(string), nil // Lakukan type assertion
			} else {
				// Seharusnya tidak terjadi, tapi bagus untuk dicatat
				log.Printf("CACHE MISS. Ditemukan item cache (Skor: %f) tapi payload 'sql_query' hilang.", topScore)
			}
		} else {
			// Skor GAGAL threshold
			log.Printf("CACHE MISS. Skor tertinggi: %f (Dibawah Threshold: %f)", topScore, cacheThreshold)
		}
	} else {
		// Qdrant tidak menemukan apa-apa
		log.Println("CACHE MISS. Tidak ada item cache yang cocok ditemukan.")
	}

	// Hapus "CACHE MISS" dari log ini
	log.Println("Memanggil RAG (gRPC) + Groq AI...")

	// === LANGKAH 3: PROSES RAG (Tetap pakai gRPC, karena sudah bekerja) ===
	log.Println("Mencari konteks relevan di Qdrant (RAG)...")
	var searchLimit uint64 = 7

	searchResponse, err := qdrantClient.Query(ctx, &pb.QueryPoints{
		CollectionName: QDRANT_COLLECTION_NAME, // Ini collection RAG
		Query:          pb.NewQuery(promptVector...),
		WithPayload:    pb.NewWithPayload(true),
		Limit:          &searchLimit,
	})
	if err != nil {
		return "", fmt.Errorf("gagal mencari RAG di Qdrant: %w", err)
	}

	// ... (Kode perakitan RAG, DDL, dan finalPrompt Anda SAMA PERSIS) ...
	var contextBuilder strings.Builder
	contextBuilder.WriteString("Berikut adalah CONTOH DDL dan SQL yang paling relevan (IKUTI POLA INI):\n")
	for _, point := range searchResponse {
		if p := point.GetPayload(); p != nil {
			if v, ok := p["content"]; ok {
				contekan := v.GetStringValue()
				if contekan != "" {
					contextBuilder.WriteString(contekan)
					contextBuilder.WriteString("\n---\n")
				}
			}
		}
	}
	log.Println("Konteks RAG Dinamis berhasil dirakit.")
	contextContent := contextBuilder.String()
	log.Printf("--- BUKTI PERAKITAN RAG (Konteks) ---\n%s\n--------------------------------------", contextContent)
	sqlContext := contextBuilder.String()
	allDDLs, err := GetDynamicSchemaContext()
	if err != nil {
		return "", fmt.Errorf("gagal mengambil DDL dinamis untuk prompt: %w", err)
	}
	allDDLString := strings.Join(allDDLs, "\n---\n")

	finalPrompt := fmt.Sprintf(`
Anda adalah ahli SQL PostgreSQL. Tanggal hari ini (CURRENT_DATE) adalah %s.

== KAMUS DATABASE (SEMUA DDL) ==
Berikut adalah DDL LENGKAP untuk skema "bpr_supra". JANGAN halusinasi kolom/tabel di luar ini:
%s

== CONTOH SQL (PALING RELEVAN) ==
Berikut adalah CONTOH SQL yang relevan dengan pertanyaan user:
%s

Tugas Anda:
1. Berdasarkan "KAMUS DATABASE" di atas, jawab pertanyaan pengguna.
2. Gunakan "CONTOH SQL" sebagai inspirasi pola.
3. JANGAN pakai markdown (sql). JANGAN tambahkan penjelasan. Hanya SQL.
4. JANGAN PERNAH menggunakan SELECT *; selalu sebutkan nama kolomnya.

Pertanyaan Pengguna: "%s"

Query SQL:`,
		time.Now().Format("2006-01-02"),
		allDDLString, // <-- KAMUS LENGKAP
		sqlContext,   // <-- CONTOH RELEVAN
		userPrompt,
	)

	// ... (Kode Groq Request & Call SAMA PERSIS) ...
	groqReqBody := GroqRequest{
		Model:    "llama-3.1-8b-instant",
		Messages: []GroqMessage{{Role: "user", Content: finalPrompt}},
	}
	jsonBody, err := json.Marshal(groqReqBody)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest("POST", "https://api.groq.com/openai/v1/chat/completions", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Groq merespon dengan error: %s", string(respBodyBytes))
	}
	var groqResp GroqResponse
	if err := json.Unmarshal(respBodyBytes, &groqResp); err != nil {
		return "", err
	}
	if len(groqResp.Choices) == 0 {
		return "", errors.New("AI tidak memberikan balasan")
	}

	sqlQuery := strings.TrimSpace(groqResp.Choices[0].Message.Content)
	sqlQuery = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(sqlQuery, "```sql"), "```"))
	log.Println("SQL dari AI (Dynamic RAG):", sqlQuery)

	// === LANGKAH 4: SIMPAN KE SEMANTIC CACHE (Menggunakan REST) ===
	if sqlQuery != "" {
		log.Println("Menyimpan hasil ke Semantic Cache (REST)...")

		// Buat point baru menggunakan struct qdrantPoint (dari train.go)
		newPoint := qdrantPoint{
			ID:     uuid.NewString(), // ID harus UUID string
			Vector: promptVector,     // Ini sudah []float32
			Payload: map[string]interface{}{
				"prompt_asli": userPrompt,
				"sql_query":   sqlQuery,
			},
		}

		// Gunakan helper qdrantUpsertPoints (dari train.go)
		err := qdrantUpsertPoints(ctx, qdrantBase, QDRANT_CACHE_COLLECTION, []qdrantPoint{newPoint})
		if err != nil {
			log.Printf("PERINGATAN: Gagal menyimpan ke cache Qdrant: %v", err)
		}
	}

	return sqlQuery, nil
}

// --- Helper Qdrant REST API (Disalin & Dimodifikasi dari train.go) ---
// --- KONSTANTA ---
const (
	QDRANT_COLLECTION_NAME  = "bpr_supra_rag"             // Dari train.go
	EMBEDDING_MODEL         = "models/text-embedding-004" // Dari train.go
	QDRANT_CACHE_COLLECTION = "bpr_supra_cache"           // Baru
)

// --- STRUCTS ---
type qdrantCreateCollectionReq struct {
	Vectors qdrantVectors `json:"vectors"`
}
type qdrantVectors struct {
	Size     int    `json:"size"`
	Distance string `json:"distance"`
}
type qdrantUpsertPointsReq struct {
	Points []qdrantPoint `json:"points"`
}

// qdrantPoint sekarang HARUS menyertakan Vector (bukan omitempty) untuk upsert cache
type qdrantPoint struct {
	ID      string                 `json:"id"`     // string UUID
	Vector  []float32              `json:"vector"` // BUKAN omitempty
	Payload map[string]interface{} `json:"payload,omitempty"`
}

// --- FUNGSI HELPER (Disalin dari train.go) ---
func getQdrantBaseURL() string {
	// (Saya asumsikan .env sudah di-load oleh main.go)
	base := os.Getenv("QDRANT_URL")
	if base == "" {
		base = "http://localhost:6333"
	}
	return base
}

func httpDoJSON(ctx context.Context, method, url string, body any) (*http.Response, []byte, error) {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, nil, fmt.Errorf("gagal marshal JSON: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, nil, fmt.Errorf("gagal buat request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("gagal call %s %s: %w", method, url, err)
	}
	defer func() {
		// kita baca body di bawah; jadi defer close di sini tidak menutup sebelum dibaca.
	}()

	respBody, readErr := io.ReadAll(resp.Body)
	resp.Body.Close()
	if readErr != nil {
		return resp, nil, fmt.Errorf("gagal baca response body: %w", readErr)
	}
	return resp, respBody, nil
}

// Dimodifikasi untuk InitVectorService: Tidak error jika "already exists"
func qdrantCreateCollection(ctx context.Context, baseURL, name string, size int, distance string) error {
	url := fmt.Sprintf("%s/collections/%s", baseURL, name)
	req := qdrantCreateCollectionReq{
		Vectors: qdrantVectors{
			Size:     size,
			Distance: distance,
		},
	}
	resp, body, err := httpDoJSON(ctx, http.MethodPut, url, req)
	if err != nil {
		return err
	}

	if resp.StatusCode == http.StatusOK {
		log.Printf("Berhasil membuat collection '%s'.", name)
		return nil // Sukses
	}

	// Jika status 400 (Bad Request) dan pesan "already exists", itu BUKAN error
	if (resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusConflict) &&
		strings.Contains(string(body), "already exists") {
		log.Printf("Collection '%s' sudah ada, tidak perlu dibuat ulang.", name)
		return nil // Dianggap sukses
	}

	return fmt.Errorf("create collection status %d: %s", resp.StatusCode, string(body))
}

// Disalin dari train.go
func qdrantUpsertPoints(ctx context.Context, baseURL, name string, points []qdrantPoint) error {
	url := fmt.Sprintf("%s/collections/%s/points?wait=true", baseURL, name)
	req := qdrantUpsertPointsReq{Points: points}
	resp, body, err := httpDoJSON(ctx, http.MethodPut, url, req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("upsert points status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// === FUNGSI BARU UNTUK SEARCH (REST API) ===
type qdrantSearchReq struct {
	Vector         []float32 `json:"vector"`
	Limit          uint64    `json:"limit"`
	WithPayload    bool      `json:"with_payload"`
	ScoreThreshold float32   `json:"score_threshold"`
}

type qdrantSearchResp struct {
	Result []qdrantSearchResult `json:"result"`
	Status string               `json:"status"`
	Time   float64              `json:"time"`
}

type qdrantSearchResult struct {
	ID      interface{}            `json:"id"` // Bisa string UUID atau uint
	Version int                    `json:"version"`
	Score   float32                `json:"score"`
	Payload map[string]interface{} `json:"payload"`
}

func qdrantSearchPoints(ctx context.Context, baseURL, name string, req qdrantSearchReq) (qdrantSearchResp, error) {
	url := fmt.Sprintf("%s/collections/%s/points/search", baseURL, name)
	var respData qdrantSearchResp

	resp, body, err := httpDoJSON(ctx, http.MethodPost, url, req)
	if err != nil {
		return respData, err
	}
	if resp.StatusCode != http.StatusOK {
		return respData, fmt.Errorf("search points status %d: %s", resp.StatusCode, string(body))
	}

	if err := json.Unmarshal(body, &respData); err != nil {
		return respData, fmt.Errorf("gagal unmarshal search response: %w", err)
	}
	return respData, nil
}
