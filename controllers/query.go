package controllers

import (
	"encoding/json"
	ai "go-bank-api/ai"
	logger "go-bank-api/log"
	models "go-bank-api/models"
	utils "go-bank-api/utils"
	"log"
	"net/http"
	"strings"
	"time"
)

func HandleDynamicQuery(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	clientIP := r.RemoteAddr
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		utils.SendError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Metode HTTP tidak diizinkan")
		return
	}
	var req models.PromptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.SendError(w, http.StatusBadRequest, "INVALID_JSON", "Format JSON tidak valid")
		return
	}

	normalizedPrompt := strings.ToLower(strings.TrimSpace(req.Prompt))
	if normalizedPrompt == "" {
		utils.SendError(w, http.StatusBadRequest, "EMPTY_PROMPT", "Prompt tidak boleh kosong")
		return
	}

	var (
		detectedIntent string = "UKNOWN"
		generatedSQL   string = ""
		finalStatus    string = "FAILED"
		finalError     error  = nil
	)

	defer func() {
		logger.RecordActivity(clientIP, normalizedPrompt, detectedIntent, generatedSQL, finalStatus, finalError, time.Since(startTime))
	}()

	log.Printf("Menerima Prompt (Normalized): %s", normalizedPrompt)

	if isAbsurd, err := ai.IsAbsurdPrompt(r.Context(), normalizedPrompt); err != nil {
		log.Printf("Error cek absurd: %v", err)
		utils.SendError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Layanan sedang bermasalah")
		return
	} else if isAbsurd {
		utils.SendAmbiguous(w, "Pertanyaan kurang jelas", []string{
			"ada berapa orang penabung saat ini",
			"nasabah yang jenis tabungan nya deposito",
		})
		return
	}
	if err := utils.ValidateSafePrompt(normalizedPrompt); err != nil {
		detectedIntent = "ATTACK"
		finalStatus = "BLOCKED"
		finalError = err
		log.Printf("SECURITY BLOCK: %v", err)
		utils.SendError(w, http.StatusForbidden, "DANGEROUS_INTENT", err.Error())
		return
	}

	aiResp, err := ai.GetSQL(normalizedPrompt)
	if err != nil {
		if appErr, ok := err.(*models.AppError); ok {
			if appErr.Code == "CHIT_CHAT" {
				detectedIntent = "CHAT/OFF_TOPIC"
				finalStatus = "SUCCESS"
				utils.SendError(w, http.StatusBadRequest, "CHAT_RESPONSE", appErr.Message)
				return

			}
			log.Printf("Handled Error: %s - %s", appErr.Code, appErr.Message)

			statusCode := http.StatusInternalServerError
			if appErr.Code == "DANGEROUS_INTENT" {
				statusCode = http.StatusForbidden
			}
			detectedIntent = "SQL_ATTEMPT"
			finalError = appErr
			utils.SendError(w, statusCode, appErr.Code, appErr.Message)
			return
		}

		log.Printf("AI gagal generate SQL (System Error): %v", err)
		utils.SendError(w, http.StatusInternalServerError, "AI_GENERATION_FAILED", "Gagal menghasilkan query SQL")
		return
	}

	if aiResp.IsAmbiguous {
		detectedIntent = "AMBIGUOUS"
		finalStatus = "SUCCESS"
		utils.SendAmbiguous(w, "Maaf, pertanyaan Anda kurang jelas atau tidak cukup spesifik", aiResp.Suggestions)
		return
	}
	if strings.TrimSpace(aiResp.SQL) == "" {
		finalStatus = "FAILED"
		utils.SendError(w, http.StatusUnprocessableEntity, "EMPTY_SQL", "AI tidak menghasilkan query SQL yang valid")
		return
	}
	detectedIntent = "SQL"
	generatedSQL = aiResp.SQL
	log.Printf("SQL Awal: %s", aiResp.SQL)

	data, execErr := ai.ExecuteDynamicQuery(aiResp.SQL, nil)

	if execErr != nil {
		log.Printf("⚠️ Eksekusi Gagal: %v. Mencoba Self-Correction...", execErr)

		fixedSQL, repairErr := ai.RepairSQLFromAI(aiResp.PromptAsli, aiResp.SQL, execErr.Error())

		if repairErr == nil {
			log.Printf("🔄 Mencoba eksekusi SQL Perbaikan: %s", fixedSQL)
			dataRetry, execErrRetry := ai.ExecuteDynamicQuery(fixedSQL, nil)

			if execErrRetry == nil {
				log.Println("Self-Correction Berhasil menyelamatkan request!")

				data = dataRetry
				execErr = nil
				aiResp.SQL = fixedSQL
				generatedSQL = fixedSQL
			} else {
				log.Printf("Self-Correction juga gagal: %v", execErrRetry)
			}
		} else {
			log.Printf("Gagal generate perbaikan: %v", repairErr)
		}
	}
	if execErr != nil {
		finalStatus = "DB_ERROR"
		finalError = execErr
		log.Printf("FATAL: Query Gagal Total | SQL: %s", aiResp.SQL)
		utils.SendError(w, http.StatusUnprocessableEntity, "QUERY_EXECUTION_FAILED",
			"Query tidak dapat dieksekusi. Sistem mencoba memperbaiki otomatis namun gagal.",
			execErr.Error())
		return
	}
	finalStatus = "SUCCESS"
	if !aiResp.IsCached {
		go ai.SaveToCache(aiResp.PromptAsli, aiResp.Vector, aiResp.SQL)
	}

	utils.SendSuccess(w, data)
}

func HandleEnhancePrompt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		utils.WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Metode HTTP tidak diizinkan")
		return
	}

	var req models.EnhanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "BAD_FORMAT", "Request body JSON tidak valid")
		return
	}

	if strings.TrimSpace(req.DraftPrompt) == "" {
		utils.WriteError(w, http.StatusBadRequest, "EMPTY_PROMPT", "Prompt tidak boleh kosong")
		return
	}

	log.Printf("✨ Enhancing prompt: '%s'...", req.DraftPrompt)

	enhancedText, err := ai.EnhanceNaturalLanguage(req.DraftPrompt)
	if err != nil {
		log.Printf("❌ Gagal enhance prompt: %v", err)
		utils.WriteError(w, http.StatusInternalServerError, "ENHANCE_FAILED", "Gagal memperbaiki kalimat")
		return
	}

	log.Printf("✅ Hasil Enhance: '%s'", enhancedText)

	utils.WriteJSON(w, http.StatusOK, models.EnhanceResponse{
		EnhancedPrompt: enhancedText,
	})
}
