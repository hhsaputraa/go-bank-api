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
	"github.com/google/uuid"
	pb "github.com/qdrant/go-client/qdrant"
	"google.golang.org/api/option"
)

var qdrantClient *pb.Client
var geminiEmbedder *genai.EmbeddingModel

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
		Host: AppConfig.QdrantGRPCHost,
		Port: AppConfig.QdrantGRPCPort,
	})
	if err != nil {
		return fmt.Errorf("gagal membuat Qdrant gRPC client: %w", err)
	}
	qdrantClient = client

	log.Printf("Memastikan collection cache '%s' ada via REST...", AppConfig.QdrantCacheCollection)

	if err := qdrantCreateCollection(ctx, AppConfig.QdrantURL, AppConfig.QdrantCacheCollection,
		AppConfig.EmbeddingVectorSize, AppConfig.QdrantDistanceMetric); err != nil {
		return fmt.Errorf("gagal membuat/memverifikasi cache collection: %w", err)
	}

	log.Println("✅ Berhasil terkoneksi ke Layanan Vektor (Google AI & Qdrant).")
	log.Printf("   - Embedding Model: %s", AppConfig.EmbeddingModel)
	log.Printf("   - Qdrant gRPC: %s:%d", AppConfig.QdrantGRPCHost, AppConfig.QdrantGRPCPort)
	log.Printf("   - Qdrant REST: %s", AppConfig.QdrantURL)
	log.Printf("   - RAG Collection: %s", AppConfig.QdrantCollectionName)
	log.Printf("   - Cache Collection: %s", AppConfig.QdrantCacheCollection)
	return nil
}

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
}

