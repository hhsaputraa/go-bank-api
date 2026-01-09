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
	Model           string        `json:"model"`
	Messages        []GroqMessage `json:"messages"`
	Temperature     float32       `json:"temperature"`
	TopP            float32       `json:"top_p,omitempty"`
	ReasoningFormat string        `json:"reasoning_format,omitempty"`
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

type GroqOptions struct {
	Temperature     float32
	TopP            float32
	ReasoningFormat string
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
	provider := strings.ToLower(config.AppConfig.LLMProvider)
	opts := GroqOptions{
		Temperature:     0.6,
		TopP:            0.95,
		ReasoningFormat: "hidden",
	}

	// 1. Explicit OpenRouter
	if provider == "openrouter" {
		if config.AppConfig.OpenRouterAPIKey == "" {
			return "", errors.New("OpenRouter dipilih tetapi API Key kosong")
		}
		log.Println("Menggunakan OpenRouter Service...")
		return callOpenRouterAPI(prompt, config.AppConfig.OpenRouterModel, opts)
	}

	// 2. Explicit Groq
	if provider == "groq" {
		if config.AppConfig.GroqAPIKey == "" {
			return "", errors.New("Groq dipilih tetapi API Key kosong")
		}
		log.Println("Menggunakan Groq AI Service...")
		return callGroqAPI(prompt, config.AppConfig.GroqModel, opts)
	}

	// 3. Explicit Ollama
	if provider == "ollama" {
		log.Println("Menggunakan Ollama Local...")
		return callOllama(ctx, prompt)
	}

	// 4. Default Fallback (Existing Behavior: Ollama -> Groq)
	log.Println("LLM Provider tidak spesifik. Mencoba Auto-Detect (Ollama Priority)...")
	if config.AppConfig.OllamaURL != "" {
		res, err := callOllama(ctx, prompt)
		if err == nil {
			return res, nil
		}
		log.Printf("Ollama gagal (%v). Fallback ke Groq/OpenRouter...", err)
	}

	if config.AppConfig.GroqAPIKey != "" {
		return callGroqAPI(prompt, config.AppConfig.GroqModel, opts)
	}

	if config.AppConfig.OpenRouterAPIKey != "" {
		return callOpenRouterAPI(prompt, config.AppConfig.OpenRouterModel, opts)
	}

	return "", errors.New("tidak ada LLM provider yang tersedia/dikonfigurasi")
}

func callOllama(ctx context.Context, prompt string) (string, error) {
	ollamaReq := map[string]any{"model": config.AppConfig.OllamaModel, "prompt": prompt, "stream": false}
	_, respBody, err := httpDoJSON(ctx, "POST", strings.TrimRight(config.AppConfig.OllamaURL, "/")+"/api/generate", ollamaReq)
	if err != nil {
		return "", err
	}
	var oResp map[string]any
	if json.Unmarshal(respBody, &oResp) != nil {
		return "", errors.New("gagal parse ollama json")
	}
	if r, ok := oResp["response"].(string); ok && r != "" {
		log.Println("✅ Sukses Ollama.")
		return r, nil
	}
	return "", errors.New("ollama response kosong")
}

func callGroqAPI(prompt string, model string, options GroqOptions) (string, error) {
	reqBody := GroqRequest{
		Model:           model,
		Messages:        []GroqMessage{{Role: "user", Content: prompt}},
		Temperature:     options.Temperature,
		TopP:            options.TopP,
		ReasoningFormat: options.ReasoningFormat,
	}

	log.Println("\n========== 📤 OUTGOING PROMPT (Human Readable) ==========")
	log.Printf("CONFIG: Model=%s | Temp=%.1f | TopP=%.2f\n", model, options.Temperature, options.TopP)

	for _, msg := range reqBody.Messages {
		log.Printf("--- ROLE: %s ---\n", strings.ToUpper(msg.Role))

		cleanContent := msg.Content

		fmt.Println(cleanContent)
	}
	log.Println("========================================================")
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

func callOpenRouterAPI(prompt string, model string, options GroqOptions) (string, error) {
	reqBody := GroqRequest{
		Model:           model,
		Messages:        []GroqMessage{{Role: "user", Content: prompt}},
		Temperature:     options.Temperature,
		TopP:            options.TopP,
		ReasoningFormat: options.ReasoningFormat,
	}

	log.Println("\n========== 📤 OUTGOING PROMPT (OpenRouter) ==========")
	log.Printf("CONFIG: Model=%s | Temp=%.1f | TopP=%.2f\n", model, options.Temperature, options.TopP)

	// for _, msg := range reqBody.Messages {
	// 	log.Printf("--- ROLE: %s ---\n", strings.ToUpper(msg.Role))
	// 	fmt.Println(msg.Content)
	// }
	log.Println("========================================================")
	jsonBody, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", config.AppConfig.OpenRouterURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+config.AppConfig.OpenRouterAPIKey)
	req.Header.Set("Content-Type", "application/json")
	// OpenRouter specific headers for rankings/stats (optional but recommended)
	req.Header.Set("HTTP-Referer", config.AppConfig.FrontendURL)
	req.Header.Set("X-Title", "Go Bank API")

	client := &http.Client{Timeout: 60 * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("koneksi OpenRouter gagal: %w", err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("OpenRouter error %d: %s", resp.StatusCode, string(respBytes))
	}

	var groqResp GroqResponse // Structure is compatible with OpenAI/Groq standard
	if err := json.Unmarshal(respBytes, &groqResp); err != nil {
		return "", err
	}
	if len(groqResp.Choices) == 0 {
		return "", errors.New("OpenRouter tidak merespon")
	}

	result := groqResp.Choices[0].Message.Content
	log.Printf("Token Usage: %d input, %d output", groqResp.Usage.PromptTokens, groqResp.Usage.CompletionTokens)

	return result, nil
}
