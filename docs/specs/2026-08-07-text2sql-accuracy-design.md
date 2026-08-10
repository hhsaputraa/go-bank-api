# Design Specification: RAG Text-to-SQL Accuracy & 3-Tier Resilience Architecture

- **Date**: 2026-08-07
- **Target Project**: `go-bank-api`
- **Goal**: Elevate Text-to-SQL accuracy, domain understanding, and resilience against complex out-of-example queries in the banking RAG system.

---

## 1. Executive Summary

To eliminate LLM hallucinations, SQL syntax errors, and wrong table joins—especially for complex queries outside of the Few-Shot dataset—the system will implement a **3-Tier Resilience Architecture** combined with a rich **Banking Golden SQL & Business Dictionary Dataset**.

---

## 2. Architecture & Data Flow

```mermaid
flowgraph TD
    UserPrompt[User Natural Language Prompt] --> IntentCheck[Intent Classification & Chit-Chat Filter]
    IntentCheck --> CacheCheck{Check Qdrant Semantic Cache}
    CacheCheck -- Hard Hit (>=0.99) --> ReturnCached[Return Cached SQL Immediately]
    CacheCheck -- Cache Miss / Soft Hit --> SchemaPruner[Schema Pruning & FK Guide Generator]
    
    SchemaPruner --> RAGSearch[Fetch Top-K Few-Shot Examples from Qdrant]
    RAGSearch --> CoTPrompt[Assemble Chain-of-Thought System Prompt with <thought> Tag]
    CoTPrompt --> LLMGen[LLM Generation (Groq/Gemini/OpenAI)]
    
    LLMGen --> DryRunCheck{Dry-Run EXPLAIN / Syntax Check}
    DryRunCheck -- Valid SQL --> ReturnResult[Execute & Return SQL Result to User]
    DryRunCheck -- Invalid/Execution Error --> RetryLoop{Retry Count < 2?}
    RetryLoop -- Yes --> RepairPrompt[RepairSQLFromAI with Exact DB Error Message]
    RepairPrompt --> LLMGen
    RetryLoop -- No --> FallbackError[Return Safe Error Message & Log Failure]
```

---

## 3. Key Components & Detailed Specifications

### Component A: Master Banking Seed Dataset & Business Dictionary Seeder (`ai/seed.go`)
- **Purpose**: Auto-populate database tables `rag_sql_examples` and `ai_dictionary` on application startup if empty.
- **Dataset Contents**:
  - **30+ Golden SQL Pairs** spanning:
    - Nasabah & Rekening information (Status Aktif, Prioritas, Pemblokiran)
    - Transaksi & Mutasi (Filter tanggal, Nominal > X, Top 10 Transaksi)
    - Pinjaman & Kredit (*NPL/Non-Performing Loan*, *Kredit Macet*, *Tunggakan Pokok/Bunga*)
    - Laporan Agregasi (*CASA Ratio*, *Total Dana Pihak Ketiga/DPK*)
  - **Business Dictionary Entries**:
    - Mapping terms like `"kredit macet"`, `"npl"`, `"nasabah prioritas"`, `"casa"` to exact SQL WHERE logic expressions.

### Component B: Schema Pruning & FK Relationship Guide (`ai/schema_service.go`)
- **Purpose**: Prevent context overflow and LLM confusion by filtering out irrelevant table DDLs and explicitly supplying Join Relationship Hints.
- **Specification**:
  - Analyze user prompt for keywords/table names.
  - Append PK-FK Join Hints block to system prompt:
    ```
    == RELASI ANTAR TABEL (JOIN HINT) ==
    - NASABAH.ID_NASABAH <---> REKENING.ID_NASABAH
    - REKENING.NO_REKENING <---> TRANSAKSI.NO_REKENING
    - PINJAMAN.ID_NASABAH <---> NASABAH.ID_NASABAH
    ```

### Component C: Chain-of-Thought (CoT) Prompting (`ai/chain_service.go`)
- **Purpose**: Force LLM to perform step-by-step reasoning before outputting raw SQL.
- **Specification**:
  - Update system prompt template:
    ```
    TUGAS ANDA:
    1. Jelaskan pemikiran Anda di dalam tag <thought>:
       - Tabel & Kolom yang dibutuhkan.
       - Jalur JOIN antar tabel yang dipakai.
       - Filter tanggal & agregasi.
    2. Tuliskan Kode SQL final di LUAR tag <thought> dalam blok markdown ```sql.
    ```
  - Parse output to isolate `<thought>` for logging/debugging and extract clean SQL for execution.

### Component D: Automated Dry-Run & Self-Correction Retry Loop (`ai/ai_service.go`)
- **Purpose**: Guarantee zero broken SQL returned to users by validating syntax and auto-repairing errors.
- **Specification**:
  - Before sending response, execute `EXPLAIN <sql>` or dry-run query with 3-second context timeout.
  - If DB returns an error (e.g. `column does not exist` or `syntax error`):
    - Trigger `RepairSQLFromAI(failedSQL, dbErrorMsg)`.
    - Retry up to 2 times.
    - Save successful repair to `rag_sql_examples` for future learning.

---

## 4. Verification & Testing Plan

1. **Compilation Check**:
   - `go build ./...` must succeed with zero errors.
2. **Seeder Verification**:
   - Run seed routine and verify `rag_sql_examples` has 30+ records and `ai_dictionary` has banking terms.
3. **Complex Query Test Suite**:
   - Query 1: *"Tampilkan 5 nasabah dengan total saldo terbanyak yang tidak punya pinjaman macet"* (Multi-table JOIN + aggregation + subquery).
   - Query 2: *"Berapa total transaksi kredit bulan ini untuk nasabah prioritas?"* (Date filtering + business jargon).
4. **Auto-Repair Test**:
   - Intentionally pass a malformed SQL prompt to test if the retry loop corrects syntax automatically.
