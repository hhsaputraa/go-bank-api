package core

import (
	"database/sql"
	ai "go-bank-api/ai"
	config "go-bank-api/config"
)

// App is the central dependency injection container.
type App struct {
	Config      *config.Config
	DB          *sql.DB
	AIService   ai.AIService
	VectorStore ai.VectorStore
}

// NewApp creates a new App dependency container.
func NewApp(cfg *config.Config, db *sql.DB) *App {
	return &App{
		Config: cfg,
		DB:     db,
	}
}
