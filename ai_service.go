package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

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

func getSQLFromAI_Groq(userPrompt string) (string, error) {
	apiKey := os.Getenv("GROQ_API_KEY")
	if apiKey == "" {
		return "", errors.New("GROQ_API_KEY tidak ditemukan di environment")
	}

	schemaDDL := `
-- Tabel utama
CREATE TABLE nasabah (
    id_nasabah VARCHAR(20) PRIMARY KEY, -- CIF
    nama_lengkap VARCHAR(150) NOT NULL,
	alamat TEXT NULL,
	tanggal_lahir DATE NOT NULL,
    id_tipe_nasabah SMALLINT NOT NULL -- Kunci ke master_tipe_nasabah
);
CREATE TABLE rekening (
    id_rekening VARCHAR(20) PRIMARY KEY, -- Nomor Rekening
    id_nasabah VARCHAR(20) NOT NULL, -- Kunci ke nasabah(id_nasabah)
    id_jenis_rekening SMALLINT NOT NULL, -- Kunci ke master_jenis_rekening (PENTING UNTUK FILTER)
    id_status_rekening SMALLINT NOT NULL -- Kunci ke master_status_rekening (PENTING: status 1 = 'aktif')
);
CREATE TABLE transaksi (
    id_transaksi BIGSERIAL PRIMARY KEY,
    waktu_transaksi TIMESTAMP WITH TIME ZONE NOT NULL,
	id_tipe_transaksi INT NOT NULL, -- Kunci ke master_tipe_transaksi
    deskripsi VARCHAR(255)
);
CREATE TABLE jurnal_transaksi (
    id_jurnal BIGSERIAL PRIMARY KEY,
    id_transaksi BIGINT NOT NULL,
    id_rekening VARCHAR(20) NOT NULL,
    tipe_dk tipe_dk NOT NULL, -- ENUM ('DEBIT', 'KREDIT')
    jumlah NUMERIC(19, 2) NOT NULL
);

-- Tabel Master (Konteks Bisnis)
CREATE TABLE master_jenis_rekening (
    id_jenis_rekening SMALLSERIAL PRIMARY KEY,
    nama_jenis VARCHAR(50) NOT NULL -- PENTING: Ini bisa 'tabungan', 'giro', 'deposito berjangka', 'rekening valas'
);
CREATE TABLE master_status_rekening (
    id_status_rekening SMALLSERIAL PRIMARY KEY,
    nama_status VARCHAR(50) NOT NULL -- PENTING: 'aktif' (id=1) 'tidak aktif' (id=2) ditutup (id=3)
);
CREATE TABLE master_tipe_nasabah (
    id_tipe_nasabah SMALLSERIAL PRIMARY KEY,
    nama_tipe VARCHAR(50) NOT NULL -- PENTING: Ini bisa 'perseorangan', 'perusahaan', dll.
);
CREATE TABLE master_tipe_transaksi (
    id_tipe_transaksi SMALLSERIAL PRIMARY KEY,
    nama_transaksi VARCHAR(50) NOT NULL -- PENTING: Ini bisa 'setoran', 'penarikan', 'transfer', dll.
);
	`
	examples := `
-- CONTOH 1 (Total Saldo Gabungan)
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

-- CONTOH 2 (Mutasi Spesifik)
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

-- CONTOH 3 (Saldo Spesifik per Jenis Rekening)
-- Pertanyaan Pengguna: "siapa nasabah dengan saldo TABUNGAN terbesar?"
-- Query SQL:
SELECT n.nama_lengkap, SUM(CASE WHEN jt.tipe_dk = 'KREDIT' THEN jt.jumlah ELSE -jt.jumlah END) AS total_saldo_tabungan
FROM jurnal_transaksi jt
JOIN rekening r ON jt.id_rekening = r.id_rekening
JOIN nasabah n ON r.id_nasabah = n.id_nasabah
JOIN master_jenis_rekening mjr ON r.id_jenis_rekening = mjr.id_jenis_rekening
WHERE r.id_status_rekening = 1 AND mjr.nama_jenis = 'tabungan' -- Filter spesifik
GROUP BY n.id_nasabah, n.nama_lengkap
ORDER BY total_saldo_tabungan DESC
LIMIT 1;

-- CONTOH 4 (Filter Tanggal Relatif)
-- Pertanyaan Pengguna: "tampilkan semua transaksi bulan lalu"
-- (Asumsikan HARI INI)
-- Query SQL:
SELECT t.waktu_transaksi, t.deskripsi
FROM transaksi t
WHERE t.waktu_transaksi >= DATE_TRUNC('month', CURRENT_DATE - INTERVAL '1 month')
  AND t.waktu_transaksi < DATE_TRUNC('month', CURRENT_DATE)
ORDER BY t.waktu_transaksi DESC;

-- CONTOH 5 (Agregasi COUNT)
-- Pertanyaan Pengguna: "berapa jumlah transaksi Budi Santoso (CIF00001)?"
-- Query SQL:
SELECT COUNT(DISTINCT t.id_transaksi) AS jumlah_transaksi
FROM transaksi t
JOIN jurnal_transaksi jt ON t.id_transaksi = jt.id_transaksi
JOIN rekening r ON jt.id_rekening = r.id_rekening
WHERE r.id_nasabah = 'CIF00001';
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
6. Selalu prioritaskan rekening yang 'id_status_rekening = 1' (Aktif) kecuali diminta sebaliknya.
7.Jika user menyebut jenis rekening (seperti 'tabungan', 'giro'), kamu WAJIB JOIN ke 'master_jenis_rekening' dan filter berdasarkan 'nama_jenis' seperti di CONTOH 3.
8.Jika user TIDAK menyebut jenis rekening, berikan TOTAL saldo gabungan seperti di CONTOH 1.

Pertanyaan Pengguna: "%s"

Query SQL:`, schemaDDL, examples, userPrompt)

	groqReqBody := GroqRequest{
		Model: "openai/gpt-oss-120b",
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
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("gagal kirim request ke Groq: %w", err)
	}
	defer resp.Body.Close()

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

	if len(groqResp.Choices) == 0 {
		return "", errors.New("AI tidak memberikan balasan")
	}

	sqlQuery := groqResp.Choices[0].Message.Content

	sqlQuery = strings.TrimSpace(sqlQuery)
	sqlQuery = strings.TrimPrefix(sqlQuery, "```sql")
	sqlQuery = strings.TrimSuffix(sqlQuery, "```")
	sqlQuery = strings.TrimSpace(sqlQuery)

	log.Println("SQL dari AI (Groq):", sqlQuery)
	return sqlQuery, nil
}