func getSQLFromAI_Groq(userPrompt string) (AISqlResponse, error) {
	if AppConfig == nil {
		return AISqlResponse{}, fmt.Errorf("konfigurasi aplikasi belum dimuat")
	}

	ctx := context.Background()

	log.Println("Menerjemahkan prompt user ke vektor...")
	res, err := geminiEmbedder.EmbedContent(ctx, genai.Text(userPrompt))
	if err != nil {
		return AISqlResponse{}, fmt.Errorf("gagal embed prompt user: %w", err)
	}
	promptVector := res.Embedding.Values

	log.Println("Mencari di Semantic Cache Qdrant (REST)...")

	searchReq := qdrantSearchReq{
		Vector:      promptVector,
		Limit:       AppConfig.CacheSearchLimit,
		WithPayload: true,
	}

	cacheResponse, err := qdrantSearchPoints(ctx, AppConfig.QdrantURL, AppConfig.QdrantCacheCollection, searchReq)
	if err != nil {
		log.Printf("PERINGATAN: Gagal mencari di cache Qdrant: %v", err)
	}

	if len(cacheResponse.Result) > 0 {
		cachedPoint := cacheResponse.Result[0]
		topScore := cachedPoint.Score

		if topScore >= AppConfig.CacheSimilarityThreshold {
			if cachedSql, ok := cachedPoint.Payload["sql_query"]; ok {
				log.Printf("✅ SEMANTIC CACHE HIT! Skor: %f (Melebihi Threshold: %f)", topScore, AppConfig.CacheSimilarityThreshold)
				return AISqlResponse{SQL: cachedSql.(string), IsCached: true}, nil
			} else {
				log.Printf("CACHE MISS. Ditemukan item cache (Skor: %f) tapi payload 'sql_query' hilang.", topScore)
			}
		} else {
			log.Printf("CACHE MISS. Skor tertinggi: %f (Dibawah Threshold: %f)", topScore, AppConfig.CacheSimilarityThreshold)
		}
	} else {
		log.Println("CACHE MISS. Tidak ada item cache yang cocok ditemukan.")
	}

	log.Println("Memanggil RAG (gRPC) + Groq AI...")
	log.Println("Mencari konteks relevan di Qdrant (RAG)...")

	var searchLimit uint64 = 10

	searchResponse, err := qdrantClient.Query(ctx, &pb.QueryPoints{
		CollectionName: AppConfig.QdrantCollectionName,
		Query:          pb.NewQuery(promptVector...),
		WithPayload:    pb.NewWithPayload(true),
		Limit:          &searchLimit,
		Filter: &pb.Filter{
			Must: []*pb.Condition{
				{
					ConditionOneOf: &pb.Condition_Field{
						Field: &pb.FieldCondition{
							Key: "category",
							Match: &pb.Match{
								MatchValue: &pb.Match_Keyword{
									Keyword: "sql",
								},
							},
						},
					},
				},
			},
		},
	})
	if err != nil {
		return AISqlResponse{}, fmt.Errorf("gagal mencari RAG di Qdrant: %w", err)
	}
	const SimilarityConfidenceThreshold = 0.7

	if len(searchResponse) > 0 {
		topResult := searchResponse[0]
		log.Printf("🔍 Top RAG Score: %f", topResult.Score)

		if topResult.Score < SimilarityConfidenceThreshold {
			log.Println("⚠️ Prompt User Ambigu/Random. Memberikan saran...")

			seen := make(map[string]bool)
			var suggestions []string

			addSuggestion := func(s string) {
				clean := s
				if idx := strings.Index(strings.ToLower(clean), "pertanyaan:"); idx != -1 {
					clean = clean[idx+len("pertanyaan:"):]
				}
				clean = strings.ReplaceAll(clean, "--", "")
				clean = strings.ReplaceAll(clean, "\"", "")
				clean = strings.TrimSpace(clean)
				if clean == "" {
					return
				}
				lower := strings.ToLower(clean)
				if !seen[lower] {
					suggestions = append(suggestions, clean)
					seen[lower] = true
				}
			}

			for _, item := range searchResponse {
				if p := item.GetPayload(); p != nil {
					if v, ok := p["prompt_preview"]; ok {
						addSuggestion(v.GetStringValue())
					} else if v, ok := p["content"]; ok {
						fullContent := v.GetStringValue()
						lines := strings.Split(fullContent, "\n")
						if len(lines) > 0 {
							clean := strings.Replace(lines[0], "-- Pertanyaan: ", "", 1)
							clean = strings.Replace(clean, "\"", "", -1)
							addSuggestion(clean)
						}
					}
				}
				if len(suggestions) >= 3 {
					break
				}
			}

			if len(cacheResponse.Result) > 0 {
				for _, item := range cacheResponse.Result {
					if len(suggestions) >= 5 {
						break
					}

					if val, ok := item.Payload["prompt_asli"]; ok {
						if promptStr, ok := val.(string); ok {
							addSuggestion(promptStr)
						}
					}
				}
			}
			if len(suggestions) < 5 {
				for _, item := range searchResponse {
					if len(suggestions) >= 5 {
						break
					}

					if p := item.GetPayload(); p != nil {
						if v, ok := p["prompt_preview"]; ok {
							addSuggestion(v.GetStringValue())
						} else if v, ok := p["content"]; ok {
							fullContent := v.GetStringValue()
							lines := strings.Split(fullContent, "\n")
							if len(lines) > 0 {
								clean := strings.Replace(lines[0], "-- Pertanyaan: ", "", 1)
								clean = strings.Replace(clean, "\"", "", -1)
								addSuggestion(clean)
							}
						}
					}
				}
			}

			return AISqlResponse{
				IsAmbiguous: true,
				Suggestions: suggestions,
				PromptAsli:  userPrompt,
			}, nil
		}
	} else {
		return AISqlResponse{
			IsAmbiguous: true,
			Suggestions: []string{"Tidak ada data yang mirip. Coba gunakan kata kunci yang lebih spesifik."},
			PromptAsli:  userPrompt,
		}, nil
	}

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
		return AISqlResponse{}, fmt.Errorf("gagal mengambil DDL dinamis untuk prompt: %w", err)
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
		allDDLString,
		sqlContext,
		userPrompt,
	)

	groqReqBody := GroqRequest{
		Model:       AppConfig.GroqModel,
		Messages:    []GroqMessage{{Role: "user", Content: finalPrompt}},
		Temperature: 0,
	}
	jsonBody, err := json.Marshal(groqReqBody)
	if err != nil {
		return AISqlResponse{}, err
	}
	req, err := http.NewRequest("POST", AppConfig.GroqAPIURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return AISqlResponse{}, err
	}
	req.Header.Set("Authorization", "Bearer "+AppConfig.GroqAPIKey)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: AppConfig.GroqTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return AISqlResponse{}, err
	}
	defer resp.Body.Close()
	respBodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return AISqlResponse{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return AISqlResponse{}, fmt.Errorf("Groq merespon dengan error: %s", string(respBodyBytes))
	}
	var groqResp GroqResponse
	if err := json.Unmarshal(respBodyBytes, &groqResp); err != nil {
		return AISqlResponse{}, err
	}
	if len(groqResp.Choices) == 0 {
		return AISqlResponse{}, errors.New("AI tidak memberikan balasan")
	}

	sqlQuery := strings.TrimSpace(groqResp.Choices[0].Message.Content)
	sqlQuery = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(sqlQuery, "```sql"), "```"))
	log.Println("SQL dari AI (Dynamic RAG):", sqlQuery)

	return AISqlResponse{
		SQL:        sqlQuery,
		Vector:     promptVector,
		PromptAsli: userPrompt,
		IsCached:   false,
	}, nil
}

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

