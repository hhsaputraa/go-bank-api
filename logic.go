package main

import (
	"bytes" // (BARU) Untuk request HTTP
	"context"
	"encoding/json" // (BARU) Untuk request HTTP
	"errors"
	"fmt"
	"io" // (BARU) Untuk request HTTP
	"log"
	"net/http" // (BARU) Untuk request HTTP
	"strings"
	"time"
)

const GROQ_API_KEY = ""

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

func ExecuteDynamicQuery(query string, params []interface{}) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rows, err := DbInstance.QueryContext(ctx, query, params...)
	if err != nil {
		log.Printf("Error eksekusi query: %v. Query: %s", err, query)
		return nil, fmt.Errorf("gagal mengeksekusi query: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()

	var results []map[string]interface{}

	for rows.Next() {
		rowMap := make(map[string]interface{})
		rowScanners := make([]interface{}, len(columns))
		for i := range columns {
			rowScanners[i] = new(interface{})
		}

		if err := rows.Scan(rowScanners...); err != nil {
			return nil, err
		}
		for i, colName := range columns {
			rowMap[colName] = *(rowScanners[i].(*interface{}))
		}
		results = append(results, rowMap)
	}

	return results, nil
}

type GroqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type GroqRequest struct {
	Model    string        `json:"model"`
	Messages []GroqMessage `json:"messages"`
}
type GroqResponse struct {
	Choices []struct {
		Message GroqMessage `json:"message"`
	} `json:"choices"`
}

// GetSQLFromAI sekarang memanggil Groq Cloud
func GetSQLFromAI(userPrompt string) (string, error) {

	schemaDDL := `
CREATE TABLE nasabah (
    id_nasabah VARCHAR(20) PRIMARY KEY, -- CIF
    nama_lengkap VARCHAR(150) NOT NULL,
    id_tipe_nasabah SMALLINT NOT NULL
);
CREATE TABLE rekening (
    id_rekening VARCHAR(20) PRIMARY KEY, -- Nomor Rekening
    id_nasabah VARCHAR(20) NOT NULL, -- CIF
    id_jenis_rekening SMALLINT NOT NULL,
    id_status_rekening SMALLINT NOT NULL
);
CREATE TABLE transaksi (
    id_transaksi BIGSERIAL PRIMARY KEY,
    waktu_transaksi TIMESTAMP WITH TIME ZONE NOT NULL,
    id_tipe_transaksi INT NOT NULL,
    deskripsi VARCHAR(255)
);
CREATE TABLE jurnal_transaksi (
    id_jurnal BIGSERIAL PRIMARY KEY,
    id_transaksi BIGINT NOT NULL,
    id_rekening VARCHAR(20) NOT NULL,
    tipe_dk tipe_dk NOT NULL, -- ENUM ('DEBIT', 'KREDIT')
    jumlah NUMERIC(19, 2) NOT NULL
);
-- Master tables
CREATE TABLE master_tipe_nasabah ( id_tipe_nasabah SMALLSERIAL PRIMARY KEY, nama_tipe VARCHAR(50) NOT NULL);
CREATE TABLE master_jenis_rekening ( id_jenis_rekening SMALLSERIAL PRIMARY KEY, nama_jenis VARCHAR(50) NOT NULL);
CREATE TABLE master_status_rekening ( id_status_rekening SMALLSERIAL PRIMARY KEY, nama_status VARCHAR(50) NOT NULL);
CREATE TABLE master_tipe_transaksi ( id_tipe_transaksi SERIAL PRIMARY KEY, kode_transaksi VARCHAR(10) NOT NULL, nama_transaksi VARCHAR(100) NOT NULL);
	`

	examples := `
-- CONTOH 1
-- Pertanyaan Pengguna: "siapa nasabah dengan saldo terbanyak?"
-- Query SQL:
SELECT n.nama_lengkap, SUM(CASE WHEN jt.tipe_dk = 'KREDIT' THEN jt.jumlah ELSE -jt.jumlah END) AS total_saldo
FROM jurnal_transaksi jt
JOIN rekening r ON jt.id_rekening = r.id_rekening
JOIN nasabah n ON r.id_nasabah = n.id_nasabah
WHERE r.id_status_rekening = 1 -- Hanya rekening aktif
GROUP BY n.id_nasabah, n.nama_lengkap
ORDER BY total_saldo DESC
LIMIT 1;

-- CONTOH 2
-- Pertanyaan Pengguna: "tampilkan mutasi rekening 110000001"
-- Query SQL:
SELECT jt.id_rekening, t.waktu_transaksi, t.deskripsi,
CASE WHEN jt.tipe_dk = 'DEBIT' THEN jt.jumlah ELSE 0 END AS debit,
CASE WHEN jt.tipe_dk = 'KREDIT' THEN jt.jumlah ELSE 0 END AS kredit,
SUM(CASE WHEN jt.tipe_dk = 'KREDIT' THEN jt.jumlah ELSE -jt.jumlah END) OVER (PARTITION BY jt.id_rekening ORDER BY t.waktu_transaksi, t.id_transaksi) AS saldo_akhir
FROM jurnal_transaksi jt
JOIN transaksi t ON jt.id_transaksi = t.id_transaksi
WHERE jt.id_rekening = '110000001'
ORDER BY jt.id_rekening, t.waktu_transaksi, t.id_transaksi;
	`
	finalPrompt := fmt.Sprintf(`
Anda adalah ahli SQL PostgreSQL. Diberikan skema berikut (dalam skema "bpr_supra"):
Berikut adalah beberapa CONTOH cara menjawab pertanyaan. Ikuti pola ini:
%s

Tugas Anda:
1. Jawab pertanyaan pengguna dengan menulis SATU query SQL PostgreSQL yang valid.
2. JANGAN pakai markdown (seperti backtick untuk format sql).
3. JANGAN tambahkan penjelasan atau basa-basi.
4. JANGAN gunakan parameter (seperti $1), langsung tulis nilainya jika ada (misal 'CIF00001').
5. Hanya berikan query SQL-nya saja.

Pertanyaan Pengguna: "%s"

Query SQL:`, schemaDDL, examples, userPrompt)

	// 3. Panggil Groq Cloud
	groqReqBody := GroqRequest{
		Model: "openai/gpt-oss-120b", // Model Llama 3 8B di Groq
		Messages: []GroqMessage{
			{Role: "user", Content: finalPrompt},
		},
	}

	jsonBody, err := json.Marshal(groqReqBody)
	if err != nil {
		return "", fmt.Errorf("gagal encode body Groq: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.groq.com/openai/v1/chat/completions", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("gagal buat request ke Groq: %w", err)
	}
	// (PENTING) Tambahkan Kunci API-mu di Header
	req.Header.Set("Authorization", "Bearer "+GROQ_API_KEY)
	req.Header.Set("Content-Type", "application/json")

	// Kirim request
	client := &http.Client{Timeout: 30 * time.Second} // Groq cepat, 30 detik cukup
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("gagal kirim request ke Groq: %w", err)
	}
	defer resp.Body.Close()

	// 4. Baca balasan dari Groq
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("gagal baca balasan Groq: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Groq merespon dengan error: %s", string(respBody))
	}

	var groqResp GroqResponse
	if err := json.Unmarshal(respBody, &groqResp); err != nil {
		return "", fmt.Errorf("gagal decode balasan Groq: %w", err)
	}

	// Cek jika balasannya kosong
	if len(groqResp.Choices) == 0 {
		return "", errors.New("AI tidak memberikan balasan")
	}

	// Ambil SQL-nya (formatnya beda dari Ollama)
	sqlQuery := groqResp.Choices[0].Message.Content

	// Bersihkan SQL (sama seperti sebelumnya)
	sqlQuery = strings.TrimSpace(sqlQuery)
	sqlQuery = strings.TrimPrefix(sqlQuery, "```sql")
	sqlQuery = strings.TrimSuffix(sqlQuery, "```")
	sqlQuery = strings.TrimSpace(sqlQuery)

	log.Println("SQL dari AI (Groq):", sqlQuery)
	return sqlQuery, nil
}
