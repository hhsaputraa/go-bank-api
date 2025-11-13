package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env file
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

	// Initialize vector service (Qdrant + Google AI)
	if err := InitVectorService(); err != nil {
		log.Fatalf("Fatal Error: Gagal koneksi ke Qdrant (Database Vektor): %v", err)
	}

	// Register HTTP routes
	log.Println("Aplikasi siap berjalan...")
	RegisterRoutes()

	// Start server
	addr := fmt.Sprintf("%s:%s", AppConfig.ServerHost, AppConfig.ServerPort)
	log.Printf("Server web berjalan di http://%s\n", addr)
	log.Fatal(http.ListenAndServe(":"+AppConfig.ServerPort, nil))
}
