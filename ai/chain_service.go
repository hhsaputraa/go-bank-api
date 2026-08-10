package ai

import (
	"context"
	"fmt"
	"log"
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
		err := fmt.Errorf("configuration not loaded")
		log.Println("[ai][chain_service][NewSQLChainService] error:", err)
		return nil, err
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
		log.Println("[ai][chain_service][NewSQLChainService] error:", err)
		return nil, fmt.Errorf("failed to initialize LLM: %w", err)
	}

	return &SQLChainService{llm: llm}, nil
}

// GenerateSQL uses LCEL-like chain to generate SQL from prompt and context
func (s *SQLChainService) GenerateSQL(ctx context.Context, userPrompt string, contextData map[string]interface{}) (string, error) {
	promptTemplate := prompts.NewPromptTemplate(
		`
Anda adalah ahli SQL Oracle 10g/PostgreSQL senior. Tanggal hari ini: {{.today}}.

== 1. KAMUS DATA (DDL & STRUKTUR SKEMA) ==
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
1. **STRICT SCHEMA ONLY**: Hanya gunakan tabel dan kolom yang TERTULIS EKSPLISIT pada Kamus Data / DDL.
2. **NO HALLUCINATION**: Dilarang mengarang tabel atau kolom yang tidak ada di skema.
3. **Security**: Hanya query SELECT. Dilarang INSERT/UPDATE/DELETE/DROP.

== CARA BERPIKIR (CHAIN-OF-THOUGHT) ==
Sebelum menulis kode SQL, Anda WAJIB menganalisis pertanyaan di dalam tag <thought> dengan langkah:
- **Langkah 1 (Entitas)**: Sebutkan tabel dan kolom yang relevan dari DDL.
- **Langkah 2 (Relasi JOIN)**: Jika butuh lebih dari 1 tabel, sebutkan jalur JOIN berdasarkan PANDUAN RELASI (FK HINTS).
- **Langkah 3 (Kondisi & Agregasi)**: Tentukan WHERE filter, GROUP BY, atau ORDER BY yang dibutuhkan.

TULISKAN KODE SQL FINAL DI LUAR TAG <thought> DI DALAM BLOK MARKDOWN SQL.

Pertanyaan Pengguna: "{{.userPrompt}}"
`,
		[]string{"today", "ddl", "refData", "businessDict", "ragContext", "softCache", "userPrompt"},
	)

	// In langchaingo, we can use LLMChain
	chain := chains.NewLLMChain(s.llm, promptTemplate)

	// Run the chain
	prediction, err := chains.Call(ctx, chain, contextData)
	if err != nil {
		log.Println("[ai][chain_service][GenerateSQL] error:", err)
		return "", fmt.Errorf("chain execution failed: %w", err)
	}

	output, ok := prediction["text"].(string)
	if !ok {
		err := fmt.Errorf("unexpected output format from chain")
		log.Println("[ai][chain_service][GenerateSQL] error:", err)
		return "", err
	}

	// Parse CoT thought and clean SQL
	parser := &SQLOutputParser{}
	thought, cleanSQL := parser.Parse(output)
	if thought != "" {
		log.Printf("🧠 [CoT Reasoning]:\n%s", thought)
	}

	if cleanSQL != "" {
		return cleanSQL, nil
	}

	return output, nil
}

// SQLOutputParser handles cleaning and extracting SQL from LLM response
type SQLOutputParser struct{}

func (p *SQLOutputParser) Parse(input string) (string, string) {
	// 1. Extract SQL FIRST from the raw input before stripping <thought> tags
	reSQL := regexp.MustCompile("(?s)```sql\\s*(.*?)\\s*```")
	sqlMatches := reSQL.FindAllStringSubmatch(input, -1)
	sql := ""
	for _, m := range sqlMatches {
		c := strings.TrimSpace(m[1])
		if strings.HasPrefix(strings.ToUpper(c), "SELECT") || strings.HasPrefix(strings.ToUpper(c), "WITH") {
			sql = c
		}
	}
	if sql == "" && len(sqlMatches) > 0 {
		sql = strings.TrimSpace(sqlMatches[len(sqlMatches)-1][1])
	}

	// 2. Extract thought content for logging
	reThought := regexp.MustCompile("(?s)<thought>(.*?)</thought>")
	thoughtMatch := reThought.FindStringSubmatch(input)
	thought := ""
	if len(thoughtMatch) > 1 {
		thought = strings.TrimSpace(thoughtMatch[1])
	}

	// 3. Fallback: If no ```sql block was found, search for SELECT statement in clean text
	if sql == "" {
		cleanContent := reThought.ReplaceAllString(input, "")
		cleanContent = strings.TrimSpace(cleanContent)
		if strings.HasPrefix(strings.ToUpper(cleanContent), "SELECT") || strings.HasPrefix(strings.ToUpper(cleanContent), "WITH") {
			sql = cleanContent
		} else {
			reSelect := regexp.MustCompile("(?i)(SELECT|WITH)\\s+.*")
			selMatch := reSelect.FindString(cleanContent)
			if selMatch != "" {
				sql = strings.TrimSpace(selMatch)
			}
		}
	}

	return thought, sql
}

// GetSQLWithChain is a high-level entry point that uses the new LangChain implementation
func GetSQLWithChain(ctx context.Context, userPrompt string, contextData map[string]interface{}) (string, error) {
	// Initialize service
	service, err := NewSQLChainService()
	if err != nil {
		log.Println("[ai][chain_service][GetSQLWithChain] error:", err)
		return "", err
	}

	// Generate SQL using chain
	return service.GenerateSQL(ctx, userPrompt, contextData)
}
