# Use Cases & Examples - Go Bank API

Dokumentasi lengkap use cases dan contoh penggunaan sistem.

---

## 📋 Daftar Isi

1. [Use Case 1: Query Natural Language (Cache Miss)](#use-case-1-query-natural-language-cache-miss)
2. [Use Case 2: Query Natural Language (Cache Hit)](#use-case-2-query-natural-language-cache-hit)
3. [Use Case 3: Feedback & Correction](#use-case-3-feedback--correction)
4. [Use Case 4: Retraining RAG](#use-case-4-retraining-rag)
5. [Use Case 5: Pindah Database/Schema](#use-case-5-pindah-databaseschema)

---

## Use Case 1: Query Natural Language (Cache Miss)

### Scenario

User bertanya untuk pertama kalinya, belum ada di cache.

### Request

```bash
curl -X POST http://localhost:8080/api/query \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "tampilkan semua nasabah"
  }'
```

### Flow Execution

**1. Handler menerima request**

```
HandleDynamicQuery() receives POST /api/query
Prompt (normalized): "tampilkan semua nasabah"
```

**2. Embedding prompt**

```
geminiEmbedder.EmbedContent("tampilkan semua nasabah")
→ Returns: [0.123, 0.456, ..., 0.789] (768 dimensions)
```

**3. Semantic cache check**

```
qdrantSearchPoints(cache_collection, vector)
→ Result: []
→ CACHE MISS. Tidak ada item cache yang cocok ditemukan.
```

**4. RAG search**

```
qdrantClient.Query(rag_collection, vector, filter: category="sql", limit: 10)
→ Returns top 10 relevant SQL examples:
  1. "-- Pertanyaan: \"tampilkan semua nasabah\"\nSELECT id_nasabah, nama_lengkap FROM nasabah;"
  2. "-- Pertanyaan: \"ada berapa nasabah?\"\nSELECT COUNT(*) FROM nasabah;"
  ...
```

**5. Get DDL**

```
GetDynamicSchemaContext()
→ Returns all CREATE TABLE statements from information_schema
```

**6. Build prompt & call LLM**

```
Groq API receives:
{
  "model": "llama-3.1-8b-instant",
  "messages": [{
    "role": "user",
    "content": "Anda adalah ahli SQL PostgreSQL...\n\n== KAMUS DATABASE ==\n...\n\n== CONTOH SQL ==\n...\n\nPertanyaan: \"tampilkan semua nasabah\""
  }]
}

Groq API returns:
{
  "choices": [{
    "message": {
      "content": "SELECT id_nasabah, nama_lengkap, alamat, tanggal_lahir FROM nasabah;"
    }
  }]
}
```

**7. Execute SQL**

```
ExecuteDynamicQuery("SELECT id_nasabah, nama_lengkap, alamat, tanggal_lahir FROM nasabah;")
→ Query executed successfully
```

**8. Save to cache (async)**

```
SaveToCache("tampilkan semua nasabah", vector, "SELECT ...")
→ Background goroutine saves to Qdrant cache collection
```

### Response

```json
{
  "columns": ["id_nasabah", "nama_lengkap", "alamat", "tanggal_lahir"],
  "rows": [
    ["CIF00001", "Budi Santoso", "Jl. Merdeka No. 1", "1980-05-15"],
    ["CIF00002", "Ani Wijaya", "Jl. Sudirman No. 2", "1985-08-20"],
    ["CIF00003", "Citra Dewi", "Jl. Gatot Subroto No. 3", "1990-12-10"]
  ]
}
```

### Log Output

```
Menerima Prompt (Normalized): tampilkan semua nasabah
Memanggil AI Service (dengan semantic cache)...
Menerjemahkan prompt user ke vektor...
Mencari di Semantic Cache Qdrant (REST)...
CACHE MISS. Tidak ada item cache yang cocok ditemukan.
Memanggil RAG (gRPC) + Groq AI...
Mencari konteks relevan di Qdrant (RAG)...
Konteks RAG Dinamis berhasil dirakit.
SQL dari AI (Dynamic RAG): SELECT id_nasabah, nama_lengkap, alamat, tanggal_lahir FROM nasabah;
SQL yang akan dieksekusi: SELECT id_nasabah, nama_lengkap, alamat, tanggal_lahir FROM nasabah;
Menyimpan hasil (yang sudah tervalidasi) ke Semantic Cache (REST)...
Berhasil menyimpan ke cache.
```

### Performance

- **Total time**: ~2-3 seconds
  - Embedding: ~200ms
  - Cache search: ~50ms
  - RAG search: ~100ms
  - DDL fetch: ~100ms
  - Groq LLM: ~1-2s
  - SQL execution: ~50ms
  - Cache save (async): doesn't block response

---

## Use Case 2: Query Natural Language (Cache Hit)

### Scenario

User bertanya dengan pertanyaan yang mirip dengan sebelumnya (similarity ≥ 0.95).

### Request

```bash
curl -X POST http://localhost:8080/api/query \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "tampilkan seluruh nasabah"
  }'
```

### Flow Execution

**1. Handler menerima request**

```
HandleDynamicQuery() receives POST /api/query
Prompt (normalized): "tampilkan seluruh nasabah"
```

**2. Embedding prompt**

```
geminiEmbedder.EmbedContent("tampilkan seluruh nasabah")
→ Returns: [0.125, 0.458, ..., 0.791] (768 dimensions)
```

**3. Semantic cache check**

```
qdrantSearchPoints(cache_collection, vector)
→ Result: [
  {
    "score": 0.97,
    "payload": {
      "prompt_asli": "tampilkan semua nasabah",
      "sql_query": "SELECT id_nasabah, nama_lengkap, alamat, tanggal_lahir FROM nasabah;"
    }
  }
]
→ ✅ SEMANTIC CACHE HIT! Skor: 0.97 (Melebihi Threshold: 0.95)
```

**4. Skip RAG & LLM, use cached SQL**

```
SQL from cache: "SELECT id_nasabah, nama_lengkap, alamat, tanggal_lahir FROM nasabah;"
```

**5. Execute SQL**

```
ExecuteDynamicQuery("SELECT id_nasabah, nama_lengkap, alamat, tanggal_lahir FROM nasabah;")
→ Query executed successfully
```

**6. Skip cache save (already cached)**

### Response

```json
{
  "columns": ["id_nasabah", "nama_lengkap", "alamat", "tanggal_lahir"],
  "rows": [
    ["CIF00001", "Budi Santoso", "Jl. Merdeka No. 1", "1980-05-15"],
    ["CIF00002", "Ani Wijaya", "Jl. Sudirman No. 2", "1985-08-20"],
    ["CIF00003", "Citra Dewi", "Jl. Gatot Subroto No. 3", "1990-12-10"]
  ]
}
```

### Log Output

```
Menerima Prompt (Normalized): tampilkan seluruh nasabah
Memanggil AI Service (dengan semantic cache)...
Menerjemahkan prompt user ke vektor...
Mencari di Semantic Cache Qdrant (REST)...
✅ SEMANTIC CACHE HIT! Skor: 0.97 (Melebihi Threshold: 0.95)
SQL yang akan dieksekusi: SELECT id_nasabah, nama_lengkap, alamat, tanggal_lahir FROM nasabah;
```

### Performance

- **Total time**: ~300-500ms (6x lebih cepat!)
  - Embedding: ~200ms
  - Cache search: ~50ms
  - SQL execution: ~50ms
  - **No RAG search, no LLM call!**

### Benefits

- Drastically faster response time
- Reduced API costs (no Groq call)
- Consistent results for similar questions

---

## Use Case 3: Feedback & Correction

### Scenario

AI menghasilkan SQL yang salah, user memberikan koreksi.

### Request

```bash
curl -X POST http://localhost:8080/api/feedback/koreksi \
  -H "Content-Type: application/json" \
  -d '{
    "prompt_asli": "tampilkan nasabah yang lahir tahun 1980",
    "sql_koreksi": "SELECT id_nasabah, nama_lengkap, tanggal_lahir FROM nasabah WHERE EXTRACT(YEAR FROM tanggal_lahir) = 1980;"
  }'
```

### Flow Execution

**1. Handler menerima feedback**

```
HandleFeedbackKoreksi() receives POST /api/feedback/koreksi
prompt_asli: "tampilkan nasabah yang lahir tahun 1980"
sql_koreksi: "SELECT id_nasabah, nama_lengkap, tanggal_lahir FROM nasabah WHERE EXTRACT(YEAR FROM tanggal_lahir) = 1980;"
```

**2. Format prompt sebagai comment**

```
prompt_example = "-- Pertanyaan: \"tampilkan nasabah yang lahir tahun 1980\""
sql_example = "SELECT id_nasabah, nama_lengkap, tanggal_lahir FROM nasabah WHERE EXTRACT(YEAR FROM tanggal_lahir) = 1980;"
```

**3. Save to database**

```
INSERT INTO bpr_supra.rag_sql_examples (prompt_example, sql_example)
VALUES (
  '-- Pertanyaan: "tampilkan nasabah yang lahir tahun 1980"',
  'SELECT id_nasabah, nama_lengkap, tanggal_lahir FROM nasabah WHERE EXTRACT(YEAR FROM tanggal_lahir) = 1980;'
);
```

### Response

```json
{
  "status": "sukses",
  "message": "Feedback koreksi berhasil disimpan. Silakan 'retrain' untuk menerapkan."
}
```

### Log Output

```
Menerima feedback koreksi...
✅ Berhasil! Menyimpan contekan baru ke 'rag_sql_examples' untuk prompt: tampilkan nasabah yang lahir tahun 1980
```

### Next Step

User harus call `/admin/retrain` untuk memasukkan koreksi ini ke RAG database.

---

## Use Case 4: Retraining RAG

### Scenario

Setelah menambahkan feedback koreksi, admin trigger retraining untuk update RAG database.

### Request

```bash
curl -X POST http://localhost:8080/admin/retrain
```

### Flow Execution

**1. Handler menerima request**

```
HandleAdminRetrain() receives POST /admin/retrain
Launch mainTrain() in background goroutine
```

**2. Immediate response (202 Accepted)**

```json
{
  "message": "Proses retraining RAG telah dimulai di background. Periksa log server untuk status."
}
```

**3. Background process starts**

**Step 1: Load config & connect DB**

```
godotenv.Load()
LoadConfig()
ConnectDB()
```

**Step 2: Initialize Google AI client**

```
genai.NewClient(ctx, option.WithAPIKey(GoogleAPIKey))
embedder := geminiClient.EmbeddingModel("text-embedding-004")
```

**Step 3: Recreate Qdrant collection**

```
DELETE collection "bpr_supra_rag" (if exists)
CREATE collection "bpr_supra_rag" with:
  - vector_size: 768
  - distance: Cosine
```

**Step 4: Fetch training data**

```
GetDynamicSchemaContext()
→ Returns 15 DDL statements

GetDynamicSqlExamples()
→ Returns 26 SQL examples (including new correction!)
```

**Step 5: Embed DDL**

```
For each DDL (15 items):
  vector = embedder.EmbedContent(ddl)
  points.append({
    id: uuid,
    vector: vector,
    payload: {content: ddl, category: "ddl"}
  })
```

**Step 6: Embed SQL examples**

```
For each SQL example (26 items):
  vector = embedder.EmbedContent(sql)
  points.append({
    id: uuid,
    vector: vector,
    payload: {content: sql, category: "sql"}
  })
```

**Step 7: Bulk upsert to Qdrant**

```
qdrantUpsertPoints(rag_collection, points)
→ 41 vectors inserted (15 DDL + 26 SQL)
```

### Log Output

```
ADMIN: Menerima permintaan /admin/retrain...
ADMIN: Proses retraining RAG (Embedding) dimulai di background...
Memulai proses 'Training' Database Vektor...
✅ Konfigurasi berhasil dimuat
Koneksi DB Postgres untuk baca skema... OK.
Koneksi 'Penerjemah' (Google AI)... OK. Model: models/text-embedding-004
Membuat koleksi baru 'bpr_supra_rag'...
Mulai 'melatih' (meng-embed dan menyimpan) contekan...
Memproses 15 DDL...
Memproses 26 Contoh SQL...
Menyimpan semua vektor ke Qdrant via REST...
-----------------------------------------------
✅ 'Training' selesai! Database Vektor 'bpr_supra_rag' sudah terisi (Dinamis).
-----------------------------------------------
```

### Performance

- **Total time**: ~30-60 seconds (depends on data size)
- **Process**: Runs in background, doesn't block API
- **Effect**: Next query will use updated RAG data

### Result

Sekarang ketika user bertanya "tampilkan nasabah yang lahir tahun 1980", AI akan menggunakan SQL yang sudah dikoreksi sebagai referensi.

---

## Use Case 5: Pindah Database/Schema

### Scenario

Developer ingin pindah dari schema `bpr_supra` ke schema `bpr_mandiri` atau database berbeda.

### Steps

**1. Update `.env` file**

```bash
# OLD:
DB_CONN_STRING=postgres://user:pass@localhost:5432/bank_db?sslmode=disable&search_path=bpr_supra

# NEW:
DB_CONN_STRING=postgres://user:pass@localhost:5432/bank_db?sslmode=disable&search_path=bpr_mandiri
```

**2. Update Qdrant collection names (optional)**

```bash
# OLD:
QDRANT_COLLECTION_NAME=bpr_supra_rag
QDRANT_CACHE_COLLECTION=bpr_supra_cache

# NEW:
QDRANT_COLLECTION_NAME=bpr_mandiri_rag
QDRANT_CACHE_COLLECTION=bpr_mandiri_cache
```

**3. Restart aplikasi**

```bash
# Stop current process (Ctrl+C)
# Start again
go run .
```

**4. Run initial training**

```bash
curl -X POST http://localhost:8080/admin/retrain
```

### What Happens

**1. Application startup**

```
LoadConfig() reads new DB_CONN_STRING
ConnectDB() connects to new schema
InitVectorService() creates new Qdrant collections
```

**2. Retraining process**

```
GetDynamicSchemaContext() reads DDL from bpr_mandiri schema
GetDynamicSqlExamples() reads SQL examples from bpr_mandiri.rag_sql_examples
Embed all data and save to bpr_mandiri_rag collection
```

**3. Ready to use**

```
All queries now work with bpr_mandiri schema
No code changes needed!
```

### Benefits

- **Zero code changes**: Hanya ubah environment variables
- **Dynamic schema detection**: Otomatis baca struktur database baru
- **Isolated collections**: Setiap schema punya RAG & cache sendiri
- **Easy rollback**: Tinggal ubah `.env` kembali

### Example: Multiple Environments

**Development (.env.dev)**

```bash
DB_CONN_STRING=postgres://dev:dev@localhost:5432/bank_dev?search_path=bpr_dev
QDRANT_COLLECTION_NAME=bpr_dev_rag
QDRANT_CACHE_COLLECTION=bpr_dev_cache
```

**Staging (.env.staging)**

```bash
DB_CONN_STRING=postgres://staging:staging@staging-db:5432/bank_staging?search_path=bpr_staging
QDRANT_COLLECTION_NAME=bpr_staging_rag
QDRANT_CACHE_COLLECTION=bpr_staging_cache
```

**Production (.env.prod)**

```bash
DB_CONN_STRING=postgres://prod:prod@prod-db:5432/bank_prod?search_path=bpr_supra
QDRANT_COLLECTION_NAME=bpr_supra_rag
QDRANT_CACHE_COLLECTION=bpr_supra_cache
```

**Switch environment**:

```bash
# Development
cp .env.dev .env
go run .

# Staging
cp .env.staging .env
go run .

# Production
cp .env.prod .env
go run .
```

---

## Advanced Use Cases

### Use Case 6: Complex Query with Aggregation

**Request**:

```bash
curl -X POST http://localhost:8080/api/query \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "berapa total saldo semua rekening per nasabah?"
  }'
```

**AI Generated SQL**:

```sql
SELECT
  n.id_nasabah,
  n.nama_lengkap,
  SUM(r.saldo) AS total_saldo
FROM
  nasabah n
  JOIN rekening r ON n.id_nasabah = r.id_nasabah
GROUP BY
  n.id_nasabah, n.nama_lengkap
ORDER BY
  total_saldo DESC;
```

**Response**:

```json
{
  "columns": ["id_nasabah", "nama_lengkap", "total_saldo"],
  "rows": [
    ["CIF00001", "Budi Santoso", 150000000],
    ["CIF00002", "Ani Wijaya", 75000000],
    ["CIF00003", "Citra Dewi", 50000000]
  ]
}
```

---

### Use Case 7: Date Range Query

**Request**:

```bash
curl -X POST http://localhost:8080/api/query \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "tampilkan transaksi bulan januari 2024"
  }'
```

**AI Generated SQL**:

```sql
SELECT
  id_transaksi,
  tanggal_transaksi,
  jenis_transaksi,
  nominal
FROM
  transaksi
WHERE
  tanggal_transaksi >= '2024-01-01'
  AND tanggal_transaksi < '2024-02-01'
ORDER BY
  tanggal_transaksi DESC;
```

---

**Dokumentasi ini mencakup semua use cases utama dan contoh penggunaan sistem Go Bank API.**
