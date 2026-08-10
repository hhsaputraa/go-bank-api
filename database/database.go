package database

import (
	"context"
	"database/sql"
	"fmt"
	config "go-bank-api/config"
	"log"
	"strings"
	"time"

	_ "github.com/sijms/go-ora/v2"
)

var DbInstance *sql.DB

func ConnectDB() error {
	if config.AppConfig == nil {
		return fmt.Errorf("konfigurasi aplikasi belum dimuat")
	}

	var err error
	DbInstance, err = sql.Open("oracle", config.AppConfig.DBConnString)
	if err != nil {
		return fmt.Errorf("gagal membuka koneksi database: %w", err)
	}

	// Set connection pool settings from config
	DbInstance.SetMaxOpenConns(config.AppConfig.DBMaxOpenConns)
	DbInstance.SetMaxIdleConns(config.AppConfig.DBMaxIdleConns)
	DbInstance.SetConnMaxLifetime(config.AppConfig.DBConnMaxLifetime)
	DbInstance.SetConnMaxIdleTime(2 * time.Minute)

	// Test connection with timeout from config
	ctx, cancel := context.WithTimeout(context.Background(), config.AppConfig.DBPingTimeout)
	defer cancel()

	err = DbInstance.PingContext(ctx)
	if err != nil {
		return fmt.Errorf("gagal melakukan ping ke database: %w", err)
	}

	log.Printf("[INFO] Berhasil terkoneksi ke database ORACLE!")
	log.Printf("   - Max Open Connections: %d", config.AppConfig.DBMaxOpenConns)
	log.Printf("   - Max Idle Connections: %d", config.AppConfig.DBMaxIdleConns)
	log.Printf("   - Connection Max Lifetime: %v", config.AppConfig.DBConnMaxLifetime)

	// Ensure DB performance indexes exist
	EnsureDatabaseIndexes()

	return nil
}

func EnsureDatabaseIndexes() {
	if DbInstance == nil {
		return
	}

	indexes := []string{
		"CREATE INDEX idx_user_sessions_token ON user_sessions(token, expires_at)",
		"CREATE INDEX idx_app_users_username ON app_users(username)",
		"CREATE INDEX idx_user_sess_device ON user_sessions(id_app_users, device_info)",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, stmt := range indexes {
		_, err := DbInstance.ExecContext(ctx, stmt)
		if err != nil {
			errStr := strings.ToLower(err.Error())
			// Ignore if index or column list already indexed (ORA-01408 / ORA-00955)
			if strings.Contains(errStr, "ora-01408") || strings.Contains(errStr, "ora-00955") || strings.Contains(errStr, "already indexed") || strings.Contains(errStr, "already used") {
				continue
			}
			log.Printf("[DB Performance] Warning executing index creation: %v", err)
		} else {
			log.Printf("[DB Performance] Index dipastikan aktif: %s", stmt)
		}
	}
}