type OllamaResponse struct {
	Response string `json:"response"`
}

func getSQLFromAI_Ollama(userPrompt string) (string, error) {
	log.Println("Mencoba AI Lokal (Ollama)...")

	schemaDDL := `
-- Tabel utama
CREATE TABLE nasabah (
    id_nasabah VARCHAR(20) PRIMARY KEY, -- CIF
    nama_lengkap VARCHAR(150) NOT NULL,
	alamat TEXT NULL,
	tanggal_lahir DATE NOT NULL,
    id_tipe_nasabah SMALLINT NOT NULL -- Kunci ke master_tipe_nasabah
);
CREATE TABLE rekening (
    id_rekening VARCHAR(20) PRIMARY KEY, -- Nomor Rekening
    id_nasabah VARCHAR(20) NOT NULL, -- Kunci ke nasabah(id_nasabah)
    id_jenis_rekening SMALLINT NOT NULL, -- Kunci ke master_jenis_rekening (PENTING UNTUK FILTER)
    id_status_rekening SMALLINT NOT NULL -- Kunci ke master_status_rekening (PENTING: status 1 = 'aktif')
);
CREATE TABLE transaksi (
    id_transaksi BIGSERIAL PRIMARY KEY,
    waktu_transaksi TIMESTAMP WITH TIME ZONE NOT NULL,
	id_tipe_transaksi INT NOT NULL, -- Kunci ke master_tipe_transaksi
    deskripsi VARCHAR(255)
);
CREATE TABLE jurnal_transaksi (
    id_jurnal BIGSERIAL PRIMARY KEY,
    id_transaksi BIGINT NOT NULL,
    id_rekening VARCHAR(20) NOT NULL,
    tipe_dk tipe_dk NOT NULL, -- ENUM ('DEBIT', 'KREDIT')
    jumlah NUMERIC(19, 2) NOT NULL
);

-- Tabel Master (Konteks Bisnis)
CREATE TABLE master_jenis_rekening (
    id_jenis_rekening SMALLSERIAL PRIMARY KEY,
    nama_jenis VARCHAR(50) NOT NULL -- PENTING: Ini bisa 'tabungan', 'giro', 'deposito berjangka', 'rekening valas'
);
CREATE TABLE master_status_rekening (
    id_status_rekening SMALLSERIAL PRIMARY KEY,
    nama_status VARCHAR(50) NOT NULL -- PENTING: 'aktif' (id=1) 'tidak aktif' (id=2) ditutup (id=3)
);
CREATE TABLE master_tipe_nasabah (
    id_tipe_nasabah SMALLSERIAL PRIMARY KEY,
    nama_tipe VARCHAR(50) NOT NULL -- PENTING: Ini bisa 'perseorangan', 'perusahaan', dll.
);
CREATE TABLE master_tipe_transaksi (
    id_tipe_transaksi SMALLSERIAL PRIMARY KEY,
    nama_transaksi VARCHAR(50) NOT NULL -- PENTING: Ini bisa 'setoran', 'penarikan', 'transfer', dll.
);
	`
	examples := `
-- CONTOH 1 (Total Saldo Gabungan)
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

-- CONTOH 2 (Mutasi Spesifik)
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

-- CONTOH 3 (Saldo Spesifik per Jenis Rekening)
-- Pertanyaan Pengguna: "siapa nasabah dengan saldo TABUNGAN terbesar?"
-- Query SQL:
SELECT n.nama_lengkap, SUM(CASE WHEN jt.tipe_dk = 'KREDIT' THEN jt.jumlah ELSE -jt.jumlah END) AS total_saldo_tabungan
FROM jurnal_transaksi jt
JOIN rekening r ON jt.id_rekening = r.id_rekening
JOIN nasabah n ON r.id_nasabah = n.id_nasabah
JOIN master_jenis_rekening mjr ON r.id_jenis_rekening = mjr.id_jenis_rekening
WHERE r.id_status_rekening = 1 AND mjr.nama_jenis = 'tabungan' -- Filter spesifik
GROUP BY n.id_nasabah, n.nama_lengkap
ORDER BY total_saldo_tabungan DESC
LIMIT 1;

-- CONTOH 4 (Filter Tanggal Relatif)
-- Pertanyaan Pengguna: "tampilkan semua transaksi bulan lalu"
-- (Asumsikan HARI INI)
-- Query SQL:
SELECT t.waktu_transaksi, t.deskripsi
FROM transaksi t
WHERE t.waktu_transaksi >= DATE_TRUNC('month', CURRENT_DATE - INTERVAL '1 month')
  AND t.waktu_transaksi < DATE_TRUNC('month', CURRENT_DATE)
ORDER BY t.waktu_transaksi DESC;

-- CONTOH 5 (Agregasi COUNT)
-- Pertanyaan Pengguna: "berapa jumlah transaksi Budi Santoso (CIF00001)?"
-- Query SQL:
SELECT COUNT(DISTINCT t.id_transaksi) AS jumlah_transaksi
FROM transaksi t
JOIN jurnal_transaksi jt ON t.id_transaksi = jt.id_transaksi
JOIN rekening r ON jt.id_rekening = r.id_rekening
WHERE r.id_nasabah = 'CIF00001';
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
6. Selalu prioritaskan rekening yang 'id_status_rekening = 1' (Aktif) kecuali diminta sebaliknya.
7.Jika user menyebut jenis rekening (seperti 'tabungan', 'giro'), kamu WAJIB JOIN ke 'master_jenis_rekening' dan filter berdasarkan 'nama_jenis' seperti di CONTOH 3.
8.Jika user TIDAK menyebut jenis rekening, berikan TOTAL saldo gabungan seperti di CONTOH 1.

Pertanyaan Pengguna: "%s"

Query SQL:`, schemaDDL, examples, userPrompt)

	ollamaReqBody := map[string]interface{}{
		"model":  "gemma3:1b",
		"prompt": finalPrompt,
		"stream": false,
		"options": map[string]interface{}{
			"temperature": 0.0,
		},
	}

	jsonBody, err := json.Marshal(ollamaReqBody)
	if err != nil {
		return "", fmt.Errorf("gagal encode body Ollama: %w", err)
	}

	client := &http.Client{Timeout: 15 * time.Second}

	req, err := http.NewRequest("POST", "http://localhost:11434/api/generate", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("gagal buat request ke Ollama: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("gagal kirim request ke Ollama: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("gagal baca balasan Ollama: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Ollama merespon dengan error: %s", string(respBody))
	}

	var ollamaResp OllamaResponse
	if err := json.Unmarshal(respBody, &ollamaResp); err != nil {
		return "", fmt.Errorf("gagal decode balasan Ollama: %w", err)
	}

	if ollamaResp.Response == "" {
		return "", errors.New("Ollama mengembalikan balasan kosong")
	}

	sqlQuery := strings.TrimSpace(ollamaResp.Response)
	sqlQuery = strings.TrimPrefix(sqlQuery, "```sql")
	sqlQuery = strings.TrimSuffix(sqlQuery, "```")
	sqlQuery = strings.TrimSpace(sqlQuery)

	log.Println("SQL dari AI (Ollama):", sqlQuery)
	return sqlQuery, nil
}

func GetSQLFromAI(userPrompt string) (string, error) {

	sql, err := getSQLFromAI_Ollama(userPrompt)

	if err == nil {
		log.Println("Sukses menggunakan AI Lokal (Ollama).")
		return sql, nil
	}

	log.Printf("PERINGATAN: Gagal panggil Ollama lokal (%v). Fallback ke Groq Cloud...", err)

	return getSQLFromAI_Groq(userPrompt)
}
