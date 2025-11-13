package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// ============================================
// QDRANT REST API TYPES
// ============================================

type qdrantCreateCollectionReq struct {
	Vectors qdrantVectors `json:"vectors"`
}

type qdrantVectors struct {
	Size     int    `json:"size"`
	Distance string `json:"distance"`
}

type qdrantUpsertPointsReq struct {
	Points []qdrantPoint `json:"points"`
}

type qdrantPoint struct {
	ID      string                 `json:"id"`
	Vector  []float32              `json:"vector,omitempty"`
	Vectors map[string][]float32   `json:"vectors,omitempty"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

type qdrantSearchReq struct {
	Vector      []float32 `json:"vector"`
	Limit       uint64    `json:"limit"`
	WithPayload bool      `json:"with_payload"`
}

type qdrantSearchResp struct {
	Result []qdrantSearchResult `json:"result"`
	Status string               `json:"status"`
	Time   float64              `json:"time"`
}

type qdrantSearchResult struct {
	ID      interface{}            `json:"id"`
	Version int                    `json:"version"`
	Score   float32                `json:"score"`
	Payload map[string]interface{} `json:"payload"`
}

// ============================================
// QDRANT REST API HELPER FUNCTIONS
// ============================================

func getQdrantBaseURL() string {
	base := GetEnv("QDRANT_URL", "http://localhost:6333")
	return base
}

func httpDoJSON(ctx context.Context, method, url string, body any) (*http.Response, []byte, error) {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, nil, fmt.Errorf("gagal marshal JSON: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, nil, fmt.Errorf("gagal buat request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Baca timeout dari environment variable
	qdrantTimeout := GetEnvAsDuration("QDRANT_TIMEOUT", 60*time.Second)
	client := &http.Client{Timeout: qdrantTimeout}

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("gagal call %s %s: %w", method, url, err)
	}

	respBody, readErr := io.ReadAll(resp.Body)
	resp.Body.Close()
	if readErr != nil {
		return resp, nil, fmt.Errorf("gagal baca response body: %w", readErr)
	}
	return resp, respBody, nil
}

func qdrantCreateCollection(ctx context.Context, baseURL, name string, size int, distance string) error {
	url := fmt.Sprintf("%s/collections/%s", baseURL, name)
	req := qdrantCreateCollectionReq{
		Vectors: qdrantVectors{
			Size:     size,
			Distance: distance,
		},
	}
	resp, body, err := httpDoJSON(ctx, http.MethodPut, url, req)
	if err != nil {
		return err
	}

	if resp.StatusCode == http.StatusOK {
		log.Printf("Berhasil membuat collection '%s'.", name)
		return nil
	}

	if (resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusConflict) &&
		strings.Contains(string(body), "already exists") {
		log.Printf("Collection '%s' sudah ada, tidak perlu dibuat ulang.", name)
		return nil
	}

	return fmt.Errorf("create collection status %d: %s", resp.StatusCode, string(body))
}

func qdrantUpsertPoints(ctx context.Context, baseURL, name string, points []qdrantPoint) error {
	url := fmt.Sprintf("%s/collections/%s/points?wait=true", baseURL, name)
	req := qdrantUpsertPointsReq{Points: points}
	resp, body, err := httpDoJSON(ctx, http.MethodPut, url, req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("upsert points status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func qdrantSearchPoints(ctx context.Context, baseURL, name string, req qdrantSearchReq) (qdrantSearchResp, error) {
	url := fmt.Sprintf("%s/collections/%s/points/search", baseURL, name)
	var respData qdrantSearchResp

	resp, body, err := httpDoJSON(ctx, http.MethodPost, url, req)
	if err != nil {
		return respData, err
	}
	if resp.StatusCode != http.StatusOK {
		return respData, fmt.Errorf("search points status %d: %s", resp.StatusCode, string(body))
	}

	if err := json.Unmarshal(body, &respData); err != nil {
		return respData, fmt.Errorf("gagal unmarshal search response: %w", err)
	}
	return respData, nil
}

