package controllers

import (
	"encoding/json"
	"fmt"
	ai "go-bank-api/ai"
	auth "go-bank-api/auth"
	config "go-bank-api/config"
	utils "go-bank-api/utils"
	"log"
	"net/http"
	"strings"
)

func HandleAdminRetrain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteError(w, http.StatusMethodNotAllowed, "Method Not Allowed", "Metode HTTP tidak diizinkan")
		return
	}

	log.Println("ADMIN: Menerima permintaan /admin/retrain...")

	go func() {
		log.Println("ADMIN: training RAG (Embedding) dimulai")
		ai.MainTrain()
	}()

	utils.WriteJSON(w, http.StatusAccepted, utils.APIResponse{
		Status:  "success",
		Message: "Proses retraining RAG telah dimulai di latar belakang",
	})
}

func HandleAdminRetrainStatus(w http.ResponseWriter, r *http.Request) {
	status := ai.GetTrainingStatus()
	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":      "success",
		"is_training": status.IsTraining,
		"current":     status.Current,
		"total":       status.Total,
		"percentage":  status.Percentage,
		"step":        status.CurrentStep,
		"logs":        status.Logs,
	})
}

func HandleAdminListQdrant(w http.ResponseWriter, r *http.Request) {
	collectionName := r.URL.Query().Get("collection")
	if collectionName == "" {
		utils.WriteError(w, http.StatusBadRequest, "Validasi Gagal", "Parameter 'collection' wajib diisi")
		return
	}

	data, err := ai.GetAllQdrantPoints(collectionName, 1000)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Gagal mengambil data Qdrant", err.Error())
		return
	}

	utils.WriteJSON(w, http.StatusOK, data)
}

func HandleAdminCacheCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteError(w, http.StatusMethodNotAllowed, "Method Not Allowed", "Metode HTTP tidak diizinkan")
		return
	}

	var req struct {
		Prompt string `json:"prompt"`
		SQL    string `json:"sql"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Bad Request", "Request body JSON tidak valid")
		return
	}

	if req.Prompt == "" || req.SQL == "" {
		utils.WriteError(w, http.StatusBadRequest, "Validasi Gagal", "prompt dan sql tidak boleh kosong")
		return
	}

	if err := utils.IsReadOnlySQL(req.SQL); err != nil {
		log.Printf("⚠️ Percobaan inject query berbahaya: %s", req.SQL)
		utils.WriteError(w, http.StatusBadRequest, "SQL Ditolak", err.Error())
		return
	}

	if err := ai.ManualInjectCache(req.Prompt, req.SQL); err != nil {
		log.Printf("Gagal inject cache: %v", err)
		utils.WriteError(w, http.StatusInternalServerError, "Gagal menyimpan ke cache", err.Error())
		return
	}

	utils.WriteJSON(w, http.StatusCreated, utils.APIResponse{
		Status:  "success",
		Message: "Cache berhasil disuntikkan! Pertanyaan ini sekarang akan di-bypass dari LLM.",
	})
}

func HandleAdminQdrantUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		utils.WriteError(w, http.StatusMethodNotAllowed, "Method Not Allowed", "Metode HTTP tidak diizinkan")
		return
	}

	var req struct {
		Collection string `json:"collection"` // Nama collection (wajib)
		ID         string `json:"id"`         // ID data yang mau diedit (wajib)
		Prompt     string `json:"prompt"`     // Data baru
		SQL        string `json:"sql"`        // Data baru
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Bad Request", "Request body JSON tidak valid")
		return
	}

	if req.Collection == "" || req.ID == "" || req.Prompt == "" || req.SQL == "" {
		utils.WriteError(w, http.StatusBadRequest, "Validasi Gagal", "collection, id, prompt, dan sql tidak boleh kosong")
		return
	}

	if strings.Contains(req.Collection, "cache") {
		if err := utils.IsReadOnlySQL(req.SQL); err != nil {

			log.Printf("SECURITY ALERT: Percobaan update query berbahaya pada ID %s. Query: %s", req.ID, req.SQL)

			utils.WriteError(w, http.StatusBadRequest, "SQL Ditolak", err.Error())
			return
		}
	}

	if err := ai.UpdateQdrantPoint(req.Collection, req.ID, req.Prompt, req.SQL); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Gagal update data", err.Error())
		return
	}

	utils.WriteJSON(w, http.StatusOK, utils.APIResponse{
		Status:  "updated",
		Message: fmt.Sprintf("Data ID %s berhasil diperbarui.", req.ID),
	})
}

func HandleAdminDeleteQdrant(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Collection string `json:"collection"`
		ID         string `json:"id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	if req.ID == "" {
		http.Error(w, "Parameter 'id' wajib diisi", http.StatusBadRequest)
		return
	}

	targetCollection := req.Collection
	if targetCollection == "" {
		if config.AppConfig != nil {
			targetCollection = config.AppConfig.QdrantCacheCollection
		} else {
			targetCollection = "bpr_supra_cache"
		}
	}

	log.Printf("Menerima request delete untuk ID: %s di Collection: %s", req.ID, targetCollection)

	err := ai.DeleteQdrantPoint(r.Context(), targetCollection, req.ID)
	if err != nil {
		log.Printf("Error deleting Qdrant point: %v", err)
		http.Error(w, fmt.Sprintf("Gagal menghapus: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"status":     "success",
		"message":    "Item berhasil dihapus permanen dari vector database.",
		"id_deleted": req.ID,
		"collection": targetCollection,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

func HandleAdminGenerateOTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Hanya POST yang diizinkan")
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "INVALID_BODY", "Format JSON salah")
		return
	}

	if req.Username == "" || req.Password == "" {
		utils.WriteError(w, http.StatusBadRequest, "INVALID_DATA", "Username dan Password (default) wajib diisi")
		return
	}

	plainPassword, err := utils.DecryptField(req.Password)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "DECRYPT_ERROR", "Gagal mendekripsi password")
		return
	}

	otp, err := auth.GenerateOTP(req.Username, plainPassword)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "GENERATE_FAILED", err.Error())
		return
	}

	utils.WriteJSON(w, http.StatusOK, map[string]string{
		"message": "OTP Berhasil digenerate. Berikan kode ini ke user segera.",
		"otp":     otp,
	})
}
