package main

import (
	logger "go-bank-api/log"
	"log"
	"net/http"
	"time"

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
	_, err = LoadConfig()
	if err != nil {
		log.Fatalf("Fatal Error: Gagal memuat konfigurasi: %v", err)
	}
	log.Println("✅ Konfigurasi berhasil dimuat dari environment variables")

	// Connect to database
	err = ConnectDB()
	if err != nil {
		log.Fatalf("Fatal Error: Gagal koneksi ke database. %v", err)
	}
	logger.InitLogger()
	defer logger.CloseLogger()

	// Initialize vector service (Qdrant + Google AI)
	if err := InitVectorService(); err != nil {
		log.Fatalf("Fatal Error: Gagal koneksi ke Qdrant (Database Vektor): %v", err)
	}

	// Register HTTP routes
	log.Println("Aplikasi siap berjalan...")
	RegisterRoutes()

	srv := &http.Server{
		Addr:              ":" + AppConfig.ServerPort,
		Handler:           nil,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Fatal(srv.ListenAndServe())

}
