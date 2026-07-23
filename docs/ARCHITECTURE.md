# Arsitektur Sistem Go Bank API

> **📝 Last Updated**: 2026-01-05 (After Major Refactoring)
> **Version**: 2.0 (Refactored with Middleware Pattern)

## 📋 Daftar Isi

1. [Gambaran Umum](#gambaran-umum)
2. [Komponen Utama](#komponen-utama)
3. [Middleware Layer](#middleware-layer)
4. [Flow Diagram](#flow-diagram)
5. [Teknologi Stack](#teknologi-stack)
6. [Security & Best Practices](#security--best-practices)

---

## Gambaran Umum

**Go Bank API** adalah sistem backend yang menggunakan **Natural Language Processing (NLP)** untuk mengkonversi pertanyaan dalam bahasa natural menjadi SQL query. Sistem ini menggunakan pendekatan **RAG (Retrieval-Augmented Generation)** dengan **Semantic Caching** untuk meningkatkan akurasi dan performa.

### ✨ Fitur Baru (v2.0 - Refactored)

- **Middleware Pattern**: CORS, Logging, Rate Limiting
- **Graceful Shutdown**: Clean exit dengan signal handling
- **Constants Management**: Semua magic numbers di satu tempat
- **Multi-Origin CORS**: Support multiple frontend URLs
- **Rate Limiting**: Protection untuk admin endpoints

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

### 1. **Configuration Layer** (`config/config.go`)

- Mengelola semua konfigurasi aplikasi dari environment variables
- Menyediakan helper functions untuk type conversion
- Validasi required fields (DB_CONN_STRING, API Keys)
- **NEW**: Support `FrontendURL` untuk CORS configuration

### 2. **Constants Layer** (`constants/constants.go`) ✨ NEW

- **Error Codes**: Semua error codes (ErrCodeMethodNotAllowed, dll)
- **Intent Types**: IntentSQL, IntentChat, IntentOffTopic, dll
- **Cache Thresholds**: CacheHardHitThreshold (0.99), CacheSoftHitThreshold (0.80)
- **RAG Thresholds**: RAGMinimumScore (0.45), DDLMinimumScore (0.30)
- **User Roles**: AdminRoleValue (7), RegularUserRole (0)
- **Context Keys**: ContextKeyUserID, ContextKeyUsername, dll

### 3. **Middleware Layer** (`middleware/`) ✨ NEW

- **`cors.go`**: Centralized CORS handling dengan multi-origin support
- **`logging.go`**: HTTP request logging (method, path, status, duration, IP)
- **`ratelimit.go`**: Token bucket rate limiter untuk protection

### 4. **Database Layer** (`database/database.go`)

- Koneksi ke Oracle 10g+ menggunakan driver `go-ora`
- Connection pooling dengan konfigurasi dinamis
- Health check dengan timeout

### 5. **HTTP Layer**

- **`routes/routes.go`**: Routing HTTP endpoints dengan mux parameter
- **`controllers/*.go`**: Handler functions untuk setiap endpoint
- **`models/models.go`**: Data structures untuk request/response

### 6. **Business Logic Layer** (`ai/logic.go`)

- `GetSQL()`: Orchestrator untuk mendapatkan SQL dari AI
- `ExecuteDynamicQuery()`: Eksekusi SQL query dengan timeout
- `RepairSQLFromAI()`: Self-correction untuk SQL yang error

### 7. **AI Service Layer** (`ai/ai_service.go`)

- **Vector Service Initialization**: Setup Qdrant + Google AI
- **Semantic Cache**: Search & save cache menggunakan vector similarity
- **RAG Search**: Mencari context relevan dari vector database
- **LLM Integration**: Call Groq API untuk generate SQL
- **Intent Classification**: Deteksi CHAT vs SQL vs OFF_TOPIC
- **Qdrant Operations**: REST API calls untuk vector database

### 8. **Schema Service Layer** (`ai/schema_service.go`)

- `GetDynamicSchemaContext()`: Ambil DDL dari database
- `GetDynamicReferenceData()`: Ambil data referensi (status, tipe, dll)
- `GetBusinessDictionary()`: Ambil kamus istilah bisnis
- `AddSqlExample()`: Simpan feedback koreksi SQL

### 9. **Training Module** (`ai/train.go`)

- Proses embedding DDL dan SQL examples
- Upsert vectors ke Qdrant collection
- Dipanggil via endpoint `/admin/retrain`

### 10. **Authentication Layer** (`auth/auth_service.go`)

- JWT-based authentication
- Session management di database
- Password hashing dengan bcrypt
- AuthMiddleware untuk protected routes

---

## Middleware Layer

### CORS Middleware (`middleware/cors.go`)

**Fungsi**: Menangani Cross-Origin Resource Sharing

**Fitur**:

- Multi-origin support (comma-separated di config)
- Automatic origin validation
- Credentials support
- Preflight request handling

**Flow**:

```
Request → Check Origin → Validate → Set Headers → Next Handler
```

### Logging Middleware (`middleware/logging.go`)

**Fungsi**: Log semua HTTP requests

**Log Format**:

```
[HTTP] POST /api/query | Status: 200 | Duration: 1.234s | IP: 127.0.0.1 | UA: curl/7.68.0
```

### Rate Limiting Middleware (`middleware/ratelimit.go`)

**Fungsi**: Limit requests per IP

**Algorithm**: Token Bucket

- Default: 10 requests per minute untuk admin endpoints
- Automatic cleanup old visitors (prevent memory leak)

**Response saat limit exceeded**:

```json
{
  "error": "RATE_LIMIT_EXCEEDED",
  "message": "Terlalu banyak permintaan. Silakan coba lagi nanti."
}
```

---

## Flow Diagram

### A. Application Startup Flow (Updated v2.0)

```
┌─────────────────────────────────────────────────────────────┐
│                    STARTUP SEQUENCE                          │
└─────────────────────────────────────────────────────────────┘

main()
  │
  ├─► godotenv.Load()                    // Load .env file
  │
  ├─► LoadConfig()                       // config/config.go
  │     │
  │     ├─► Validate FrontendURL         // NEW: CORS config
  │     ├─► Validate JWT_SECRET
  │     └─► Validate AES_KEY (32 chars)
  │
  ├─► ConnectDB()                        // database/database.go
  │
  ├─► InitLogger()                       // log/logger.go
  │
  ├─► InitVectorService()                // ai/ai_service.go
  │     ├─► Connect to Qdrant
  │     ├─► Setup Google AI Embedder
  │     └─► Create collections if not exist
  │
  ├─► Create ServeMux                    // NEW: Dedicated mux
  │
  ├─► RegisterRoutes(mux)                // routes/routes.go
  │     ├─► Public routes
  │     ├─► Protected routes (with AuthMiddleware)
  │     └─► Admin routes (with RateLimitMiddleware)
  │
  ├─► Apply Middleware Chain             // NEW: Middleware pattern
  │     ├─► LoggingMiddleware
  │     └─► CORSMiddleware
  │
  ├─► Create HTTP Server
  │     ├─► Set timeouts (Read, Write, Idle)
  │     └─► Set handler to middleware chain
  │
  ├─► Start Server (goroutine)           // NEW: Non-blocking
  │
  └─► Setup Graceful Shutdown            // NEW: Signal handling
        ├─► Listen for SIGINT/SIGTERM
        ├─► Shutdown with 30s timeout
        └─► Clean exit
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
- **Custom Middleware**: CORS, Logging, Rate Limiting

### Database

- **Oracle 10g+**: Relational database
- **go-ora/v2**: Oracle driver for Go

### Vector Database

- **Qdrant**: Vector similarity search
  - gRPC client untuk query (port 6334)
  - REST API untuk management (port 6333)
  - Cloud support dengan TLS

### AI Services

- **Google AI (Gemini)**: Text embedding
  - Model: `text-embedding-004`
  - Vector size: 768 dimensions

- **Groq**: LLM for SQL generation
  - Model: `qwen/qwen3.6-27b` (configurable)
  - Fallback: `llama-3.1-8b-instant`
  - API: OpenAI-compatible endpoint

- **Ollama** (Optional): Local LLM fallback
  - Model: `gemma3:4b`

### Libraries

- `github.com/joho/godotenv`: Environment variables
- `github.com/google/generative-ai-go`: Google AI SDK
- `github.com/qdrant/go-client`: Qdrant gRPC client
- `github.com/google/uuid`: UUID generation
- `github.com/sijms/go-ora/v2`: Oracle driver
- `github.com/golang-jwt/jwt/v5`: JWT authentication
- `golang.org/x/crypto`: Password hashing (bcrypt)

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

## Security & Best Practices

### 1. **Environment Variables**

- ✅ API keys tidak di-commit ke repository
- ✅ `.env.example` sebagai template
- ✅ Validation untuk required fields

### 2. **CORS Configuration**

- ✅ Centralized di middleware
- ✅ Multi-origin support dari config
- ✅ Credentials support dengan origin validation
- ❌ Tidak menggunakan wildcard `*` dengan credentials

### 3. **Rate Limiting**

- ✅ Token bucket algorithm
- ✅ Per-IP tracking
- ✅ Automatic cleanup (prevent memory leak)
- ✅ Applied to admin endpoints

### 4. **Authentication**

- ✅ JWT-based dengan expiry
- ✅ Session tracking di database
- ✅ Password hashing dengan bcrypt
- ✅ Token validation di middleware

### 5. **Graceful Shutdown**

- ✅ Signal handling (SIGINT, SIGTERM)
- ✅ 30-second timeout untuk in-flight requests
- ✅ Clean resource cleanup

### 6. **Constants Management**

- ✅ Semua magic numbers di `constants/constants.go`
- ✅ Error codes standardized
- ✅ Thresholds configurable

### 7. **Logging**

- ✅ Structured HTTP logging
- ✅ Request duration tracking
- ✅ IP & User-Agent logging

---

## Migration Notes (v1.0 → v2.0)

### Breaking Changes

1. **CORS**: Sekarang di-handle oleh middleware, bukan per-handler
2. **Routes**: `RegisterRoutes()` sekarang menerima `*http.ServeMux` parameter
3. **Config**: Tambahan field `FrontendURL` (required)
4. **Shutdown**: Server sekarang graceful shutdown, bukan `log.Fatal()`

### New Features

1. **Middleware Pattern**: CORS, Logging, Rate Limiting
2. **Constants**: Semua magic numbers sekarang di constants
3. **Multi-Origin CORS**: Support multiple frontend URLs
4. **Rate Limiting**: Protection untuk admin endpoints
5. **Graceful Shutdown**: Clean exit dengan signal handling

### Migration Steps

1. Update `.env` dengan `FRONTEND_URL`
2. Rebuild aplikasi: `go build -o bin/app.exe .`
3. Test graceful shutdown dengan `Ctrl+C`
4. Verify CORS dari frontend

---

**Dokumentasi ini menjelaskan arsitektur lengkap sistem Go Bank API v2.0 dengan pendekatan RAG, Semantic Caching, dan Middleware Pattern untuk Natural Language to SQL conversion.**

**Last Updated**: 2026-01-05
**Version**: 2.0 (Refactored)