type qdrantPoint struct {
	ID      string                 `json:"id"`
	Vector  []float32              `json:"vector"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

func getQdrantBaseURL() string {
	if AppConfig != nil {
		return AppConfig.QdrantURL
	}
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

	timeout := 60 * time.Second
	if AppConfig != nil {
		timeout = AppConfig.QdrantTimeout
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("gagal call %s %s: %w", method, url, err)
	}
	defer func() {

	}()

	respBody, readErr := io.ReadAll(resp.Body)
	resp.Body.Close()
	if readErr != nil {
		return resp, nil, fmt.Errorf("gagal baca response body: %w", readErr)
	}
	return resp, respBody, nil
}

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
		return nil
	}

	if (resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusConflict) &&
		strings.Contains(string(body), "already exists") {
		log.Printf("Collection '%s' sudah ada, tidak perlu dibuat ulang.", name)
		return nil
	}

	return fmt.Errorf("create collection status %d: %s", resp.StatusCode, string(body))
}

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
	ID      interface{}            `json:"id"`
	Version int                    `json:"version"`
	Score   float32                `json:"score"`
	Payload map[string]interface{} `json:"payload"`
}

type QdrantDataResponse struct {
	ID      string                 `json:"id"`
	Payload map[string]interface{} `json:"payload"`
	Vector  []float32              `json:"vector,omitempty"` // Opsional, kalau mau lihat vectornya
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

func SaveToCache(promptAsli string, promptVector []float32, sqlQuery string) {
	go func() {
		if AppConfig == nil {
			log.Println("PERINGATAN: Konfigurasi belum dimuat, tidak bisa menyimpan ke cache")
			return
		}

		ctx := context.Background()

		log.Println("Menyimpan hasil (yang sudah tervalidasi) ke Semantic Cache (REST)...")

		newPoint := qdrantPoint{
			ID:     uuid.NewString(),
			Vector: promptVector,
			Payload: map[string]interface{}{
				"prompt_asli": promptAsli,
				"sql_query":   sqlQuery,
			},
		}

		err := qdrantUpsertPoints(ctx, AppConfig.QdrantURL, AppConfig.QdrantCacheCollection, []qdrantPoint{newPoint})
		if err != nil {
			log.Printf("PERINGATAN: Gagal menyimpan ke cache Qdrant: %v", err)
		} else {
			log.Println("Berhasil menyimpan ke cache.")
		}
	}()
}

func convertQdrantValue(value *pb.Value) interface{} {
	switch k := value.Kind.(type) {
	case *pb.Value_NullValue:
		return nil
	case *pb.Value_DoubleValue:
		return k.DoubleValue
	case *pb.Value_IntegerValue:
		return k.IntegerValue
	case *pb.Value_StringValue:
		return k.StringValue
	case *pb.Value_BoolValue:
		return k.BoolValue
	case *pb.Value_StructValue:
		result := make(map[string]interface{})
		for key, v := range k.StructValue.Fields {
			result[key] = convertQdrantValue(v)
		}
		return result
	case *pb.Value_ListValue:
		var result []interface{}
		for _, v := range k.ListValue.Values {
			result = append(result, convertQdrantValue(v))
		}
		return result
	default:
		return nil
	}
}

func GetAllQdrantPoints(collectionName string, limit uint32) ([]QdrantDataResponse, error) {
	ctx := context.Background()

	scrollResp, err := qdrantClient.Scroll(ctx, &pb.ScrollPoints{
		CollectionName: collectionName,
		Limit:          &limit,
		WithPayload:    pb.NewWithPayload(true),
		WithVectors:    pb.NewWithVectors(false),
		Offset:         nil,
	})

	if err != nil {
		return nil, fmt.Errorf("gagal scroll data qdrant: %w", err)
	}

	var results []QdrantDataResponse
	for _, item := range scrollResp {
		var idStr string
		if item.Id.GetUuid() != "" {
			idStr = item.Id.GetUuid()
		} else {
			idStr = fmt.Sprintf("%d", item.Id.GetNum())
		}

		cleanPayload := make(map[string]interface{})
		for key, value := range item.Payload {
			cleanPayload[key] = convertQdrantValue(value)
		}

		results = append(results, QdrantDataResponse{
			ID:      idStr,
			Payload: cleanPayload,
		})
	}

	return results, nil
}

func UpdateQdrantPoint(collectionName string, id string, prompt string, sqlQuery string) error {
	vector, err := GenerateEmbedding(prompt)
	if err != nil {
		return fmt.Errorf("gagal generate embedding saat update: %w", err)
	}

	point := qdrantPoint{
		ID:     id,
		Vector: vector,
		Payload: map[string]interface{}{
			"prompt_asli": prompt,
			"sql_query":   sqlQuery,
		},
	}

	ctx := context.Background()
	err = qdrantUpsertPoints(ctx, AppConfig.QdrantURL, collectionName, []qdrantPoint{point})
	if err != nil {
		return fmt.Errorf("gagal update ke qdrant: %w", err)
	}

	log.Printf("✅ UPDATE SUKSES: Collection '%s', ID '%s'", collectionName, id)
	return nil
}

func GenerateEmbedding(text string) ([]float32, error) {
	if geminiEmbedder == nil {
		return nil, fmt.Errorf("service embedding belum diinisialisasi")
	}

	ctx := context.Background()
	res, err := geminiEmbedder.EmbedContent(ctx, genai.Text(text))
	if err != nil {
		return nil, err
	}

	return res.Embedding.Values, nil
}

func ManualInjectCache(promptAsli string, sqlQuery string) error {
	vector, err := GenerateEmbedding(promptAsli)
	if err != nil {
		return fmt.Errorf("gagal membuat embedding: %w", err)
	}

	point := qdrantPoint{
		ID:     uuid.NewString(),
		Vector: vector,
		Payload: map[string]interface{}{
			"prompt_asli": promptAsli,
			"sql_query":   sqlQuery,
		},
	}

	// 3. Simpan langsung ke Collection Cache
	ctx := context.Background()
	err = qdrantUpsertPoints(ctx, AppConfig.QdrantURL, AppConfig.QdrantCacheCollection, []qdrantPoint{point})
	if err != nil {
		return fmt.Errorf("gagal upsert ke qdrant: %w", err)
	}

	log.Printf("✅ MANUAL CACHE INJECT: Berhasil menyimpan prompt '%s'", promptAsli)
	return nil
}

func DeleteCacheByPrompt(prompt string) error {
	if AppConfig == nil {
		return fmt.Errorf("konfigurasi belum dimuat")
	}
	ctx := context.Background()
	_, err := qdrantClient.Delete(ctx, &pb.DeletePoints{
		CollectionName: AppConfig.QdrantCacheCollection,
		Points: &pb.PointsSelector{
			PointsSelectorOneOf: &pb.PointsSelector_Filter{
				Filter: &pb.Filter{
					Must: []*pb.Condition{
						{
							ConditionOneOf: &pb.Condition_Field{
								Field: &pb.FieldCondition{
									Key: "prompt_asli",
									Match: &pb.Match{
										MatchValue: &pb.Match_Keyword{
											Keyword: prompt,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	})

	if err != nil {
		return fmt.Errorf("gagal menghapus cache qdrant: %w", err)
	}

	log.Printf("Berhasil menghapus memori cache untuk prompt: '%s'", prompt)
	return nil
}
