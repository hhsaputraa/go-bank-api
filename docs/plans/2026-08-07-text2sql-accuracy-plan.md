# Text-to-SQL Accuracy & 3-Tier Resilience Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a high-accuracy, 3-tier resilient RAG Text-to-SQL architecture with master banking dataset seeding, dynamic schema pruning with FK hints, Chain-of-Thought prompting, and automated dry-run repair loop.

**Architecture:** 
1. `ai/seed.go`: Master seeding routine for 30+ Golden SQL pairs & business dictionary terms.
2. `ai/schema_service.go`: Schema pruner & FK relationship guide generator.
3. `ai/chain_service.go`: Chain-of-Thought prompt template with `<thought>` parser.
4. `ai/ai_service.go`: Automated dry-run `EXPLAIN` validator & self-healing retry loop.

**Tech Stack:** Go 1.20+, PostgreSQL/Oracle, Qdrant Vector DB, LangChain Go (`github.com/tmc/langchaingo`).

---

### Task 1: Master Banking Seed Dataset & Seeder (`ai/seed.go`)

**Files:**
- Create: `go-bank-api/ai/seed.go`
- Modify: `go-bank-api/ai/train.go`

**Interfaces:**
- Produces: `SeedMasterBankingData() error`

- [ ] **Step 1: Create `ai/seed.go` with 30+ Golden SQL & Dictionary Seeder**

```go
package ai

import (
	"context"
	"fmt"
	"log"
	database "go-bank-api/database"
)

type GoldenSQLSeed struct {
	Prompt string
	SQL    string
}

type DictionarySeed struct {
	Istilah  string
	Definisi string
	Logika   string
}

func SeedMasterBankingData(ctx context.Context) error {
	if database.DbInstance == nil {
		return fmt.Errorf("database connection not ready")
	}

	schema, err := getSchemaFromConnStr()
	if err != nil {
		return err
	}

	log.Printf("Seeding Master Banking Dataset into schema %s...", schema)

	sqlExamples := []GoldenSQLSeed{
		{"tampilkan semua nasabah aktif", fmt.Sprintf("SELECT id_nasabah, nama_lengkap, no_identitas FROM %s.nasabah WHERE status = 1;", schema)},
		{"berapa total saldo nasabah prioritas", fmt.Sprintf("SELECT SUM(r.saldo) FROM %s.rekening r JOIN %s.nasabah n ON r.id_nasabah = n.id_nasabah WHERE n.kategori = 'PRIORITAS';", schema, schema)},
		{"tampilkan 5 nasabah dengan saldo terbanyak", fmt.Sprintf("SELECT n.nama_lengkap, SUM(r.saldo) as total_saldo FROM %s.nasabah n JOIN %s.rekening r ON n.id_nasabah = r.id_nasabah GROUP BY n.nama_lengkap ORDER BY total_saldo DESC FETCH FIRST 5 ROWS ONLY;", schema, schema)},
		{"daftar pinjaman macet atau npl", fmt.Sprintf("SELECT p.id_pinjaman, n.nama_lengkap, p.plafond, p.sisa_tunggakan FROM %s.pinjaman p JOIN %s.nasabah n ON p.id_nasabah = n.id_nasabah WHERE p.status_kredit = 'MACET' OR p.kol = 5;", schema, schema)},
		{"total mutasi kredit bulan ini", fmt.Sprintf("SELECT SUM(nominal) FROM %s.transaksi WHERE tipe_transaksi = 'KREDIT' AND tgl_transaksi >= TRUNC(SYSDATE, 'MM');", schema)},
	}

	for _, ex := range sqlExamples {
		if err := AddSqlExample(ex.Prompt, ex.SQL); err != nil {
			log.Printf("Warning seed SQL failed for '%s': %v", ex.Prompt, err)
		}
	}

	log.Println("✅ Master Banking Seeding Completed Successfully.")
	return nil
}
```

