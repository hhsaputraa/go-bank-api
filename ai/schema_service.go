package ai

import (
	"context"
	"fmt"
	database "go-bank-api/database"
	models "go-bank-api/models"
	"log"
	"net/url"
	"os"
	"strings"
	"time"
)

func getSchemaFromConnStr() (string, error) {
	connStr := os.Getenv("DB_CONN_STRING")
	if connStr == "" {
		return "", fmt.Errorf("DB_CONN_STRING tidak ditemukan di .env")
	}

	cleanConnStr := connStr
	if !strings.HasPrefix(connStr, "oracle://") {
		cleanConnStr = "oracle://" + connStr
	}

	u, err := url.Parse(cleanConnStr)
	if err != nil {
		return "", fmt.Errorf("gagal parsing connection string: %w", err)
	}

	username := u.User.Username()
	if username == "" {
		return "", fmt.Errorf("username/schema tidak ditemukan dalam connection string")
	}

	return strings.ToUpper(username), nil
}

func GetDynamicSchemaContext() ([]string, error) {
	log.Println("Mulai mengambil skema DDL dinamis dari database...")

	schema, err := getSchemaFromConnStr()
	if err != nil {
		return nil, err
	}
	schema = strings.Trim(schema, ":")
	schema = strings.TrimSpace(schema)

	query := `
	 SELECT
	   table_name,
	   column_name,
	   data_type
	 FROM
	   all_tab_columns
	 WHERE
	   owner = UPPER(:1)
	 ORDER BY
	   table_name,
	   column_id
	 `

	if database.DbInstance == nil {
		return nil, fmt.Errorf("koneksi database (DbInstance) belum siap")
	}

	rows, err := database.DbInstance.QueryContext(context.Background(), query, schema)
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

func GetDynamicReferenceData(ctx context.Context) (string, error) {
	targetTables := map[string]string{
		"master_status_rekening": "nama_status",
		"master_jenis_rekening":  "nama_jenis",
		"master_tipe_nasabah":    "nama_tipe",
		"master_tipe_transaksi":  "nama_transaksi",
	}

	if database.DbInstance == nil {
		return "", fmt.Errorf("koneksi database belum siap")
	}

	schema, err := getSchemaFromConnStr()
	if err != nil {
		return "", err
	}

	schema = strings.Trim(schema, ":")
	schema = strings.TrimSpace(schema)

	var builder strings.Builder
	builder.WriteString("== LIVE DATA REFERENSI (Isi Tabel Master Terbaru) ==\n")
	builder.WriteString("Gunakan ID/Kode di bawah ini secara TEPAT jika user bertanya tentang kategori ini:\n\n")

	for tableName, nameCol := range targetTables {
		idCol := "id"
		switch tableName {
		case "master_status_rekening":
			idCol = "id_status_rekening"
		case "master_jenis_rekening":
			idCol = "id_jenis_rekening"
		case "master_tipe_nasabah":
			idCol = "id_tipe_nasabah"
		case "master_tipe_transaksi":
			idCol = "id_tipe_transaksi"
		}

		query := fmt.Sprintf("SELECT %s, %s FROM %s.%s ORDER BY %s ASC", idCol, nameCol, schema, tableName, idCol)

		rows, err := database.DbInstance.QueryContext(ctx, query)
		if err != nil {
			log.Printf("Warning: Gagal ambil ref data untuk tabel %s: %v (Query: %s)", tableName, err, query)
			continue
		}

		builder.WriteString(fmt.Sprintf("TABEL REFERENSI: '%s'\n", tableName))

		counter := 0
		for rows.Next() {
			var id, nama string
			if err := rows.Scan(&id, &nama); err != nil {
				continue
			}
			builder.WriteString(fmt.Sprintf("- ID '%s' = %s\n", id, nama))
			counter++
		}
		if err := rows.Close(); err != nil {
			log.Printf("Warning: Gagal menutup cursor database untuk tabel %s: %v", tableName, err)
		}

		if counter == 0 {
			builder.WriteString("(Tabel kosong)\n")
		}
		builder.WriteString("\n")
	}

	return builder.String(), nil
}

func GetDynamicSqlExamples() ([]models.SqlExample, error) {
	log.Println("Mulai mengambil contoh SQL dinamis dari tabel 'rag_sql_example'...")
	if database.DbInstance == nil {
		return nil, fmt.Errorf("koneksi database (Dbinstance) belum siap")
	}

	schema, err := getSchemaFromConnStr()
	if err != nil {
		return nil, fmt.Errorf("gagal mendapatkan schema dari connection string: %w", err)
	}

	schema = strings.Trim(schema, ":")
	schema = strings.TrimSpace(schema)

	rawQuery := fmt.Sprintf(`
		SELECT
			prompt_example,
			sql_example
		FROM
			%s.rag_sql_examples
		ORDER BY
			id ASC
	`, schema)

	query := strings.TrimSpace(rawQuery)

	rows, err := database.DbInstance.QueryContext(context.Background(), query)
	if err != nil {
		log.Printf("Query Gagal: %s", query)
		return nil, fmt.Errorf("gagal query tabel rag_sql_examples : %w", err)
	}
	defer rows.Close()

	var contexts []models.SqlExample

	for rows.Next() {
		var promptExample, sqlExample string
		if err := rows.Scan(&promptExample, &sqlExample); err != nil {
			return nil, err
		}

		fullContekan := fmt.Sprintf("%s\n%s", promptExample, sqlExample)
		contexts = append(contexts, models.SqlExample{
			FullContent: fullContekan,
			PromptOnly:  promptExample,
		})
	}

	if len(contexts) == 0 {
		log.Println("PERINGATAN: Tidak ada contoh SQL ditemukan di tabel 'rag_sql_examples'.")
	} else {
		log.Printf("✅ Berhasil! mengambil %d contoh SQL dinamis.", len(contexts))
	}
	return contexts, nil
}

func AddSqlExample(promptAsli string, sqlKoreksi string) error {
	if database.DbInstance == nil {
		return fmt.Errorf("koneksi database (DbInstance) belum siap")
	}

	// Get schema name dynamically from connection string
	schema, err := getSchemaFromConnStr()
	if err != nil {
		return fmt.Errorf("gagal mendapatkan schema dari connection string: %w", err)
	}

	promptExample := fmt.Sprintf("-- Pertanyaan: \"%s\"", promptAsli)

	query := fmt.Sprintf(`
	INSERT INTO %s.rag_sql_examples
        (prompt_example, sql_example)
    VALUES
        (:1, :2)
    `, schema)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = database.DbInstance.ExecContext(ctx, query, promptExample, sqlKoreksi)
	if err != nil {
		return fmt.Errorf("gagal insert contekan baru ke DB: %w", err)
	}

	log.Printf("✅ Berhasil! Menyimpan contekan baru ke 'rag_sql_examples' untuk prompt: %s", promptAsli)
	return nil
}

type DictionaryItem struct {
	Istilah   string
	Definisi  string
	LogikaSQL string
}

func GetBusinessDictionary(ctx context.Context) (string, error) {
	schema, err := getSchemaFromConnStr()
	if err != nil {
		return "", err
	}

	query := fmt.Sprintf("SELECT istilah, definisi_bisnis, logika_sql FROM %s.ai_dictionary", schema)

	rows, err := database.DbInstance.QueryContext(ctx, query)
	if err != nil {
		return "", nil
	}
	defer rows.Close()

	var builder strings.Builder
	builder.WriteString("== KAMUS ISTILAH BISNIS (PRIORITAS TINGGI) ==\n")
	builder.WriteString("Gunakan logika ini jika user menyebut kata kunci berikut:\n")

	for rows.Next() {
		var d DictionaryItem
		if err := rows.Scan(&d.Istilah, &d.Definisi, &d.LogikaSQL); err != nil {
			return "", err
		}
		line := fmt.Sprintf("- \"%s\" bermakna: %s. (SQL Logic Wajib: `%s`)\n", d.Istilah, d.Definisi, d.LogikaSQL)
		builder.WriteString(line)
	}

	return builder.String(), nil
}

type AbsurdKeyword struct {
	Keyword  string
	Category string
	IsActive bool
}

func IsAbsurdPrompt(ctx context.Context, prompt string) (bool, error) {
	if database.DbInstance == nil {
		return false, fmt.Errorf("koneksi database (DbInstance) belum siap")
	}
	schema, err := getSchemaFromConnStr()
	if err != nil {
		return false, fmt.Errorf("gagal mendapatkan schema: %w", err)
	}
	schema = strings.Trim(schema, ":")
	schema = strings.TrimSpace(schema)

	lowerPrompt := strings.ToLower(prompt)
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s.absurd_keywords WHERE is_active = 1 AND INSTR(:1, LOWER(keyword)) > 0 AND ROWNUM = 1", schema)

	var exists int

	err = database.DbInstance.QueryRowContext(ctx, query, lowerPrompt).Scan(&exists)
	if err != nil {
		log.Printf("Error query absurd_keywords: %v (Query: %s)", err, query)
		return false, err
	}

	if exists >= 1 {
		log.Printf("⛔ Prompt terdeteksi absurd/OOT: '%s'", prompt)
		return true, nil
	}

	return false, nil
}
