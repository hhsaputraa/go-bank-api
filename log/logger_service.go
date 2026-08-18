package logger

import (
	"bufio"
	"encoding/json"
	"log"
	"os"
	"sync"
	"sync/atomic"
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

var (
	logFile      *os.File
	bufWriter    *bufio.Writer
	logQueue     chan []byte
	workerDone   chan struct{}
	initOnce     sync.Once
	droppedCount int64
)

func InitLogger() {
	initOnce.Do(func() {
		var err error
		logFile, err = os.OpenFile("activity.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			log.Fatalf("Gagal membuat file log: %v", err)
		}

		bufWriter = bufio.NewWriterSize(logFile, 64*1024) // 64KB write buffer
		logQueue = make(chan []byte, 10000)
		workerDone = make(chan struct{})

		// Start background log writer worker
		go startLogWorker()

		log.Println("[INFO] Audit Logger siap (Asynchronous Worker). Menulis ke activity.log")
	})
}

func startLogWorker() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	defer close(workerDone)

	for {
		select {
		case data, ok := <-logQueue:
			if !ok {
				// Channel closed, flush remaining buffer and return
				if bufWriter != nil {
					_ = bufWriter.Flush()
				}
				return
			}
			if bufWriter != nil {
				_, _ = bufWriter.Write(data)
				_ = bufWriter.WriteByte('\n')
			}
		case <-ticker.C:
			if bufWriter != nil {
				_ = bufWriter.Flush()
			}
		}
	}
}

func CloseLogger() {
	if logQueue != nil {
		close(logQueue)
		// Wait for worker to finish flushing
		<-workerDone
		logQueue = nil
	}
	if logFile != nil {
		err := logFile.Close()
		if err != nil {
			log.Printf("Error saat menutup file log: %v", err)
		}
		logFile = nil
	}
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

	// Non-blocking send to background worker queue
	if logQueue != nil {
		select {
		case logQueue <- jsonEntry:
		default:
			atomic.AddInt64(&droppedCount, 1)
		}
	}
}
