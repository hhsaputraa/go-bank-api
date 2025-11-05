package main

import (
	"log"
	"net/http"
)

func main() {
	err := ConnectDB()
	if err != nil {
		log.Fatalf("Fatal Error: Gagal koneksi ke database. %v", err)
	}
	log.Println("Aplikasi siap berjalan...")
	RegisterRoutes()
	port := ":8080"
	log.Printf("Server web berjalan di http://localhost%s\n", port)
	log.Fatal(http.ListenAndServe(port, nil))
}
