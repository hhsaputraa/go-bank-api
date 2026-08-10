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
		err := fmt.Errorf("DB_CONN_STRING tidak ditemukan di .env")
		log.Println("[ai][schema_service][getSchemaFromConnStr] error:", err)
		return "", err
	}

	cleanConnStr := connStr
	if !strings.HasPrefix(connStr, "oracle://") {
		cleanConnStr = "oracle://" + connStr
	}

	u, err := url.Parse(cleanConnStr)
	if err != nil {
		log.Println("[ai][schema_service][getSchemaFromConnStr] error:", err)
		return "", fmt.Errorf("gagal parsing connection string: %w", err)
	}

	username := u.User.Username()
	if username == "" {
		err := fmt.Errorf("username/schema tidak ditemukan dalam connection string")
		log.Println("[ai][schema_service][getSchemaFromConnStr] error:", err)
		return "", err
	}

	return strings.ToUpper(username), nil
}

func GetDynamicSchemaContext() ([]string, error) {
	log.Println("Mulai mengambil skema DDL dinamis dari database...")

	schema, err := getSchemaFromConnStr()
	if err != nil {
		log.Println("[ai][schema_service][GetDynamicSchemaContext] error:", err)
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
		err := fmt.Errorf("koneksi database (DbInstance) belum siap")
		log.Println("[ai][schema_service][GetDynamicSchemaContext] error:", err)
		return nil, err
	}

	rows, err := database.DbInstance.QueryContext(context.Background(), query, schema)
	if err != nil {
		log.Println("[ai][schema_service][GetDynamicSchemaContext] error:", err)
		return nil, fmt.Errorf("gagal query information_schema: %w", err)
	}
	defer rows.Close()

	var contexts []string
	var currentTable string
	var sb strings.Builder

	for rows.Next() {
		var tableName, columnName, dataType string
		if err := rows.Scan(&tableName, &columnName, &dataType); err != nil {
			log.Println("[ai][schema_service][GetDynamicSchemaContext] error:", err)
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
		err := fmt.Errorf("tidak ada tabel ditemukan di skema '%s'", schema)
		log.Println("[ai][schema_service][GetDynamicSchemaContext] error:", err)
		return nil, err
	}

	// Append dynamic FK Relationship Hints extracted from database catalog
	if fkHints := GetDynamicFKRelationshipHints(context.Background()); fkHints != "" {
		contexts = append(contexts, fkHints)
	}

	log.Printf("[INFO] Berhasil! Mengambil %d potongan DDL dinamis (termasuk Dynamic FK Hints).", len(contexts))
	return contexts, nil
}

// GetDynamicFKRelationshipHints queries database constraints & schema metadata to dynamically generate real JOIN hints
func GetDynamicFKRelationshipHints(ctx context.Context) string {
	if database.DbInstance == nil {
		return ""
	}

	schema, err := getSchemaFromConnStr()
	if err != nil {
		return ""
	}
	schema = strings.Trim(strings.TrimSpace(schema), ":")

	query := `
	SELECT 
		a.table_name,
		a.column_name,
		c_pk.table_name AS r_table_name,
		b.column_name AS r_column_name
	FROM all_cons_columns a
	JOIN all_constraints c ON a.constraint_name = c.constraint_name AND a.owner = c.owner
	JOIN all_constraints c_pk ON c.r_constraint_name = c_pk.constraint_name AND c.r_owner = c_pk.owner
	JOIN all_cons_columns b ON c_pk.constraint_name = b.constraint_name AND c_pk.owner = b.owner AND a.position = b.position
	WHERE c.constraint_type = 'R' AND a.owner = UPPER(:1)
	ORDER BY a.table_name, a.position
	`

	rows, err := database.DbInstance.QueryContext(ctx, query, schema)
	var sb strings.Builder

	if err == nil {
		defer rows.Close()
		count := 0
		for rows.Next() {
			var tbl, col, rTbl, rCol string
			if err := rows.Scan(&tbl, &col, &rTbl, &rCol); err == nil {
				if count == 0 {
					sb.WriteString("== PANDUAN RELASI JOIN ANTAR TABEL (DINAMIS DARI DATABASE) ==\n")
				}
				sb.WriteString(fmt.Sprintf("- %s.%s <---> %s.%s\n", tbl, col, rTbl, rCol))
				count++
			}
		}
		if count > 0 {
			return sb.String()
		}
	}

	// Fallback: Dynamic Column Matching (Infer FKs by matching common key column names across tables)
	fallbackQuery := `
	SELECT table_name, column_name 
	FROM all_tab_columns 
	WHERE owner = UPPER(:1) 
	  AND (column_name LIKE 'ID_%' OR column_name LIKE '%_ID' OR column_name LIKE 'NO_%' OR column_name LIKE '%_NO' OR column_name LIKE 'KODE_%')
	ORDER BY column_name, table_name
	`

	fallbackRows, err := database.DbInstance.QueryContext(ctx, fallbackQuery, schema)
	if err != nil {
		return ""
	}
	defer fallbackRows.Close()

	colToTables := make(map[string][]string)
	for fallbackRows.Next() {
		var tbl, col string
		if err := fallbackRows.Scan(&tbl, &col); err == nil {
			colToTables[col] = append(colToTables[col], tbl)
		}
	}

	fbCount := 0
	for col, tables := range colToTables {
		if len(tables) > 1 {
			if fbCount == 0 {
				sb.WriteString("== PANDUAN RELASI JOIN ANTAR TABEL (INFERRED DARI KOLOM SKEMA) ==\n")
			}
			for i := 0; i < len(tables)-1; i++ {
				sb.WriteString(fmt.Sprintf("- %s.%s <---> %s.%s\n", tables[i], col, tables[i+1], col))
				fbCount++
			}
		}
	}

	return sb.String()
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
		err := fmt.Errorf("koneksi database (Dbinstance) belum siap")
		log.Println("[ai][schema_service][GetDynamicSqlExamples] error:", err)
		return nil, err
	}

	schema, err := getSchemaFromConnStr()
	if err != nil {
		log.Println("[ai][schema_service][GetDynamicSqlExamples] error:", err)
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
		log.Println("[ai][schema_service][GetDynamicSqlExamples] error:", err)
		log.Printf("Query Gagal: %s", query)
		return nil, fmt.Errorf("gagal query tabel rag_sql_examples : %w", err)
	}
	defer rows.Close()

	var contexts []models.SqlExample

	for rows.Next() {
		var promptExample, sqlExample string
		if err := rows.Scan(&promptExample, &sqlExample); err != nil {
			log.Println("[ai][schema_service][GetDynamicSqlExamples] error:", err)
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
		log.Printf("[INFO] Berhasil! mengambil %d contoh SQL dinamis.", len(contexts))
	}
	return contexts, nil
}

func AddSqlExample(promptAsli string, sqlKoreksi string) error {
	if database.DbInstance == nil {
		err := fmt.Errorf("koneksi database (DbInstance) belum siap")
		log.Println("[ai][schema_service][AddSqlExample] error:", err)
		return err
	}

	// Get schema name dynamically from connection string
	schema, err := getSchemaFromConnStr()
	if err != nil {
		log.Println("[ai][schema_service][AddSqlExample] error:", err)
		return fmt.Errorf("gagal mendapatkan schema dari connection string: %w", err)
	}

	if err := helper.ValidateIdentifier(schema); err != nil {
		log.Println("[ai][schema_service][AddSqlExample] error:", err)
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
		log.Println("[ai][schema_service][AddSqlExample] error:", err)
		return fmt.Errorf("gagal insert contekan baru ke DB: %w", err)
	}

	log.Printf("[INFO] Berhasil! Menyimpan contekan baru ke 'rag_sql_examples' untuk prompt: %s", promptAsli)
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
