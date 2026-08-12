package ai

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
)

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

// CallOpenAILLM executes chat completion against OpenAI-compatible LLM endpoint
func CallOpenAILLM(systemPrompt, userPrompt string) (string, error) {
	if config.AppConfig == nil {
		return "", fmt.Errorf("config.AppConfig is not initialized")
	}

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

// GetMeltedColumns extracts header columns from CSV, excluding month columns
func GetMeltedColumns(filePath string) ([]string, error) {
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

func GetPythonRunnerDir() string {
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

func ExtractPythonCode(text string) string {
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
	if len(parts) >= 2 {
		return strings.TrimSpace(parts[1])
	}
	return strings.TrimSpace(text)
}

func StripThinkTags(text string) string {
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

func CleanPandasCode(text string) string {
	cleaned := StripThinkTags(text)
	extracted := ExtractPythonCode(cleaned)

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

func ExecutePandasWithRetry(absFilePath string, initialCode string, userPrompt string, dfInfo string) (RunnerOutput, string, error) {
	currentCode := CleanPandasCode(initialCode)
	maxRetries := 2
	runnerDir := GetPythonRunnerDir()

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

		repairedCodeRaw, llmErr := CallOpenAILLM(repairSystemPrompt, userPrompt)
		if llmErr != nil {
			log.Printf("Gagal memanggil LLM Self-Correction: %v", llmErr)
			break
		}
		currentCode = CleanPandasCode(repairedCodeRaw)
		log.Printf("🔄 Auto-Repair Code Generated: %s", currentCode)
	}

	return RunnerOutput{}, currentCode, fmt.Errorf("gagal mengeksekusi kode pandas setelah %d kali percobaan", maxRetries+1)
}

// GetDataFrameInfo runs python query_runner.py --info to inspect dataframe schema
func GetDataFrameInfo(absFilePath string, cols []string) string {
	runnerDir := GetPythonRunnerDir()
	infoCmd := exec.Command("python", "query_runner.py", absFilePath, "--info")
	infoCmd.Dir = runnerDir
	var infoStdout, infoStderr bytes.Buffer
	infoCmd.Stdout = &infoStdout
	infoCmd.Stderr = &infoStderr

	if err := infoCmd.Run(); err != nil {
		log.Printf("Warning: Gagal mengambil info DataFrame: %v | stderr: %s", err, infoStderr.String())
		return fmt.Sprintf("Kolom: %v", cols)
	}

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
		return fmt.Sprintf("\nTipe Data Kolom (dtypes):\n%s\n\nPreview 3 Baris Pertama (head):\n%s\n\nContoh Nilai Unik per Kolom (Uniques):\n%s", string(dtypesBytes), string(headBytes), string(uniquesBytes))
	}
	return fmt.Sprintf("Kolom: %v", cols)
}
