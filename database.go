package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var DbInstance *sql.DB

func ConnectDB() error {
	if AppConfig == nil {
		return fmt.Errorf("konfigurasi aplikasi belum dimuat")
	}

	var err error
	DbInstance, err = sql.Open("pgx", AppConfig.DBConnString)
	if err != nil {
		return fmt.Errorf("gagal membuka koneksi database: %w", err)
	}

	// Set connection pool settings from config
	DbInstance.SetMaxOpenConns(AppConfig.DBMaxOpenConns)
	DbInstance.SetMaxIdleConns(AppConfig.DBMaxIdleConns)
	DbInstance.SetConnMaxLifetime(AppConfig.DBConnMaxLifetime)

	// Test connection with timeout from config
	ctx, cancel := context.WithTimeout(context.Background(), AppConfig.DBPingTimeout)
	defer cancel()

	err = DbInstance.PingContext(ctx)
	if err != nil {
		return fmt.Errorf("gagal melakukan ping ke database: %w", err)
	}

	log.Printf("✅ Berhasil terkoneksi ke database PostgreSQL!")
	log.Printf("   - Max Open Connections: %d", AppConfig.DBMaxOpenConns)
	log.Printf("   - Max Idle Connections: %d", AppConfig.DBMaxIdleConns)
	log.Printf("   - Connection Max Lifetime: %v", AppConfig.DBConnMaxLifetime)
	return nil
}
