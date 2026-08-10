package ai

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"go-bank-api/database"
	helper "go-bank-api/utils"
)

type MemoryCache struct {
	mu            sync.RWMutex
	ReferenceData string
	BusinessDict  string
	AbsurdWords   []string
}

var GlobalCache *MemoryCache

func InitMemoryCache() {
	GlobalCache = &MemoryCache{
		AbsurdWords: make([]string, 0),
	}

	log.Println("Memuat data statis ke dalam In-Memory Cache...")
	if err := GlobalCache.LoadAllCacheFromDB(); err != nil {
		log.Println("[ai][memory_cache][InitMemoryCache] error:", err)
		log.Printf("Peringatan: Gagal memuat cache saat startup: %v", err)
	} else {
		log.Println("[INFO] Data referensi statis & filter berhasil dimuat ke cache!")
	}

	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			if err := GlobalCache.LoadAllCacheFromDB(); err != nil {
				log.Println("[ai][memory_cache][InitMemoryCache] error:", err)
				log.Printf("Peringatan: Gagal auto-refresh in-memory cache: %v", err)
			}
		}
	}()
}

func (c *MemoryCache) LoadAllCacheFromDB() error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	refData, errRef := buildReferenceData(ctx)
	if errRef != nil {
		log.Println("[ai][memory_cache][LoadAllCacheFromDB] error:", errRef)
		log.Printf("Gagal load reference data ke cache: %v", errRef)
	}

	dictData, errDict := buildBusinessDictionary(ctx)
	if errDict != nil {
		log.Println("[ai][memory_cache][LoadAllCacheFromDB] error:", errDict)
		log.Printf("Gagal load business dictionary ke cache: %v", errDict)
	}

	absurdData, errAbsurd := buildAbsurdKeywords(ctx)
	if errAbsurd != nil {
		log.Println("[ai][memory_cache][LoadAllCacheFromDB] error:", errAbsurd)
		log.Printf("Gagal load absurd keywords ke cache: %v", errAbsurd)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if errRef == nil {
		c.ReferenceData = refData
	}
	if errDict == nil {
		c.BusinessDict = dictData
	}
	if errAbsurd == nil {
		c.AbsurdWords = absurdData
	}
	return nil
}

func buildReferenceData(ctx context.Context) (string, error) {
	if database.DbInstance == nil {
		err := fmt.Errorf("koneksi database belum siap")
		log.Println("[ai][memory_cache][buildReferenceData] error:", err)
		return "", err
	}

	schema, err := getSchemaFromConnStr()
	if err != nil {
		log.Println("[ai][memory_cache][buildReferenceData] error:", err)
		return "", err
	}
	schema = strings.Trim(strings.TrimSpace(schema), ":")

	if err := helper.ValidateIdentifier(schema); err != nil {
		log.Println("[ai][memory_cache][buildReferenceData] error:", err)
		return "", err
	}

	targetTables := map[string]string{
		"master_status_rekening": "nama_status",
		"master_jenis_rekening":  "nama_jenis",
		"master_tipe_nasabah":    "nama_tipe",
		"master_tipe_transaksi":  "nama_transaksi",
		"master_pinjaman":        "jenis_pinjaman",
		"master_kd_kntr":         "nama_kantor",
	}

	var builder strings.Builder
	builder.WriteString("== LIVE DATA REFERENSI (Isi Tabel Master Terbaru) ==\n")
	builder.WriteString("Gunakan ID/Kode di bawah ini secara TEPAT jika user bertanya tentang kategori ini:\n\n")

	const queryTemplate = "SELECT {ID}, {NAME} FROM {SCHEMA}.{TABLE} ORDER BY {ID} ASC"
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
		case "master_pinjaman":
			idCol = "id_pinjaman"
		case "master_kd_kntr":
			idCol = "id_kantor"
		}

		query := strings.Replace(queryTemplate, "{SCHEMA}", schema, 1)
		query = strings.Replace(query, "{TABLE}", tableName, 1)
		query = strings.ReplaceAll(query, "{ID}", idCol)
		query = strings.Replace(query, "{NAME}", nameCol, 1)

		func() {
			rows, err := database.DbInstance.QueryContext(ctx, query)
			if err != nil {
				log.Println("[ai][memory_cache][buildReferenceData] error:", err)
				log.Printf("Warning: Gagal ambil ref data untuk %s: %v", tableName, err)
				return
			}
			defer rows.Close()

			builder.WriteString(fmt.Sprintf("TABEL REFERENSI: '%s'\n", tableName))
			counter := 0
			for rows.Next() {
				var id, nama string
				if err := rows.Scan(&id, &nama); err == nil {
					builder.WriteString(fmt.Sprintf("- ID '%s' = %s\n", id, nama))
					counter++
				}
			}

			if counter == 0 {
				builder.WriteString("(Tabel kosong)\n")
			}
			builder.WriteString("\n")
		}()
	}

	return builder.String(), nil
}

