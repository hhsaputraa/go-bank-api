package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	ai "go-bank-api/ai"
	config "go-bank-api/config"
	database "go-bank-api/database"
	logger "go-bank-api/log"
	"go-bank-api/middleware"
	routes "go-bank-api/routes"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Peringatan: Tidak bisa memuat file .env")
	} else {
		log.Println("Berhasil memuat file .env")
	}

	// Load configuration from environment variables
	_, err = config.LoadConfig()
	if err != nil {
		log.Fatalf("Fatal Error: Gagal memuat konfigurasi: %v", err)
	}
	log.Println("[INFO] Konfigurasi berhasil dimuat dari environment variables")

	// Connect to database
	err = database.ConnectDB()
	if err != nil {
		log.Fatalf("Fatal Error: Gagal koneksi ke database. %v", err)
	}
	logger.InitLogger()
	defer logger.CloseLogger()

	// Initialize vector service (Qdrant + Google AI)
	if err := ai.InitVectorService(); err != nil {
		log.Fatalf("Fatal Error: Gagal koneksi ke Qdrant (Database Vektor): %v", err)
	}

	// Create a new ServeMux for better control
	mux := http.NewServeMux()

	// Register HTTP routes
	log.Println("Aplikasi siap berjalan...")
	routes.RegisterRoutes(mux)

	// Apply middleware chain
	handler := middleware.LoggingMiddleware(
		middleware.SecurityHeadersMiddleware(
			middleware.CORSMiddleware(mux),
		),
	)


	// Create server with proper configuration
	srv := &http.Server{
		Addr:              config.AppConfig.ServerHost + ":" + config.AppConfig.ServerPort,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Server berjalan di http://%s:%s", config.AppConfig.ServerHost, config.AppConfig.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Fatal Error: Server gagal start: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[WARNING] Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("[INFO] Server exited gracefully")
}
