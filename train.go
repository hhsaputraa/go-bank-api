package main

import (
	"context"
	"log"
	"os"

	"github.com/google/generative-ai-go/genai"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"google.golang.org/api/option"
)

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
			Vector: res.Embedding.Values,
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
