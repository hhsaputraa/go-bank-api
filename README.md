# Go Bank API - RAG-Powered SQL Generator

API untuk menghasilkan SQL query dari natural language menggunakan RAG (Retrieval-Augmented Generation) dengan Qdrant vector database dan Groq AI.

## 🚀 Fitur

- **Natural Language to SQL**: Konversi pertanyaan bahasa natural menjadi SQL query
- **RAG (Retrieval-Augmented Generation)**: Menggunakan vector database untuk meningkatkan akurasi
- **Semantic Cache**: Cache semantik di Qdrant untuk query yang mirip (threshold 0.95)
- **Smart Caching Strategy**:
  1. **Semantic Cache Check** (Qdrant collection) - Cek similarity dengan threshold 0.95
  2. **RAG + AI Generation** - Jika tidak ada di cache, generate SQL baru
  3. **Auto-Save on Success** - SQL otomatis masuk cache setelah berhasil dieksekusi
  4. **Manual Feedback** - User bisa koreksi SQL via endpoint feedback
- **Dynamic Schema**: Otomatis membaca schema database dari PostgreSQL
- **Feedback Loop**: User bisa memberikan koreksi SQL untuk training ulang
- **Cache Poisoning Prevention**: SQL error tidak masuk cache, hanya SQL yang berhasil dieksekusi
- **Admin Retrain**: Endpoint untuk retrain vector database
- **Fully Configurable**: Semua konfigurasi melalui environment variables

## 📋 Prerequisites

