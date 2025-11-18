package main

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all application configuration
type Config struct {
	// Database
	DBConnString      string
	DBMaxOpenConns    int
	DBMaxIdleConns    int
	DBConnMaxLifetime time.Duration
	DBPingTimeout     time.Duration

	// Groq AI
	GroqAPIKey  string
	GroqModel   string
	GroqAPIURL  string
	GroqTimeout time.Duration

	// Google AI
	GoogleAPIKey        string
	EmbeddingModel      string
	EmbeddingVectorSize int

	// Qdrant
	QdrantGRPCHost        string
	QdrantGRPCPort        int
	QdrantURL             string
	QdrantCollectionName  string
	QdrantCacheCollection string
	QdrantDistanceMetric  string
	QdrantTimeout         time.Duration

	// Cache
	CacheSimilarityThreshold float32
	CacheSearchLimit         uint64

	// RAG
	RAGSearchLimit uint64

	// Server
	ServerPort string
	ServerHost string

	// Query
	QueryTimeout time.Duration

	// Environment
	AppEnv string
	Debug  bool
}

var AppConfig *Config

func LoadConfig() (*Config, error) {
	cfg := &Config{
		// Database
		DBConnString:      getEnv("DB_CONN_STRING", ""),
		DBMaxOpenConns:    getEnvAsInt("DB_MAX_OPEN_CONNS", 25),
		DBMaxIdleConns:    getEnvAsInt("DB_MAX_IDLE_CONNS", 10),
		DBConnMaxLifetime: time.Duration(getEnvAsInt("DB_CONN_MAX_LIFETIME_MINUTES", 5)) * time.Minute,
		DBPingTimeout:     time.Duration(getEnvAsInt("DB_PING_TIMEOUT_SECONDS", 5)) * time.Second,

		// Groq AI
		GroqAPIKey:  getEnv("GROQ_API_KEY", ""),
		GroqModel:   getEnv("GROQ_MODEL", ""),
		GroqAPIURL:  getEnv("GROQ_API_URL", ""),
		GroqTimeout: time.Duration(getEnvAsInt("GROQ_TIMEOUT_SECONDS", 30)) * time.Second,

		// Google AI
		GoogleAPIKey:        getEnv("GOOGLE_API_KEY", ""),
		EmbeddingModel:      getEnv("EMBEDDING_MODEL", "models/text-embedding-004"),
		EmbeddingVectorSize: getEnvAsInt("EMBEDDING_VECTOR_SIZE", 768),

		// Qdrant
		QdrantGRPCHost:        getEnv("QDRANT_GRPC_HOST", ""),
		QdrantGRPCPort:        getEnvAsInt("QDRANT_GRPC_PORT", 6334),
		QdrantURL:             getEnv("QDRANT_URL", ""),
		QdrantCollectionName:  getEnv("QDRANT_COLLECTION_NAME", ""),
		QdrantCacheCollection: getEnv("QDRANT_CACHE_COLLECTION", ""),
		QdrantDistanceMetric:  getEnv("QDRANT_DISTANCE_METRIC", ""),
		QdrantTimeout:         time.Duration(getEnvAsInt("QDRANT_TIMEOUT_SECONDS", 60)) * time.Second,

		// Cache
		CacheSimilarityThreshold: getEnvAsFloat32("CACHE_SIMILARITY_THRESHOLD", 0.95),
		CacheSearchLimit:         uint64(getEnvAsInt("CACHE_SEARCH_LIMIT", 1)),

		// RAG
		RAGSearchLimit: uint64(getEnvAsInt("RAG_SEARCH_LIMIT", 7)),

		// Server
		ServerPort: getEnv("SERVER_PORT", ""),
		ServerHost: getEnv("SERVER_HOST", ""),

		// Query
		QueryTimeout: time.Duration(getEnvAsInt("QUERY_TIMEOUT_SECONDS", 10)) * time.Second,

		// Environment
		AppEnv: getEnv("APP_ENV", ""),
		Debug:  getEnvAsBool("DEBUG", false),
	}

	// Validate required fields
	if cfg.DBConnString == "" {
		return nil, fmt.Errorf("DB_CONN_STRING is required")
	}
	if cfg.GroqAPIKey == "" {
		return nil, fmt.Errorf("GROQ_API_KEY is required")
	}
	if cfg.GoogleAPIKey == "" {
		return nil, fmt.Errorf("GOOGLE_API_KEY is required")
	}

	AppConfig = cfg
	return cfg, nil
}

// Helper functions to read environment variables with defaults

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

func getEnvAsFloat32(key string, defaultValue float32) float32 {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.ParseFloat(valueStr, 32)
	if err != nil {
		return defaultValue
	}
	return float32(value)
}

func getEnvAsBool(key string, defaultValue bool) bool {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.ParseBool(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}
