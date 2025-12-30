package controllers

import (
	"encoding/json"
	ai "go-bank-api/ai"
	models "go-bank-api/models"
	utils "go-bank-api/utils"
	"log"
	"net/http"
	"strings"
)

func HandleFeedbackKoreksi(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		utils.WriteError(w, http.StatusMethodNotAllowed, "Method Not Allowed", "Metode HTTP tidak diizinkan")
		return
	}

	var req models.FeedbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusMethodNotAllowed, "Method Not Allowed", "Request body JSON tidak valid")
		return
	}

	promptAsli := strings.TrimSpace(req.PromptAsli)
	sqlKoreksi := strings.TrimSpace(req.SqlKoreksi)

	if promptAsli == "" || sqlKoreksi == "" {
		utils.WriteError(w, http.StatusBadRequest, "Validasi Gagal", "prompt_asli dan sql_koreksi tidak boleh kosong")
		return
	}

	log.Printf("Menerima Feedback Koreksi Baru. Prompt: %s", promptAsli)

	if err := ai.AddSqlExample(promptAsli, sqlKoreksi); err != nil {
		log.Printf("ERROR: Gagal menyimpan feedback: %v", err)
		utils.WriteError(w, http.StatusInternalServerError, "Server Error", "Gagal menyimpan feedback ke database")
		return
	}

	utils.WriteJSON(w, http.StatusCreated, utils.APIResponse{
		Status:  "sukses",
		Message: "Feedback koreksi berhasil disimpan. Silakan 'retrain' untuk menerapkan.",
	})
}
