package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"
)

var gzipWriterPool = sync.Pool{
	New: func() interface{} {
		w, _ := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed)
		return w
	},
}

type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
	statusCode int
	wroteHeader bool
}

func (w *gzipResponseWriter) WriteHeader(status int) {
	if !w.wroteHeader {
		w.statusCode = status
		w.wroteHeader = true
		w.Header().Del("Content-Length")
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Vary", "Accept-Encoding")
		w.ResponseWriter.WriteHeader(status)
	}
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.Writer.Write(b)
}

func (w *gzipResponseWriter) Flush() {
	if gz, ok := w.Writer.(*gzip.Writer); ok {
		gz.Flush()
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// CompressionMiddleware applies Gzip compression to responses when accepted by the client.
// Automatically bypasses SSE (Server-Sent Events) or WebSocket streams.
func CompressionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Do not compress if client does not accept gzip
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		// Bypass compression for SSE stream endpoints or websocket upgrades
		if r.Header.Get("Accept") == "text/event-stream" ||
			strings.EqualFold(r.Header.Get("Upgrade"), "websocket") ||
			strings.Contains(r.URL.Path, "/api/query") {
			next.ServeHTTP(w, r)
			return
		}

		gz := gzipWriterPool.Get().(*gzip.Writer)
		defer gzipWriterPool.Put(gz)

		gz.Reset(w)
		defer gz.Close()

		gzw := &gzipResponseWriter{
			Writer:         gz,
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(gzw, r)
	})
}
