package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/google/generative-ai-go/genai"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"google.golang.org/api/option"
)

// Konstanta ini sudah dipindahkan ke environment variables
// Lihat file .env untuk konfigurasi
// Shared types dan helper functions ada di qdrant_helpers.go

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

	// Baca konfigurasi dari environment variables
	embeddingModel := GetEnv("EMBEDDING_MODEL", "models/text-embedding-004")
	qdrantCollectionName := GetEnv("QDRANT_COLLECTION_NAME", "bpr_supra_rag")
	vectorSize := GetEnvAsInt("EMBEDDING_VECTOR_SIZE", 768)

	geminiClient, err := genai.NewClient(ctx, option.WithAPIKey(googleApiKey))
	if err != nil {
		log.Fatalf("Gagal membuat client Gemini: %v", err)
	}
	defer geminiClient.Close()
	embedder := geminiClient.EmbeddingModel(embeddingModel)
	log.Println("Koneksi 'Penerjemah' (Google AI)... OK.")

	log.Printf("Menghapus koleksi lama '%s' (jika ada)...", qdrantCollectionName)
	if err := qdrantDeleteCollection(ctx, qdrantBase, qdrantCollectionName); err != nil {
		log.Printf("Peringatan: Gagal hapus koleksi lama (mungkin belum ada): %v", err)
	}

	log.Printf("Membuat koleksi baru '%s'...", qdrantCollectionName)
	if err := qdrantCreateCollection(ctx, qdrantBase, qdrantCollectionName, vectorSize, "Cosine"); err != nil {
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
		if err := qdrantUpsertPoints(ctx, qdrantBase, qdrantCollectionName, points); err != nil {
			log.Fatalf("Gagal menyimpan vektor ke Qdrant: %v", err)
		}
	}

	log.Println("-----------------------------------------------")
	log.Printf("✅ 'Training' selesai! Database Vektor '%s' sudah terisi (Dinamis).", qdrantCollectionName)
	log.Println("-----------------------------------------------")
}
