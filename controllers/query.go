package controllers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	ai "go-bank-api/ai"
	"go-bank-api/constants"
	logger "go-bank-api/log"
	models "go-bank-api/models"
	utils "go-bank-api/utils"
)

func HandleDynamicQuery(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	clientIP := r.RemoteAddr
	// CORS is handled by global middleware

	normalizedPrompt, ok := parseAndValidateRequest(w, r)
	if !ok {
		return
	}

	var (
		detectedIntent string = constants.IntentUnknown
		generatedSQL   string = ""
		finalStatus    string = constants.StatusFailed
		finalError     error  = nil
	)

	defer func() {
		logger.RecordActivity(clientIP, normalizedPrompt, detectedIntent, generatedSQL, finalStatus, finalError, time.Since(startTime))
	}()

	log.Printf("Menerima Prompt (Normalized): %s", normalizedPrompt)

	if handleAbsurdityCheck(r.Context(), w, normalizedPrompt) {
		return
	}

	if err := utils.ValidateSafePrompt(normalizedPrompt); err != nil {
		detectedIntent = constants.IntentAttack
		finalStatus = "BLOCKED"
		finalError = err
		log.Printf("SECURITY BLOCK: %v", err)
		utils.SendError(w, http.StatusForbidden, constants.ErrCodeDangerousIntent, err.Error())
		return
	}

	aiResp, err := ai.GetSQL(normalizedPrompt)
	if err != nil {
		handleAIError(w, err, &detectedIntent, &finalStatus, &finalError)
		return
	}

	if aiResp.IsAmbiguous {
		detectedIntent = constants.IntentAmbiguous
		finalStatus = constants.StatusSuccess
		utils.SendAmbiguous(w, "Maaf, pertanyaan Anda kurang jelas atau tidak cukup spesifik", aiResp.Suggestions)
		return
	}

	if strings.TrimSpace(aiResp.SQL) == "" {
		finalStatus = constants.StatusFailed
		utils.SendError(w, http.StatusUnprocessableEntity, constants.ErrCodeEmptySQL, "AI tidak menghasilkan query SQL yang valid")
		return
	}

	detectedIntent = constants.IntentSQL
	generatedSQL = aiResp.SQL
	log.Printf("SQL Awal: %s", aiResp.SQL)

	data, fixedSQL, execErr := executeWithRetry(aiResp)
	if execErr != nil {
		finalStatus = "DB_ERROR"
		finalError = execErr
		log.Printf("FATAL: Query Gagal Total | SQL: %s", aiResp.SQL)
		utils.SendError(w, http.StatusUnprocessableEntity, constants.ErrCodeQueryFailed,
			"Query tidak dapat dieksekusi. Sistem mencoba memperbaiki otomatis namun gagal.",
			execErr.Error())
		return
	}

	finalStatus = constants.StatusSuccess
	generatedSQL = fixedSQL
	if !aiResp.IsCached {
		go ai.SaveToCache(aiResp.PromptAsli, aiResp.Vector, fixedSQL)
	}

	utils.SendSuccess(w, data)
}

func HandleEnhancePrompt(w http.ResponseWriter, r *http.Request) {
	// CORS is handled by global middleware

	if r.Method != http.MethodPost {
		utils.WriteError(w, http.StatusMethodNotAllowed, constants.ErrCodeMethodNotAllowed, "Metode HTTP tidak diizinkan")
		return
	}

	var req models.EnhanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, constants.ErrCodeBadFormat, "Request body JSON tidak valid")
		return
	}

	if strings.TrimSpace(req.DraftPrompt) == "" {
		utils.WriteError(w, http.StatusBadRequest, constants.ErrCodeEmptyPrompt, "Prompt tidak boleh kosong")
		return
	}

	log.Printf("Enhancing prompt: '%s'...", req.DraftPrompt)

	enhancedText, err := ai.EnhanceNaturalLanguage(req.DraftPrompt)
	if err != nil {
		log.Printf("Gagal enhance prompt: %v", err)
		utils.WriteError(w, http.StatusInternalServerError, constants.ErrCodeEnhanceFailed, "Gagal memperbaiki kalimat")
		return
	}

	log.Printf("✅ Hasil Enhance: '%s'", enhancedText)

	utils.WriteJSON(w, http.StatusOK, models.EnhanceResponse{
		EnhancedPrompt: enhancedText,
	})
}