func buildBusinessDictionary(ctx context.Context) (string, error) {
	if database.DbInstance == nil {
		err := fmt.Errorf("koneksi database belum siap")
		log.Println("[ai][memory_cache][buildBusinessDictionary] error:", err)
		return "", err
	}

	schema, err := getSchemaFromConnStr()
	if err != nil {
		log.Println("[ai][memory_cache][buildBusinessDictionary] error:", err)
		return "", err
	}
	schema = strings.Trim(strings.TrimSpace(schema), ":")

	query := fmt.Sprintf("SELECT istilah, definisi_bisnis, logika_sql FROM %s.ai_dictionary", schema)
	rows, err := database.DbInstance.QueryContext(ctx, query)
	if err != nil {
		log.Println("[ai][memory_cache][buildBusinessDictionary] error:", err)
		return "", err
	}
	defer rows.Close()

	var builder strings.Builder
	builder.WriteString("== KAMUS ISTILAH BISNIS (PRIORITAS TINGGI) ==\n")
	builder.WriteString("Gunakan logika ini jika user menyebut kata kunci berikut:\n")

	for rows.Next() {
		var istilah, definisi, logika string
		if err := rows.Scan(&istilah, &definisi, &logika); err == nil {
			builder.WriteString(fmt.Sprintf("- \"%s\" bermakna: %s. (SQL Logic Wajib: `%s`)\n", istilah, definisi, logika))
		}
	}

	return builder.String(), nil
}

func buildAbsurdKeywords(ctx context.Context) ([]string, error) {
	if database.DbInstance == nil {
		err := fmt.Errorf("koneksi database belum siap")
		log.Println("[ai][memory_cache][buildAbsurdKeywords] error:", err)
		return nil, err
	}

	schema, err := getSchemaFromConnStr()
	if err != nil {
		log.Println("[ai][memory_cache][buildAbsurdKeywords] error:", err)
		return nil, err
	}
	schema = strings.Trim(strings.TrimSpace(schema), ":")

	query := fmt.Sprintf("SELECT keyword FROM %s.absurd_keywords WHERE is_active = 1", schema)
	rows, err := database.DbInstance.QueryContext(ctx, query)
	if err != nil {
		log.Println("[ai][memory_cache][buildAbsurdKeywords] error:", err)
		return nil, err
	}
	defer rows.Close()

	var words []string
	for rows.Next() {
		var keyword string
		if err := rows.Scan(&keyword); err == nil && keyword != "" {
			words = append(words, strings.ToLower(keyword))
		}
	}

	return words, nil
}

func (c *MemoryCache) GetReferenceData() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ReferenceData
}

func (c *MemoryCache) GetBusinessDict() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.BusinessDict
}

func (c *MemoryCache) GetAbsurdKeywords() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	words := make([]string, len(c.AbsurdWords))
	copy(words, c.AbsurdWords)
	return words
}
