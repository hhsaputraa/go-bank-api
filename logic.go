package main

import (
	"context"
	"database/sql"
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
	if aiResp.SQL == "" {
		return AISqlResponse{}, errors.New("AI tidak mengembalikan query")
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
	cleanQuery := strings.TrimSpace(strings.ToUpper(query))

	if !strings.HasPrefix(cleanQuery, "SELECT") && !strings.HasPrefix(cleanQuery, "WITH") {
		return result, fmt.Errorf("KEAMANAN: Hanya query SELECT yang diizinkan. Query Anda: %s", query)
	}

	forbidden := []string{"DROP ", "DELETE ", "UPDATE ", "INSERT ", "TRUNCATE ", "ALTER ", "GRANT ", "REVOKE "}
	for _, word := range forbidden {
		if strings.Contains(cleanQuery, word) {
			return result, fmt.Errorf("KEAMANAN: Ditemukan kata kunci terlarang '%s'", word)
		}
	}

	// 3. Cek Multiple Statements (mencegah "SELECT ...; DROP ...")
	if strings.Contains(query, ";") {
		// Opsional: Bisa ditolak, atau dibiarkan jika yakin driver pgx menolak multiple statement
		// Untuk keamanan maksimal, tolak jika ada titik koma di tengah
		// return result, fmt.Errorf("KEAMANAN: Query chaining (titik koma) tidak diizinkan.")
	}

	timeout := 10 * time.Second
	if AppConfig != nil {
		timeout = AppConfig.QueryTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	txOptions := &sql.TxOptions{
		Isolation: sql.LevelDefault,
		ReadOnly:  true, // <--- INI KUNCINYA! Database akan menolak write apapun.
	}

	tx, err := DbInstance.BeginTx(ctx, txOptions)
	if err != nil {
		return result, fmt.Errorf("gagal memulai transaksi read-only: %w", err)
	}
	// Selalu Rollback di akhir (karena kita cuma baca, tidak perlu Commit)
	defer tx.Rollback()

	// Eksekusi query menggunakan tx (bukan DbInstance langsung)
	rows, err := tx.QueryContext(ctx, query, params...)
	if err != nil {
		log.Printf("Error eksekusi query: %v. Query: %s", err, query)
		// Pesan error generik ke user agar tidak membocorkan struktur internal
		return result, fmt.Errorf("gagal mengeksekusi query (mungkin query tidak valid atau melanggar aturan read-only)")
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
