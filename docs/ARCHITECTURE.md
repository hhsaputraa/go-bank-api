# Arsitektur Sistem Go Bank API

## 📋 Daftar Isi

1. [Gambaran Umum](#gambaran-umum)
2. [Komponen Utama](#komponen-utama)
3. [Flow Diagram](#flow-diagram)
4. [Teknologi Stack](#teknologi-stack)

---

## Gambaran Umum

**Go Bank API** adalah sistem backend yang menggunakan **Natural Language Processing (NLP)** untuk mengkonversi pertanyaan dalam bahasa natural menjadi SQL query. Sistem ini menggunakan pendekatan **RAG (Retrieval-Augmented Generation)** dengan **Semantic Caching** untuk meningkatkan akurasi dan performa.

### Konsep Utama

1. **RAG (Retrieval-Augmented Generation)**

   - Menggunakan vector database (Qdrant) untuk menyimpan "contekan" (DDL schema + contoh SQL)
   - Saat user bertanya, sistem mencari contekan yang paling relevan
   - Contekan tersebut diberikan ke LLM sebagai context untuk menghasilkan SQL yang akurat

2. **Semantic Caching**

   - Menyimpan hasil query yang sudah berhasil dieksekusi
   - Menggunakan vector similarity untuk mendeteksi pertanyaan yang mirip
   - Jika similarity score ≥ threshold (default 0.95), langsung return hasil dari cache

3. **Dynamic Schema Detection**
   - Otomatis membaca struktur database dari `information_schema`
   - Schema ditentukan dari parameter `search_path` di connection string
   - Tidak perlu hardcode nama schema/tabel

---

## Komponen Utama

### 1. **Configuration Layer** (`config.go`)

- Mengelola semua konfigurasi aplikasi dari environment variables
- Menyediakan helper functions untuk type conversion
- Validasi required fields (DB_CONN_STRING, API Keys)

### 2. **Database Layer** (`database.go`)

- Koneksi ke PostgreSQL menggunakan driver `pgx`
- Connection pooling dengan konfigurasi dinamis
- Health check dengan timeout

### 3. **HTTP Layer**

- **`routes.go`**: Routing HTTP endpoints
- **`handlers.go`**: Handler functions untuk setiap endpoint
- **`models.go`**: Data structures untuk request/response

### 4. **Business Logic Layer** (`logic.go`)

- `GetSQL()`: Orchestrator untuk mendapatkan SQL dari AI
- `ExecuteDynamicQuery()`: Eksekusi SQL query dengan timeout
- `BuildDynamicQuery()`: Query builder (legacy, tidak digunakan untuk NLP)

### 5. **AI Service Layer** (`ai_service.go`)

- **Vector Service Initialization**: Setup Qdrant + Google AI
- **Semantic Cache**: Search & save cache menggunakan vector similarity
- **RAG Search**: Mencari context relevan dari vector database
- **LLM Integration**: Call Groq API untuk generate SQL
- **Qdrant Operations**: REST API calls untuk vector database

### 6. **Schema Service Layer** (`schema_service.go`)

- `GetDynamicSchemaContext()`: Ambil DDL dari `information_schema`
- `GetDynamicSqlExamples()`: Ambil contoh SQL dari tabel `rag_sql_examples`
- `AddSqlExample()`: Simpan feedback koreksi SQL
- `getSchemaFromConnStr()`: Extract schema name dari connection string

### 7. **Training Module** (`train.go`)

- Proses embedding DDL dan SQL examples
- Upsert vectors ke Qdrant collection
- Dipanggil via endpoint `/admin/retrain`

---

## Flow Diagram

### A. Application Startup Flow

```
┌─────────────────────────────────────────────────────────────┐
│                    STARTUP SEQUENCE                          │
└─────────────────────────────────────────────────────────────┘

main()
  │
  ├─► godotenv.Load()                    // Load .env file
  │
  ├─► LoadConfig()                       // config.go
  │     │
  │     ├─► Read all environment variables
  │     ├─► Apply default values
  │     ├─► Validate required fields
  │     └─► Store in global AppConfig
  │
  ├─► ConnectDB()                        // database.go
  │     │
  │     ├─► sql.Open() with connection string
  │     ├─► Set connection pool settings
  │     └─► Ping database with timeout
  │
  ├─► InitVectorService()                // ai_service.go
  │     │
  │     ├─► Create Google AI client (Gemini)
  │     ├─► Initialize embedding model
  │     ├─► Create Qdrant gRPC client
  │     └─► Ensure cache collection exists
  │
  ├─► RegisterRoutes()                   // routes.go
  │     │
  │     ├─► /health → HandleHealthCheck
  │     ├─► /api/query → HandleDynamicQuery
  │     ├─► /api/feedback/koreksi → HandleFeedbackKoreksi
  │     └─► /admin/retrain → HandleAdminRetrain
  │
  └─► http.ListenAndServe()              // Start HTTP server
```

### B. Query Processing Flow (Main Feature)

```
┌─────────────────────────────────────────────────────────────┐
│              USER QUERY TO SQL EXECUTION                     │
└─────────────────────────────────────────────────────────────┘

POST /api/query
  │
  ▼
HandleDynamicQuery()                     // handlers.go
  │
  ├─► Parse JSON request body
  ├─► Normalize prompt (lowercase, trim)
  │
  ├─► GetSQL(prompt)                     // logic.go
  │     │
  │     └─► getSQLFromAI_Groq(prompt)    // ai_service.go
  │           │
  │           ├─► [STEP 1: EMBEDDING]
  │           │   └─► geminiEmbedder.EmbedContent(prompt)
  │           │       └─► Returns: promptVector (768 dimensions)
  │           │
  │           ├─► [STEP 2: SEMANTIC CACHE CHECK]
  │           │   │
  │           │   └─► qdrantSearchPoints()
  │           │       ├─► Search in cache collection
  │           │       ├─► Compare similarity score
  │           │       │
  │           │       ├─► IF score >= threshold (0.95)
  │           │       │   └─► ✅ CACHE HIT! Return cached SQL
  │           │       │
  │           │       └─► ELSE: CACHE MISS, continue...
  │           │
  │           ├─► [STEP 3: RAG CONTEXT RETRIEVAL]
  │           │   │
  │           │   └─► qdrantClient.Query()
  │           │       ├─► Search in RAG collection
  │           │       ├─► Filter: category = "sql"
  │           │       ├─► Limit: 10 results
  │           │       └─► Returns: relevant SQL examples
  │           │
  │           ├─► [STEP 4: GET FULL DDL]
  │           │   │
  │           │   └─► GetDynamicSchemaContext()  // schema_service.go
  │           │       ├─► Query information_schema.columns
  │           │       ├─► Build CREATE TABLE statements
  │           │       └─► Returns: all DDL strings
  │           │
  │           ├─► [STEP 5: BUILD PROMPT]
  │           │   │
  │           │   └─► Combine:
  │           │       ├─► Current date
  │           │       ├─► All DDL (database dictionary)
  │           │       ├─► Relevant SQL examples (from RAG)
  │           │       └─► User's question
  │           │
  │           └─► [STEP 6: CALL LLM]
  │               │
  │               └─► HTTP POST to Groq API
  │                   ├─► Model: llama-3.1-8b-instant
  │                   ├─► Timeout: 30 seconds
  │                   └─► Returns: SQL query string
  │
  ├─► ExecuteDynamicQuery(sql)           // logic.go
  │     │
  │     ├─► Create context with timeout (10s)
  │     ├─► DbInstance.QueryContext()
  │     ├─► Scan all rows and columns
  │     └─► Returns: QueryResult{Columns, Rows}
  │
  ├─► IF query successful AND not from cache:
  │   └─► SaveToCache()                  // ai_service.go (async)
  │       └─► qdrantUpsertPoints() to cache collection
  │
  └─► Return JSON response to client
```

### C. Feedback & Correction Flow

```
┌─────────────────────────────────────────────────────────────┐
│                  FEEDBACK CORRECTION FLOW                    │
└─────────────────────────────────────────────────────────────┘

POST /api/feedback/koreksi
  │
  ▼
HandleFeedbackKoreksi()                  // handlers.go
  │
  ├─► Parse JSON: {prompt_asli, sql_koreksi}
  │
  ├─► AddSqlExample()                    // schema_service.go
  │     │
  │     ├─► getSchemaFromConnStr()
  │     │   └─► Extract schema from DB_CONN_STRING
  │     │
  │     ├─► Format prompt as comment
  │     │   └─► "-- Pertanyaan: \"...\""
  │     │
  │     └─► INSERT INTO {schema}.rag_sql_examples
  │         └─► Save (prompt_example, sql_example)
  │
  └─► Return success response
      └─► Message: "Silakan 'retrain' untuk menerapkan"
```

### D. Retraining Flow

```
┌─────────────────────────────────────────────────────────────┐
│                    RETRAINING FLOW                           │
└─────────────────────────────────────────────────────────────┘

POST /admin/retrain
  │
  ▼
HandleAdminRetrain()                     // handlers.go
  │
  ├─► Launch goroutine (background process)
  │     │
  │     └─► mainTrain()                  // train.go
  │           │
  │           ├─► LoadConfig()
  │           ├─► ConnectDB()
  │           │
  │           ├─► Create Google AI client
  │           ├─► Initialize embedder
  │           │
  │           ├─► qdrantCreateCollection()
  │           │   └─► Create/recreate RAG collection
  │           │
  │           ├─► GetDynamicSchemaContext()
  │           │   └─► Fetch all DDL from database
  │           │
  │           ├─► GetDynamicSqlExamples()
  │           │   └─► Fetch all SQL examples from rag_sql_examples
  │           │
  │           ├─► FOR EACH DDL:
  │           │   ├─► embedder.EmbedContent(ddl)
  │           │   └─► Create point with category="ddl"
  │           │
  │           ├─► FOR EACH SQL Example:
  │           │   ├─► embedder.EmbedContent(sql)
  │           │   └─► Create point with category="sql"
  │           │
  │           └─► qdrantUpsertPoints()
  │               └─► Save all vectors to Qdrant
  │
  └─► Return 202 Accepted
      └─► Message: "Proses retraining dimulai di background"
```

---

## Teknologi Stack

### Backend Framework

- **Go 1.24+**: Programming language
- **net/http**: HTTP server (standard library)

### Database

- **PostgreSQL**: Relational database
- **pgx/v5**: PostgreSQL driver for Go

### Vector Database

- **Qdrant**: Vector similarity search
  - gRPC client untuk query (port 6334)
  - REST API untuk management (port 6333)

### AI Services

- **Google AI (Gemini)**: Text embedding

  - Model: `text-embedding-004`
  - Vector size: 768 dimensions

- **Groq**: LLM for SQL generation
  - Model: `llama-3.1-8b-instant`
  - API: OpenAI-compatible endpoint

### Libraries

- `github.com/joho/godotenv`: Environment variables
- `github.com/google/generative-ai-go`: Google AI SDK
- `github.com/qdrant/go-client`: Qdrant gRPC client
- `github.com/google/uuid`: UUID generation
- `github.com/jackc/pgx/v5`: PostgreSQL driver

---

## Data Flow Summary

```
┌──────────────┐
│     User     │
└──────┬───────┘
       │ HTTP Request (Natural Language)
       ▼
┌──────────────────────────────────────────────────────────┐
│                    Go Bank API                            │
│  ┌────────────────────────────────────────────────────┐  │
│  │  1. Embed prompt → Vector (768D)                   │  │
│  └────────────────────────────────────────────────────┘  │
│                          │                                │
│                          ▼                                │
│  ┌────────────────────────────────────────────────────┐  │
│  │  2. Search Semantic Cache (Qdrant)                 │  │
│  │     - IF similarity ≥ 0.95 → Return cached SQL     │  │
│  └────────────────────────────────────────────────────┘  │
│                          │                                │
│                          ▼ (Cache Miss)                   │
│  ┌────────────────────────────────────────────────────┐  │
│  │  3. RAG Search (Qdrant)                            │  │
│  │     - Find relevant SQL examples                   │  │
│  └────────────────────────────────────────────────────┘  │
│                          │                                │
│                          ▼                                │
│  ┌────────────────────────────────────────────────────┐  │
│  │  4. Get DDL (PostgreSQL information_schema)        │  │
│  └────────────────────────────────────────────────────┘  │
│                          │                                │
│                          ▼                                │
│  ┌────────────────────────────────────────────────────┐  │
│  │  5. Build Prompt (DDL + Examples + Question)       │  │
│  └────────────────────────────────────────────────────┘  │
│                          │                                │
│                          ▼                                │
│  ┌────────────────────────────────────────────────────┐  │
│  │  6. Call Groq LLM → Generate SQL                   │  │
│  └────────────────────────────────────────────────────┘  │
│                          │                                │
│                          ▼                                │
│  ┌────────────────────────────────────────────────────┐  │
│  │  7. Execute SQL (PostgreSQL)                       │  │
│  └────────────────────────────────────────────────────┘  │
│                          │                                │
│                          ▼                                │
│  ┌────────────────────────────────────────────────────┐  │
│  │  8. Save to Cache (async, if successful)           │  │
│  └────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────┘
       │
       ▼ JSON Response (Query Results)
┌──────────────┐
│     User     │
└──────────────┘
```

---

## Performance Optimizations

1. **Semantic Caching**

   - Mengurangi calls ke LLM untuk pertanyaan yang mirip
   - Threshold 0.95 memastikan akurasi tinggi
   - Async save untuk tidak block response

2. **Connection Pooling**

   - Max 25 open connections
   - Max 10 idle connections
   - 5 minutes connection lifetime

3. **Timeouts**

   - Database ping: 5 seconds
   - Query execution: 10 seconds
   - Groq API: 30 seconds
   - Qdrant operations: 60 seconds

4. **Async Operations**
   - Cache saving dilakukan di goroutine
   - Retraining dilakukan di background

---

## Security Features

1. **Environment Variables**

   - Semua kredensial di `.env` (tidak di-commit)
   - Validasi required fields saat startup
   - Type-safe configuration loading

2. **SQL Injection Prevention**

   - Menggunakan parameterized queries
   - AI-generated SQL di-validate sebelum eksekusi

3. **CORS Headers**
   - Configured untuk cross-origin requests
   - OPTIONS method support

---

## Error Handling

1. **Graceful Degradation**

   - Cache failure tidak menghentikan query
   - Fallback ke default values jika env var tidak ada

2. **Comprehensive Logging**

   - Setiap step di-log untuk debugging
   - Error messages yang informatif

3. **Validation**
   - Request body validation
   - Empty prompt rejection
   - SQL result validation

---

**Dokumentasi ini menjelaskan arsitektur lengkap sistem Go Bank API dengan pendekatan RAG dan Semantic Caching untuk Natural Language to SQL conversion.**
