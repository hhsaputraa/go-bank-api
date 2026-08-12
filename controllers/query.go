package controllers

import (
	"context"
	"encoding/json"
	"fmt"
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

	normalizedPrompt, selectedModel, ok := parseAndValidateRequest(w, r)
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

	log.Printf("Menerima Prompt (Normalized): %s | Model: %s", normalizedPrompt, selectedModel)

	// --- EARLY CACHE CHECK (CONTINUOUS CHAT FIX) ---
	aiResp := checkEarlySemanticCache(r.Context(), normalizedPrompt)

	// Jika belum ada di cache (Cache Miss)
	if aiResp == nil {
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

		resp, err := ai.GetSQLWithModel(normalizedPrompt, selectedModel)
		if err != nil {
			handleAIError(w, err, &detectedIntent, &finalStatus, &finalError)
			return
		}

		// If prompt was rewritten, save original prompt so it can be cached accurately later
		resp.PromptAsli = normalizedPrompt
		aiResp = &resp
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

	data, fixedSQL, execErr := ai.ExecuteWithRetry(*aiResp)
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
	ai.ProcessPostExecution(*aiResp, fixedSQL)

	streamSSEExecution(w, r, normalizedPrompt, data, selectedModel)
}

func checkEarlySemanticCache(ctx context.Context, normalizedPrompt string) *models.AISqlResponse {
	promptVector, vectorErr := ai.GenerateEmbedding(normalizedPrompt)
	if vectorErr != nil {
		log.Printf("Warning: Gagal generate embedding awal: %v", vectorErr)
		return nil
	}

	hardHit, _, checkErr := ai.CheckSemanticCache(ctx, promptVector)
	if checkErr == nil && hardHit != nil {
		log.Printf("⚡ EARLY CACHE HIT (%s): Langsung menggunakan query cache", normalizedPrompt)
		hardHit.PromptAsli = normalizedPrompt
		hardHit.Vector = promptVector
		hardHit.IsCached = true
		return hardHit
	}
	return nil
}

func streamSSEExecution(w http.ResponseWriter, r *http.Request, prompt string, data ai.QueryResult, selectedModel string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// 1. Kirim payload tabel segera
	tablePayload := map[string]any{"type": "data", "data": data}
	tableBytes, _ := json.Marshal(tablePayload)
	fmt.Fprintf(w, "data: %s\n\n", tableBytes)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	// 2. Siapkan Stream Analytics AI
	chunkChan := make(chan string)
	errChan := make(chan error)

	go ai.GenerateInsightStream(r.Context(), prompt, data, selectedModel, chunkChan, errChan)

	// 3. Render event listener loop
	for {
		select {
		case chunk, ok := <-chunkChan:
			if !ok {
				fmt.Fprintf(w, "data: [DONE]\n\n")
				if flusher, ok := w.(http.Flusher); ok {
					flusher.Flush()
				}
				return
			}
			msgPayload := map[string]any{"type": "text", "content": chunk}
			msgBytes, _ := json.Marshal(msgPayload)
			fmt.Fprintf(w, "data: %s\n\n", msgBytes)
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		case errVal := <-errChan:
			if errVal != nil {
				log.Printf("Error AI Insight Stream: %v", errVal)
			}
			fmt.Fprintf(w, "data: [DONE]\n\n")
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			return
		case <-r.Context().Done():
			return
		}
	}
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

	log.Printf("[INFO] Hasil Enhance: '%s'", enhancedText)

	utils.WriteJSON(w, http.StatusOK, models.EnhanceResponse{
		EnhancedPrompt: enhancedText,
	})
}

func parseAndValidateRequest(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	if r.Method != http.MethodPost {
		utils.SendError(w, http.StatusMethodNotAllowed, constants.ErrCodeMethodNotAllowed, "Metode HTTP tidak diizinkan")
		return "", "", false
	}
	var req models.PromptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.SendError(w, http.StatusBadRequest, constants.ErrCodeInvalidJSON, "Format JSON tidak valid")
		return "", "", false
	}

	normalizedPrompt := strings.ToLower(strings.TrimSpace(req.Prompt))
	if normalizedPrompt == "" {
		utils.SendError(w, http.StatusBadRequest, constants.ErrCodeEmptyPrompt, "Prompt tidak boleh kosong")
		return "", "", false
	}

	if err := utils.ValidatePromptSanity(normalizedPrompt); err != nil {
		utils.SendError(w, http.StatusBadRequest, constants.ErrCodeInvalidPrompt, err.Error())
		return "", "", false
	}

	// Default model if not specified
	selectedModel := req.Model
	if selectedModel == "" {
		selectedModel = constants.GroqModelDefault
	}

	// Validate model against allowed list
	allowedModels := map[string]bool{
		constants.ModelQwen32B:    true,
		constants.ModelGPTOss120B: true,
		constants.ModelGPTOss20B:  true,
	}

	if !allowedModels[selectedModel] {
		// Fallback or Error?
		// For now, let's strictly enforce usage of known models to avoid billing surprises or errors
		// But if they send something else, maybe we just warn and use default?
		// Let's stick to the request: "bisa disesuaikan dengan pilihan 3 model berikut"
		// If it's not one of them, we return error.
		utils.SendError(w, http.StatusBadRequest, constants.ErrCodeInvalidPrompt, "Model AI tidak valid. Pilih antara: "+constants.ModelQwen32B+", "+constants.ModelGPTOss120B+", "+constants.ModelGPTOss20B)
		return "", "", false
	}

	return normalizedPrompt, selectedModel, true
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
		if appErr.Code == constants.ErrCodeChitChat || appErr.Code == constants.ErrCodeAiRefusal {
			*intent = "CHAT/OFF_TOPIC"
			if appErr.Code == constants.ErrCodeAiRefusal {
				*intent = "AI_REFUSAL"
			}
			*status = constants.StatusSuccess
			// Kita kirim sebagai CHAT_RESPONSE agar frontend menampilkannya sebagai pesan bot (tipe warning/text)
			utils.SendError(w, http.StatusOK, "CHAT_RESPONSE", appErr.Message)
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

