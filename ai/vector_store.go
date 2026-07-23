package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	config "go-bank-api/config"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	pb "github.com/qdrant/go-client/qdrant"
)

type qdrantPoint struct {
	ID      string                 `json:"id"`
	Vector  []float32              `json:"vector"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

type qdrantSearchReq struct {
	Vector         []float32 `json:"vector"`
	Limit          uint64    `json:"limit"`
	WithPayload    bool      `json:"with_payload"`
	ScoreThreshold float32   `json:"score_threshold"`
}

type qdrantSearchResp struct {
	Result []qdrantSearchResult `json:"result"`
}

type qdrantSearchResult struct {
	ID      interface{}            `json:"id"`
	Score   float32                `json:"score"`
	Payload map[string]interface{} `json:"payload"`
}

type QdrantDataResponse struct {
	ID      string                 `json:"id"`
	Payload map[string]interface{} `json:"payload"`
}

var vectorHTTPClient = &http.Client{
	Timeout: 60 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	},
}

func httpDoJSON(ctx context.Context, method, url string, body any) (*http.Response, []byte, error) {
	var reqBody io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, nil, err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if config.AppConfig != nil && config.AppConfig.QdrantAPIKey != "" {
		req.Header.Set("api-key", config.AppConfig.QdrantAPIKey)
	}

	resp, err := vectorHTTPClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	return resp, respBody, nil
}

func qdrantSearchPoints(ctx context.Context, baseURL, name string, req qdrantSearchReq) (qdrantSearchResp, error) {
	url := fmt.Sprintf("%s/collections/%s/points/search", baseURL, name)
	var data qdrantSearchResp
	resp, body, err := httpDoJSON(ctx, "POST", url, req)
	if err != nil {
		log.Println("[ai][vector_store][qdrantSearchPoints] error:", err)
		return data, err
	}
	if resp.StatusCode != 200 {
		err := fmt.Errorf("err %d: %s", resp.StatusCode, string(body))
		log.Println("[ai][vector_store][qdrantSearchPoints] error:", err)
		return data, err
	}
	if err := json.Unmarshal(body, &data); err != nil {
		log.Println("[ai][vector_store][qdrantSearchPoints] error:", err)
		return data, fmt.Errorf("gagal unmarshal response dari qdrant: %w", err)
	}

	return data, nil
}

func qdrantUpsertPoints(ctx context.Context, baseURL, name string, points []qdrantPoint) error {
	url := fmt.Sprintf("%s/collections/%s/points?wait=true", baseURL, name)
	req := map[string]any{"points": points}
	resp, body, err := httpDoJSON(ctx, "PUT", url, req)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("err %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func qdrantCreateCollection(ctx context.Context, baseURL, name string, size int, distance string) error {
	url := fmt.Sprintf("%s/collections/%s", baseURL, name)
	req := map[string]any{"vectors": map[string]any{"size": size, "distance": distance}}
	resp, body, err := httpDoJSON(ctx, "PUT", url, req)
	if err != nil {
		return err
	}
	if resp.StatusCode == 200 {
		return nil
	}
	if (resp.StatusCode == 400 || resp.StatusCode == 409) && strings.Contains(string(body), "already exists") {
		return nil
	}
	err = fmt.Errorf("err %d: %s", resp.StatusCode, string(body))
	log.Println("[ai][vector_store][qdrantCreateCollection] error:", err)
	return err
}

func qdrantCreatePayloadIndex(ctx context.Context, baseURL, collectionName, fieldName, schemaType string) error {
	url := fmt.Sprintf("%s/collections/%s/index", baseURL, collectionName)
	req := map[string]string{"field_name": fieldName, "field_schema": schemaType}
	resp, body, err := httpDoJSON(ctx, "PUT", url, req)
	if err != nil {
		return err
	}
	if resp.StatusCode == 200 {
		return nil
	}
	err = fmt.Errorf("err %d: %s", resp.StatusCode, string(body))
	log.Println("[ai][vector_store][qdrantCreatePayloadIndex] error:", err)
	return err
}

func DeleteQdrantPoint(ctx context.Context, collectionName string, pointID string) error {
	url := fmt.Sprintf("%s/collections/%s/points/delete?wait=true", config.AppConfig.QdrantURL, collectionName)
	req := map[string]any{"points": []string{pointID}}
	resp, body, err := httpDoJSON(ctx, "POST", url, req)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("err %d: %s", resp.StatusCode, string(body))
	}
	log.Printf("Berhasil hapus Point ID '%s'", pointID)
	return nil
}

func UpdateQdrantPoint(collectionName string, id string, prompt string, sqlQuery string) error {
	vector, err := GenerateEmbedding(prompt)
	if err != nil {
		return err
	}
	point := qdrantPoint{
		ID: id, Vector: vector,
		Payload: map[string]interface{}{"prompt_asli": prompt, "sql_query": sqlQuery},
	}
	return qdrantUpsertPoints(context.Background(), config.AppConfig.QdrantURL, collectionName, []qdrantPoint{point})
}

func GetAllQdrantPoints(collectionName string, limit uint32) ([]QdrantDataResponse, error) {
	url := fmt.Sprintf("%s/collections/%s/points/scroll", config.AppConfig.QdrantURL, collectionName)
	req := map[string]interface{}{
		"limit":        limit,
		"with_payload": true,
		"with_vector":  false,
	}

	resp, body, err := httpDoJSON(context.Background(), "POST", url, req)
	if err != nil {
		log.Println("[ai][vector_store][GetAllQdrantPoints] error:", err)
		return nil, err
	}
	if resp.StatusCode != 200 {
		err := fmt.Errorf("qdrant scroll error %d: %s", resp.StatusCode, string(body))
		log.Println("[ai][vector_store][GetAllQdrantPoints] error:", err)
		return nil, err
	}

	var scrollResp struct {
		Result struct {
			Points []struct {
				ID      interface{}            `json:"id"`
				Payload map[string]interface{} `json:"payload"`
			} `json:"points"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &scrollResp); err != nil {
		log.Println("[ai][vector_store][GetAllQdrantPoints] error:", err)
		return nil, fmt.Errorf("gagal unmarshal scroll response: %w", err)
	}

	var results []QdrantDataResponse
	for _, p := range scrollResp.Result.Points {
		idStr := fmt.Sprintf("%v", p.ID)
		results = append(results, QdrantDataResponse{
			ID:      idStr,
			Payload: p.Payload,
		})
	}
	return results, nil
}

