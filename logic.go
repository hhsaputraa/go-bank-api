package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

type QueryResult struct {
	Columns []string        `json:"columns"`
	Rows    [][]interface{} `json:"rows"`
}

const CACHE_FILE = "cache.json"

var queryCache = make(map[string]string)

var cacheMutex = &sync.RWMutex{}

func LoadCache() error {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	file, err := os.ReadFile(CACHE_FILE)
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("File cache.json tidak ditemukan, cache baru akan dibuat.")
			queryCache = make(map[string]string)
			return nil
		}
		return err
	}

	err = json.Unmarshal(file, &queryCache)
	if err != nil {
		log.Printf("PERINGATAN: Gagal parse cache.json, cache baru akan dibuat. Error: %v", err)
		queryCache = make(map[string]string)
	}
	return nil
}

func saveCache() error {
	file, err := json.MarshalIndent(queryCache, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(CACHE_FILE, file, 0644)
}

func GetSQL(userPrompt string) (string, error) {
	cacheMutex.RLock()
	sql, found := queryCache[userPrompt]
	cacheMutex.RUnlock()

	if found {
		log.Println("CACHE HIT! Menggunakan SQL dari cache.json.")
		return sql, nil
	}

	log.Println("CACHE MISS. Memanggil Groq AI...")
	newSQL, err := getSQLFromAI_Groq(userPrompt)
	if err != nil {
		return "", err
	}
	if newSQL == "" {
		return "", errors.New("AI tidak mengembalikan query")
	}

	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	queryCache[userPrompt] = newSQL

	if err := saveCache(); err != nil {
		log.Printf("PERINGATAN: Gagal menyimpan cache ke file: %v", err)
	}

	return newSQL, nil
}

func BuildDynamicQuery(req QueryRequest) (string, []interface{}, error) {
	var query strings.Builder
	var params []interface{}

	if req.Laporan == "daftar_nasabah" {
		query.WriteString("SELECT n.id_nasabah, n.nama_lengkap, SUM(CASE WHEN jt.tipe_dk = 'KREDIT' THEN jt.jumlah ELSE -jt.jumlah END) AS total_saldo ")
		query.WriteString("FROM jurnal_transaksi jt ")
		query.WriteString("JOIN rekening r ON jt.id_rekening = r.id_rekening ")
		query.WriteString("JOIN nasabah n ON r.id_nasabah = n.id_nasabah ")
		query.WriteString("JOIN transaksi t ON jt.id_transaksi = t.id_transaksi ")
		query.WriteString("WHERE r.id_status_rekening = 1 ")
		switch req.Periode {
		case "3_bulan":
			query.WriteString(fmt.Sprintf("AND t.waktu_transaksi >= $%d ", len(params)+1))
			params = append(params, time.Now().AddDate(0, -3, 0))
		case "hari_ini":
			query.WriteString(fmt.Sprintf("AND t.waktu_transaksi >= $%d ", len(params)+1))
			params = append(params, time.Now().Truncate(24*time.Hour))
		case "semua_waktu":
		default:
			return "", nil, errors.New("periode tidak valid")
		}

		query.WriteString("GROUP BY n.id_nasabah, n.nama_lengkap ")
		query.WriteString("ORDER BY total_saldo DESC ")
		return query.String(), params, nil
	}

	switch req.Laporan {
	case "saldo":
		query.WriteString("SELECT SUM(CASE WHEN jt.tipe_dk = 'KREDIT' THEN jt.jumlah ELSE -jt.jumlah END) AS saldo_akhir ")
	case "mutasi":
		query.WriteString("SELECT jt.id_rekening, t.waktu_transaksi, t.deskripsi, ")
		query.WriteString("CASE WHEN jt.tipe_dk = 'DEBIT' THEN jt.jumlah ELSE 0 END AS debit, ")
		query.WriteString("CASE WHEN jt.tipe_dk = 'KREDIT' THEN jt.jumlah ELSE 0 END AS kredit, ")
		query.WriteString("SUM(CASE WHEN jt.tipe_dk = 'KREDIT' THEN jt.jumlah ELSE -jt.jumlah END) OVER (PARTITION BY jt.id_rekening ORDER BY t.waktu_transaksi, t.id_transaksi) AS saldo_akhir ")
	default:
		return "", nil, errors.New("tipe laporan tidak valid")
	}

	query.WriteString("FROM jurnal_transaksi jt ")
	query.WriteString("JOIN transaksi t ON jt.id_transaksi = t.id_transaksi ")

	if req.Target == "nasabah" {
		query.WriteString("JOIN rekening r ON jt.id_rekening = r.id_rekening ")
		query.WriteString("JOIN nasabah n ON r.id_nasabah = n.id_nasabah ")
	}

	query.WriteString("WHERE 1=1 ")
	switch req.Target {
	case "rekening":
		query.WriteString(fmt.Sprintf("AND jt.id_rekening = $%d ", len(params)+1))
		params = append(params, req.ID)
	case "nasabah":
		query.WriteString(fmt.Sprintf("AND n.id_nasabah = $%d ", len(params)+1))
		params = append(params, req.ID)
	default:
		return "", nil, errors.New("target tidak valid")
	}

	switch req.Periode {
	case "3_bulan":
		query.WriteString(fmt.Sprintf("AND t.waktu_transaksi >= $%d ", len(params)+1))
		params = append(params, time.Now().AddDate(0, -3, 0))
	case "hari_ini":
		query.WriteString(fmt.Sprintf("AND t.waktu_transaksi >= $%d ", len(params)+1))
		params = append(params, time.Now().Truncate(24*time.Hour))
	case "semua_waktu":
	default:
		return "", nil, errors.New("periode tidak valid")
	}
	switch req.Laporan {
	case "saldo":
	case "mutasi":
		query.WriteString("ORDER BY jt.id_rekening, t.waktu_transaksi, t.id_transaksi")
	}
	return query.String(), params, nil
}

func ExecuteDynamicQuery(query string, params []interface{}) (QueryResult, error) {

	var result QueryResult

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := DbInstance.QueryContext(ctx, query, params...)
	if err != nil {
		log.Printf("Error eksekusi query: %v. Query: %s", err, query)
		return result, fmt.Errorf("gagal mengeksekusi query: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return result, err
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
			return result, err
		}

		result.Rows = append(result.Rows, rowValues)
	}

	return result, nil
}