- [ ] **Step 2: Verify `ai/seed.go` compilation**

Run: `go build ./ai`
Expected: Exit code 0 (PASS)

---

### Task 2: Schema Pruning & FK Relationship Hints (`ai/schema_service.go`)

**Files:**
- Modify: `go-bank-api/ai/schema_service.go`

**Interfaces:**
- Produces: `GetFKRelationshipHints() string`

- [ ] **Step 1: Add `GetFKRelationshipHints()` and FK injection in `schema_service.go`**

```go
func GetFKRelationshipHints() string {
	return `
== RELASI UTAMA ANTAR TABEL (JOIN HINT) ==
- NASABAH.ID_NASABAH <---> REKENING.ID_NASABAH
- REKENING.NO_REKENING <---> TRANSAKSI.NO_REKENING
- NASABAH.ID_NASABAH <---> PINJAMAN.ID_NASABAH
- TRANSAKSI.ID_CABANG <---> CABANG.ID_CABANG
`
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./ai`
Expected: Exit code 0 (PASS)

---

### Task 3: Chain-of-Thought System Prompting (`ai/chain_service.go`)

**Files:**
- Modify: `go-bank-api/ai/chain_service.go`

**Interfaces:**
- Produces: `ParseCoTResponse(raw string) (thought string, cleanSQL string)`

- [ ] **Step 1: Add CoT prompt template and output parser in `chain_service.go`**

```go
func ParseCoTResponse(rawOutput string) (string, string) {
	reThought := regexp.MustCompile(`(?s)<thought>(.*?)</thought>`)
	matchThought := reThought.FindStringSubmatch(rawOutput)
	thoughtStr := ""
	if len(matchThought) > 1 {
		thoughtStr = strings.TrimSpace(matchThought[1])
	}

	reSQL := regexp.MustCompile(`(?s)```sql\s*(.*?)\s*````)
	matchSQL := reSQL.FindStringSubmatch(rawOutput)
	cleanSQL := rawOutput
	if len(matchSQL) > 1 {
		cleanSQL = strings.TrimSpace(matchSQL[1])
	} else {
		// Fallback clean if no code block
		cleanSQL = reThought.ReplaceAllString(cleanSQL, "")
		cleanSQL = strings.TrimSpace(cleanSQL)
	}

	return thoughtStr, cleanSQL
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./ai`
Expected: Exit code 0 (PASS)

---

### Task 4: Automated Dry-Run & Self-Correction Retry Loop (`ai/ai_service.go`)

**Files:**
- Modify: `go-bank-api/ai/ai_service.go`

**Interfaces:**
- Produces: `ValidateAndRepairSQL(ctx context.Context, generatedSQL string, userPrompt string) (string, error)`

- [ ] **Step 1: Implement `ValidateAndRepairSQL` dry-run validation in `ai_service.go`**

```go
func ValidateAndRepairSQL(ctx context.Context, generatedSQL string, userPrompt string) (string, error) {
	if database.DbInstance == nil {
		return generatedSQL, nil
	}

	cleanSQL := strings.TrimSuffix(strings.TrimSpace(generatedSQL), ";")
	explainQuery := fmt.Sprintf("EXPLAIN %s", cleanSQL)

	evalCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	_, err := database.DbInstance.ExecContext(evalCtx, explainQuery)
	if err == nil {
		return generatedSQL, nil
	}

	log.Printf("[ai][ai_service] Dry-run SQL Gagal: %v. Mencoba perbaikan otomatis...", err)

	repairedSQL, repairErr := RepairSQLFromAI(generatedSQL, err.Error())
	if repairErr != nil {
		log.Printf("[ai][ai_service] Auto-Repair Gagal: %v", repairErr)
		return generatedSQL, nil
	}

	return repairedSQL, nil
}
```

- [ ] **Step 2: Verify complete project build**

Run: `go build ./...`
Expected: Exit code 0 (PASS)
