# 📚 Dokumentasi Go Bank API

Selamat datang di dokumentasi lengkap **Go Bank API** - sistem Natural Language to SQL dengan RAG (Retrieval-Augmented Generation) dan Semantic Caching.

---

## 📋 Daftar Dokumentasi

### 1. [ARCHITECTURE.md](./ARCHITECTURE.md) - Arsitektur & Flow Sistem
**Isi**: Dokumentasi lengkap tentang bagaimana sistem bekerja dari awal hingga akhir.

**Topik yang dibahas**:
- Overview sistem dan konsep utama (RAG, Semantic Caching, Dynamic Schema)
- 7 komponen utama aplikasi
- 4 flow diagram detail:
  - Application Startup Flow
  - Query Processing Flow (Main Feature)
  - Feedback & Correction Flow
  - Retraining Flow
- Technology stack
- Data flow dengan ASCII diagram
- Performance optimizations
- Security features
- Error handling strategies

**Cocok untuk**: Developer yang ingin memahami big picture sistem dan bagaimana semua komponen bekerja bersama.

---

### 2. [FUNCTION_REFERENCE.md](./FUNCTION_REFERENCE.md) - Referensi Fungsi
**Isi**: Dokumentasi detail semua fungsi dalam sistem, dikelompokkan per file/module.

**Topik yang dibahas**:
- **config.go**: Configuration management functions
- **database.go**: Database connection functions
- **main.go**: Application entry point
- **routes.go**: HTTP routing
- **handlers.go**: HTTP request handlers
- **logic.go**: Business logic layer
- **ai_service.go**: AI & vector database operations
- **schema_service.go**: Dynamic schema management
- **train.go**: RAG training/retraining
- **models.go**: Data structures
- Global variables

**Setiap fungsi didokumentasikan dengan**:
- Tujuan fungsi
- Parameters & return values
- Flow execution detail
- Dipanggil oleh / Memanggil fungsi lain
- Side effects
- Log output
- Error messages
- Contoh penggunaan

**Cocok untuk**: Developer yang ingin memahami detail implementasi setiap fungsi atau mencari referensi cepat.

---

### 3. [USE_CASES.md](./USE_CASES.md) - Use Cases & Examples
**Isi**: Dokumentasi lengkap use cases dan contoh penggunaan sistem dalam skenario nyata.

**Use cases yang dibahas**:

**Use Case 1: Query Natural Language (Cache Miss)**
- Scenario: User bertanya untuk pertama kalinya
- Flow: Embedding → Cache check (miss) → RAG search → LLM → Execute SQL → Save cache
- Performance: ~2-3 seconds

**Use Case 2: Query Natural Language (Cache Hit)**
- Scenario: User bertanya dengan pertanyaan mirip
- Flow: Embedding → Cache check (hit) → Execute SQL
- Performance: ~300-500ms (6x lebih cepat!)

**Use Case 3: Feedback & Correction**
- Scenario: AI menghasilkan SQL salah, user koreksi
- Flow: Save feedback ke database → Perlu retrain

**Use Case 4: Retraining RAG**
- Scenario: Admin trigger retraining setelah ada feedback baru
- Flow: Background process → Fetch data → Embed → Upsert to Qdrant
- Performance: ~30-60 seconds

**Use Case 5: Pindah Database/Schema**
- Scenario: Developer ingin pindah schema atau environment
- Flow: Update .env → Restart → Retrain
- Benefits: Zero code changes!

**Advanced Use Cases**:
- Complex query with aggregation
- Date range queries

**Cocok untuk**: Developer yang ingin melihat contoh penggunaan nyata atau memahami bagaimana sistem digunakan dalam berbagai skenario.

---

## 🚀 Quick Start

### Untuk Developer Baru

**1. Pahami arsitektur sistem terlebih dahulu**
```
Baca: ARCHITECTURE.md
```
Ini akan memberikan Anda pemahaman tentang bagaimana sistem bekerja secara keseluruhan.

**2. Lihat contoh penggunaan**
```
Baca: USE_CASES.md
```
Ini akan menunjukkan bagaimana sistem digunakan dalam skenario nyata.

**3. Dive into implementation details**
```
Baca: FUNCTION_REFERENCE.md
```
Gunakan sebagai referensi ketika Anda perlu memahami detail implementasi fungsi tertentu.

---

### Untuk Developer yang Sudah Familiar

**Mencari fungsi tertentu?**
→ Gunakan `FUNCTION_REFERENCE.md` dengan Ctrl+F

**Ingin memahami flow tertentu?**
→ Lihat diagram di `ARCHITECTURE.md`

**Butuh contoh implementasi?**
→ Lihat `USE_CASES.md`

---

## 🔍 Cara Menggunakan Dokumentasi

### Mencari Informasi Berdasarkan Kebutuhan

| Kebutuhan | Dokumentasi | Section |
|-----------|-------------|---------|
| Memahami konsep RAG | ARCHITECTURE.md | Overview → RAG |
| Memahami semantic caching | ARCHITECTURE.md | Overview → Semantic Caching |
| Melihat startup sequence | ARCHITECTURE.md | Flow A: Application Startup |
| Memahami query processing | ARCHITECTURE.md | Flow B: Query Processing |
| Detail fungsi `getSQLFromAI_Groq()` | FUNCTION_REFERENCE.md | ai_service.go |
| Cara pindah database | USE_CASES.md | Use Case 5 |
| Contoh request/response | USE_CASES.md | Any use case |
| Performance metrics | USE_CASES.md | Use Case 1 & 2 |

---

## 📊 Diagram & Visualisasi

Semua dokumentasi menggunakan ASCII art untuk diagram agar mudah dibaca di text editor manapun.

**Contoh diagram yang tersedia**:
- Application startup flow (ARCHITECTURE.md)
- Query processing flow dengan 8 steps detail (ARCHITECTURE.md)
- Data flow diagram (ARCHITECTURE.md)
- Function call chains (FUNCTION_REFERENCE.md)

---

## 🛠️ Maintenance

### Kapan Update Dokumentasi?

**ARCHITECTURE.md** - Update ketika:
- Menambah komponen baru
- Mengubah flow utama
- Menambah teknologi baru

**FUNCTION_REFERENCE.md** - Update ketika:
- Menambah fungsi baru
- Mengubah signature fungsi
- Mengubah behavior fungsi

**USE_CASES.md** - Update ketika:
- Menambah endpoint baru
- Menambah fitur baru
- Menemukan use case baru yang penting

---

## 📝 Konvensi Dokumentasi

### Format
- Semua file menggunakan Markdown
- Code blocks menggunakan syntax highlighting
- Diagram menggunakan ASCII art

### Struktur
- Setiap file dimulai dengan daftar isi
- Section dipisahkan dengan `---`
- Subsection menggunakan heading hierarchy

### Bahasa
- Dokumentasi menggunakan Bahasa Indonesia
- Code, variable names, dan technical terms dalam Bahasa Inggris
- Log output sesuai dengan yang ada di kode (mixed)

---

## 🤝 Kontribusi

Jika Anda menemukan:
- Informasi yang kurang jelas
- Error dalam dokumentasi
- Use case yang belum terdokumentasi
- Saran improvement

Silakan update dokumentasi yang relevan!

---

**Dokumentasi ini dibuat untuk memudahkan pemahaman dan maintenance sistem Go Bank API. Happy coding! 🚀**

