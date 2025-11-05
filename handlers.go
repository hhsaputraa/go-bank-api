package main

import (
	"encoding/json"
	"log"
	"net/http"
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

	// Ini adalah 'izin' CORS kita
	w.Header().Set("Access-Control-Allow-Origin", "*")              // Izinkan SEMUA origin (alamat)
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS") // Izinkan metode POST dan OPTIONS
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")  // Izinkan header Content-Type

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

	log.Printf("Menerima Prompt: %s\n", req.Prompt)

	// 1. Panggil "Otak AI" (bukan 'Pabrik Query')
	sqlQuery, err := GetSQLFromAI(req.Prompt)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if sqlQuery == "" {
		respondWithError(w, http.StatusBadRequest, "AI tidak mengembalikan query SQL. Coba prompt lain.")
		return
	}

	data, err := ExecuteDynamicQuery(sqlQuery, nil)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, data)
}
