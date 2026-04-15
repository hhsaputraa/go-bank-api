package ai

import (
	"context"
	"log"
	"strings"
	"time"

	config "go-bank-api/config"
	database "go-bank-api/database"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

func MainTrain() {
	log.Println("Memulai proses Training Pengetahuan")

	if err := godotenv.Load(); err != nil {
		log.Println("[ai][train][MainTrain] error:", err)
		log.Fatalf("⚠️Error memuat .env: %v", err)
	}

	_, err := config.LoadConfig()
	if err != nil {
		log.Println("[ai][train][MainTrain] error:", err)
		log.Fatalf("⚠️Error memuat konfigurasi: %v", err)
	}
	log.Println("✅ Konfigurasi berhasil dimuat")

	ctx := context.Background()

	// Connect to database
	if err := database.ConnectDB(); err != nil {
		log.Println("[ai][train][MainTrain] error:", err)
		log.Fatalf("⚠️Gagal koneksi ke DB Postgres: %v", err)
	}
	log.Println("Koneksi DB Postgres untuk baca skema... OK.")

	log.Printf("agar bersih...", config.AppConfig.QdrantCollectionName)
	if err := qdrantDeleteCollection(ctx, config.AppConfig.QdrantURL, config.AppConfig.QdrantCollectionName); err != nil {
		log.Println("[ai][train][MainTrain] error:", err)
		log.Printf("Gagal menghapus collection (mungkin belum ada): %v", err)
	}
	time.Sleep(3 * time.Second)
	log.Printf("Koneksi 'Penerjemah' (Google AI)... OK. Model: %s", config.AppConfig.EmbeddingModel)

	// Create/recreate Qdrant collection
	log.Printf("🆕 Membuat ulang koleksi '%s'...", config.AppConfig.QdrantCollectionName)
	if err := qdrantCreateCollection(ctx, config.AppConfig.QdrantURL, config.AppConfig.QdrantCollectionName,
		config.AppConfig.EmbeddingVectorSize, config.AppConfig.QdrantDistanceMetric); err != nil {
		log.Println("[ai][train][MainTrain] error:", err)
		log.Fatalf("❌ Gagal membuat koleksi di Qdrant: %v", err)
	}

	log.Println("Mulai 'melatih' (meng-embed dan menyimpan) contekan...")

	dynamicDDLs, err := GetDynamicSchemaContext()
	if err != nil {
		log.Println("[ai][train][MainTrain] error:", err)
		log.Fatalf("Gagal mengambil DDL dinamis: %v", err)
	}

	dynamicSQLExamples, err := GetDynamicSqlExamples()
	if err != nil {
		log.Println("[ai][train][MainTrain] error:", err)
		log.Fatalf("Gagal mengambil contoh SQL dinamis: %v", err)
	}

	var points []qdrantPoint

	// A. PROSES DDL (Label: "ddl")
	log.Printf("Memproses %d DDL...", len(dynamicDDLs))
	for i, content := range dynamicDDLs {
		vector, err := GenerateEmbedding(content)
		if err != nil {
			log.Println("[ai][train][MainTrain] error:", err)
			log.Printf("Skip DDL #%d: %v", i, err)
			continue
		}

		point := qdrantPoint{
			ID:     uuid.NewString(),
			Vector: vector,
			Payload: map[string]interface{}{
				"content":  content,
				"category": "ddl",
			},
		}
		points = append(points, point)
	}

	log.Printf("Memproses %d Contoh SQL...", len(dynamicSQLExamples))
	for i, item := range dynamicSQLExamples {
		cleanPrompt := item.PromptOnly
		cleanPrompt = strings.Replace(cleanPrompt, "-- Pertanyaan: ", "", 1) // Hapus prefix
		cleanPrompt = strings.Replace(cleanPrompt, "\"", "", -1)             // Hapus tanda kutip
		cleanPrompt = strings.TrimSpace(cleanPrompt)
		log.Printf("Embedding Prompt Bersih: '%s'", cleanPrompt)

		vector, err := GenerateEmbedding(cleanPrompt)
		if err != nil {
			log.Println("[ai][train][MainTrain] error:", err)
			log.Printf("Skip SQL #%d: %v", i, err)
			continue
		}

		point := qdrantPoint{
			ID:     uuid.NewString(),
			Vector: vector,
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
		if err := qdrantUpsertPoints(ctx, config.AppConfig.QdrantURL, config.AppConfig.QdrantCollectionName, points); err != nil {
			log.Println("[ai][train][MainTrain] error:", err)
			log.Fatalf("Gagal menyimpan vektor ke Qdrant: %v", err)
		}
	}

	log.Println("-----------------------------------------------")
	log.Printf("✅ 'Training' selesai! Database Vektor '%s' sudah terisi (Dinamis).", config.AppConfig.QdrantCollectionName)
	log.Println("-----------------------------------------------")
}
