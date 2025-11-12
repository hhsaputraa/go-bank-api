package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/google/generative-ai-go/genai"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"google.golang.org/api/option"
)

const (
	QDRANT_COLLECTION_NAME = "bpr_supra_rag"
	EMBEDDING_MODEL        = "models/text-embedding-004"
)

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
	ID      string                 `json:"id"`               // string UUID
	Vector  []float32              `json:"vector,omitempty"` // satu vektor
	Vectors map[string][]float32   `json:"vectors,omitempty"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

func getQdrantBaseURL() string {
	if err := godotenv.Load(); err == nil {
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

func qdrantDeleteCollection(ctx context.Context, baseURL, name string) error {
	url := fmt.Sprintf("%s/collections/%s", baseURL, name)
	resp, body, err := httpDoJSON(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound {
		return nil
	}
	return fmt.Errorf("delete collection status %d: %s", resp.StatusCode, string(body))
}

func qdrantCreateCollection(ctx context.Context, baseURL, name string, size int, distance string) error {
	url := fmt.Sprintf("%s/collections/%s", baseURL, name)
	req := qdrantCreateCollectionReq{
		Vectors: qdrantVectors{
			Size:     size,
			Distance: distance, // "Cosine"
		},
	}
	resp, body, err := httpDoJSON(ctx, http.MethodPut, url, req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("create collection status %d: %s", resp.StatusCode, string(body))
	}
	return nil
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

func mainTrain() {
	log.Println("Memulai proses 'Training' Database Vektor...")

	if err := godotenv.Load(); err != nil {
		log.Fatalf("Error memuat .env: %v", err)
	}
	googleApiKey := os.Getenv("GOOGLE_API_KEY")
	if googleApiKey == "" {
		log.Fatal("GOOGLE_API_KEY tidak ditemukan di .env")
	}

	qdrantBase := getQdrantBaseURL()

	ctx := context.Background()

	if err := ConnectDB(); err != nil {
		log.Fatalf("Gagal koneksi ke DB Postgres: %v", err)
	}
	log.Println("Koneksi DB Postgres untuk baca skema... OK.")

	geminiClient, err := genai.NewClient(ctx, option.WithAPIKey(googleApiKey))
	if err != nil {
		log.Fatalf("Gagal membuat client Gemini: %v", err)
	}
	defer geminiClient.Close()
	embedder := geminiClient.EmbeddingModel(EMBEDDING_MODEL)
	log.Println("Koneksi 'Penerjemah' (Google AI)... OK.")

	log.Printf("Menghapus koleksi lama '%s' (jika ada)...", QDRANT_COLLECTION_NAME)
	if err := qdrantDeleteCollection(ctx, qdrantBase, QDRANT_COLLECTION_NAME); err != nil {
		log.Printf("Peringatan: Gagal hapus koleksi lama (mungkin belum ada): %v", err)
	}

	log.Printf("Membuat koleksi baru '%s'...", QDRANT_COLLECTION_NAME)
	const vectorSize = 768
	if err := qdrantCreateCollection(ctx, qdrantBase, QDRANT_COLLECTION_NAME, vectorSize, "Cosine"); err != nil {
		log.Fatalf("Gagal membuat koleksi di Qdrant: %v", err)
	}

	log.Println("Mulai 'melatih' (meng-embed dan menyimpan) contekan...")

	dynamicDDLs, err := GetDynamicSchemaContext()
	if err != nil {
		log.Fatalf("Gagal mengambil DDL dinamis: %v", err)
	}

	dynamicSQLExamples, err := GetDynamicSqlExamples()
	if err != nil {
		log.Fatalf("Gagal mengambil contoh SQL dinamis: %v", err)
	}

	allContekan := append(dynamicDDLs, dynamicSQLExamples...)

	log.Printf("Total %d potongan konteks (DDL + Contoh SQL) akan di-embed.", len(allContekan))

	points := make([]qdrantPoint, 0, len(allContekan))

	for i, contekan := range allContekan {
		res, err := embedder.EmbedContent(ctx, genai.Text(contekan))
		if err != nil {
			log.Printf("PERINGATAN: Gagal embed contekan #%d, dilewati. Error: %v", i, err)
			continue
		}
		if res == nil || res.Embedding == nil || len(res.Embedding.Values) == 0 {
			log.Printf("PERINGATAN: Embedding kosong untuk contekan #%d, dilewati.", i)
			continue
		}

		point := qdrantPoint{
			ID:     uuid.NewString(),
			Vector: res.Embedding.Values, // []float32 dari SDK genai
			Payload: map[string]interface{}{
				"content": contekan,
			},
		}
		points = append(points, point)
		if (i+1)%50 == 0 {
			log.Printf("Progress embed: %d/%d", i+1, len(allContekan))
		}
	}

	log.Println("Menyimpan semua vektor ke Qdrant via REST...")
	if len(points) == 0 {
		log.Println("Tidak ada point untuk di-upsert (semua gagal embed?).")
	} else {
		if err := qdrantUpsertPoints(ctx, qdrantBase, QDRANT_COLLECTION_NAME, points); err != nil {
			log.Fatalf("Gagal menyimpan vektor ke Qdrant: %v", err)
		}
	}

	log.Println("-----------------------------------------------")
	log.Printf("✅ 'Training' selesai! Database Vektor '%s' sudah terisi (Dinamis).", QDRANT_COLLECTION_NAME)
	log.Println("-----------------------------------------------")
}
