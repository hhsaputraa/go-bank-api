package ai

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	config "go-bank-api/config"
	database "go-bank-api/database"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

type TrainingProgressState struct {
	mu          sync.RWMutex
	IsTraining  bool     `json:"is_training"`
	Current     int      `json:"current"`
	Total       int      `json:"total"`
	Percentage  int      `json:"percentage"`
	CurrentStep string   `json:"current_step"`
	Logs        []string `json:"logs"`
}

var CurrentTrainProgress = &TrainingProgressState{
	Logs: make([]string, 0),
}

func GetTrainingStatus() TrainingProgressState {
	CurrentTrainProgress.mu.RLock()
	defer CurrentTrainProgress.mu.RUnlock()

	logsCopy := make([]string, len(CurrentTrainProgress.Logs))
	copy(logsCopy, CurrentTrainProgress.Logs)

	return TrainingProgressState{
		IsTraining:  CurrentTrainProgress.IsTraining,
		Current:     CurrentTrainProgress.Current,
		Total:       CurrentTrainProgress.Total,
		Percentage:  CurrentTrainProgress.Percentage,
		CurrentStep: CurrentTrainProgress.CurrentStep,
		Logs:        logsCopy,
	}
}

func setTrainProgress(current, total, percentage int, step string, logMsg string) {
	CurrentTrainProgress.mu.Lock()
	defer CurrentTrainProgress.mu.Unlock()

	CurrentTrainProgress.Current = current
	CurrentTrainProgress.Total = total
	CurrentTrainProgress.Percentage = percentage
	CurrentTrainProgress.CurrentStep = step
	if logMsg != "" {
		if len(CurrentTrainProgress.Logs) > 25 {
			CurrentTrainProgress.Logs = CurrentTrainProgress.Logs[1:]
		}
		CurrentTrainProgress.Logs = append(CurrentTrainProgress.Logs, logMsg)
	}
}

func MainTrain() {
	CurrentTrainProgress.mu.Lock()
	CurrentTrainProgress.IsTraining = true
	CurrentTrainProgress.Logs = make([]string, 0)
	CurrentTrainProgress.mu.Unlock()

	setTrainProgress(0, 100, 5, "Inisialisasi & Reset Koleksi", "Memulai proses retraining RAG...")

	log.Println("Memulai proses Training Pengetahuan")

	if err := godotenv.Load(); err != nil {
		log.Println("[ai][train][MainTrain] warning load .env:", err)
	}

	if _, err := config.LoadConfig(); err != nil {
		log.Println("[ai][train][MainTrain] warning load config:", err)
	}

	ctx := context.Background()

	if err := database.ConnectDB(); err != nil {
		log.Println("[ai][train][MainTrain] error:", err)
		setTrainProgress(0, 100, 0, "Error", fmt.Sprintf("Gagal koneksi ke DB: %v", err))
		CurrentTrainProgress.mu.Lock()
		CurrentTrainProgress.IsTraining = false
		CurrentTrainProgress.mu.Unlock()
		return
	}

	setTrainProgress(0, 100, 10, "Inisialisasi & Reset Koleksi", "Mereset koleksi Qdrant lama...")
	if err := qdrantDeleteCollection(ctx, config.AppConfig.QdrantURL, config.AppConfig.QdrantCollectionName); err != nil {
		log.Printf("Gagal menghapus collection: %v", err)
	}
	time.Sleep(1 * time.Second)

	setTrainProgress(0, 100, 20, "Ekstraksi Skema & Relasi DB", "Membuat ulang koleksi Qdrant...")
	if err := qdrantCreateCollection(ctx, config.AppConfig.QdrantURL, config.AppConfig.QdrantCollectionName,
		config.AppConfig.EmbeddingVectorSize, config.AppConfig.QdrantDistanceMetric); err != nil {
		log.Printf("Gagal membuat koleksi di Qdrant: %v", err)
	}

	dynamicDDLs, err := GetDynamicSchemaContext()
	if err != nil {
		log.Printf("Gagal mengambil DDL dinamis: %v", err)
	}

	dynamicSQLExamples, err := GetDynamicSqlExamples()
	if err != nil {
		log.Printf("Gagal mengambil contoh SQL dinamis: %v", err)
	}

	totalItems := len(dynamicDDLs) + len(dynamicSQLExamples)
	if totalItems == 0 {
		totalItems = 1
	}

	setTrainProgress(0, totalItems, 25, "Ekstraksi Skema & Relasi DB", fmt.Sprintf("Membaca %d DDL dan %d Contoh SQL...", len(dynamicDDLs), len(dynamicSQLExamples)))

	var points []qdrantPoint
	processedCount := 0

	// A. PROSES DDL
	for i, content := range dynamicDDLs {
		processedCount++
		pct := 25 + int(float64(processedCount)/float64(totalItems)*60)
		logMsg := fmt.Sprintf("Embedding DDL [%d/%d]", i+1, len(dynamicDDLs))
		setTrainProgress(processedCount, totalItems, pct, "Generasi Embedding Vektor (Google AI)", logMsg)

		vector, err := GenerateEmbedding(content)
		if err != nil {
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

	// B. PROSES SQL EXAMPLES
	for i, item := range dynamicSQLExamples {
		processedCount++
		pct := 25 + int(float64(processedCount)/float64(totalItems)*60)
		cleanPrompt := item.PromptOnly
		cleanPrompt = strings.Replace(cleanPrompt, "-- Pertanyaan: ", "", 1)
		cleanPrompt = strings.Replace(cleanPrompt, "\"", "", -1)
		cleanPrompt = strings.TrimSpace(cleanPrompt)
		logMsg := fmt.Sprintf("Embedding Prompt SQL [%d/%d]: '%s'", i+1, len(dynamicSQLExamples), cleanPrompt)
		setTrainProgress(processedCount, totalItems, pct, "Generasi Embedding Vektor (Google AI)", logMsg)

		vector, err := GenerateEmbedding(cleanPrompt)
		if err != nil {
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

	// C. UPSERT POINTS TO QDRANT
	setTrainProgress(processedCount, totalItems, 90, "Indexing & Memuat In-Memory Cache", fmt.Sprintf("Menyimpan %d vektor ke Qdrant...", len(points)))
	if len(points) > 0 {
		if err := qdrantUpsertPoints(ctx, config.AppConfig.QdrantURL, config.AppConfig.QdrantCollectionName, points); err != nil {
			log.Printf("Gagal menyimpan vektor ke Qdrant: %v", err)
		}
	}

	setTrainProgress(totalItems, totalItems, 100, "Indexing & Memuat In-Memory Cache", "Training selesai! Database Vektor siap.")

	CurrentTrainProgress.mu.Lock()
	CurrentTrainProgress.IsTraining = false
	CurrentTrainProgress.mu.Unlock()

	log.Println("[INFO] Training selesai!")
}
