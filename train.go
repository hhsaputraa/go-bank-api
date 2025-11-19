package main

import (
	"context"
	"log"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"google.golang.org/api/option"
)

func mainTrain() {
	log.Println("Memulai proses 'Training' Database Vektor...")

	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Fatalf("⚠️Error memuat .env: %v", err)
	}

	// Load configuration
	_, err := LoadConfig()
	if err != nil {
		log.Fatalf("⚠️Error memuat konfigurasi: %v", err)
	}
	log.Println("✅ Konfigurasi berhasil dimuat")

	ctx := context.Background()

	// Connect to database
	if err := ConnectDB(); err != nil {
		log.Fatalf("⚠️Gagal koneksi ke DB Postgres: %v", err)
	}
	log.Println("Koneksi DB Postgres untuk baca skema... OK.")

	// Initialize Google AI client for embeddings
	geminiClient, err := genai.NewClient(ctx, option.WithAPIKey(AppConfig.GoogleAPIKey))
	if err != nil {
		log.Fatalf("Gagal membuat client Gemini: %v", err)
	}
	defer geminiClient.Close()
	embedder := geminiClient.EmbeddingModel(AppConfig.EmbeddingModel)
	log.Printf("Koneksi 'Penerjemah' (Google AI)... OK. Model: %s", AppConfig.EmbeddingModel)

	// Create/recreate Qdrant collection
	log.Printf("Membuat koleksi baru '%s'...", AppConfig.QdrantCollectionName)
	if err := qdrantCreateCollection(ctx, AppConfig.QdrantURL, AppConfig.QdrantCollectionName,
		AppConfig.EmbeddingVectorSize, AppConfig.QdrantDistanceMetric); err != nil {
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

	var points []qdrantPoint

	// A. PROSES DDL (Label: "ddl")
	log.Printf("Memproses %d DDL...", len(dynamicDDLs))
	for i, content := range dynamicDDLs {
		res, err := embedder.EmbedContent(ctx, genai.Text(content))
		if err != nil {
			log.Printf("Skip DDL #%d: %v", i, err)
			continue
		}

		point := qdrantPoint{
			ID:     uuid.NewString(),
			Vector: res.Embedding.Values,
			Payload: map[string]interface{}{
				"content":  content,
				"category": "ddl",
			},
		}
		points = append(points, point)
	}

	// B. PROSES SQL EXAMPLES (Label: "sql")
	log.Printf("Memproses %d Contoh SQL...", len(dynamicSQLExamples))
	for i, item := range dynamicSQLExamples {
		cleanPrompt := item.PromptOnly
		cleanPrompt = strings.Replace(cleanPrompt, "-- Pertanyaan: ", "", 1) // Hapus prefix
		cleanPrompt = strings.Replace(cleanPrompt, "\"", "", -1)             // Hapus tanda kutip
		cleanPrompt = strings.TrimSpace(cleanPrompt)
		log.Printf("Embedding Prompt Bersih: '%s'", cleanPrompt)

		res, err := embedder.EmbedContent(ctx, genai.Text(cleanPrompt)) // Asumsi struct baru
		if err != nil {
			log.Printf("Skip SQL #%d: %v", i, err)
			continue
		}

		point := qdrantPoint{
			ID:     uuid.NewString(),
			Vector: res.Embedding.Values,
			Payload: map[string]interface{}{
				"content":        item.FullContent,
				"prompt_preview": cleanPrompt,
				"category":       "sql",
			},
		}
		points = append(points, point)
	}

	log.Println("Menyimpan semua vektor ke Qdrant via REST...")
	if len(points) == 0 {
		log.Println("Tidak ada point untuk di-upsert (semua gagal embed?).")
	} else {
		if err := qdrantUpsertPoints(ctx, AppConfig.QdrantURL, AppConfig.QdrantCollectionName, points); err != nil {
			log.Fatalf("Gagal menyimpan vektor ke Qdrant: %v", err)
		}
	}

	log.Println("-----------------------------------------------")
	log.Printf("✅ 'Training' selesai! Database Vektor '%s' sudah terisi (Dinamis).", AppConfig.QdrantCollectionName)
	log.Println("-----------------------------------------------")
}
