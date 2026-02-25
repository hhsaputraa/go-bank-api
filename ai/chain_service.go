package ai

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	config "go-bank-api/config"

	"github.com/tmc/langchaingo/chains"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/prompts"
)

// SQLChainService handles AI orchestration using LangChain patterns
type SQLChainService struct {
	llm llms.Model
}

// NewSQLChainService creates a new instance of SQLChainService
func NewSQLChainService() (*SQLChainService, error) {
	if config.AppConfig == nil {
		return nil, fmt.Errorf("configuration not loaded")
	}

	// Trim the suffix if it exists, as langchaingo appends it
	baseURL := config.AppConfig.GroqAPIURL
	baseURL = strings.TrimSuffix(baseURL, "/chat/completions")
	baseURL = strings.TrimSuffix(baseURL, "/")

	// Use OpenAI-compatible provider for Groq
	llm, err := openai.New(
		openai.WithBaseURL(baseURL),
		openai.WithToken(config.AppConfig.GroqAPIKey),
		openai.WithModel(config.AppConfig.GroqModel),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize LLM: %w", err)
	}

	return &SQLChainService{llm: llm}, nil
}

// GenerateSQL uses LCEL-like chain to generate SQL from prompt and context
func (s *SQLChainService) GenerateSQL(ctx context.Context, userPrompt string, contextData map[string]interface{}) (string, error) {
	promptTemplate := prompts.NewPromptTemplate(
		`
Anda adalah ahli SQL Oracle 10g senior. Tanggal hari ini: {{.today}}.

== 1. KAMUS DATA (DDL & STRUKTUR) ==
{{.ddl}}

== 2. LIVE DATA REFERENSI ==
{{.refData}}

== 3. KAMUS ISTILAH BISNIS ==
{{.businessDict}}

== 4. CONTOH SQL (RAG CONTEXT) ==
{{.ragContext}}

== 5. SOFT CACHE HIT ==
{{.softCache}}

== ATURAN PENULISAN SQL ==
1. **STRICT SCHEMA ONLY**: Hanya gunakan tabel dan kolom yang TERTULIS EKSPLISIT.
2. **NO HALLUCINATION**: Jangan mengarang tabel.
3. **Security**: Hanya SELECT. Dilarang INSERT/UPDATE/DELETE.

TUGAS ANDA:
Sebelum menulis kode SQL, jelaskan langkah berpikir Anda di dalam tag <thought>.
Di LUAR tag <thought>, berikan Kode SQL dalam blok markdown.

Pertanyaan Pengguna: "{{.userPrompt}}"
`,
		[]string{"today", "ddl", "refData", "businessDict", "ragContext", "softCache", "userPrompt"},
	)

	// In langchaingo, we can use LLMChain
	chain := chains.NewLLMChain(s.llm, promptTemplate)

	// Run the chain
	prediction, err := chains.Call(ctx, chain, contextData)
	if err != nil {
		return "", fmt.Errorf("chain execution failed: %w", err)
	}

	output, ok := prediction["text"].(string)
	if !ok {
		return "", fmt.Errorf("unexpected output format from chain")
	}

	return output, nil
}

// SQLOutputParser handles cleaning and extracting SQL from LLM response
type SQLOutputParser struct{}

func (p *SQLOutputParser) Parse(input string) (string, string) {
	// Extract thought
	reThought := regexp.MustCompile("(?s)<thought>(.*?)</thought>")
	thoughtMatch := reThought.FindStringSubmatch(input)
	thought := ""
	if len(thoughtMatch) > 1 {
		thought = strings.TrimSpace(thoughtMatch[1])
	}

	// Remove thought from output
	cleanContent := reThought.ReplaceAllString(input, "")
	cleanContent = strings.TrimSpace(cleanContent)

	// Extract SQL
	reSQL := regexp.MustCompile("(?s)```sql(.*?)```")
	sqlMatch := reSQL.FindStringSubmatch(cleanContent)
	sql := ""
	if len(sqlMatch) > 1 {
		sql = strings.TrimSpace(sqlMatch[1])
	} else if strings.HasPrefix(strings.ToUpper(cleanContent), "SELECT") {
		sql = cleanContent
	}

	return thought, sql
}

// GetSQLWithChain is a high-level entry point that uses the new LangChain implementation
func GetSQLWithChain(ctx context.Context, userPrompt string, contextData map[string]interface{}) (string, error) {
	// Initialize service
	service, err := NewSQLChainService()
	if err != nil {
		return "", err
	}

	// Generate SQL using chain
	return service.GenerateSQL(ctx, userPrompt, contextData)
}