func parseAndValidateRequest(w http.ResponseWriter, r *http.Request) (string, bool) {
	if r.Method != http.MethodPost {
		utils.SendError(w, http.StatusMethodNotAllowed, constants.ErrCodeMethodNotAllowed, "Metode HTTP tidak diizinkan")
		return "", false
	}
	var req models.PromptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.SendError(w, http.StatusBadRequest, constants.ErrCodeInvalidJSON, "Format JSON tidak valid")
		return "", false
	}

	normalizedPrompt := strings.ToLower(strings.TrimSpace(req.Prompt))
	if normalizedPrompt == "" {
		utils.SendError(w, http.StatusBadRequest, constants.ErrCodeEmptyPrompt, "Prompt tidak boleh kosong")
		return "", false
	}

	if err := utils.ValidatePromptSanity(normalizedPrompt); err != nil {
		utils.SendError(w, http.StatusBadRequest, constants.ErrCodeInvalidPrompt, err.Error())
		return "", false
	}

	return normalizedPrompt, true
}

func handleAbsurdityCheck(ctx context.Context, w http.ResponseWriter, prompt string) bool {
	if isAbsurd, err := ai.IsAbsurdPrompt(ctx, prompt); err != nil {
		log.Printf("Error cek absurd: %v", err)
		utils.SendError(w, http.StatusInternalServerError, constants.ErrCodeInternalError, "Layanan sedang bermasalah")
		return true
	} else if isAbsurd {
		utils.SendAmbiguous(w, "Pertanyaan kurang jelas", []string{
			"ada berapa orang penabung saat ini",
			"nasabah yang jenis tabungan nya deposito",
		})
		return true
	}
	return false
}

func handleAIError(w http.ResponseWriter, err error, intent *string, status *string, finalErr *error) {
	if appErr, ok := err.(*models.AppError); ok {
		if appErr.Code == constants.ErrCodeChitChat {
			*intent = "CHAT/OFF_TOPIC"
			*status = constants.StatusSuccess
			utils.SendError(w, http.StatusBadRequest, "CHAT_RESPONSE", appErr.Message)
			return
		}
		log.Printf("Handled Error: %s - %s", appErr.Code, appErr.Message)

		statusCode := http.StatusInternalServerError
		if appErr.Code == constants.ErrCodeDangerousIntent {
			statusCode = http.StatusForbidden
		}
		*intent = constants.IntentSQLAttempt
		*finalErr = appErr
		utils.SendError(w, statusCode, appErr.Code, appErr.Message)
		return
	}

	log.Printf("AI gagal generate SQL (System Error): %v", err)
	utils.SendError(w, http.StatusInternalServerError, constants.ErrCodeAIGenerationFailed, "Gagal menghasilkan query SQL")
}

func executeWithRetry(aiResp models.AISqlResponse) (ai.QueryResult, string, error) {
	data, execErr := ai.ExecuteDynamicQuery(aiResp.SQL, nil)
	fixedSQL := aiResp.SQL

	if execErr != nil {
		log.Printf("Eksekusi Gagal: %v. Mencoba Self-Correction...", execErr)

		repairedSQL, repairErr := ai.RepairSQLFromAI(aiResp.PromptAsli, aiResp.SQL, execErr.Error())
		if repairErr == nil {
			log.Printf("🔄 Mencoba eksekusi SQL Perbaikan: %s", repairedSQL)
			dataRetry, execErrRetry := ai.ExecuteDynamicQuery(repairedSQL, nil)

			if execErrRetry == nil {
				log.Println("Self-Correction Berhasil menyelamatkan request!")
				return dataRetry, repairedSQL, nil
			}
			log.Printf("Self-Correction juga gagal: %v", execErrRetry)
		} else {
			log.Printf("Gagal generate perbaikan: %v", repairErr)
		}
		return ai.QueryResult{}, fixedSQL, execErr
	}

	return data, fixedSQL, nil
}
