package ai

import (
	"context"
	"fmt"
	database "go-bank-api/database"
	models "go-bank-api/models"
	helper "go-bank-api/utils"
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
	if GlobalCache != nil {
		return GlobalCache.GetReferenceData(), nil
	}
	log.Println("Peringatan: GlobalCache nil, mengembalikan data referensi kosong.")
	return "", nil
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

	if err := helper.ValidateIdentifier(schema); err != nil {
		log.Printf("SECURITY ALERT: Schema validation failed in AddSqlExample: %s", schema)
		return err
	}

	promptExample := fmt.Sprintf("-- Pertanyaan: \"%s\"", promptAsli)

	const queryTemplate = `
	INSERT INTO {SCHEMA}.rag_sql_examples
		(prompt_example, sql_example)
	VALUES
		(:1, :2)
	`
	query := strings.Replace(queryTemplate, "{SCHEMA}", schema, 1)

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
	if GlobalCache != nil {
		return GlobalCache.GetBusinessDict(), nil
	}
	return "", nil
}

type AbsurdKeyword struct {
	Keyword  string
	Category string
	IsActive bool
}

func IsAbsurdPrompt(ctx context.Context, prompt string) (bool, error) {
	if GlobalCache == nil {
		return false, nil
	}

	lowerPrompt := strings.ToLower(prompt)
	words := GlobalCache.GetAbsurdKeywords()

	for _, w := range words {
		if strings.Contains(lowerPrompt, w) {
			log.Printf("⛔ Prompt terdeteksi absurd/OOT berdasarkan cache memori: '%s' (mengandung: %s)", prompt, w)
			return true, nil
		}
	}

	return false, nil
}
