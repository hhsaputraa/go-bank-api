package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var DbInstance *sql.DB

func ConnectDB() error {
	connStr := os.Getenv("DB_CONN_STRING")
	if connStr == "" {
		return fmt.Errorf("DB_CONN_STRING tidak ditemukan di environment")
	}

	var err error
	DbInstance, err = sql.Open("pgx", connStr)
	if err != nil {
		return fmt.Errorf("gagal membuka koneksi database: %w", err)
	}

	// Konfigurasi connection pool dari environment variables
	maxOpenConns := GetEnvAsInt("DB_MAX_OPEN_CONNS", 25)
	maxIdleConns := GetEnvAsInt("DB_MAX_IDLE_CONNS", 10)
	connMaxLifetime := GetEnvAsDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute)
	connTimeout := GetEnvAsDuration("DB_CONN_TIMEOUT", 5*time.Second)

	DbInstance.SetMaxOpenConns(maxOpenConns)
	DbInstance.SetMaxIdleConns(maxIdleConns)
	DbInstance.SetConnMaxLifetime(connMaxLifetime)

	ctx, cancel := context.WithTimeout(context.Background(), connTimeout)
	defer cancel()

	err = DbInstance.PingContext(ctx)
	if err != nil {
		return fmt.Errorf("gagal melakukan ping ke database: %w", err)
	}

	log.Println("✅ Berhasil terkoneksi ke database 'postgres' dengan skema 'bpr_supra'!")
	return nil
}