- Go 1.24 atau lebih tinggi
- PostgreSQL database
- Qdrant vector database (lokal atau cloud)
- API Keys:
  - Groq API Key ([dapatkan di sini](https://console.groq.com/keys))
  - Google AI API Key ([dapatkan di sini](https://makersuite.google.com/app/apikey))

## ⚙️ Konfigurasi Environment Variables

### Setup Awal

1. Copy file `.env.example` menjadi `.env`:

   ```bash
   cp .env.example .env
   ```

2. Edit file `.env` dan isi dengan kredensial Anda

### Kategori Konfigurasi

#### 🗄️ Database Configuration

| Variable               | Default | Deskripsi                                   |
| ---------------------- | ------- | ------------------------------------------- |
| `DB_CONN_STRING`       | -       | **REQUIRED** - Connection string PostgreSQL |
| `DB_MAX_OPEN_CONNS`    | 25      | Maksimum koneksi database yang terbuka      |
| `DB_MAX_IDLE_CONNS`    | 10      | Maksimum koneksi database yang idle         |
| `DB_CONN_MAX_LIFETIME` | 5m      | Maksimum waktu hidup koneksi                |
| `DB_CONN_TIMEOUT`      | 5s      | Timeout untuk koneksi database              |

**Contoh DB_CONN_STRING:**

```
postgres://username:password@localhost:5432/database?sslmode=disable&search_path=schema_name
```

**Tips:** Untuk pindah schema/database, cukup ganti nilai `search_path` di connection string.

#### 🤖 Groq AI Configuration

| Variable       | Default                                         | Deskripsi                            |
| -------------- | ----------------------------------------------- | ------------------------------------ |
| `GROQ_API_KEY` | -                                               | **REQUIRED** - API Key untuk Groq AI |
| `GROQ_API_URL` | https://api.groq.com/openai/v1/chat/completions | URL endpoint Groq API                |
| `GROQ_MODEL`   | llama-3.1-8b-instant                            | Model Groq yang digunakan            |
| `GROQ_TIMEOUT` | 30s                                             | Timeout untuk request ke Groq        |

#### 🧠 Google AI Configuration

| Variable                | Default                   | Deskripsi                                         |
| ----------------------- | ------------------------- | ------------------------------------------------- |
| `GOOGLE_API_KEY`        | -                         | **REQUIRED** - API Key untuk Google Generative AI |
| `EMBEDDING_MODEL`       | models/text-embedding-004 | Model embedding yang digunakan                    |
| `EMBEDDING_VECTOR_SIZE` | 768                       | Dimensi vektor embedding                          |

#### 🔍 Qdrant Vector Database Configuration

| Variable                 | Default               | Deskripsi                             |
| ------------------------ | --------------------- | ------------------------------------- |
| `QDRANT_HOST`            | localhost             | Host Qdrant                           |
| `QDRANT_PORT`            | 6334                  | Port Qdrant untuk gRPC                |
| `QDRANT_URL`             | http://localhost:6333 | URL Qdrant untuk HTTP API             |
| `QDRANT_COLLECTION_NAME` | bpr_supra_rag         | Nama collection Qdrant                |
| `QDRANT_SEARCH_LIMIT`    | 7                     | Jumlah hasil pencarian vektor         |
| `QDRANT_TIMEOUT`         | 10s                   | Timeout untuk request ke Qdrant       |
| `QDRANT_API_KEY`         | -                     | API Key untuk Qdrant Cloud (opsional) |

#### 🌐 Server Configuration

| Variable      | Default | Deskripsi                         |
| ------------- | ------- | --------------------------------- |
| `SERVER_PORT` | 8080    | Port untuk menjalankan web server |

#### 💾 Cache Configuration

| Variable     | Default    | Deskripsi                             |
| ------------ | ---------- | ------------------------------------- |
| `CACHE_FILE` | cache.json | Nama file untuk menyimpan cache query |

#### 🎓 Feedback & Training Configuration

| Variable                  | Default                    | Deskripsi                                                      |
| ------------------------- | -------------------------- | -------------------------------------------------------------- |
| `FEEDBACK_TABLE_NAME`     | bpr_supra.rag_sql_examples | Nama tabel untuk menyimpan feedback koreksi SQL                |
| `QDRANT_CACHE_COLLECTION` | bpr_supra_cache            | Nama collection Qdrant untuk semantic cache                    |
| `QDRANT_CACHE_THRESHOLD`  | 0.95                       | Threshold untuk semantic cache hit (0.0 - 1.0, semakin strict) |

## 🏃 Cara Menjalankan

1. **Install dependencies:**

   ```bash
   go mod download
   ```

2. **Setup environment variables:**

   ```bash
   cp .env.example .env
   # Edit .env dengan kredensial Anda
   ```

3. **Jalankan aplikasi:**

   ```bash
   go run .
   ```

4. **Build untuk production:**
   ```bash
   go build -o go-bank-api.exe .
   ./go-bank-api.exe
   ```

## 📡 API Endpoints

### Health Check

```
GET /health
```

**Response:**

```json
{
  "status": "API is up and running!"
}
```

### Query dengan Natural Language

```
POST /api/query
Content-Type: application/json

{
  "prompt": "tampilkan semua nasabah"
}
```

**Response:**

```json
{
  "data": [
    {
      "id_nasabah": "CIF00001",
      "nama_lengkap": "Budi Santoso",
      ...
    }
  ]
}
```

### Feedback Koreksi SQL

Endpoint untuk user memberikan feedback koreksi SQL yang salah:

```
POST /api/feedback/koreksi
Content-Type: application/json

{
  "prompt_asli": "tampilkan semua nasabah",
  "sql_koreksi": "SELECT id_nasabah, nama_lengkap FROM nasabah;"
}
```

**Response:**

```json
{
  "message": "Feedback berhasil disimpan. Terima kasih!"
}
```

### Admin Retrain (Background Process)

Endpoint untuk admin melakukan retrain vector database dengan data terbaru:

```
POST /admin/retrain
```

**Response:**

```json
{
  "message": "Proses retrain dimulai di background. Silakan cek log server."
}
```

**Note:** Proses retrain berjalan di background dan tidak akan memblokir request lain.

## 🔄 Pindah Database/Schema

Untuk pindah ke database atau schema lain, cukup ubah `DB_CONN_STRING` di file `.env`:

```env
# Contoh pindah ke schema 'bpr_lain'
DB_CONN_STRING="postgres://postgres:password@localhost:5432/postgres?sslmode=disable&search_path=bpr_lain"
```

Restart aplikasi untuk menerapkan perubahan.

## 🔒 Keamanan

- ⚠️ **JANGAN** commit file `.env` ke version control
- ✅ File `.env` sudah ada di `.gitignore`
- ✅ Gunakan `.env.example` sebagai template
- ✅ Simpan kredensial sensitif hanya di `.env` lokal

## 📝 License

MIT License
