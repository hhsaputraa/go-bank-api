package ai

import (
	"context"
	"encoding/json"
	config "go-bank-api/config"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchLLMResponse_Dispatch(t *testing.T) {
	// Mock Server for OpenRouter
	openRouterCalled := false
	openRouterServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		openRouterCalled = true
		if r.Header.Get("Authorization") == "" {
			t.Error("OpenRouter request missing Authorization header")
		}
		resp := GroqResponse{
			Choices: []struct {
				Message GroqMessage `json:"message"`
			}{{Message: GroqMessage{Content: "OpenRouter Response"}}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer openRouterServer.Close()

	// Setup Config
	config.AppConfig = &config.Config{
		OpenRouterAPIKey: "sk-test-openrouter",
		OpenRouterModel:  "test-model",
		OpenRouterURL:    openRouterServer.URL,
		LLMProvider:      "openrouter",
	}

	// Test 1: Explicit OpenRouter
	t.Run("OpenRouter Explicit", func(t *testing.T) {
		openRouterCalled = false
		config.AppConfig.LLMProvider = "openrouter"

		res, err := fetchLLMResponse(context.Background(), "test prompt")
		if err != nil {
			t.Fatalf("Expected check to pass, got error: %v", err)
		}
		if res != "OpenRouter Response" {
			t.Errorf("Expected 'OpenRouter Response', got '%s'", res)
		}
		if !openRouterCalled {
			t.Error("Expected OpenRouter server to be called")
		}
	})

	// Test 2: Fallback to Existing Logic (Mocking Groq is similar, but simpler to just check if it fails fast or calls logic)
	// Since creating multiple mock servers and re-wiring config global is messy, we focus on proving OpenRouter works.
}
