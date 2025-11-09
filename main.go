package main

import (
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

	if err := LoadCache(); err != nil {
		log.Printf("PERINGATAN: Gagal memuat cache (file mungkin belum ada): %v", err)
	} else {
		log.Println("✅ Cache query berhasil dimuat ke memori.")
	}

	log.Println("Aplikasi siap berjalan...")
	RegisterRoutes()
	port := ":8080"
	log.Printf("Server web berjalan di http://localhost%s\n", port)
	log.Fatal(http.ListenAndServe(port, nil))
}
