package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCompressionMiddleware(t *testing.T) {
	sampleJSON := `{"status":"success","data":{"nasabah":[{"id":1,"nama":"Budi"},{"id":2,"nama":"Ani"}]}}`

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(sampleJSON))
	})

	compHandler := CompressionMiddleware(handler)

	// Case 1: Client without gzip header
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	compHandler.ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") == "gzip" {
		t.Errorf("Expected uncompressed response without Accept-Encoding header")
	}
	if w.Body.String() != sampleJSON {
		t.Errorf("Expected plain body match")
	}

	// Case 2: Client with gzip header
	reqGzip := httptest.NewRequest(http.MethodGet, "/test", nil)
	reqGzip.Header.Set("Accept-Encoding", "gzip, deflate")
	wGzip := httptest.NewRecorder()
	compHandler.ServeHTTP(wGzip, reqGzip)

	if wGzip.Header().Get("Content-Encoding") != "gzip" {
		t.Errorf("Expected gzip Content-Encoding header")
	}

	gr, err := gzip.NewReader(bytes.NewReader(wGzip.Body.Bytes()))
	if err != nil {
		t.Fatalf("Failed to create gzip reader: %v", err)
	}
	defer gr.Close()

	decompressed, err := io.ReadAll(gr)
	if err != nil {
		t.Fatalf("Failed to decompress body: %v", err)
	}

	if string(decompressed) != sampleJSON {
		t.Errorf("Decompressed content does not match original payload")
	}

	// Case 3: SSE stream bypass
	reqSSE := httptest.NewRequest(http.MethodGet, "/api/query", nil)
	reqSSE.Header.Set("Accept-Encoding", "gzip")
	reqSSE.Header.Set("Accept", "text/event-stream")
	wSSE := httptest.NewRecorder()
	compHandler.ServeHTTP(wSSE, reqSSE)

	if wSSE.Header().Get("Content-Encoding") == "gzip" {
		t.Errorf("Expected SSE stream to bypass compression")
	}
}

func BenchmarkCompression(b *testing.B) {
	largeJSON := strings.Repeat(`{"id":1234,"nama":"PT MAJU MUNDUR SEJAHTERA","saldo":45000000.50},`, 500)
	payload := []byte(`[` + strings.TrimRight(largeJSON, ",") + `]`)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(payload)
	})

	compHandler := CompressionMiddleware(handler)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/data", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		w := httptest.NewRecorder()
		compHandler.ServeHTTP(w, req)
	}
}
