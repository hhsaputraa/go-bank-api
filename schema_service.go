package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
)

func getSchemaFromConnStr() (string, error) {
	connStr := os.Getenv("DB_CONN_STRING")
	if connStr == "" {
		return "", fmt.Errorf("DB_CONN_STRING tidak ditemukan di .env")
	}

	parsedURL, err := url.Parse(connStr)
	if err != nil {
		return "", fmt.Errorf("gagal parse DB_CONN_STRING: %w", err)
	}

	schema := parsedURL.Query().Get("search_path")
	if schema == "" {
		return "", fmt.Errorf("search_path tidak ditemukan di DB_CONN_STRING (contoh: ...?search_path=nama_skema)")
	}

	return schema, nil
}

func GetDynamicSchemaContext() ([]string, error) {
	log.Println("Mulai mengambil skema DDL dinamis dari database...")

	schema, err := getSchemaFromConnStr()
	if err != nil {
		return nil, err
	}

	query := `
	SELECT 
		table_name, 
		column_name, 
		data_type 
	FROM 
		information_schema.columns 
	WHERE 
		table_schema = $1
	ORDER BY 
		table_name, 
		ordinal_position;
	`

	if DbInstance == nil {
		return nil, fmt.Errorf("koneksi database (DbInstance) belum siap")
	}

	rows, err := DbInstance.QueryContext(context.Background(), query, schema)
	if err != nil {
		return nil, fmt.Errorf("gagal query information_schema: %w", err)
	}
	defer rows.Close()

	var contexts []string
	var currentTable string
	var sb strings.Builder

	for rows.Next() {
		var tableName, columnName, dataType string
		if err := rows.Scan(&tableName, &columnName, &dataType); err != nil {
			return nil, err
		}

		if tableName != currentTable {
			if sb.Len() > 0 {
				contexts = append(contexts, strings.TrimRight(sb.String(), ",\n")+"\n);")
			}
			sb.Reset()
			sb.WriteString(fmt.Sprintf("CREATE TABLE %s (\n", tableName))
			currentTable = tableName
		}

		sb.WriteString(fmt.Sprintf("    %s %s,\n", columnName, dataType))
	}

	if sb.Len() > 0 {
		contexts = append(contexts, strings.TrimRight(sb.String(), ",\n")+"\n);")
	}

	if len(contexts) == 0 {
		return nil, fmt.Errorf("tidak ada tabel ditemukan di skema '%s'", schema)
	}

	log.Printf("✅ Berhasil! Mengambil %d potongan DDL dinamis.", len(contexts))
	return contexts, nil
}

func GetDynamicSqlExamples() ([]string, error) {
	log.Println("Mulai mengambil contoh SQL dinamis dari tabel 'rag_sql_example'...")
	if DbInstance == nil {
		return nil, fmt.Errorf("koneksi database (Dbinstance) belum siap")
	}

	// Baca nama tabel dari environment variable (untuk fleksibilitas schema)
	tableName := GetEnv("FEEDBACK_TABLE_NAME", "bpr_supra.rag_sql_examples")

	query := fmt.Sprintf(`
	SELECT
		prompt_example,
		sql_example
	FROM
		%s
	ORDER BY
		id
	`, tableName)

	rows, err := DbInstance.QueryContext(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("gagal query tabel rag_sql_examples : %w", err)
	}
	defer rows.Close()

	var contexts []string

	for rows.Next() {
		var promptExample, sqlExample string
		if err := rows.Scan(&promptExample, &sqlExample); err != nil {
			return nil, err
		}

		fullContekan := fmt.Sprintf("%s\n%s", promptExample, sqlExample)
		contexts = append(contexts, fullContekan)
	}

	if len(contexts) == 0 {
		log.Println("PERINGATAN: Tidak ada contoh SQL ditemukan di tabel 'rag_sql_examples'.")
	} else {
		log.Printf("✅ Berhasil! mengambil %d contoh SQL dinamis.", len(contexts))
	}
	return contexts, nil
}

// SaveFeedbackToDatabase - Menyimpan feedback koreksi SQL dari user ke database
func SaveFeedbackToDatabase(promptAsli, sqlKoreksi string) error {
	if DbInstance == nil {
		return fmt.Errorf("koneksi database (DbInstance) belum siap")
	}

	// Baca nama tabel dari environment variable (untuk fleksibilitas schema)
	tableName := GetEnv("FEEDBACK_TABLE_NAME", "bpr_supra.rag_sql_examples")

	query := fmt.Sprintf(`
		INSERT INTO %s (prompt_example, sql_example, created_at)
		VALUES ($1, $2, NOW())
	`, tableName)

	_, err := DbInstance.ExecContext(context.Background(), query, promptAsli, sqlKoreksi)
	if err != nil {
		return fmt.Errorf("gagal menyimpan feedback ke tabel %s: %w", tableName, err)
	}

	log.Printf("✅ Feedback berhasil disimpan ke tabel %s", tableName)
	return nil
}
