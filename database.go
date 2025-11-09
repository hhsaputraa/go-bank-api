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

	DbInstance.SetMaxOpenConns(25)
	DbInstance.SetMaxIdleConns(10)
	DbInstance.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = DbInstance.PingContext(ctx)
	if err != nil {
		return fmt.Errorf("gagal melakukan ping ke database: %w", err)
	}

	log.Println("✅ Berhasil terkoneksi ke database 'postgres' dengan skema 'bpr_supra'!")
	return nil
}
