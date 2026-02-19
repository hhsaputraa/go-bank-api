package ai

import (
	"context"
	"fmt"
	"log"
	"os"

	config "go-bank-api/config"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// This function is run when we execute the file as main
func VerifyTruncationResult() {
	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		log.Fatal("GOOGLE_API_KEY missing")
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		log.Fatalf("Client init failed: %v", err)
	}

	// Set the private variable
	geminiEmbedder = client.EmbeddingModel("models/gemini-embedding-001")

	// Set config manually for truncation logic
	config.AppConfig = &config.Config{
		EmbeddingVectorSize: 768,
	}

	text := "Test internal truncation"
	vector, err := GenerateEmbedding(text)
	if err != nil {
		log.Fatalf("GenerateEmbedding failed: %v", err)
	}

	fmt.Printf("Config Size: %d\n", config.AppConfig.EmbeddingVectorSize)
	fmt.Printf("Vector len: %d\n", len(vector))

	if len(vector) == 768 {
		fmt.Println("SUCCESS: Vector truncated internally.")
	} else {
		fmt.Printf("FAIL: Vector len is %d\n", len(vector))
	}
}
