package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Peringatan: Tidak bisa memuat file .env")
	} else {
		log.Println("Berhasil memuat file .env")
	}

	err = ConnectDB()
	if err != nil {
		log.Fatalf("Fatal Error: Gagal koneksi ke database. %v", err)
	}

	// if err := LoadCache(); err != nil {
	// 	log.Printf("PERINGATAN: Gagal memuat cache: %v", err)
	// }

	if err := InitVectorService(); err != nil {
		log.Fatalf("Fatal Error: Gagal koneksi ke Qdrant (Database Vektor): %v", err)
	}

	log.Println("Aplikasi siap berjalan...")
	RegisterRoutes()
	serverPort := GetEnvAsInt("SERVER_PORT", 8080)
	port := fmt.Sprintf(":%d", serverPort)

	log.Printf("Server web berjalan di http://localhost%s\n", port)
	log.Fatal(http.ListenAndServe(port, nil))
}
