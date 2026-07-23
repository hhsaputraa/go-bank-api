# Go Bank API - RAG-Powered SQL Query System

> **Version**: 2.0 (Refactored)
> **Last Updated**: 2026-01-05
> **Status**: ✅ Production Ready

API backend untuk sistem query database menggunakan Natural Language Processing dengan RAG (Retrieval-Augmented Generation) dan Semantic Caching.

## 🚀 Fitur Utama

### Core Features

- **Natural Language to SQL**: Konversi pertanyaan bahasa natural menjadi SQL query menggunakan AI
- **RAG (Retrieval-Augmented Generation)**: Menggunakan vector database untuk meningkatkan akurasi query
- **Semantic Caching**: Cache hasil query berdasarkan similarity untuk performa lebih cepat
- **Dynamic Schema Detection**: Otomatis membaca struktur database
- **Feedback System**: Sistem koreksi untuk meningkatkan akurasi AI
- **JWT Authentication**: Secure authentication dengan session management

### New in v2.0 ✨

- **Middleware Pattern**: CORS, Logging, Rate Limiting
- **Graceful Shutdown**: Clean exit dengan signal handling (SIGINT/SIGTERM)
- **Multi-Origin CORS**: Support multiple frontend URLs
- **Rate Limiting**: Protection untuk admin endpoints (10 req/min)
- **Constants Management**: No more magic numbers
- **Structured Logging**: HTTP request logging dengan metrics

## 📋 Prerequisites

