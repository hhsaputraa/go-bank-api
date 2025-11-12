package main

import (
	"log"
	"net/http"

	"github.com/joho/godotenv"
)

func main() {
	// mainTrain()
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

	if err := LoadCache(); err != nil {
		log.Printf("PERINGATAN: Gagal memuat cache: %v", err)
	}

	if err := InitVectorService(); err != nil {
		log.Fatalf("Fatal Error: Gagal koneksi ke Qdrant (Database Vektor): %v", err)
	}

	log.Println("Aplikasi siap berjalan...")
	RegisterRoutes()
	port := ":8080"
	log.Printf("Server web berjalan di http://localhost%s\n", port)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
