package ai

import (
	"fmt"
	models "go-bank-api/models"
	"testing"
)

func BenchmarkEmbeddingCacheConcurrent(b *testing.B) {
	// Pre-populate 500 vectors
	for i := 0; i < 500; i++ {
		dummyVec := make([]float32, 768)
		dummyVec[0] = float32(i)
		PutCachedEmbedding(fmt.Sprintf("prompt nasabah ke %d", i), dummyVec)
	}

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		idx := 0
		for pb.Next() {
			key := fmt.Sprintf("prompt nasabah ke %d", idx%500)
			vec, found := GetCachedEmbedding(key)
			if !found || len(vec) != 768 {
				b.Fatalf("cache lookup failed for %s", key)
			}
			idx++
		}
	})
}

func BenchmarkL1QueryCacheConcurrent(b *testing.B) {
	// Pre-populate 500 SQL queries
	for i := 0; i < 500; i++ {
		resp := models.AISqlResponse{
			SQL:        fmt.Sprintf("SELECT * FROM nasabah WHERE id = %d", i),
			IsCached:   true,
			PromptAsli: fmt.Sprintf("cari nasabah %d", i),
		}
		PutL1QueryCache(fmt.Sprintf("cari nasabah %d", i), resp)
	}

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		idx := 0
		for pb.Next() {
			key := fmt.Sprintf("cari nasabah %d", idx%500)
			resp, found := GetL1QueryCache(key)
			if !found || resp.SQL == "" {
				b.Fatalf("query cache lookup failed for %s", key)
			}
			idx++
		}
	})
}
