package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

func HandleHealthCheck(w http.ResponseWriter, r *http.Request) {
	respondWithJSON(w, http.StatusOK, map[string]string{"status": "API is up and running!"})
}

func HandleDynamicQuery(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		respondWithError(w, http.StatusMethodNotAllowed, "Metode tidak diizinkan")
		return
	}
	var req PromptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Request body JSON tidak valid")
		return
	}
	normalizedPrompt := strings.ToLower(strings.TrimSpace(req.Prompt))
	if normalizedPrompt == "" {
		respondWithError(w, http.StatusBadRequest, "Prompt tidak boleh kosong")
		return
	}
	log.Printf("Menerima Prompt (Normalized): %s\n", normalizedPrompt)

	sqlQuery, err := GetSQL(normalizedPrompt)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if sqlQuery == "" {
		respondWithError(w, http.StatusBadRequest, "AI tidak mengembalikan query SQL.")
		return
	}
	log.Println("SQL yang akan dieksekusi:", sqlQuery)

	data, err := ExecuteDynamicQuery(sqlQuery, nil)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondWithJSON(w, http.StatusOK, data)
}

// HandleFeedbackKoreksi - Endpoint untuk menerima feedback koreksi SQL dari user
func HandleFeedbackKoreksi(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		respondWithError(w, http.StatusMethodNotAllowed, "Metode tidak diizinkan")
		return
	}

	var req FeedbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Request body JSON tidak valid")
		return
	}

	if req.PromptAsli == "" || req.SqlKoreksi == "" {
		respondWithError(w, http.StatusBadRequest, "prompt_asli dan sql_koreksi harus diisi")
		return
	}

	log.Printf("Menerima feedback koreksi - Prompt: %s, SQL Koreksi: %s", req.PromptAsli, req.SqlKoreksi)

	// Simpan ke database untuk training ulang nanti
	if err := SaveFeedbackToDatabase(req.PromptAsli, req.SqlKoreksi); err != nil {
		log.Printf("PERINGATAN: Gagal menyimpan feedback ke database: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Gagal menyimpan feedback")
		return
	}

	log.Println("✅ Feedback berhasil disimpan ke database")
	respondWithJSON(w, http.StatusOK, map[string]string{
		"message": "Feedback berhasil disimpan. Terima kasih!",
	})
}

// HandleAdminRetrain - Endpoint untuk admin melakukan retrain vector database
func HandleAdminRetrain(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		respondWithError(w, http.StatusMethodNotAllowed, "Metode tidak diizinkan")
		return
	}

	log.Println("🔄 Admin meminta retrain vector database...")

	// Jalankan proses training ulang di goroutine agar tidak blocking
	go func() {
		log.Println("Memulai proses retrain di background...")
		mainTrain()
		log.Println("✅ Proses retrain selesai!")
	}()

	respondWithJSON(w, http.StatusAccepted, map[string]string{
		"message": "Proses retrain dimulai di background. Silakan cek log server.",
	})
}
