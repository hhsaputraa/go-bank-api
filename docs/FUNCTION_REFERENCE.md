# Function Reference - Go Bank API

Dokumentasi lengkap semua fungsi dalam sistem, dikelompokkan berdasarkan file/module.

---

## 📋 Daftar Isi

1. [config.go - Configuration Management](#configgo---configuration-management)
2. [database.go - Database Connection](#databasego---database-connection)
3. [main.go - Application Entry Point](#maingo---application-entry-point)
4. [routes.go - HTTP Routing](#routesgo---http-routing)
5. [handlers.go - HTTP Handlers](#handlersgo---http-handlers)
6. [logic.go - Business Logic](#logicgo---business-logic)
7. [ai_service.go - AI & Vector Services](#ai_servicego---ai--vector-services)
8. [schema_service.go - Schema Management](#schema_servicego---schema-management)
9. [train.go - Training Module](#traingo---training-module)

---

## config.go - Configuration Management

### `LoadConfig() (*Config, error)`

**Tujuan**: Memuat semua konfigurasi aplikasi dari environment variables

**Flow**:

1. Membaca semua environment variables menggunakan helper functions
2. Menerapkan default values jika variable tidak ditemukan
3. Validasi required fields (DB_CONN_STRING, GROQ_API_KEY, GOOGLE_API_KEY)
4. Menyimpan konfigurasi ke global variable `AppConfig`
5. Return pointer ke Config struct

**Return**:

- `*Config`: Pointer ke configuration struct
- `error`: Error jika required field tidak ada

**Dipanggil oleh**:

- `main()` saat aplikasi startup
- `mainTrain()` saat retraining

**Contoh Error**:

```
"DB_CONN_STRING is required"
"GROQ_API_KEY is required"
"GOOGLE_API_KEY is required"
```

---

### `getEnv(key, defaultValue string) string`

**Tujuan**: Helper function untuk membaca environment variable sebagai string

**Parameters**:

- `key`: Nama environment variable
- `defaultValue`: Nilai default jika variable tidak ditemukan

**Return**: String value dari environment variable atau default value

**Contoh**:

```go
serverPort := getEnv("SERVER_PORT", "8080")
// Jika SERVER_PORT tidak ada, return "8080"
```

---

### `getEnvAsInt(key string, defaultValue int) int`

**Tujuan**: Helper function untuk membaca environment variable sebagai integer

**Parameters**:

- `key`: Nama environment variable
- `defaultValue`: Nilai default jika variable tidak ditemukan atau parsing gagal

**Return**: Integer value dari environment variable atau default value

**Contoh**:

```go
maxConns := getEnvAsInt("DB_MAX_OPEN_CONNS", 25)
// Jika DB_MAX_OPEN_CONNS="50", return 50
// Jika tidak ada atau invalid, return 25
```

---

### `getEnvAsFloat32(key string, defaultValue float32) float32`

**Tujuan**: Helper function untuk membaca environment variable sebagai float32

**Parameters**:

- `key`: Nama environment variable
- `defaultValue`: Nilai default jika variable tidak ditemukan atau parsing gagal

**Return**: Float32 value dari environment variable atau default value

**Contoh**:

```go
threshold := getEnvAsFloat32("CACHE_SIMILARITY_THRESHOLD", 0.95)
// Jika CACHE_SIMILARITY_THRESHOLD="0.90", return 0.90
```

---

### `getEnvAsBool(key string, defaultValue bool) bool`

**Tujuan**: Helper function untuk membaca environment variable sebagai boolean

**Parameters**:

- `key`: Nama environment variable
- `defaultValue`: Nilai default jika variable tidak ditemukan atau parsing gagal

**Return**: Boolean value dari environment variable atau default value

**Contoh**:

```go
debug := getEnvAsBool("DEBUG", false)
// Jika DEBUG="true", return true
// Jika DEBUG="1", return true
```

---

## database.go - Database Connection

### `ConnectDB() error`

**Tujuan**: Membuat koneksi ke PostgreSQL database dengan connection pooling

**Flow**:

1. Validasi bahwa `AppConfig` sudah dimuat
2. Open database connection menggunakan `pgx` driver
3. Set connection pool settings dari config:
   - `MaxOpenConns`: Maximum open connections
   - `MaxIdleConns`: Maximum idle connections
   - `ConnMaxLifetime`: Connection lifetime
4. Ping database dengan timeout untuk validasi koneksi
5. Log connection details

**Return**: `error` jika koneksi gagal

**Dipanggil oleh**:

- `main()` saat aplikasi startup
- `mainTrain()` saat retraining

**Side Effects**:

- Set global variable `DbInstance`
- Log connection status dan settings

**Contoh Error**:

```
"konfigurasi aplikasi belum dimuat"
"gagal membuka koneksi database: ..."
"gagal melakukan ping ke database: ..."
```

---

## main.go - Application Entry Point

### `main()`

**Tujuan**: Entry point aplikasi, orchestrate startup sequence

**Flow**:

1. Load `.env` file menggunakan `godotenv.Load()`
2. Load configuration dengan `LoadConfig()`
3. Connect ke database dengan `ConnectDB()`
4. Initialize vector service dengan `InitVectorService()`
5. Register HTTP routes dengan `RegisterRoutes()`
6. Start HTTP server pada configured port

**Exit Conditions**:

- Fatal error jika config loading gagal
- Fatal error jika database connection gagal
- Fatal error jika vector service initialization gagal
- Fatal error jika HTTP server gagal start

**Log Output**:

```
"Berhasil memuat file .env"
"✅ Konfigurasi berhasil dimuat dari environment variables"
"✅ Berhasil terkoneksi ke database PostgreSQL!"
"✅ Berhasil terkoneksi ke Layanan Vektor (Google AI & Qdrant)."
"Aplikasi siap berjalan..."
"Server web berjalan di http://localhost:8080"
```

---

## routes.go - HTTP Routing

### `RegisterRoutes()`

**Tujuan**: Mendaftarkan semua HTTP endpoints ke router

**Endpoints yang didaftarkan**:

- `GET /health` → `HandleHealthCheck`
- `POST /api/query` → `HandleDynamicQuery`
- `POST /api/feedback/koreksi` → `HandleFeedbackKoreksi`
- `POST /admin/retrain` → `HandleAdminRetrain`

**Dipanggil oleh**: `main()` saat startup

**Side Effects**: Register handlers ke `http.DefaultServeMux`

---

## handlers.go - HTTP Handlers

### `respondWithJSON(w http.ResponseWriter, code int, payload interface{})`

**Tujuan**: Helper function untuk mengirim JSON response

**Parameters**:

- `w`: HTTP response writer
- `code`: HTTP status code
- `payload`: Data yang akan di-marshal ke JSON

**Side Effects**: Write JSON response dengan Content-Type header

---

### `respondWithError(w http.ResponseWriter, code int, message string)`

**Tujuan**: Helper function untuk mengirim error response dalam format JSON

**Parameters**:

- `w`: HTTP response writer
- `code`: HTTP status code
- `message`: Error message

**Response Format**:

```json
{
  "error": "error message here"
}
```

---

### `HandleHealthCheck(w http.ResponseWriter, r *http.Request)`

**Tujuan**: Health check endpoint untuk monitoring

**HTTP Method**: `GET`

**Response**:

```json
{
  "status": "API is up and running!"
}
```

**Status Code**: `200 OK`

---

### `HandleDynamicQuery(w http.ResponseWriter, r *http.Request)`

**Tujuan**: Main endpoint untuk Natural Language to SQL query

**HTTP Method**: `POST`

**Request Body**:

```json
{
  "prompt": "tampilkan semua nasabah"
}
```

**Flow**:

1. Handle CORS preflight (OPTIONS)
2. Validate HTTP method (harus POST)
3. Parse JSON request body
4. Normalize prompt (lowercase, trim)
5. Validate prompt tidak kosong
6. Call `GetSQL(prompt)` untuk mendapatkan SQL dari AI
7. Execute SQL dengan `ExecuteDynamicQuery()`
8. Jika berhasil dan bukan dari cache, save ke cache (async)
9. Return query results

**Success Response** (200 OK):

```json
{
  "columns": ["id_nasabah", "nama_lengkap"],
  "rows": [
    ["CIF00001", "Budi Santoso"],
    ["CIF00002", "Ani Wijaya"]
  ]
}
```

**Error Responses**:

- `405 Method Not Allowed`: Bukan POST request
- `400 Bad Request`: JSON invalid atau prompt kosong
- `500 Internal Server Error`: AI error atau SQL execution error

**CORS Headers**:

- `Access-Control-Allow-Origin: *`
- `Access-Control-Allow-Methods: POST, OPTIONS`
- `Access-Control-Allow-Headers: Content-Type`

**Log Output**:

```
"Menerima Prompt (Normalized): tampilkan semua nasabah"
"SQL yang akan dieksekusi: SELECT ..."
"GAGAL EKSEKUSI: ... SQL 'ngawur' TIDAK akan disimpan ke cache."
```

---

### `HandleFeedbackKoreksi(w http.ResponseWriter, r *http.Request)`

**Tujuan**: Endpoint untuk menerima feedback koreksi SQL dari user

**HTTP Method**: `POST`

**Request Body**:

```json
{
  "prompt_asli": "tampilkan semua nasabah",
  "sql_koreksi": "SELECT id_nasabah, nama_lengkap FROM nasabah;"
}
```

**Flow**:

1. Handle CORS preflight (OPTIONS)
2. Validate HTTP method (harus POST)
3. Parse JSON request body
4. Validate prompt_asli dan sql_koreksi tidak kosong
5. Call `AddSqlExample()` untuk save ke database
6. Return success response

**Success Response** (201 Created):

```json
{
  "status": "sukses",
  "message": "Feedback koreksi berhasil disimpan. Silakan 'retrain' untuk menerapkan."
}
```

**Error Responses**:

- `405 Method Not Allowed`: Bukan POST request
- `400 Bad Request`: JSON invalid atau field kosong
- `500 Internal Server Error`: Database error

**Side Effects**:

- Insert new row ke tabel `rag_sql_examples`
- Log feedback yang diterima

---

### `HandleAdminRetrain(w http.ResponseWriter, r *http.Request)`

**Tujuan**: Endpoint untuk trigger retraining RAG database

**HTTP Method**: `POST`

**Flow**:

1. Validate HTTP method (harus POST)
2. Launch `mainTrain()` di goroutine (background process)
3. Immediately return 202 Accepted

**Success Response** (202 Accepted):

```json
{
  "message": "Proses retraining RAG telah dimulai di background. Periksa log server untuk status."
}
```

**Error Responses**:

- `405 Method Not Allowed`: Bukan POST request

**Side Effects**:

- Start background goroutine untuk retraining
- Retraining process akan:
  - Fetch semua DDL dari database
  - Fetch semua SQL examples dari `rag_sql_examples`
  - Embed semua content
  - Recreate Qdrant collection
  - Upsert semua vectors

**Log Output**:

```
"ADMIN: Menerima permintaan /admin/retrain..."
"ADMIN: Proses retraining RAG (Embedding) dimulai di background..."
```

---

## logic.go - Business Logic

### `GetSQL(userPrompt string) (AISqlResponse, error)`

**Tujuan**: Orchestrator untuk mendapatkan SQL query dari AI service

**Parameters**:

- `userPrompt`: Pertanyaan user dalam bahasa natural

**Return**:

- `AISqlResponse`: Struct berisi SQL, Vector, PromptAsli, IsCached
- `error`: Error jika AI service gagal

**Flow**:

1. Call `getSQLFromAI_Groq(userPrompt)`
2. Validate SQL tidak kosong
3. Return response

**Dipanggil oleh**: `HandleDynamicQuery()`

**Memanggil**: `getSQLFromAI_Groq()` di `ai_service.go`

---

### `ExecuteDynamicQuery(query string, params []interface{}) (QueryResult, error)`

**Tujuan**: Eksekusi SQL query dan return hasil dalam format JSON-friendly

**Parameters**:

- `query`: SQL query string
- `params`: Query parameters (untuk parameterized queries)

**Return**:

- `QueryResult`: Struct berisi Columns ([]string) dan Rows ([][]interface{})
- `error`: Error jika query gagal

**Flow**:

1. Determine timeout (dari config atau default 10s)
2. Create context dengan timeout
3. Execute query dengan `DbInstance.QueryContext()`
4. Scan column names
5. Iterate semua rows dan scan values
6. Build QueryResult struct

**Dipanggil oleh**: `HandleDynamicQuery()`

**Timeout**: Configurable via `QUERY_TIMEOUT_SECONDS` (default: 10 detik)

**Error Handling**:

- Context timeout jika query terlalu lama
- SQL syntax error
- Database connection error

---

### `BuildDynamicQuery(req QueryRequest) (string, []interface{}, error)`

**Tujuan**: Legacy query builder (tidak digunakan untuk NLP flow)

**Note**: Fungsi ini untuk backward compatibility, tidak digunakan dalam main NLP flow

---

## ai_service.go - AI & Vector Services

### `InitVectorService() error`

**Tujuan**: Initialize koneksi ke Google AI (Gemini) dan Qdrant

**Flow**:

1. Validate `AppConfig` sudah dimuat
2. Create Google AI client dengan API key
3. Initialize embedding model (`text-embedding-004`)
4. Create Qdrant gRPC client
5. Ensure cache collection exists (create jika belum ada)
6. Log connection details

**Return**: `error` jika initialization gagal

**Dipanggil oleh**: `main()` saat startup

**Side Effects**:

- Set global variable `qdrantClient`
- Set global variable `geminiEmbedder`
- Create Qdrant cache collection jika belum ada

**Log Output**:

```
"Memastikan collection cache 'bpr_supra_cache' ada via REST..."
"✅ Berhasil terkoneksi ke Layanan Vektor (Google AI & Qdrant)."
"   - Embedding Model: models/text-embedding-004"
"   - Qdrant gRPC: localhost:6334"
"   - Qdrant REST: http://localhost:6333"
"   - RAG Collection: bpr_supra_rag"
"   - Cache Collection: bpr_supra_cache"
```

---

### `getSQLFromAI_Groq(userPrompt string) (AISqlResponse, error)`

**Tujuan**: Core function untuk convert Natural Language ke SQL menggunakan RAG + LLM

**Parameters**:

- `userPrompt`: Pertanyaan user dalam bahasa natural

**Return**:

- `AISqlResponse`: Struct berisi:
  - `SQL`: Generated SQL query
  - `Vector`: Embedding vector dari prompt (768 dimensions)
  - `PromptAsli`: Original user prompt
  - `IsCached`: Boolean, true jika dari cache
- `error`: Error jika proses gagal

**Flow Detail**:

**STEP 1: Embedding Prompt**

```go
geminiEmbedder.EmbedContent(ctx, genai.Text(userPrompt))
// Returns: promptVector []float32 (768 dimensions)
```

**STEP 2: Semantic Cache Check**

```go
qdrantSearchPoints(ctx, QdrantURL, CacheCollection, searchReq)
// Search dengan:
// - Vector: promptVector
// - Limit: 1
// - WithPayload: true

// Jika score >= threshold (0.95):
//   ✅ CACHE HIT! Return cached SQL
// Else:
//   Continue ke RAG...
```

**STEP 3: RAG Context Retrieval**

```go
qdrantClient.Query(ctx, &pb.QueryPoints{
  CollectionName: "bpr_supra_rag",
  Query: promptVector,
  Limit: 10,
  Filter: category = "sql"  // Hanya ambil SQL examples
})
// Returns: 10 most relevant SQL examples
```

**STEP 4: Get Full DDL**

```go
GetDynamicSchemaContext()
// Returns: All CREATE TABLE statements
```

**STEP 5: Build Prompt untuk LLM**

```
Anda adalah ahli SQL PostgreSQL. Tanggal hari ini adalah 2024-01-15.

== KAMUS DATABASE (SEMUA DDL) ==
[All DDL statements here...]

== CONTOH SQL (PALING RELEVAN) ==
[Top 10 relevant SQL examples from RAG...]

Tugas Anda:
1. Berdasarkan "KAMUS DATABASE", jawab pertanyaan pengguna
2. Gunakan "CONTOH SQL" sebagai inspirasi pola
3. JANGAN pakai markdown. JANGAN tambahkan penjelasan. Hanya SQL.
4. JANGAN PERNAH menggunakan SELECT *; selalu sebutkan nama kolomnya.

Pertanyaan Pengguna: "tampilkan semua nasabah"

Query SQL:
```

**STEP 6: Call Groq LLM**

```go
HTTP POST to https://api.groq.com/openai/v1/chat/completions
Headers:
  - Authorization: Bearer {GROQ_API_KEY}
  - Content-Type: application/json
Body:
  {
    "model": "llama-3.1-8b-instant",
    "messages": [{"role": "user", "content": finalPrompt}]
  }
Timeout: 30 seconds

Response:
  {
    "choices": [{
      "message": {
        "content": "SELECT id_nasabah, nama_lengkap FROM nasabah;"
      }
    }]
  }
```

**STEP 7: Clean & Return SQL**

````go
// Remove markdown code blocks if any
sqlQuery = strings.TrimPrefix(sqlQuery, "```sql")
sqlQuery = strings.TrimSuffix(sqlQuery, "```")
sqlQuery = strings.TrimSpace(sqlQuery)
````

**Dipanggil oleh**: `GetSQL()` di `logic.go`

**Memanggil**:

- `geminiEmbedder.EmbedContent()` - Google AI
- `qdrantSearchPoints()` - Semantic cache search
- `qdrantClient.Query()` - RAG search
- `GetDynamicSchemaContext()` - Get DDL
- Groq API - Generate SQL

**Log Output**:

```
"Menerjemahkan prompt user ke vektor..."
"Mencari di Semantic Cache Qdrant (REST)..."
"✅ SEMANTIC CACHE HIT! Skor: 0.97 (Melebihi Threshold: 0.95)"
// OR
"CACHE MISS. Skor tertinggi: 0.85 (Dibawah Threshold: 0.95)"
"Memanggil RAG (gRPC) + Groq AI..."
"Mencari konteks relevan di Qdrant (RAG)..."
"Konteks RAG Dinamis berhasil dirakit."
"SQL dari AI (Dynamic RAG): SELECT ..."
```

---

### `SaveToCache(promptAsli string, promptVector []float32, sqlQuery string)`

**Tujuan**: Menyimpan hasil query yang berhasil ke semantic cache (async)

**Parameters**:

- `promptAsli`: Original user prompt
- `promptVector`: Embedding vector dari prompt
- `sqlQuery`: SQL query yang berhasil dieksekusi

**Return**: void (goroutine)

**Flow**:

1. Launch goroutine (async, tidak block response)
2. Validate `AppConfig` sudah dimuat
3. Create new point dengan UUID
4. Upsert ke Qdrant cache collection

**Point Structure**:

```go
{
  "id": "uuid-string",
  "vector": [0.123, 0.456, ...], // 768 dimensions
  "payload": {
    "prompt_asli": "tampilkan semua nasabah",
    "sql_query": "SELECT id_nasabah, nama_lengkap FROM nasabah;"
  }
}
```

**Dipanggil oleh**: `HandleDynamicQuery()` setelah SQL berhasil dieksekusi

**Memanggil**: `qdrantUpsertPoints()`

**Side Effects**: Insert new vector ke Qdrant cache collection

**Log Output**:

```
"Menyimpan hasil (yang sudah tervalidasi) ke Semantic Cache (REST)..."
"Berhasil menyimpan ke cache."
// OR
"PERINGATAN: Gagal menyimpan ke cache Qdrant: ..."
```

**Note**: Async operation, error tidak akan menghentikan response ke user

---

### `qdrantCreateCollection(ctx context.Context, baseURL, name string, size int, distance string) error`

**Tujuan**: Create Qdrant collection via REST API (idempotent)

**Parameters**:

- `ctx`: Context
- `baseURL`: Qdrant REST URL (e.g., "http://localhost:6333")
- `name`: Collection name
- `size`: Vector dimension size (768)
- `distance`: Distance metric ("Cosine", "Euclid", "Dot")

**Return**: `error` jika gagal create (kecuali jika already exists)

**HTTP Request**:

```
PUT {baseURL}/collections/{name}
Body:
{
  "vectors": {
    "size": 768,
    "distance": "Cosine"
  }
}
```

**Behavior**:

- Jika collection belum ada → Create new collection
- Jika collection sudah ada → Return success (idempotent)

**Dipanggil oleh**:

- `InitVectorService()` - Create cache collection
- `mainTrain()` - Create/recreate RAG collection

---

### `qdrantUpsertPoints(ctx context.Context, baseURL, name string, points []qdrantPoint) error`

**Tujuan**: Insert/update vectors ke Qdrant collection via REST API

**Parameters**:

- `ctx`: Context
- `baseURL`: Qdrant REST URL
- `name`: Collection name
- `points`: Array of points to upsert

**Return**: `error` jika upsert gagal

**HTTP Request**:

```
PUT {baseURL}/collections/{name}/points?wait=true
Body:
{
  "points": [
    {
      "id": "uuid-1",
      "vector": [0.1, 0.2, ...],
      "payload": {"content": "...", "category": "sql"}
    },
    ...
  ]
}
```

**Dipanggil oleh**:

- `SaveToCache()` - Save single cache point
- `mainTrain()` - Bulk upsert training data

**Note**: `wait=true` ensures operation completes before returning

---

### `qdrantSearchPoints(ctx context.Context, baseURL, name string, req qdrantSearchReq) (qdrantSearchResp, error)`

**Tujuan**: Search vectors by similarity via REST API

**Parameters**:

- `ctx`: Context
- `baseURL`: Qdrant REST URL
- `name`: Collection name
- `req`: Search request dengan vector, limit, threshold

**Return**:

- `qdrantSearchResp`: Search results dengan score dan payload
- `error`: Error jika search gagal

**HTTP Request**:

```
POST {baseURL}/collections/{name}/points/search
Body:
{
  "vector": [0.1, 0.2, ...],
  "limit": 1,
  "with_payload": true,
  "score_threshold": 0.0
}
```

**Response**:

```json
{
  "result": [
    {
      "id": "uuid",
      "version": 1,
      "score": 0.97,
      "payload": {
        "prompt_asli": "...",
        "sql_query": "..."
      }
    }
  ],
  "status": "ok",
  "time": 0.002
}
```

**Dipanggil oleh**: `getSQLFromAI_Groq()` untuk semantic cache check

---

## schema_service.go - Schema Management

### `getSchemaFromConnStr() (string, error)`

**Tujuan**: Extract schema name dari DB connection string

**Return**:

- `string`: Schema name (e.g., "bpr_supra")
- `error`: Error jika parsing gagal atau search_path tidak ada

**Flow**:

1. Read `DB_CONN_STRING` dari environment
2. Parse URL
3. Extract `search_path` query parameter
4. Return schema name

**Example Connection String**:

```
postgres://user:pass@localhost:5432/db?sslmode=disable&search_path=bpr_supra
                                                         ^^^^^^^^^^^^^^^^^^^^
                                                         Extracted: "bpr_supra"
```

**Dipanggil oleh**:

- `GetDynamicSchemaContext()`
- `GetDynamicSqlExamples()`
- `AddSqlExample()`

**Error Messages**:

```
"DB_CONN_STRING tidak ditemukan di .env"
"gagal parse DB_CONN_STRING: ..."
"search_path tidak ditemukan di DB_CONN_STRING"
```

---

### `GetDynamicSchemaContext() ([]string, error)`

**Tujuan**: Fetch semua DDL (CREATE TABLE statements) dari database secara dinamis

**Return**:

- `[]string`: Array of DDL strings (satu per tabel)
- `error`: Error jika query gagal

**Flow**:

1. Get schema name dari connection string
2. Query `information_schema.columns` untuk schema tersebut
3. Group columns by table_name
4. Build CREATE TABLE statement untuk setiap tabel
5. Return array of DDL strings

**SQL Query**:

```sql
SELECT
  table_name,
  column_name,
  data_type
FROM
  information_schema.columns
WHERE
  table_schema = $1
ORDER BY
  table_name,
  ordinal_position;
```

**Output Example**:

```go
[
  "CREATE TABLE nasabah (\n    id_nasabah character varying,\n    nama_lengkap character varying,\n    alamat text\n);",
  "CREATE TABLE rekening (\n    id_rekening character varying,\n    id_nasabah character varying,\n    saldo numeric\n);",
  ...
]
```

**Dipanggil oleh**:

- `getSQLFromAI_Groq()` - Untuk build prompt LLM
- `mainTrain()` - Untuk training RAG

**Log Output**:

```
"Mulai mengambil skema DDL dinamis dari database..."
"✅ Berhasil! Mengambil 15 potongan DDL dinamis."
```

---

### `GetDynamicSqlExamples() ([]string, error)`

**Tujuan**: Fetch semua SQL examples dari tabel `rag_sql_examples`

**Return**:

- `[]string`: Array of SQL example strings
- `error`: Error jika query gagal

**Flow**:

1. Get schema name dari connection string
2. Query tabel `{schema}.rag_sql_examples`
3. Combine prompt_example + sql_example
4. Return array of examples

**SQL Query**:

```sql
SELECT
  prompt_example,
  sql_example
FROM
  {schema}.rag_sql_examples
ORDER BY
  id;
```

**Output Example**:

```go
[
  "-- Pertanyaan: \"tampilkan semua nasabah\"\nSELECT id_nasabah, nama_lengkap FROM nasabah;",
  "-- Pertanyaan: \"ada berapa nasabah?\"\nSELECT COUNT(*) AS jumlah_nasabah FROM nasabah;",
  ...
]
```

**Dipanggil oleh**: `mainTrain()` untuk training RAG

**Log Output**:

```
"Mulai mengambil contoh SQL dinamis dari tabel 'rag_sql_example'..."
"✅ Berhasil! mengambil 25 contoh SQL dinamis."
// OR
"PERINGATAN: Tidak ada contoh SQL ditemukan di tabel 'rag_sql_examples'."
```

---

### `AddSqlExample(promptAsli string, sqlKoreksi string) error`

**Tujuan**: Simpan feedback koreksi SQL ke database

**Parameters**:

- `promptAsli`: Original user prompt
- `sqlKoreksi`: Corrected SQL query

**Return**: `error` jika insert gagal

**Flow**:

1. Get schema name dari connection string
2. Format prompt sebagai comment
3. INSERT ke tabel `{schema}.rag_sql_examples`
4. Log success

**SQL Query**:

```sql
INSERT INTO {schema}.rag_sql_examples
  (prompt_example, sql_example)
VALUES
  ($1, $2)
```

**Example Data**:

```
prompt_example: "-- Pertanyaan: \"tampilkan semua nasabah\""
sql_example: "SELECT id_nasabah, nama_lengkap FROM nasabah;"
```

**Dipanggil oleh**: `HandleFeedbackKoreksi()`

**Side Effects**: Insert new row ke database

**Timeout**: 5 seconds

**Log Output**:

```
"✅ Berhasil! Menyimpan contekan baru ke 'rag_sql_examples' untuk prompt: tampilkan semua nasabah"
```

---

## train.go - Training Module

### `mainTrain()`

**Tujuan**: Retrain RAG database dengan embedding semua DDL dan SQL examples

**Flow**:

**STEP 1: Setup**

```go
godotenv.Load()           // Load .env
LoadConfig()              // Load configuration
ConnectDB()               // Connect to database
```

**STEP 2: Initialize AI Client**

```go
genai.NewClient(ctx, option.WithAPIKey(GoogleAPIKey))
embedder := geminiClient.EmbeddingModel(EmbeddingModel)
```

**STEP 3: Create/Recreate Qdrant Collection**

```go
qdrantCreateCollection(ctx, QdrantURL, QdrantCollectionName, 768, "Cosine")
// This will recreate the collection (delete old data)
```

**STEP 4: Fetch Training Data**

```go
dynamicDDLs := GetDynamicSchemaContext()
// Returns: ["CREATE TABLE ...", "CREATE TABLE ...", ...]

dynamicSQLExamples := GetDynamicSqlExamples()
// Returns: ["-- Pertanyaan: ...\nSELECT ...", ...]
```

**STEP 5: Embed DDL (category="ddl")**

```go
for each DDL:
  vector := embedder.EmbedContent(ctx, genai.Text(ddl))
  point := {
    id: uuid,
    vector: vector,
    payload: {
      content: ddl,
      category: "ddl"
    }
  }
  points.append(point)
```

**STEP 6: Embed SQL Examples (category="sql")**

```go
for each SQL example:
  vector := embedder.EmbedContent(ctx, genai.Text(sql))
  point := {
    id: uuid,
    vector: vector,
    payload: {
      content: sql,
      category: "sql"
    }
  }
  points.append(point)
```

**STEP 7: Bulk Upsert to Qdrant**

```go
qdrantUpsertPoints(ctx, QdrantURL, QdrantCollectionName, points)
```

**Dipanggil oleh**: `HandleAdminRetrain()` di goroutine

**Side Effects**:

- Recreate Qdrant RAG collection (data lama hilang)
- Insert semua vectors baru

**Log Output**:

```
"Memulai proses 'Training' Database Vektor..."
"✅ Konfigurasi berhasil dimuat"
"Koneksi DB Postgres untuk baca skema... OK."
"Koneksi 'Penerjemah' (Google AI)... OK. Model: models/text-embedding-004"
"Membuat koleksi baru 'bpr_supra_rag'..."
"Mulai 'melatih' (meng-embed dan menyimpan) contekan..."
"Memproses 15 DDL..."
"Memproses 25 Contoh SQL..."
"Menyimpan semua vektor ke Qdrant via REST..."
"-----------------------------------------------"
"✅ 'Training' selesai! Database Vektor 'bpr_supra_rag' sudah terisi (Dinamis)."
"-----------------------------------------------"
```

**Duration**: Tergantung jumlah data (biasanya 30-60 detik untuk ~40 items)

**Note**:

- Process berjalan di background (goroutine)
- Tidak mengganggu query yang sedang berjalan
- Setelah selesai, query berikutnya akan menggunakan data baru

---

## Data Structures (models.go)

### `Config` struct

```go
type Config struct {
  // Database
  DBConnString      string
  DBMaxOpenConns    int
  DBMaxIdleConns    int
  DBConnMaxLifetime time.Duration
  DBPingTimeout     time.Duration

  // Groq AI
  GroqAPIKey  string
  GroqModel   string
  GroqAPIURL  string
  GroqTimeout time.Duration

  // Google AI
  GoogleAPIKey        string
  EmbeddingModel      string
  EmbeddingVectorSize int

  // Qdrant
  QdrantGRPCHost        string
  QdrantGRPCPort        int
  QdrantURL             string
  QdrantCollectionName  string
  QdrantCacheCollection string
  QdrantDistanceMetric  string
  QdrantTimeout         time.Duration

  // Cache
  CacheSimilarityThreshold float32
  CacheSearchLimit         uint64

  // RAG
  RAGSearchLimit uint64

  // Server
  ServerPort string
  ServerHost string

  // Query
  QueryTimeout time.Duration

  // Environment
  AppEnv string
  Debug  bool
}
```

### `QueryResult` struct

```go
type QueryResult struct {
  Columns []string        `json:"columns"`
  Rows    [][]interface{} `json:"rows"`
}
```

### `AISqlResponse` struct

```go
type AISqlResponse struct {
  SQL        string      // Generated SQL query
  Vector     []float32   // Embedding vector (768D)
  PromptAsli string      // Original user prompt
  IsCached   bool        // True if from cache
}
```

### `PromptRequest` struct

```go
type PromptRequest struct {
  Prompt string `json:"prompt"`
}
```

### `FeedbackRequest` struct

```go
type FeedbackRequest struct {
  PromptAsli string `json:"prompt_asli"`
  SqlKoreksi string `json:"sql_koreksi"`
}
```

---

## Global Variables

### `AppConfig *Config`

- **File**: `config.go`
- **Tujuan**: Menyimpan konfigurasi aplikasi yang sudah dimuat
- **Diset oleh**: `LoadConfig()`
- **Digunakan oleh**: Semua fungsi yang membutuhkan konfigurasi

### `DbInstance *sql.DB`

- **File**: `database.go`
- **Tujuan**: Database connection pool
- **Diset oleh**: `ConnectDB()`
- **Digunakan oleh**: Semua fungsi yang query database

### `qdrantClient *pb.Client`

- **File**: `ai_service.go`
- **Tujuan**: Qdrant gRPC client untuk RAG search
- **Diset oleh**: `InitVectorService()`
- **Digunakan oleh**: `getSQLFromAI_Groq()` untuk RAG query

### `geminiEmbedder *genai.EmbeddingModel`

- **File**: `ai_service.go`
- **Tujuan**: Google AI embedding model
- **Diset oleh**: `InitVectorService()`
- **Digunakan oleh**: `getSQLFromAI_Groq()` dan `mainTrain()` untuk embedding

---

**Dokumentasi ini mencakup semua fungsi utama dalam sistem Go Bank API dengan detail parameter, return values, flow, dan contoh penggunaan.**
