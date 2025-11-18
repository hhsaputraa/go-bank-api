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

	aiResp, err := GetSQL(normalizedPrompt)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if aiResp.SQL == "" {
		respondWithError(w, http.StatusBadRequest, "AI tidak mengembalikan query SQL.")
		return
	}

	log.Println("SQL yang akan dieksekusi:", aiResp.SQL)

	data, err := ExecuteDynamicQuery(aiResp.SQL, nil)

	if err != nil {
		log.Printf("GAGAL EKSEKUSI: %v. SQL 'ngawur' TIDAK akan disimpan ke cache.", err)
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if !aiResp.IsCached {
		SaveToCache(aiResp.PromptAsli, aiResp.Vector, aiResp.SQL)
	}

	respondWithJSON(w, http.StatusOK, data)
}

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

	promptAsli := strings.TrimSpace(req.PromptAsli)
	sqlKoreksi := strings.TrimSpace(req.SqlKoreksi)

	if promptAsli == "" || sqlKoreksi == "" {
		respondWithError(w, http.StatusBadRequest, "prompt_asli dan sql_koreksi tidak boleh kosong")
		return
	}

	log.Printf("Menerima Feedback Koreksi Baru. Prompt: %s", promptAsli)

	if err := AddSqlExample(promptAsli, sqlKoreksi); err != nil {
		log.Printf("ERROR: Gagal menyimpan feedback: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Gagal menyimpan feedback ke database")
		return
	}

	respondWithJSON(w, http.StatusCreated, map[string]string{
		"status":  "sukses",
		"message": "Feedback koreksi berhasil disimpan. Silakan 'retrain' untuk menerapkan.",
	})
}

func HandleAdminRetrain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondWithError(w, http.StatusMethodNotAllowed, "Metode tidak diizinkan")
		return
	}

	log.Println("ADMIN: Menerima permintaan /admin/retrain...")

	go func() {
		log.Println("ADMIN: Proses retraining RAG (Embedding) dimulai di background...")
		mainTrain()
	}()

	respondWithJSON(w, http.StatusAccepted, map[string]string{
		"message": "Proses retraining RAG telah dimulai di background. Periksa log server untuk status.",
	})
}

func HandleAdminListQdrant(w http.ResponseWriter, r *http.Request) {
	collectionName := r.URL.Query().Get("collection")
	if collectionName == "" {
		respondWithError(w, http.StatusBadRequest, "Parameter 'collection' wajib diisi")
		return
	}

	data, err := GetAllQdrantPoints(collectionName, 100)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, data)
}

func HandleAdminCacheCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondWithError(w, http.StatusMethodNotAllowed, "Hanya POST yang diizinkan")
		return
	}

	var req struct {
		Prompt string `json:"prompt"`
		SQL    string `json:"sql"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "JSON Body tidak valid")
		return
	}

	if req.Prompt == "" || req.SQL == "" {
		respondWithError(w, http.StatusBadRequest, "Prompt dan SQL tidak boleh kosong")
		return
	}

	// Panggil fungsi inject yang baru kita buat
	if err := ManualInjectCache(req.Prompt, req.SQL); err != nil {
		log.Printf("Gagal inject cache: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Gagal menyimpan ke cache: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusCreated, map[string]string{
		"status":  "success",
		"message": "Cache berhasil disuntikkan! Pertanyaan ini sekarang akan di-bypass dari LLM.",
	})
}
