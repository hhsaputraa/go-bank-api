package controllers

import (
	"bufio"
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
	"time"

	config "go-bank-api/config"
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

	r.ParseMultipartForm(10 << 20) // 10 MB limit
	file, handler, err := r.FormFile("file")
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Gagal membaca file dari request", err.Error())
		return
	}
	defer file.Close()

	sessionID := uuid.New().String()
	ext := filepath.Ext(handler.Filename)
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

type OpenAIRequestMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIRequest struct {
	Model    string                 `json:"model"`
	Messages []OpenAIRequestMessage `json:"messages"`
}

type OpenAIResponseChoice struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
}

type OpenAIResponse struct {
	Choices []OpenAIResponseChoice `json:"choices"`
}

func callLLM(systemPrompt, userPrompt string) (string, error) {
	reqBody := OpenAIRequest{
		Model: config.AppConfig.LLMModel,
		Messages: []OpenAIRequestMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", config.AppConfig.LLMBaseURL+"/chat/completions", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	if config.AppConfig.LLMAPIKey != "" && config.AppConfig.LLMAPIKey != "none" {
		req.Header.Set("Authorization", "Bearer "+config.AppConfig.LLMAPIKey)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("LLM API error status %d: %s", resp.StatusCode, string(respBytes))
	}

	var openAIResp OpenAIResponse
	if err := json.Unmarshal(respBytes, &openAIResp); err != nil {
		return "", err
	}

	if len(openAIResp.Choices) == 0 {
		return "", fmt.Errorf("empty choices from LLM")
	}

	return openAIResp.Choices[0].Message.Content, nil
}

func getMeltedColumns(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	firstLine, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return nil, err
	}

	firstLine = strings.TrimSpace(firstLine)
	if firstLine == "" {
		return []string{"BULAN", "NILAI_BUNGA"}, nil
	}

	// Detect delimiter
	delimiter := ","
	if strings.Contains(firstLine, ";") {
		delimiter = ";"
	}

	headers := strings.Split(firstLine, delimiter)
	months := map[string]bool{
		"JANUARI": true, "FEBRUARI": true, "MARET": true, "APRIL": true,
		"MEI": true, "JUNI": true, "JULI": true, "AGUSTUS": true,
		"SEPTEMBER": true, "OKTOBER": true, "NOVEMBER": true, "DESEMBER": true,
	}

	var idVars []string
	for _, h := range headers {
		h = strings.TrimSpace(h)
		hUpper := strings.ToUpper(h)
		if h != "" && !months[hUpper] {
			idVars = append(idVars, hUpper)
		}
	}

	return append(idVars, "BULAN", "NILAI_BUNGA"), nil
}

func HandleChatSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteError(w, http.StatusMethodNotAllowed, "Metode tidak diizinkan", "METHOD_NOT_ALLOWED")
		return
	}

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
	cols, err := getMeltedColumns(filePath)
	if err != nil {
		log.Printf("Gagal membaca header CSV: %v", err)
		cols = []string{"KODE_KANTOR", "KETERANGAN_JENIS_PINJAM", "BULAN", "NILAI_BUNGA"}
	}

	runnerDir := getPythonRunnerDir()

	// Get DataFrame info dynamically (dtypes, head, unique samples) by running Python runner with --info
	infoCmd := exec.Command("python", "query_runner.py", absFilePath, "--info")
	infoCmd.Dir = runnerDir
	var infoStdout, infoStderr bytes.Buffer
	infoCmd.Stdout = &infoStdout
	infoCmd.Stderr = &infoStderr

	var dfInfo string = ""
	if err := infoCmd.Run(); err != nil {
		log.Printf("Warning: Gagal mengambil info DataFrame: %v | stderr: %s", err, infoStderr.String())
		dfInfo = fmt.Sprintf("Kolom: %v", cols)
	} else {
		type InfoOutput struct {
			Status        string              `json:"status"`
			Dtypes        map[string]string   `json:"dtypes"`
			Head          []any               `json:"head"`
			UniqueSamples map[string][]string `json:"unique_samples"`
			Message       string              `json:"message"`
		}
		var infoOut InfoOutput
		if err := json.Unmarshal(infoStdout.Bytes(), &infoOut); err == nil && infoOut.Status == "success" {
			dtypesBytes, _ := json.Marshal(infoOut.Dtypes)
			headBytes, _ := json.Marshal(infoOut.Head)
			uniquesBytes, _ := json.Marshal(infoOut.UniqueSamples)
			dfInfo = fmt.Sprintf("\nTipe Data Kolom (dtypes):\n%s\n\nPreview 3 Baris Pertama (head):\n%s\n\nContoh Nilai Unik per Kolom (Uniques):\n%s", string(dtypesBytes), string(headBytes), string(uniquesBytes))
		} else {
			dfInfo = fmt.Sprintf("Kolom: %v", cols)
		}
	}

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

	codeText, err := callLLM(systemPrompt, req.Message)
	if err != nil {
		log.Printf("Gagal memanggil LLM untuk kode: %v", err)
		utils.WriteError(w, http.StatusInternalServerError, "Gagal merumuskan perintah analisa", err.Error())
		return
	}

	// Extract & clean code block
	pandasCode := cleanPandasCode(codeText)
	log.Printf("Pandas code initial generated: %s", pandasCode)

	// Step 2: Execute python query_runner.py with Self-Correction retry loop
	runnerOut, finalCode, execErr := executePandasWithRetry(absFilePath, pandasCode, req.Message, dfInfo)

	// Fallback Mechanism Tier 1: Exec Default Summary Query if AI Self-Correction fails
	if execErr != nil {
		log.Printf("⚠️ Self-Correction Gagal total. Menjalankan Fallback Query Default...")
		fallbackCode := "result = df.head(10).to_dict(orient='records')"
		fallbackCmd := exec.Command("python", "query_runner.py", absFilePath, fallbackCode)
		fallbackCmd.Dir = runnerDir
		var fallbackStdout bytes.Buffer
		fallbackCmd.Stdout = &fallbackStdout

		if fbErr := fallbackCmd.Run(); fbErr == nil {
			var fbOut RunnerOutput
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

		// Fallback Mechanism Tier 2: User friendly error message (No raw 500 error)
		utils.WriteError(w, http.StatusUnprocessableEntity, "PANDAS_EXECUTION_FAILED", "Sistem tidak dapat mengolah query pada file data ini. Silakan perjelas pertanyaan Anda atau periksa format file.")
		return
	}

	// Step 3: Summarize result using LLM with concise and to-the-point prompt
	summarizerSystemPrompt := `Anda adalah asisten analisis data perbankan yang to-the-point dan profesional.
Tugas Anda adalah menyajikan hasil kalkulasi angka secara ringkas, jelas, akurat, dan langsung menjawab pertanyaan pengguna.

Aturan Penting:
1. JAWAB LANGSUNG: Berikan angka hasil kalkulasi secara langsung di awal kalimat. Jangan bertele-tele atau membuat pembukaan/penutup yang panjang.
2. FORMAT RUPIAH: Selalu format nilai uang ke dalam Rupiah Indonesia yang lengkap dengan titik sebagai pemisah ribuan (contoh: Rp 37.766.662.538). Jangan gunakan notasi ilmiah.
3. TANPA BASA-BASI (NO FLUFF): Jangan membuat analisis teoretis yang berlebihan, saran bisnis, atau rekomendasi fiktif yang tidak diminta oleh pengguna. Cukup sampaikan fakta angka hasil kalkulasi.
4. MAKSIMAL 2 KALIMAT: Batasi respons Anda hanya untuk menjawab pertanyaan secara padat dan informatif.`

	summaryPrompt := fmt.Sprintf("Pertanyaan Pengguna: \"%s\"\nHasil Kalkulasi Program: %v\n\nJawablah pertanyaan tersebut menggunakan hasil kalkulasi yang diberikan dengan mengikuti aturan penting di atas.", req.Message, runnerOut.Result)
	summaryText, err := callLLM(summarizerSystemPrompt, summaryPrompt)
	if err != nil {
		log.Printf("Gagal membuat rangkuman LLM: %v", err)
		utils.WriteError(w, http.StatusInternalServerError, "Gagal merangkum jawaban", err.Error())
		return
	}

	summaryText = stripThinkTags(summaryText)

	// Send final response
	respPayload := map[string]any{
		"status":  "success",
		"message": summaryText,
		"result":  runnerOut.Result,
		"code":    finalCode,
	}
	utils.WriteJSON(w, http.StatusOK, respPayload)
}

