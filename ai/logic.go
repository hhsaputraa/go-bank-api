package ai

import (
	"context"
	"fmt"
	config "go-bank-api/config"
	database "go-bank-api/database"
	"log"
	"strings"
	"time"
)

type QueryResult struct {
	Columns []string        `json:"columns"`
	Rows    [][]interface{} `json:"rows"`
}

func ExecuteDynamicQuery(query string, params []interface{}) (QueryResult, error) {
	var result QueryResult

	checkQuery := strings.ToUpper(query)
	checkQuery = strings.TrimSpace(checkQuery)

	if !strings.HasPrefix(checkQuery, "SELECT") && !strings.HasPrefix(checkQuery, "WITH") {
		err := fmt.Errorf("KEAMANAN: Hanya query SELECT yang diizinkan. Query Anda: %s", query)
		log.Println("[ai][logic][ExecuteDynamicQuery] error:", err)
		return result, err
	}

	query = strings.TrimSpace(query)
	query = strings.TrimRight(query, ";")

	forbidden := []string{
		"DROP ", "DELETE ", "UPDATE ", "INSERT ", "TRUNCATE ",
		"ALTER ", "GRANT ", "REVOKE ", "CREATE ", "MERGE ", "RENAME ",
		"ALL_TABS", "ALL_TABLES", "USER_TABLES", "DBA_TABLES", "ALL_VIEWS",
		"DBA_VIEWS", "ALL_SOURCE", "USER_SOURCE", "ALL_USERS", "DBA_USERS",
		"V$", "GV$",
	}

	for _, word := range forbidden {
		if strings.Contains(checkQuery, word) {
			err := fmt.Errorf("KEAMANAN: Ditemukan kata kunci terlarang '%s'", word)
			log.Println("[ai][logic][ExecuteDynamicQuery] error:", err)
			return result, err
		}
	}

	timeout := 10 * time.Second
	if config.AppConfig != nil {
		timeout = config.AppConfig.QueryTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	rows, err := database.DbInstance.QueryContext(ctx, query, params...)
	if err != nil {
		log.Println("[ai][logic][ExecuteDynamicQuery] error:", err)
		log.Printf("Error eksekusi query SQL: %v. Query: %s", err, query)
		return result, fmt.Errorf("gagal mengeksekusi query SQL (Pastikan syntax Oracle 10g valid)")
	}
	defer rows.Close()


	columns, err := rows.Columns()
	if err != nil {
		log.Println("[ai][logic][ExecuteDynamicQuery] error:", err)
		return result, fmt.Errorf("gagal membaca kolom: %w", err)
	}
	result.Columns = columns
	result.Rows = make([][]interface{}, 0)

	for rows.Next() {
		rowValues := make([]interface{}, len(columns))
		rowScanners := make([]interface{}, len(columns))

		for i := range rowValues {
			rowScanners[i] = &rowValues[i]
		}

		if err := rows.Scan(rowScanners...); err != nil {
			log.Println("[ai][logic][ExecuteDynamicQuery] error:", err)
			return result, fmt.Errorf("gagal scanning baris data: %w", err)
		}

		for i, val := range rowValues {
			if b, ok := val.([]byte); ok {
				rowValues[i] = string(b)
			}
		}

		result.Rows = append(result.Rows, rowValues)
	}

	if err = rows.Err(); err != nil {
		log.Println("[ai][logic][ExecuteDynamicQuery] error:", err)
		return result, fmt.Errorf("error saat iterasi baris: %w", err)
	}

	return result, nil
}
