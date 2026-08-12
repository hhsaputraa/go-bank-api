package ai

import (
	"log"
	"strings"

	models "go-bank-api/models"
)

// ExecuteWithRetry executes SQL and attempts AI self-repair if initial execution fails.
func ExecuteWithRetry(aiResp models.AISqlResponse) (QueryResult, string, error) {
	data, execErr := ExecuteDynamicQuery(aiResp.SQL, nil)
	fixedSQL := aiResp.SQL

	if execErr != nil {
		log.Printf("Eksekusi Gagal: %v. Mencoba Self-Correction...", execErr)

		repairedSQL, repairErr := RepairSQLFromAI(aiResp.PromptAsli, aiResp.SQL, execErr.Error())
		if repairErr == nil {
			log.Printf("🔄 Mencoba eksekusi SQL Perbaikan: %s", repairedSQL)
			dataRetry, execErrRetry := ExecuteDynamicQuery(repairedSQL, nil)

			if execErrRetry == nil {
				log.Println("Self-Correction Berhasil menyelamatkan request!")
				return dataRetry, repairedSQL, nil
			}
			log.Printf("Self-Correction juga gagal: %v", execErrRetry)
		} else {
			log.Printf("Gagal generate perbaikan: %v", repairErr)
		}
		return QueryResult{}, fixedSQL, execErr
	}

	return data, fixedSQL, nil
}

// ProcessPostExecution handles caching and continuous learning after successful query execution.
func ProcessPostExecution(aiResp models.AISqlResponse, fixedSQL string) {
	if !aiResp.IsCached {
		go SaveToCache(aiResp.PromptAsli, aiResp.Vector, fixedSQL)
	}
	if strings.TrimSpace(fixedSQL) != strings.TrimSpace(aiResp.SQL) {
		log.Println("REINFORCEMENT: Terdeteksi perbaikan SQL. Menyimpan koreksi...")
		go LearnFromCorrection(aiResp.PromptAsli, fixedSQL)
	}
}