func getPythonRunnerDir() string {
	if dir := os.Getenv("PYTHON_RUNNER_DIR"); dir != "" {
		return dir
	}
	localScriptsPath := filepath.Join(".", "scripts")
	if _, err := os.Stat(filepath.Join(localScriptsPath, "query_runner.py")); err == nil {
		return localScriptsPath
	}
	defaultPath := filepath.Join("c:", "Users", "Keamanan Saber", "Documents", "dataanalis")
	if _, err := os.Stat(defaultPath); err == nil {
		return defaultPath
	}
	return "."
}

func extractPythonCode(text string) string {
	if !strings.Contains(text, "```") {
		return strings.TrimSpace(text)
	}
	parts := strings.Split(text, "```")
	for _, part := range parts {
		if strings.HasPrefix(part, "python") {
			return strings.TrimSpace(strings.TrimPrefix(part, "python"))
		}
		if strings.HasPrefix(part, "py") {
			return strings.TrimSpace(strings.TrimPrefix(part, "py"))
		}
	}
	// Fallback to second block
	if len(parts) >= 2 {
		return strings.TrimSpace(parts[1])
	}
	return strings.TrimSpace(text)
}

func stripThinkTags(text string) string {
	for {
		start := strings.Index(text, "<think>")
		if start == -1 {
			break
		}
		end := strings.Index(text, "</think>")
		if end == -1 {
			text = text[:start]
			break
		}
		text = text[:start] + text[end+len("</think>"):]
	}
	return strings.TrimSpace(text)
}

func cleanPandasCode(text string) string {
	cleaned := stripThinkTags(text)
	extracted := extractPythonCode(cleaned)

	lines := strings.Split(extracted, "\n")
	for _, line := range lines {
		lineTrim := strings.TrimSpace(line)
		if strings.HasPrefix(lineTrim, "result =") || strings.HasPrefix(lineTrim, "result=") {
			return lineTrim
		}
	}
	return strings.TrimSpace(extracted)
}

type RunnerOutput struct {
	Status  string      `json:"status"`
	Result  interface{} `json:"result,omitempty"`
	Message string      `json:"message,omitempty"`
}

func executePandasWithRetry(absFilePath string, initialCode string, userPrompt string, dfInfo string) (RunnerOutput, string, error) {
	currentCode := cleanPandasCode(initialCode)
	maxRetries := 2
	runnerDir := getPythonRunnerDir()

	for attempt := 0; attempt <= maxRetries; attempt++ {
		cmd := exec.Command("python", "query_runner.py", absFilePath, currentCode)
		cmd.Dir = runnerDir
		var stdoutBuf, stderrBuf bytes.Buffer
		cmd.Stdout = &stdoutBuf
		cmd.Stderr = &stderrBuf

		err := cmd.Run()
		var runnerOut RunnerOutput
		jsonErr := json.Unmarshal(stdoutBuf.Bytes(), &runnerOut)

		if err == nil && jsonErr == nil && runnerOut.Status == "success" {
			log.Printf("[INFO] Eksekusi Pandas Sukses (Attempt %d): %s", attempt+1, currentCode)
			return runnerOut, currentCode, nil
		}

		errMsg := stderrBuf.String()
		if runnerOut.Message != "" {
			errMsg = runnerOut.Message
		}

		log.Printf("⚠️ Attempt %d Gagal | Error: %s | Code: %s", attempt+1, errMsg, currentCode)

		if attempt == maxRetries {
			break
		}

		repairSystemPrompt := fmt.Sprintf(`Kode Python Pandas yang Anda buat sebelumnya mengalami ERROR saat dieksekusi:
Kode Sebelumnya: %s
Pesan Error Python: %s

Informasi Struktur DataFrame:
%s

Tolong perbaiki kode Pandas tersebut.
Aturan:
- Simpan hasil akhir ke variabel 'result'.
- Pastikan kodenya tahan banting (gunakan .astype(str).str.strip().str.upper() untuk teks, dan pd.to_numeric() untuk numerik).
- Hanya tulis 1 baris kode murni tanpa markdown, penjelasan, atau komentar.`, currentCode, errMsg, dfInfo)

		repairedCodeRaw, llmErr := callLLM(repairSystemPrompt, userPrompt)
		if llmErr != nil {
			log.Printf("Gagal memanggil LLM Self-Correction: %v", llmErr)
			break
		}
		currentCode = cleanPandasCode(repairedCodeRaw)
		log.Printf("🔄 Auto-Repair Code Generated: %s", currentCode)
	}

	return RunnerOutput{}, currentCode, fmt.Errorf("gagal mengeksekusi kode pandas setelah %d kali percobaan", maxRetries+1)
}
