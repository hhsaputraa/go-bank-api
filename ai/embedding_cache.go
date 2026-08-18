package ai

import (
	"strings"
	"sync"
	"time"

	models "go-bank-api/models"
)

type l1QueryCacheEntry struct {
	sqlResponse models.AISqlResponse
	createdAt   time.Time
}

type l1EmbeddingCacheEntry struct {
	vector    []float32
	createdAt time.Time
}

var (
	// In-memory cache for prompt vectors (avoids remote Google AI calls)
	l1EmbeddingCache   = make(map[string]l1EmbeddingCacheEntry)
	l1EmbeddingCacheMu sync.RWMutex
	maxEmbeddingSize   = 5000
	embeddingTTL       = 24 * time.Hour

	// L1 In-memory cache for exact SQL query responses (avoids Qdrant gRPC roundtrip)
	l1QueryCache   = make(map[string]l1QueryCacheEntry)
	l1QueryCacheMu sync.RWMutex
	maxL1CacheSize = 2000
	l1CacheTTL     = 1 * time.Hour
)

// GetCachedEmbedding looks up an in-memory vector for a prompt
func GetCachedEmbedding(prompt string) ([]float32, bool) {
	key := strings.ToLower(strings.TrimSpace(prompt))
	l1EmbeddingCacheMu.RLock()
	defer l1EmbeddingCacheMu.RUnlock()

	if entry, ok := l1EmbeddingCache[key]; ok {
		if time.Since(entry.createdAt) < embeddingTTL {
			vecCopy := make([]float32, len(entry.vector))
			copy(vecCopy, entry.vector)
			return vecCopy, true
		}
	}
	return nil, false
}

// PutCachedEmbedding stores an in-memory vector for a prompt
func PutCachedEmbedding(prompt string, vector []float32) {
	key := strings.ToLower(strings.TrimSpace(prompt))
	if len(vector) == 0 {
		return
	}

	l1EmbeddingCacheMu.Lock()
	defer l1EmbeddingCacheMu.Unlock()

	if len(l1EmbeddingCache) >= maxEmbeddingSize {
		// Evict oldest 500 entries
		for k := range l1EmbeddingCache {
			delete(l1EmbeddingCache, k)
			if len(l1EmbeddingCache) < maxEmbeddingSize-500 {
				break
			}
		}
	}

	vecCopy := make([]float32, len(vector))
	copy(vecCopy, vector)

	l1EmbeddingCache[key] = l1EmbeddingCacheEntry{
		vector:    vecCopy,
		createdAt: time.Now(),
	}
}

// GetL1QueryCache looks up an in-memory SQL response for an exact prompt
func GetL1QueryCache(prompt string) (*models.AISqlResponse, bool) {
	key := strings.ToLower(strings.TrimSpace(prompt))
	l1QueryCacheMu.RLock()
	defer l1QueryCacheMu.RUnlock()

	if entry, ok := l1QueryCache[key]; ok {
		if time.Since(entry.createdAt) < l1CacheTTL {
			resp := entry.sqlResponse
			return &resp, true
		}
	}
	return nil, false
}

// PutL1QueryCache stores an in-memory SQL response for an exact prompt
func PutL1QueryCache(prompt string, resp models.AISqlResponse) {
	key := strings.ToLower(strings.TrimSpace(prompt))
	if strings.TrimSpace(resp.SQL) == "" {
		return
	}

	l1QueryCacheMu.Lock()
	defer l1QueryCacheMu.Unlock()

	if len(l1QueryCache) >= maxL1CacheSize {
		// Evict oldest 200 entries
		for k := range l1QueryCache {
			delete(l1QueryCache, k)
			if len(l1QueryCache) < maxL1CacheSize-200 {
				break
			}
		}
	}

	l1QueryCache[key] = l1QueryCacheEntry{
		sqlResponse: resp,
		createdAt:   time.Now(),
	}
}

// ClearL1QueryCache resets the L1 memory cache (useful for admin retrain / reload)
func ClearL1QueryCache() {
	l1QueryCacheMu.Lock()
	l1QueryCache = make(map[string]l1QueryCacheEntry)
	l1QueryCacheMu.Unlock()
}