func convertQdrantValue(value *pb.Value) interface{} {
	switch k := value.Kind.(type) {
	case *pb.Value_StringValue:
		return k.StringValue
	case *pb.Value_IntegerValue:
		return k.IntegerValue
	case *pb.Value_DoubleValue:
		return k.DoubleValue
	case *pb.Value_BoolValue:
		return k.BoolValue
	default:
		return nil
	}
}

func qdrantDeleteCollection(ctx context.Context, baseURL, name string) error {
	url := fmt.Sprintf("%s/collections/%s", baseURL, name)

	resp, body, err := httpDoJSON(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound {
		log.Printf("Collection '%s' berhasil dihapus (atau belum ada).", name)
		return nil
	}

	err = fmt.Errorf("gagal hapus collection status %d: %s", resp.StatusCode, string(body))
	log.Println("[ai][vector_store][qdrantDeleteCollection] error:", err)
	return err
}

var asyncTaskQueue = make(chan func(), 100)

func init() {
	// Bounded worker pool of 5 workers for background AI tasks
	for i := 0; i < 5; i++ {
		go func() {
			for task := range asyncTaskQueue {
				task()
			}
		}()
	}
}

func SubmitAsyncTask(task func()) {
	select {
	case asyncTaskQueue <- task:
	default:
		log.Println("[ai] Warning: Async task queue is full, dropping non-critical background task")
	}
}

func SaveToCache(promptAsli string, promptVector []float32, sqlQuery string) {
	SubmitAsyncTask(func() {
		if config.AppConfig == nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		log.Println("Menyimpan ke Semantic Cache...")
		point := qdrantPoint{
			ID:      uuid.NewString(),
			Vector:  promptVector,
			Payload: map[string]interface{}{"prompt_asli": promptAsli, "sql_query": sqlQuery},
		}
		if err := qdrantUpsertPoints(ctx, config.AppConfig.QdrantURL, config.AppConfig.QdrantCacheCollection, []qdrantPoint{point}); err == nil {
			log.Println("Berhasil update cache.")
		}
	})
}

func ManualInjectCache(promptAsli string, sqlQuery string) error {
	vec, err := GenerateEmbedding(promptAsli)
	if err != nil {
		return err
	}
	point := qdrantPoint{
		ID:      uuid.NewString(),
		Vector:  vec,
		Payload: map[string]interface{}{"prompt_asli": promptAsli, "sql_query": sqlQuery},
	}
	return qdrantUpsertPoints(context.Background(), config.AppConfig.QdrantURL, config.AppConfig.QdrantCacheCollection, []qdrantPoint{point})
}
