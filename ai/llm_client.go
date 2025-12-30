package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	config "go-bank-api/config"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
)

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
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

func GenerateEmbedding(text string) ([]float32, error) {
	if geminiEmbedder == nil {
		return nil, fmt.Errorf("service embedding belum diinisialisasi")
	}
	res, err := geminiEmbedder.EmbedContent(context.Background(), genai.Text(text))
	if err != nil {
		return nil, err
	}
	return res.Embedding.Values, nil
}

func fetchLLMResponse(ctx context.Context, prompt string) (string, error) {
	if config.AppConfig != nil && config.AppConfig.OllamaURL != "" {
		log.Printf("Mencoba Ollama LLM lokal...")
		ollamaReq := map[string]any{"model": config.AppConfig.OllamaModel, "prompt": prompt, "stream": false}

		_, respBody, err := httpDoJSON(ctx, "POST", strings.TrimRight(config.AppConfig.OllamaURL, "/")+"/api/generate", ollamaReq)
		if err == nil {
			var oResp map[string]any
			if json.Unmarshal(respBody, &oResp) == nil {
				if r, ok := oResp["response"].(string); ok && r != "" {
					log.Println("✅ Sukses Ollama.")
					return r, nil
				}
			}
		}
		log.Println("Ollama gagal/kosong. Beralih ke Groq.")
	}

	log.Println("Menggunakan Layanan Groq AI...")
	return callGroqAPI(prompt, config.AppConfig.GroqModel, 0.0)
}

func callGroqAPI(prompt string, model string, temp float32) (string, error) {
	reqBody := GroqRequest{
		Model:       model,
		Messages:    []GroqMessage{{Role: "user", Content: prompt}},
		Temperature: temp,
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", config.AppConfig.GroqAPIURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+config.AppConfig.GroqAPIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: config.AppConfig.GroqTimeout}
	if client.Timeout == 0 {
		client.Timeout = 30 * time.Second
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("koneksi Groq gagal: %w", err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Groq error %d: %s", resp.StatusCode, string(respBytes))
	}

	var groqResp GroqResponse
	if err := json.Unmarshal(respBytes, &groqResp); err != nil {
		return "", err
	}
	if len(groqResp.Choices) == 0 {
		return "", errors.New("Groq tidak merespon")
	}

	result := groqResp.Choices[0].Message.Content
	log.Printf("Token Usage: %d input, %d output", groqResp.Usage.PromptTokens, groqResp.Usage.CompletionTokens)

	return result, nil
}
