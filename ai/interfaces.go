package ai

import (
	"context"
	models "go-bank-api/models"
)

// AIService defines the contract for text-to-SQL and natural language processing.
type AIService interface {
	GetSQLWithModel(userPrompt string, selectedModel string) (models.AISqlResponse, error)
	GenerateEmbedding(text string) ([]float32, error)
	EnhanceNaturalLanguage(prompt string) (string, error)
	IsAbsurdPrompt(ctx context.Context, prompt string) (bool, error)
	RepairSQLFromAI(promptAsli, badSQL, errMessage string) (string, error)
	GenerateInsightStream(ctx context.Context, prompt string, data QueryResult, modelName string, chunkChan chan<- string, errChan chan<- error)
}

// VectorStore defines the contract for semantic search and vector operations.
type VectorStore interface {
	SearchSemanticCache(ctx context.Context, promptVector []float32) (*models.AISqlResponse, float32, error)
	SaveToCache(promptAsli string, promptVector []float32, sqlQuery string)
	LearnFromCorrection(promptAsli string, correctedSQL string)
}

// CacheService defines the contract for fast key/value or memory caching.
type CacheService interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{})
	Delete(key string)
}

// DataAnalysisService defines the contract for offline dataset/pandas analysis.
type DataAnalysisService interface {
	AnalyzeSession(absFilePath string, userMessage string) (RunnerOutput, string, string, error)
}
