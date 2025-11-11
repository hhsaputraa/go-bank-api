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
	pb "github.com/qdrant/go-client/qdrant"
	"google.golang.org/api/option"
)

var qdrantClient *pb.Client
var geminiEmbedder *genai.EmbeddingModel

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

	client, err := pb.NewClient(&pb.Config{
		Host: "localhost",
		Port: 6334,
	})
	if err != nil {
		return fmt.Errorf("gagal membuat Qdrant client: %w", err)
	}
	qdrantClient = client

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
	log.Println("Menerjemahkan prompt user ke vektor...")
	res, err := geminiEmbedder.EmbedContent(ctx, genai.Text(userPrompt))
	if err != nil {
		return "", fmt.Errorf("gagal embed prompt user: %w", err)
	}
	promptVector := res.Embedding.Values

	log.Println("Mencari konteks relevan di Qdrant...")
	var searchLimit uint64 = 3

	searchResponse, err := qdrantClient.Query(ctx, &pb.QueryPoints{
		CollectionName: QDRANT_COLLECTION_NAME,
		Query:          pb.NewQuery(promptVector...),
		WithPayload:    pb.NewWithPayload(true),
		Limit:          &searchLimit,
	})
	if err != nil {
		return "", fmt.Errorf("gagal mencari di Qdrant: %w", err)
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

	finalPrompt := fmt.Sprintf(`
Anda adalah ahli SQL PostgreSQL. Tanggal hari ini (CURRENT_DATE) adalah %s.

Diberikan beberapa CONTOH DDL dan SQL yang paling relevan dengan pertanyaan user (dalam skema "bpr_supra"):
%s

Tugas Anda:
1. Jawab pertanyaan pengguna di bawah ini dengan menulis SATU query SQL PostgreSQL yang valid.
2. JANGAN pakai markdown (sql). JANGAN tambahkan penjelasan. Hanya SQL.
3. Untuk pencarian teks atau nama, WAJIB gunakan 'ILIKE' dengan pola '%'.

Pertanyaan Pengguna: "%s"

Query SQL:`, time.Now().Format("2006-01-02"), contextBuilder.String(), userPrompt)

	groqReqBody := GroqRequest{
		Model:    "openai/gpt-oss-120b",
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
	return sqlQuery, nil
}
