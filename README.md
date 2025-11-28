# Go Bank API - RAG-Powered SQL Query System

API backend untuk sistem query database menggunakan Natural Language Processing dengan RAG (Retrieval-Augmented Generation) dan Semantic Caching.

## 🚀 Fitur Utama

- **Natural Language to SQL**: Konversi pertanyaan bahasa natural menjadi SQL query menggunakan AI
- **RAG (Retrieval-Augmented Generation)**: Menggunakan vector database untuk meningkatkan akurasi query
- **Semantic Caching**: Cache hasil query berdasarkan similarity untuk performa lebih cepat
- **Dynamic Schema Detection**: Otomatis membaca struktur database dari connection string
- **Feedback System**: Sistem koreksi untuk meningkatkan akurasi AI

## 📋 Prerequisites

- Go 1.24 atau lebih tinggi
- Oracle 10g atau lebih tinggi database
- Qdrant vector database
- API Keys:
  - Groq API Key (untuk LLM)
  - Google AI API Key (untuk embeddings)

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
DB_CONN_STRING="username/password@localhost:1521/XE"

# API Keys
GROQ_API_KEY="your_groq_api_key_here"
GOOGLE_API_KEY="your_google_api_key_here"

# Server Configuration
SERVER_PORT=8080
SERVER_HOST=localhost
```

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

### 5. Jalankan Aplikasi

```bash
go run .
```

## 🔧 Environment Variables

Berikut adalah daftar lengkap environment variables yang tersedia:

### Database Configuration

| Variable                       | Default    | Deskripsi                                                                                                          |
| ------------------------------ | ---------- | ------------------------------------------------------------------------------------------------------------------ |
| `DB_CONN_STRING`               | _required_ | PostgreSQL connection string dengan format: `postgres://user:pass@host:port/db?sslmode=disable&search_path=schema` |
| `DB_MAX_OPEN_CONNS`            | `25`       | Maximum number of open connections                                                                                 |
| `DB_MAX_IDLE_CONNS`            | `10`       | Maximum number of idle connections                                                                                 |
| `DB_CONN_MAX_LIFETIME_MINUTES` | `5`        | Connection max lifetime (minutes)                                                                                  |
| `DB_PING_TIMEOUT_SECONDS`      | `5`        | Database ping timeout (seconds)                                                                                    |

### AI Service Configuration (Groq)

| Variable               | Default                                           | Deskripsi                      |
| ---------------------- | ------------------------------------------------- | ------------------------------ |
| `GROQ_API_KEY`         | _required_                                        | Groq API key untuk LLM service |
| `GROQ_MODEL`           | `llama-3.1-8b-instant`                            | Model yang digunakan           |
| `GROQ_API_URL`         | `https://api.groq.com/openai/v1/chat/completions` | Groq API endpoint              |
| `GROQ_TIMEOUT_SECONDS` | `30`                                              | HTTP timeout untuk Groq API    |

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

| Variable      | Default     | Deskripsi   |
| ------------- | ----------- | ----------- |
| `SERVER_PORT` | `8080`      | Port server |
| `SERVER_HOST` | `localhost` | Host server |

### Query Execution

| Variable                | Default | Deskripsi                    |
| ----------------------- | ------- | ---------------------------- |
| `QUERY_TIMEOUT_SECONDS` | `10`    | Timeout untuk eksekusi query |

### Environment

| Variable  | Default       | Deskripsi                                    |
| --------- | ------------- | -------------------------------------------- |
| `APP_ENV` | `development` | Environment (development/production/staging) |
| `DEBUG`   | `false`       | Enable debug logging                         |

## 📚 API Endpoints

### Health Check

```
GET /health
```

### Query dengan Natural Language

```
POST /api/query
Content-Type: application/json

{
  "prompt": "tampilkan semua nasabah"
}
```

### Feedback/Koreksi SQL

```
POST /api/feedback/koreksi
Content-Type: application/json

{
  "prompt_asli": "tampilkan semua nasabah",
  "sql_koreksi": "SELECT id_nasabah, nama_lengkap FROM nasabah;"
}
```

### Admin: Retrain RAG

```
POST /admin/retrain
```

## 🔄 Cara Pindah Database/Schema

Untuk pindah ke database atau schema lain, cukup ubah `DB_CONN_STRING` di file `.env`:

```env
# Contoh pindah ke schema lain (username = schema di Oracle)
DB_CONN_STRING="schema_baru/password@localhost:1521/XE"

# Contoh pindah ke database lain
DB_CONN_STRING="username/password@localhost:1521/database_baru"

# Contoh pindah ke server lain
DB_CONN_STRING="username/password@192.168.1.100:1521/XE"
```

Setelah mengubah connection string, restart aplikasi.

## 🔒 Security Notes

- **JANGAN** commit file `.env` ke version control
- File `.env` sudah ada di `.gitignore`
- Gunakan `.env.example` sebagai template
- Untuk production, gunakan secret management service (AWS Secrets Manager, HashiCorp Vault, dll)

## 📝 License

[Your License Here]
