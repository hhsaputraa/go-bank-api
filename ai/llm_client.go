package ai

import (
	"bufio"
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
	"sync"

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
	Stream          bool          `json:"stream,omitempty"`
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

type embeddingCacheEntry struct {
	vector    []float32
	createdAt time.Time
}

var (
	embeddingCache = make(map[string]embeddingCacheEntry)
	embeddingMu    sync.RWMutex
	maxCacheSize   = 1000
)

func GenerateEmbedding(text string) ([]float32, error) {
	cleanText := strings.TrimSpace(text)
	if cleanText == "" {
		return nil, errors.New("teks embedding kosong")
	}

	// 1. Check in-memory cache
	embeddingMu.RLock()
	if entry, found := embeddingCache[cleanText]; found {
		if time.Since(entry.createdAt) < 24*time.Hour {
			embeddingMu.RUnlock()
			log.Printf("⚡ EMBEDDING CACHE HIT (RAM): '%s'", cleanText)
			return entry.vector, nil
		}
	}
	embeddingMu.RUnlock()

	// 2. Fetch from Google AI Embedder if cache miss
	if geminiEmbedder == nil {
		err := fmt.Errorf("service embedding belum diinisialisasi")
		log.Println("[ai][llm_client][GenerateEmbedding] error:", err)
		return nil, err
	}
	res, err := geminiEmbedder.EmbedContent(context.Background(), genai.Text(cleanText))
	if err != nil {
		log.Println("[ai][llm_client][GenerateEmbedding] error:", err)
		return nil, err
	}

	// TRUNCATION LOGIC: Force fit to config size
	targetSize := config.AppConfig.EmbeddingVectorSize
	vec := res.Embedding.Values
	if len(vec) > targetSize {
		vec = vec[:targetSize]
	}

	// 3. Store in cache (bounded eviction if size >= 1000)
	embeddingMu.Lock()
	if len(embeddingCache) >= maxCacheSize {
		for k := range embeddingCache {
			delete(embeddingCache, k)
			if len(embeddingCache) < maxCacheSize-200 {
				break
			}
		}
	}
	embeddingCache[cleanText] = embeddingCacheEntry{
		vector:    vec,
		createdAt: time.Now(),
	}
	embeddingMu.Unlock()

	return vec, nil
}

func fetchLLMResponse(ctx context.Context, prompt string, modelOverride string) (string, error) {
	if config.AppConfig != nil && config.AppConfig.OllamaURL != "" {
		log.Printf("Mencoba Ollama LLM lokal...")
		ollamaReq := map[string]any{"model": config.AppConfig.OllamaModel, "prompt": prompt, "stream": false}

		_, respBody, err := httpDoJSON(ctx, "POST", strings.TrimRight(config.AppConfig.OllamaURL, "/")+"/api/generate", ollamaReq)
		if err == nil {
			var oResp map[string]any
			if json.Unmarshal(respBody, &oResp) == nil {
				if r, ok := oResp["response"].(string); ok && r != "" {
					log.Println("[INFO] Sukses Ollama.")
					return r, nil
				}
			}
		}
		log.Println("Ollama gagal/kosong. Beralih ke Groq.")
	}

	log.Println("Menggunakan Layanan Groq AI...")
	opts := GroqOptions{
		Temperature:     0.6,
		TopP:            0.95,
		ReasoningFormat: "hidden",
	}

	selectedModel := config.AppConfig.GroqModel
	if modelOverride != "" {
		selectedModel = modelOverride
	}

	return callGroqAPI(prompt, selectedModel, opts)
}

var sharedGroqClient = &http.Client{
	Timeout: 60 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        50,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	},
}

var sharedStreamClient = &http.Client{
	Transport: &http.Transport{
		MaxIdleConns:        50,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	},
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

		log.Println(cleanContent)
	}
	log.Println("========================================================")
	jsonBody, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", config.AppConfig.GroqAPIURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		log.Println("[ai][llm_client][callGroqAPI] error:", err)
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+config.AppConfig.GroqAPIKey)
	req.Header.Set("Content-Type", "application/json")

	client := sharedGroqClient
	if config.AppConfig != nil && config.AppConfig.GroqTimeout > 0 {
		client = &http.Client{
			Timeout:   config.AppConfig.GroqTimeout,
			Transport: sharedGroqClient.Transport,
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Println("[ai][llm_client][callGroqAPI] error:", err)
		return "", fmt.Errorf("koneksi Groq gagal: %w", err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		err := fmt.Errorf("Groq error %d: %s", resp.StatusCode, string(respBytes))
		log.Println("[ai][llm_client][callGroqAPI] error:", err)
		return "", err
	}

	var groqResp GroqResponse
	if err := json.Unmarshal(respBytes, &groqResp); err != nil {
		log.Println("[ai][llm_client][callGroqAPI] error:", err)
		return "", err
	}
	if len(groqResp.Choices) == 0 {
		err := errors.New("Groq tidak merespon")
		log.Println("[ai][llm_client][callGroqAPI] error:", err)
		return "", err
	}

	result := groqResp.Choices[0].Message.Content
	log.Printf("Token Usage: %d input, %d output", groqResp.Usage.PromptTokens, groqResp.Usage.CompletionTokens)

	return result, nil
}

func CallGroqAPIStream(ctx context.Context, prompt string, model string, options GroqOptions, chunkChan chan<- string, errChan chan<- error) {
	defer close(chunkChan)
	defer close(errChan)

	reqBody := GroqRequest{
		Model:           model,
		Messages:        []GroqMessage{{Role: "user", Content: prompt}},
		Temperature:     options.Temperature,
		TopP:            options.TopP,
		ReasoningFormat: options.ReasoningFormat,
		Stream:          true,
	}

	jsonBody, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", config.AppConfig.GroqAPIURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		log.Println("[ai][llm_client][CallGroqAPIStream] error:", err)
		errChan <- err
		return
	}
	req.Header.Set("Authorization", "Bearer "+config.AppConfig.GroqAPIKey)
	req.Header.Set("Content-Type", "application/json")

	// Reuse shared transport client for streaming connections
	resp, err := sharedStreamClient.Do(req)
	if err != nil {
		err = fmt.Errorf("koneksi Groq gagal saat stream: %w", err)
		log.Println("[ai][llm_client][CallGroqAPIStream] error:", err)
		errChan <- err
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBytes, _ := io.ReadAll(resp.Body)
		err = fmt.Errorf("Groq stream error %d: %s", resp.StatusCode, string(respBytes))
		log.Println("[ai][llm_client][CallGroqAPIStream] error:", err)
		errChan <- err
		return
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		if strings.HasPrefix(line, "data: ") {
			dataStr := strings.TrimPrefix(line, "data: ")
			dataStr = strings.TrimSpace(dataStr)

			if dataStr == "[DONE]" {
				return
			}

			var streamResp struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}

			if err := json.Unmarshal([]byte(dataStr), &streamResp); err != nil {
				continue
			}

			if len(streamResp.Choices) > 0 {
				content := streamResp.Choices[0].Delta.Content
				if content != "" {
					chunkChan <- content
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Println("[ai][llm_client][CallGroqAPIStream] error:", err)
		errChan <- err
	}
}
