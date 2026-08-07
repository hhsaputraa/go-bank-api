package controllers

import (
	"context"
	config "go-bank-api/config"
	database "go-bank-api/database"
	utils "go-bank-api/utils"
	"net/http"
	"runtime"
	"time"
)

var StartTime = time.Now()

func HandleHealthCheck(w http.ResponseWriter, r *http.Request) {
	dbStatus := "UP"
	var dbError string

	// Ping database with timeout
	pingTimeout := 2 * time.Second
	if config.AppConfig != nil && config.AppConfig.DBPingTimeout > 0 {
		pingTimeout = config.AppConfig.DBPingTimeout
	}

	if database.DbInstance == nil {
		dbStatus = "DOWN"
		dbError = "database connection instance is nil"
	} else {
		ctx, cancel := context.WithTimeout(r.Context(), pingTimeout)
		defer cancel()

		if err := database.DbInstance.PingContext(ctx); err != nil {
			dbStatus = "DOWN"
			dbError = err.Error()
		}
	}

	// Memory statistics
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Build response data
	healthData := map[string]interface{}{
		"status":     "UP",
		"timestamp":  time.Now().Format(time.RFC3339),
		"uptime":     time.Since(StartTime).String(),
		"go_version": runtime.Version(),
		"num_cpu":    runtime.NumCPU(),
		"goroutines": runtime.NumGoroutine(),
		"memory": map[string]interface{}{
			"alloc_mb": float64(memStats.Alloc) / 1024 / 1024,
			"sys_mb":   float64(memStats.Sys) / 1024 / 1024,
			"num_gc":   memStats.NumGC,
		},
		"database": map[string]interface{}{
			"status": dbStatus,
		},
	}

	if dbError != "" {
		healthData["database"].(map[string]interface{})["error"] = dbError
	}

	// If database is down, return HTTP 503 Service Unavailable, otherwise 200 OK
	statusCode := http.StatusOK
	if dbStatus == "DOWN" {
		healthData["status"] = "DOWN"
		statusCode = http.StatusServiceUnavailable
	}

	utils.WriteJSON(w, statusCode, healthData)
}
