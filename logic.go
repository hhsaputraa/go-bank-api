package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
)

type QueryResult struct {
	Columns []string        `json:"columns"`
	Rows    [][]interface{} `json:"rows"`
}

func GetSQL(userPrompt string) (AISqlResponse, error) {
	log.Println("Memanggil AI Service (dengan semantic cache)...")

	aiResp, err := getSQLFromAI_Groq(userPrompt)
	if err != nil {
		return AISqlResponse{}, err
	}
	if aiResp.SQL == "" && !aiResp.IsAmbiguous {
		return AISqlResponse{}, errors.New("AI tidak mengembalikan query SQL.")
	}

	return aiResp, nil
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

	// 1. Sanitasi & Validasi Keamanan
	// Ubah ke uppercase untuk pengecekan keyword, tapi query asli tetap disimpan
	checkQuery := strings.ToUpper(query)
	checkQuery = strings.TrimSpace(checkQuery)

	// Validasi Awal: Harus dimulai dengan SELECT atau WITH (untuk CTE)
	if !strings.HasPrefix(checkQuery, "SELECT") && !strings.HasPrefix(checkQuery, "WITH") {
		return result, fmt.Errorf("KEAMANAN: Hanya query SELECT yang diizinkan. Query Anda: %s", query)
	}

	query = strings.TrimSpace(query)
	query = strings.TrimRight(query, ";")

	// Daftar kata kunci berbahaya yang mutlak dilarang
	// Perhatikan spasi di akhir ("DROP ") agar tidak salah tangkap kata seperti "DROPBOX" (jika ada)
	forbidden := []string{
		"DROP ", "DELETE ", "UPDATE ", "INSERT ", "TRUNCATE ",
		"ALTER ", "GRANT ", "REVOKE ", "CREATE ", "MERGE ", "RENAME ",
		"ALL_TABS", "ALL_TABLES", "USER_TABLES", "DBA_TABLES", "ALL_VIEWS",
		"DBA_VIEWS", "ALL_SOURCE", "USER_SOURCE","ALL_USERS", "DBA_USERS",
		"SYS", "SYSTEM.", "V$", "GV$",
	}

	for _, word := range forbidden {
		if strings.Contains(checkQuery, word) {
			return result, fmt.Errorf("KEAMANAN: Ditemukan kata kunci terlarang '%s'", word)
		}
	}

	// 2. Setup Context Timeout
	timeout := 10 * time.Second
	if AppConfig != nil {
		timeout = AppConfig.QueryTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 3. Memulai Transaksi (Tanpa Mode ReadOnly)
	// Masalah sebelumnya: Driver Oracle/Go 10g menolak `ReadOnly: true`.
	// Solusi: Gunakan `nil` untuk options. Keamanan dijaga oleh validasi di atas.
	tx, err := DbInstance.BeginTx(ctx, nil)
	if err != nil {
		return result, fmt.Errorf("gagal memulai transaksi database: %w", err)
	}
	// Defer Rollback wajib ada agar koneksi dikembalikan ke pool dan tidak ada perubahan yang di-commit
	defer tx.Rollback()

	// 4. Eksekusi Query
	rows, err := tx.QueryContext(ctx, query, params...)
	if err != nil {
		log.Printf("Error eksekusi query SQL: %v. Query: %s", err, query)
		return result, fmt.Errorf("gagal mengeksekusi query SQL (Pastikan syntax Oracle 10g valid)")
	}
	defer rows.Close()

	// 5. Ambil Metadata Kolom
	columns, err := rows.Columns()
	if err != nil {
		return result, fmt.Errorf("gagal membaca kolom: %w", err)
	}
	result.Columns = columns
	result.Rows = make([][]interface{}, 0)

	// 6. Scanning Data Dinamis
	// Kita harus menyiapkan slice pointer interface{} karena kita tidak tahu tipe datanya di awal
	for rows.Next() {
		rowValues := make([]interface{}, len(columns))
		rowScanners := make([]interface{}, len(columns))

		for i := range rowValues {
			rowScanners[i] = &rowValues[i]
		}

		if err := rows.Scan(rowScanners...); err != nil {
			return result, fmt.Errorf("gagal scanning baris data: %w", err)
		}

		// (Opsional) Normalisasi tipe data khusus Oracle jika perlu
		// Misal: Mengubah []byte menjadi string jika driver mengembalikan raw bytes
		for i, val := range rowValues {
			if b, ok := val.([]byte); ok {
				rowValues[i] = string(b)
			}
		}

		result.Rows = append(result.Rows, rowValues)
	}

	// Cek error setelah loop selesai (best practice)
	if err = rows.Err(); err != nil {
		return result, fmt.Errorf("error saat iterasi baris: %w", err)
	}

	return result, nil
}
