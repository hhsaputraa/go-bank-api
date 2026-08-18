package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	ai "go-bank-api/ai"
	models "go-bank-api/models"
	utils "go-bank-api/utils"

	"github.com/google/uuid"
)

var uploadsDir = filepath.Join(".", "uploads")

func init() {
	os.MkdirAll(uploadsDir, os.ModePerm)
}

func HandleUploadSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteError(w, http.StatusMethodNotAllowed, "Metode tidak diizinkan", "METHOD_NOT_ALLOWED")
		return
	}

	// Enforce 10 MB max body size on multipart uploads
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Ukuran file melebihi batas 10 MB", "PAYLOAD_TOO_LARGE")
		return
	}
	file, handler, err := r.FormFile("file")
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Gagal membaca file dari request", err.Error())
		return
	}
	defer file.Close()

	// Server-side Extension Allowlist Check
	ext := strings.ToLower(filepath.Ext(handler.Filename))
	allowedExtensions := map[string]bool{
		".csv":  true,
		".xlsx": true,
		".xls":  true,
		".json": true,
		".pdf":  true,
	}
	if !allowedExtensions[ext] {
		utils.WriteError(w, http.StatusBadRequest, "Format file tidak diizinkan. Hanya .csv, .xlsx, .xls, .json, .pdf yang didukung.", "INVALID_FILE_TYPE")
		return
	}

	sessionID := uuid.New().String()
	targetPath := filepath.Join(uploadsDir, fmt.Sprintf("session_%s%s", sessionID, ext))

	out, err := os.Create(targetPath)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Gagal menyimpan file di server", err.Error())
		return
	}
	defer out.Close()

	_, err = io.Copy(out, file)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Gagal menyalin file", err.Error())
		return
	}

	// Return session ID
	resp := models.UploadSessionResponse{
		SessionID: sessionID,
		Columns:   []string{"ID_KANTOR", "JENIS_PINJAMAN", "BULAN", "NILAI_BUNGA"},
		Status:    "success",
	}
	utils.WriteJSON(w, http.StatusOK, resp)
}

func HandleChatSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteError(w, http.StatusMethodNotAllowed, "Metode tidak diizinkan", "METHOD_NOT_ALLOWED")
		return
	}

	// Bound JSON payload size
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req models.ChatSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Request body JSON tidak valid", err.Error())
		return
	}


	// Locate file
	var filePath string
	files, _ := filepath.Glob(filepath.Join(uploadsDir, fmt.Sprintf("session_%s.*", req.SessionID)))
	if len(files) == 0 {
		utils.WriteError(w, http.StatusNotFound, "Sesi file tidak ditemukan atau sudah kadaluarsa", "NOT_FOUND")
		return
	}
	filePath = files[0]

	// Convert filePath to absolute path for the Python subprocess
	absFilePath, err := filepath.Abs(filePath)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Gagal melacak path file absolut", err.Error())
		return
	}

	// Get melted columns dynamically from CSV header as fallback
	cols, err := ai.GetMeltedColumns(filePath)
	if err != nil {
		log.Printf("Gagal membaca header CSV: %v", err)
		cols = []string{"KODE_KANTOR", "KETERANGAN_JENIS_PINJAM", "BULAN", "NILAI_BUNGA"}
	}

	runnerDir := ai.GetPythonRunnerDir()
	dfInfo := ai.GetDataFrameInfo(absFilePath, cols)

	// Step 1: Tell LLM to write Pandas code with Semantic Intent & Value Mapping
	systemPrompt := fmt.Sprintf(`Anda adalah asisten analisis data pintar yang bertugas menganalisis pertanyaan pengguna dan merumuskan 1 baris kode Python Pandas untuk DataFrame 'df'.

INFORMASI DATASET DARI FILE PENGGUNA:
%s

LANGKAH 1: ANALISIS INTENT & KERELEVANAN KATA KUNCI (SEMANTIC VALUE MAPPING)
- Analisislah pertanyaan pengguna terhadap 'Contoh Nilai Unik per Kolom (Uniques)' di atas.
- Istilah pengguna MUNGKIN TIDAK SAMA PERSIS dengan nilai di CSV (contoh: pengguna menyebut 'kopi' / 'kredit kopi', tetapi di data tertulis 'KREDIT KOPI 2' atau 'KOP-2').
- Jika ditemukan kemiripan/kerelevanan, gunakan kata kunci dari data yang paling relevan dengan fungsi .astype(str).str.contains('KEYWORD_RELEVAN', case=False, na=False).

LANGKAH 2: ATURAN KODE PANDAS TAHAN BANTING & DINAMIS
1. PENCARIAN TEKS/KATEGORI (Fuzzy & Case-Insensitive):
   - WAJIB gunakan .astype(str).str.contains('KEYWORD', case=False, na=False) alih-alih perbandingan persis ==.
   - Contoh: df[df['JENIS_PINJAMAN'].astype(str).str.contains('KOPI', case=False, na=False)]
2. JIKA DITANYA "TERTINGGI / TERBESAR / TERKECIL DI CABANG/KATEGORI MANA":
   - WAJIB kembalikan seluruh baris/kolom terkait (seperti nama/kode cabang DAN nilai angkanya), JANGAN hanya mengembalikan angkanya saja.
   - Gunakan: result = df[filter].nlargest(1, 'NILAI_BUNGA') atau .sort_values(by='NILAI_BUNGA', ascending=False).head(1)
3. Kolom NUMERIK (int64/float64): JANGAN gunakan tanda kutip untuk filter angka (contoh: df[kolom] > 1000). Gunakan pd.to_numeric(df[kolom], errors='coerce') jika kalkulasi agregasi.

Aturan Penulisan Kode:
- Anda WAJIB menyimpan hasil kalkulasi akhir ke dalam variabel 'result'.
- Hanya tuliskan potongan kode Python Pandas saja tanpa markdown, penjelasan, atau komentar. Cukup satu baris kode saja.`, dfInfo)

	codeText, err := ai.CallOpenAILLM(systemPrompt, req.Message)
	if err != nil {
		log.Printf("Gagal memanggil LLM untuk kode: %v", err)
		utils.WriteError(w, http.StatusInternalServerError, "Gagal merumuskan perintah analisa", err.Error())
		return
	}

	pandasCode := ai.CleanPandasCode(codeText)
	log.Printf("Pandas code initial generated: %s", pandasCode)

	// Step 2: Execute python query_runner.py with Self-Correction retry loop
	runnerOut, finalCode, execErr := ai.ExecutePandasWithRetry(absFilePath, pandasCode, req.Message, dfInfo)

	// Fallback Mechanism Tier 1: Exec Default Summary Query if AI Self-Correction fails
	if execErr != nil {
		log.Printf("⚠️ Self-Correction Gagal total. Menjalankan Fallback Query Default...")
		fallbackCode := "result = df.head(10).to_dict(orient='records')"
		fallbackCmd := exec.Command("python", "query_runner.py", absFilePath, fallbackCode)
		fallbackCmd.Dir = runnerDir
		var fallbackStdout bytes.Buffer
		fallbackCmd.Stdout = &fallbackStdout

		if fbErr := fallbackCmd.Run(); fbErr == nil {
			var fbOut ai.RunnerOutput
			if json.Unmarshal(fallbackStdout.Bytes(), &fbOut) == nil && fbOut.Status == "success" {
				utils.WriteJSON(w, http.StatusOK, map[string]any{
					"status":  "success",
					"message": "Maaf, kalkulasi spesifik tidak dapat diproses secara langsung. Berikut pratinjau 10 data pertama dari file yang diunggah:",
					"result":  fbOut.Result,
					"code":    fallbackCode,
				})
				return
			}
		}

		// Fallback Mechanism Tier 2: User friendly error message
		utils.WriteError(w, http.StatusUnprocessableEntity, "PANDAS_EXECUTION_FAILED", "Sistem tidak dapat mengolah query pada file data ini. Silakan perjelas pertanyaan Anda atau periksa format file.")
		return
	}

	// Step 3: Summarize result using LLM with concise prompt
	summarizerSystemPrompt := `Anda adalah asisten analisis data perbankan yang to-the-point dan profesional.
Tugas Anda adalah menyajikan hasil kalkulasi angka secara ringkas, jelas, akurat, dan langsung menjawab pertanyaan pengguna.

Aturan Penting:
1. JAWAB LANGSUNG: Berikan angka hasil kalkulasi secara langsung di awal kalimat. Jangan bertele-tele atau membuat pembukaan/penutup yang panjang.
2. FORMAT RUPIAH: Selalu format nilai uang ke dalam Rupiah Indonesia yang lengkap dengan titik sebagai pemisah ribuan (contoh: Rp 37.766.662.538). Jangan gunakan notasi ilmiah.
3. TANPA BASA-BASI (NO FLUFF): Jangan membuat analisis teoretis yang berlebihan, saran bisnis, atau rekomendasi fiktif yang tidak diminta oleh pengguna. Cukup sampaikan fakta angka hasil kalkulasi.
4. MAKSIMAL 2 KALIMAT: Batasi respons Anda hanya untuk menjawab pertanyaan secara padat dan informatif.`

	summaryPrompt := fmt.Sprintf("Pertanyaan Pengguna: \"%s\"\nHasil Kalkulasi Program: %v\n\nJawablah pertanyaan tersebut menggunakan hasil kalkulasi yang diberikan dengan mengikuti aturan penting di atas.", req.Message, runnerOut.Result)
	summaryText, err := ai.CallOpenAILLM(summarizerSystemPrompt, summaryPrompt)
	if err != nil {
		log.Printf("Gagal membuat rangkuman LLM: %v", err)
		utils.WriteError(w, http.StatusInternalServerError, "Gagal merangkum jawaban", err.Error())
		return
	}

	summaryText = ai.StripThinkTags(summaryText)

	// Send final response
	respPayload := map[string]any{
		"status":  "success",
		"message": summaryText,
		"result":  runnerOut.Result,
		"code":    finalCode,
	}
	utils.WriteJSON(w, http.StatusOK, respPayload)
}
