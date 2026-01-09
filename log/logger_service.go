package logger

import (
	"encoding/json"
	"log"
	"os"
	"time"
)

type AuditLog struct {
	Timestamp    string `json:"timestamp"`
	Level        string `json:"level"`
	ClientIP     string `json:"client_ip"`
	UserPrompt   string `json:"user_prompt"`
	Intent       string `json:"intent"`
	SQLGenerated string `json:"sql_query"`
	Status       string `json:"status"`
	ErrorMessage string `json:"error_msg"`
	LatencyMs    int64  `json:"latency_ms"`
}

var logFile *os.File

func CloseLogger() {
	if logFile != nil {
		err := logFile.Close()
		if err != nil {
			log.Printf("Error saat menutup file log: %v", err)
		}
	}
}

func InitLogger() {
	var err error
	logFile, err = os.OpenFile("activity.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		log.Fatalf("Gagal membuat file log: %v", err)
	}
	log.Println("✅ Audit Logger siap. Menulis ke activity.log")
}

func RecordActivity(clientIP, prompt, intent, sql string, status string, errCb error, duration time.Duration) {
	errMsg := ""
	if errCb != nil {
		errMsg = errCb.Error()
	}

	entry := AuditLog{
		Timestamp:    time.Now().Format(time.RFC3339),
		Level:        "INFO",
		ClientIP:     clientIP,
		UserPrompt:   prompt,
		Intent:       intent,
		SQLGenerated: sql,
		Status:       status,
		ErrorMessage: errMsg,
		LatencyMs:    duration.Milliseconds(),
	}

	if status == "BLOCKED" || intent == "OFF_TOPIC" {
		entry.Level = "WARN"
	} else if status == "FAILED" || errCb != nil {
		entry.Level = "ERROR"
	} else if intent == "SQL" {
		entry.Level = "INFO"
	}

	jsonEntry, err := json.Marshal(entry)
	if err != nil {
		log.Printf("Gagal marshal log: %v", err)
		return
	}

	if logFile != nil {
		if _, err := logFile.WriteString(string(jsonEntry) + "\n"); err != nil {
			log.Printf("Gagal menulis ke file log: %v", err)
		}
	}

	//log.Println(string(jsonEntry))
}
