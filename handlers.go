package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
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

func validateDangerousIntent(prompt string) error {
	dangerousKeywords := []string{
		"hapus", "delete", "drop", "remove",
		"ubah", "update", "ganti", "edit", "alter", "modify",
		"tambah", "insert", "create", "add",
		"truncate", "grant", "revoke",
	}
	pattern := `\b(` + strings.Join(dangerousKeywords, "|") + `)\b`
	re := regexp.MustCompile(pattern)

	if re.MatchString(prompt) {
		match := re.FindString(prompt)
		return fmt.Errorf("permintaan ditolak: mengandung kata kunci manipulasi '%s'. ", match)
	}

	return nil
}

func HandleDynamicQuery(w http.ResponseWriter, r *http.Request) {
	// CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		sendError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Metode HTTP tidak diizinkan")
		return
	}
	var req PromptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "INVALID_JSON", "Format JSON tidak valid")
		return
	}

	normalizedPrompt := strings.ToLower(strings.TrimSpace(req.Prompt))
	if normalizedPrompt == "" {
		sendError(w, http.StatusBadRequest, "EMPTY_PROMPT", "Prompt tidak boleh kosong")
		return
	}

	log.Printf("Menerima Prompt (Normalized): %s", normalizedPrompt)

	if isAbsurd, err := IsAbsurdPrompt(r.Context(), normalizedPrompt); err != nil {
		log.Printf("Error cek absurd: %v", err)
		sendError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Layanan sedang bermasalah")
		return
	} else if isAbsurd {
		sendAmbiguous(w, "Pertanyaan tidak relevan", []string{
			"ada berapa orang penabung saat ini",
			"nasabah yang jenis tabungan nya deposito",
		})
		return
	}
	if err := validateDangerousIntent(normalizedPrompt); err != nil {
		log.Printf("SECURITY BLOCK: %v", err)
		sendError(w, http.StatusForbidden, "DANGEROUS_INTENT", err.Error())
		return
	}

	aiResp, err := GetSQL(normalizedPrompt)
	if err != nil {
		log.Printf("AI gagal generate SQL: %v", err)
		sendError(w, http.StatusInternalServerError, "AI_GENERATION_FAILED", "Gagal menghasilkan query SQL")
		return
	}

	if aiResp.IsAmbiguous {
		sendAmbiguous(w, "Maaf, pertanyaan Anda kurang jelas atau tidak cukup spesifik", aiResp.Suggestions)
		return
	}
	if strings.TrimSpace(aiResp.SQL) == "" {
		sendError(w, http.StatusUnprocessableEntity, "EMPTY_SQL", "AI tidak menghasilkan query SQL yang valid")
		return
	}

	log.Printf("SQL yang akan dieksekusi: %s", aiResp.SQL)
	data, execErr := ExecuteDynamicQuery(aiResp.SQL, nil)
	if execErr != nil {
		log.Printf("GAGAL EKSEKUSI QUERY: %v | SQL: %s", execErr, aiResp.SQL)
		sendError(w, http.StatusUnprocessableEntity, "QUERY_EXECUTION_FAILED",
			"Query tidak dapat dieksekusi. Mungkin syntax salah atau melanggar aturan database",
			execErr.Error())
		return
	}

	if !aiResp.IsCached {
		go SaveToCache(aiResp.PromptAsli, aiResp.Vector, aiResp.SQL)
	}

	sendSuccess(w, data)
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
		log.Println("ADMIN: training RAG (Embedding) dimulai")
		mainTrain()
	}()

	respondWithJSON(w, http.StatusAccepted, map[string]string{
		"message": "Proses retraining RAG telah selesai",
	})
}

func HandleAdminListQdrant(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	collectionName := r.URL.Query().Get("collection")
	if collectionName == "" {
		respondWithError(w, http.StatusBadRequest, "Parameter 'collection' wajib diisi")
		return
	}

	data, err := GetAllQdrantPoints(collectionName, 1000)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, data)
}

func validateReadOnlySQL(query string) error {
	q := strings.TrimSpace(strings.ToUpper(query))

	if !strings.HasPrefix(q, "SELECT") && !strings.HasPrefix(q, "WITH") {
		return fmt.Errorf("query harus dimulai dengan SELECT atau WITH")
	}
	dangerousKeywords := []string{
		"DROP", "DELETE", "INSERT", "UPDATE",
		"ALTER", "TRUNCATE", "CREATE", "GRANT", "REVOKE",
	}

	pattern := `\b(` + strings.Join(dangerousKeywords, "|") + `)\b`
	re := regexp.MustCompile(pattern)

	if re.MatchString(q) {

		return fmt.Errorf("query mengandung kata kunci terlarang: %s", re.FindString(q))
	}

	return nil
}

func HandleAdminCacheCreate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

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

	if err := validateReadOnlySQL(req.SQL); err != nil {
		log.Printf("⚠️ Percobaan inject query berbahaya: %s", req.SQL)
		respondWithError(w, http.StatusBadRequest, "SQL Ditolak: "+err.Error())
		return
	}

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

func HandleAdminQdrantUpdate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "PUT,OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		Collection string `json:"collection"` // Nama collection (wajib)
		ID         string `json:"id"`         // ID data yang mau diedit (wajib)
		Prompt     string `json:"prompt"`     // Data baru
		SQL        string `json:"sql"`        // Data baru
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Body JSON tidak valid")
		return
	}

	if req.Collection == "" || req.ID == "" || req.Prompt == "" || req.SQL == "" {
		respondWithError(w, http.StatusBadRequest, "Field 'collection', 'id', 'prompt', dan 'sql' wajib diisi semua!")
		return
	}

	if strings.Contains(req.Collection, "cache") {
		if err := validateReadOnlySQL(req.SQL); err != nil {

			log.Printf("⚠️ SECURITY ALERT: Percobaan update query berbahaya pada ID %s. Query: %s", req.ID, req.SQL)

			respondWithError(w, http.StatusBadRequest, "SQL Ditolak: "+err.Error())
			return
		}
	}

	if err := UpdateQdrantPoint(req.Collection, req.ID, req.Prompt, req.SQL); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{
		"status":  "updated",
		"message": fmt.Sprintf("Data ID %s berhasil diperbarui.", req.ID),
	})
}

func HandleAdminDeleteQdrant(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

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
		if AppConfig != nil {
			targetCollection = AppConfig.QdrantCacheCollection
		} else {
			targetCollection = "bpr_supra_cache"
		}
	}

	log.Printf("Menerima request delete untuk ID: %s di Collection: %s", req.ID, targetCollection)

	err := DeleteQdrantPoint(r.Context(), targetCollection, req.ID)
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
	json.NewEncoder(w).Encode(response)
}
