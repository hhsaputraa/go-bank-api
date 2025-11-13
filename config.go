package main

import (
	"os"
	"strconv"
	"time"
)

// GetEnv mengambil environment variable dengan fallback value
func GetEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// GetEnvAsInt mengambil environment variable sebagai integer dengan fallback value
func GetEnvAsInt(key string, fallback int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return fallback
	}
	
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return fallback
	}
	return value
}

// GetEnvAsUint64 mengambil environment variable sebagai uint64 dengan fallback value
func GetEnvAsUint64(key string, fallback uint64) uint64 {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return fallback
	}
	
	value, err := strconv.ParseUint(valueStr, 10, 64)
	if err != nil {
		return fallback
	}
	return value
}

// GetEnvAsBool mengambil environment variable sebagai boolean dengan fallback value
func GetEnvAsBool(key string, fallback bool) bool {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return fallback
	}
	
	value, err := strconv.ParseBool(valueStr)
	if err != nil {
		return fallback
	}
	return value
}

// GetEnvAsDuration mengambil environment variable sebagai time.Duration dengan fallback value
// Format: "5s", "10m", "1h", dll
func GetEnvAsDuration(key string, fallback time.Duration) time.Duration {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return fallback
	}
	
	value, err := time.ParseDuration(valueStr)
	if err != nil {
		return fallback
	}
	return value
}