- **Go 1.24+** atau lebih tinggi
- **Oracle 10g+** database
- **Qdrant** vector database (local atau cloud)
- **API Keys**:
  - Groq API Key (untuk LLM) - [Get here](https://console.groq.com)
  - Google AI API Key (untuk embeddings) - [Get here](https://aistudio.google.com/apikey)
  - Qdrant API Key (jika pakai cloud) - [Get here](https://cloud.qdrant.io)

## ⚙️ Setup & Konfigurasi

### 1. Clone Repository

```bash
git clone <repository-url>
cd go-bank-api
```

### 2. Setup Environment Variables

Copy file `.env.example` menjadi `.env`:

```bash
cp .env.example .env
```

Edit file `.env` dan isi dengan kredensial Anda:

```env
# Database Configuration
DB_CONN_STRING="oracle://username:password@localhost:1521/databaseai"

# API Keys
GROQ_API_KEY="your_groq_api_key_here"
GOOGLE_API_KEY="your_google_api_key_here"
QDRANT_API_KEY="your_qdrant_api_key_here"

# Security (IMPORTANT!)
JWT_SECRET="your_random_secret_here"  # Generate: openssl rand -hex 32
AES_KEY="your_32_char_key_here"       # Generate: openssl rand -hex 16

# CORS Configuration (NEW in v2.0)
FRONTEND_URL="http://localhost:5173"  # Or multiple: http://localhost:3084,http://localhost:5173

# Server Configuration
SERVER_PORT=8097
SERVER_HOST=localhost
```

**⚠️ IMPORTANT**:

- Jangan commit file `.env` ke repository!
- Generate JWT_SECRET dan AES_KEY yang random
- AES_KEY harus tepat 32 karakter

### 3. Install Dependencies

```bash
go mod download
```

### 4. Setup Oracle Database

Pastikan Oracle Database 10g atau lebih tinggi sudah terinstall dan berjalan. Anda perlu:

- Oracle Database instance yang accessible
- User/schema dengan akses ke tabel yang diperlukan
- Oracle Instant Client (untuk development lokal)

### 5. Setup Qdrant Vector Database

Install dan jalankan Qdrant:

```bash
# Menggunakan Docker
docker run -p 6333:6333 -p 6334:6334 qdrant/qdrant
```

### 6. Build & Jalankan Aplikasi

**Build**:

```bash
go build -o bin/app.exe .
```

**Run**:

```bash
.\bin\app.exe
```

**Expected Output**:

```
Berhasil memuat file .env
✅ Konfigurasi berhasil dimuat dari environment variables
✅ Berhasil terkoneksi ke database ORACLE 10G!
   - Max Open Connections: 25
   - Max Idle Connections: 10
   - Connection Max Lifetime: 5m0s
✅ Berhasil terkoneksi ke Layanan Vektor (Google AI & Qdrant).
Aplikasi siap berjalan...
🚀 Server berjalan di http://localhost:8097
```

**Graceful Shutdown**:
Tekan `Ctrl+C` untuk shutdown. Server akan:

- Stop menerima request baru
- Wait untuk in-flight requests selesai (max 30s)
- Clean up resources
- Exit gracefully

```
🛑 Shutting down server...
✅ Server exited gracefully
```

## 🔧 Environment Variables

Berikut adalah daftar lengkap environment variables yang tersedia:

### Database Configuration

| Variable                       | Default    | Deskripsi                                                                     |
| ------------------------------ | ---------- | ----------------------------------------------------------------------------- |
| `DB_CONN_STRING`               | _required_ | Oracle connection string: `oracle://username:password@host:port/service_name` |
| `DB_MAX_OPEN_CONNS`            | `25`       | Maximum number of open connections                                            |
| `DB_MAX_IDLE_CONNS`            | `10`       | Maximum number of idle connections                                            |
| `DB_CONN_MAX_LIFETIME_MINUTES` | `5`        | Connection max lifetime (minutes)                                             |
| `DB_PING_TIMEOUT_SECONDS`      | `5`        | Database ping timeout (seconds)                                               |

### AI Service Configuration (Groq)

| Variable               | Default                                           | Deskripsi                      |
| ---------------------- | ------------------------------------------------- | ------------------------------ |
| `GROQ_API_KEY`         | _required_                                        | Groq API key untuk LLM service |
| `GROQ_MODEL`           | `qwen/qwen3.6-27b`                                | Model yang digunakan           |
| `GROQ_API_URL`         | `https://api.groq.com/openai/v1/chat/completions` | Groq API endpoint              |
| `GROQ_TIMEOUT_SECONDS` | `30`                                              | HTTP timeout untuk Groq API    |

### Ollama Configuration (Optional)

| Variable       | Default                  | Deskripsi                |
| -------------- | ------------------------ | ------------------------ |
| `OLLAMA_URL`   | `http://localhost:11434` | Ollama server URL        |
| `OLLAMA_MODEL` | `gemma3:4b`              | Local LLM model fallback |

### Google AI Configuration (Embedding)

| Variable                | Default                     | Deskripsi                         |
| ----------------------- | --------------------------- | --------------------------------- |
| `GOOGLE_API_KEY`        | _required_                  | Google AI API key untuk embedding |
| `EMBEDDING_MODEL`       | `models/text-embedding-004` | Model embedding yang digunakan    |
| `EMBEDDING_VECTOR_SIZE` | `768`                       | Dimensi vector embedding          |

### Qdrant Vector Database

| Variable                  | Default                 | Deskripsi                           |
| ------------------------- | ----------------------- | ----------------------------------- |
| `QDRANT_GRPC_HOST`        | `localhost`             | Qdrant gRPC host                    |
| `QDRANT_GRPC_PORT`        | `6334`                  | Qdrant gRPC port                    |
| `QDRANT_URL`              | `http://localhost:6333` | Qdrant REST API URL                 |
| `QDRANT_API_KEY`          | _(empty for local)_     | Qdrant API key (required for cloud) |
| `QDRANT_COLLECTION_NAME`  | `bpr_supra_rag`         | Collection name untuk RAG           |
| `QDRANT_CACHE_COLLECTION` | `bpr_supra_cache`       | Collection name untuk cache         |
| `QDRANT_DISTANCE_METRIC`  | `Cosine`                | Distance metric (Cosine/Euclid/Dot) |
| `QDRANT_TIMEOUT_SECONDS`  | `60`                    | HTTP timeout untuk Qdrant           |

### Semantic Cache Configuration

| Variable                     | Default | Deskripsi                           |
| ---------------------------- | ------- | ----------------------------------- |
| `CACHE_SIMILARITY_THRESHOLD` | `0.95`  | Threshold untuk cache hit (0.0-1.0) |
| `CACHE_SEARCH_LIMIT`         | `1`     | Jumlah hasil cache yang diambil     |

### RAG Configuration

| Variable           | Default | Deskripsi                          |
| ------------------ | ------- | ---------------------------------- |
| `RAG_SEARCH_LIMIT` | `7`     | Jumlah context chunks yang diambil |

### Server Configuration

| Variable       | Default                 | Deskripsi                                                       |
| -------------- | ----------------------- | --------------------------------------------------------------- |
| `SERVER_PORT`  | `8097`                  | Server port                                                     |
| `SERVER_HOST`  | `localhost`             | Server host                                                     |
| `FRONTEND_URL` | `http://localhost:5173` | Frontend URL untuk CORS (comma-separated untuk multiple) ✨ NEW |

### Security Configuration ✨ NEW

| Variable              | Default      | Deskripsi                                                                         |
| --------------------- | ------------ | --------------------------------------------------------------------------------- |
| `JWT_SECRET`          | _required_   | Secret key untuk JWT signing (generate: `openssl rand -hex 32`)                   |
| `AES_KEY`             | _required_   | AES encryption key - **MUST be 32 characters** (generate: `openssl rand -hex 16`) |
| `OPEN_ROUTER_API_KEY` | _(optional)_ | OpenRouter API key jika menggunakan OpenRouter                                    |

### Environment Configuration

| Variable  | Default       | Deskripsi                                    |
| --------- | ------------- | -------------------------------------------- |
| `APP_ENV` | `development` | Environment (development/production/staging) |
| `DEBUG`   | `false`       | Enable debug logging                         |

### Query Execution

| Variable                | Default | Deskripsi                    |
| ----------------------- | ------- | ---------------------------- |
| `QUERY_TIMEOUT_SECONDS` | `10`    | Timeout untuk eksekusi query |

---

## 📚 API Endpoints

### Public Endpoints

#### Health Check

```http
GET /health
```

**Response**:

```json
{
  "status": "API is up and running!"
}
```

---

### Authentication Endpoints ✨ NEW

#### Register

```http
POST /api/auth/register
Content-Type: application/json

{
  "username": "encrypted_username",
  "password": "encrypted_password",
  "full_name": "John Doe",
  "email": "john@example.com"
}
```

#### Login

```http
POST /api/auth/login
Content-Type: application/json

{
  "username": "encrypted_username",
  "password": "encrypted_password"
}
```

**Response**:

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "message": "Login berhasil"
}
```

#### Get Current User (Protected)

```http
GET /api/auth/me
Authorization: Bearer <token>
```

#### Logout (Protected)

```http
POST /api/auth/logout
Authorization: Bearer <token>
```

---

### Query Endpoints

#### Query dengan Natural Language

```http
POST /api/query
Content-Type: application/json

{
  "prompt": "tampilkan semua nasabah"
}
```

**Response**:

```json
{
  "data": [...],
  "sql": "SELECT * FROM nasabah",
  "cached": false
}
```

#### Enhance Prompt ✨ NEW

```http
POST /api/enhance
Content-Type: application/json

{
  "draft_prompt": "tabungan 20 juta"
}
```

**Response**:

```json
{
  "enhanced_prompt": "Tampilkan nasabah yang memiliki saldo tabungan sebesar 20 juta rupiah atau lebih."
}
```

---

### Feedback Endpoints (Protected)

#### Feedback/Koreksi SQL

```http
POST /api/feedback/koreksi
Authorization: Bearer <token>
Content-Type: application/json

{
  "prompt_asli": "tampilkan semua nasabah",
  "sql_koreksi": "SELECT id_nasabah, nama_lengkap FROM nasabah;"
}
```

---

### Admin Endpoints (Rate Limited: 10 req/min) ✨

#### Retrain RAG

```http
POST /admin/retrain
```

**Response**:

```json
{
  "message": "Proses retraining RAG telah selesai"
}
```

#### List Qdrant Points

```http
GET /admin/qdrant/list?collection=bpr_supra_rag&limit=10
```

#### Delete Qdrant Point

```http
DELETE /admin/qdrant/delete
Content-Type: application/json

{
  "collection": "bpr_supra_rag",
  "point_id": "uuid-here"
}
```

#### Create Cache Entry

```http
POST /admin/cache/create
Content-Type: application/json

{
  "prompt": "tampilkan nasabah",
  "sql": "SELECT * FROM nasabah"
}
```

#### Update Qdrant Point

```http
PUT /admin/qdrant/update
Content-Type: application/json

{
  "collection": "bpr_supra_rag",
  "point_id": "uuid-here",
  "content": "updated content"
}
```

---

## 🔄 Cara Pindah Database/Schema

Untuk pindah ke database atau schema lain, cukup ubah `DB_CONN_STRING` di file `.env`:

```env
# Format baru (Oracle)
DB_CONN_STRING="oracle://username:password@localhost:1521/service_name"

# Contoh pindah ke database lain
DB_CONN_STRING="oracle://HARI:hari123@localhost:1521/database_baru"

# Contoh pindah ke server lain
DB_CONN_STRING="oracle://username:password@192.168.1.100:1521/databaseai"
```

Setelah mengubah connection string, restart aplikasi.

---

## 🆕 What's New in v2.0

### Architecture Improvements

- ✅ **Middleware Pattern**: CORS, Logging, Rate Limiting di-centralized
- ✅ **Graceful Shutdown**: Clean exit dengan signal handling
- ✅ **Constants Management**: No more magic numbers (semua di `constants/constants.go`)
- ✅ **Proper ServeMux**: Tidak lagi pakai `http.DefaultServeMux`

### Security Enhancements

- ✅ **Multi-Origin CORS**: Support multiple frontend URLs
- ✅ **Rate Limiting**: Admin endpoints limited to 10 req/min
- ✅ **JWT Authentication**: Secure auth dengan session management
- ✅ **Environment Validation**: Validate required configs at startup

### Developer Experience

- ✅ **Structured Logging**: HTTP request logging dengan metrics
- ✅ **Better Error Messages**: Standardized error codes
- ✅ **Documentation**: Updated docs untuk semua changes
- ✅ **Migration Guide**: `QUICK_START_AFTER_REFACTORING.md`

### Breaking Changes

1. **CORS**: Sekarang di middleware, bukan per-handler
2. **Routes**: `RegisterRoutes()` sekarang menerima `*http.ServeMux`
3. **Config**: Field baru `FrontendURL` (required)
4. **Shutdown**: Server sekarang graceful shutdown

**Migration Guide**: Lihat `QUICK_START_AFTER_REFACTORING.md`

---

## 🔒 Security Best Practices

### Environment Variables

- ✅ **JANGAN** commit file `.env` ke version control
- ✅ File `.env` sudah ada di `.gitignore`
- ✅ Gunakan `.env.example` sebagai template
- ✅ Generate random secrets:

  ```bash
  # JWT Secret
  openssl rand -hex 32

  # AES Key (must be 32 chars)
  openssl rand -hex 16
  ```

### Production Deployment

- ✅ Gunakan secret management service (AWS Secrets Manager, HashiCorp Vault, dll)
- ✅ Enable HTTPS/TLS
- ✅ Set proper CORS origins (jangan pakai `*`)
- ✅ Monitor rate limiting metrics
- ✅ Setup proper logging & monitoring

### CORS Configuration

```bash
# Development (multiple origins)
FRONTEND_URL=http://localhost:3084,http://localhost:5173

# Production (specific domains only)
FRONTEND_URL=https://app.yourdomain.com,https://admin.yourdomain.com
```

---

## 📖 Documentation

- **Architecture**: `docs/ARCHITECTURE.md` - System architecture & flow
- **Function Reference**: `docs/FUNCTION_REFERENCE.md` - All functions documented
- **Use Cases**: `docs/USE_CASES.md` - Example use cases
- **Refactoring Summary**: `REFACTORING_SUMMARY.md` - What changed in v2.0
- **Quick Start**: `QUICK_START_AFTER_REFACTORING.md` - Migration guide

---

## 🐛 Troubleshooting

### Error: "FRONTEND_URL is required"

**Solution**: Add `FRONTEND_URL=http://localhost:5173` to `.env`

### Error: "AES_KEY harus 32 karakter"

**Solution**: Generate dengan `openssl rand -hex 16` (hasil 32 chars)

### CORS Error di Browser

**Solution**:

1. Pastikan `FRONTEND_URL` di `.env` sesuai dengan origin frontend
2. Restart server setelah update `.env`
3. Check browser console untuk detail error

### Rate Limit Error

**Solution**: Tunggu 1 menit atau kurangi frekuensi request

### Graceful Shutdown Tidak Bekerja

**Solution**: Pastikan menggunakan `Ctrl+C` (SIGINT), bukan force kill

---

## 🤝 Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to branch (`git push origin feature/AmazingFeature`)
5. Open Pull Request

---

## 📝 License

[Your License Here]

---

**Built with ❤️ using Go, RAG, and AI**
**Version**: 2.0 (Refactored)
**Last Updated**: 2026-01-05
