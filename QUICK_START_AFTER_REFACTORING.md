# 🚀 Quick Start Guide - After Refactoring

## ⚠️ IMPORTANT: Breaking Changes

Setelah refactoring, ada beberapa perubahan yang perlu Anda lakukan:

---

## 1. Update `.env` File

### Tambahkan konfigurasi baru:

```bash
# CORS Configuration (BARU!)
FRONTEND_URL=http://localhost:5173
```

Jika Anda punya multiple frontend origins:
```bash
FRONTEND_URL=http://localhost:3000,http://localhost:5173,https://yourdomain.com
```

### Pastikan semua API keys sudah diset:
```bash
GROQ_API_KEY=your_actual_key_here
GOOGLE_API_KEY=your_actual_key_here
QDRANT_API_KEY=your_actual_key_here
JWT_SECRET=your_secret_here
AES_KEY=your_32_char_key_here
```

---

## 2. Build Aplikasi

```bash
go build -o bin/app.exe .
```

---

## 3. Run Aplikasi

```bash
.\bin\app.exe
```

Anda akan melihat output seperti:
```
Berhasil memuat file .env
✅ Konfigurasi berhasil dimuat dari environment variables
✅ Berhasil terkoneksi ke database ORACLE 10G!
✅ Berhasil terkoneksi ke Layanan Vektor (Google AI & Qdrant).
Aplikasi siap berjalan...
🚀 Server berjalan di http://localhost:8097
```

---

## 4. Test Endpoints

### Health Check
```bash
curl http://localhost:8097/health
```

### Login (dengan rate limiting)
```bash
curl -X POST http://localhost:8097/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"encrypted_username","password":"encrypted_password"}'
```

### Query (sekarang dengan CORS yang proper)
```bash
curl -X POST http://localhost:8097/api/query \
  -H "Content-Type: application/json" \
  -H "Origin: http://localhost:5173" \
  -d '{"prompt":"tampilkan semua nasabah"}'
```

---

## 5. Graceful Shutdown

Tekan `Ctrl+C` untuk shutdown. Anda akan melihat:
```
🛑 Shutting down server...
✅ Server exited gracefully
```

Server akan:
- Berhenti menerima request baru
- Menunggu request yang sedang berjalan selesai (max 30 detik)
- Cleanup resources
- Exit dengan clean

---

## 🆕 Fitur Baru

### 1. Rate Limiting
Admin endpoints sekarang dibatasi 10 requests per menit:
- `/admin/retrain`

Jika melebihi limit, akan dapat response:
```json
{
  "error": "RATE_LIMIT_EXCEEDED",
  "message": "Terlalu banyak permintaan. Silakan coba lagi nanti."
}
```

### 2. Structured Logging
Setiap HTTP request sekarang di-log dengan format:
```
[HTTP] POST /api/query | Status: 200 | Duration: 1.234s | IP: 127.0.0.1 | UA: curl/7.68.0
```

### 3. CORS Middleware
CORS sekarang di-handle secara terpusat. Frontend Anda akan otomatis mendapat:
- `Access-Control-Allow-Origin` (dari config)
- `Access-Control-Allow-Credentials: true`
- `Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS`
- `Access-Control-Allow-Headers: Content-Type, Authorization, X-Requested-With`

---

## 🐛 Troubleshooting

### Error: "FRONTEND_URL is required"
**Solusi**: Tambahkan `FRONTEND_URL=http://localhost:5173` ke `.env`

### Error: "AES_KEY harus 32 karakter"
**Solusi**: Pastikan `AES_KEY` di `.env` tepat 32 karakter

### CORS Error di Browser
**Solusi**: 
1. Pastikan `FRONTEND_URL` di `.env` sesuai dengan origin frontend Anda
2. Restart server setelah update `.env`

### Rate Limit Error
**Solusi**: Tunggu 1 menit atau kurangi frekuensi request

---

## 📚 Dokumentasi Lengkap

Lihat file-file berikut untuk detail:
- `REFACTORING_SUMMARY.md` - Ringkasan semua perubahan
- `.env.example` - Template konfigurasi lengkap
- `constants/constants.go` - Semua konstanta aplikasi

---

## ✅ Checklist Migrasi

- [ ] Copy `.env.example` ke `.env`
- [ ] Isi semua API keys di `.env`
- [ ] Tambahkan `FRONTEND_URL` ke `.env`
- [ ] Pastikan `AES_KEY` 32 karakter
- [ ] Build ulang: `go build -o bin/app.exe .`
- [ ] Test run: `.\bin\app.exe`
- [ ] Test health check: `curl http://localhost:8097/health`
- [ ] Test CORS dari frontend
- [ ] Test graceful shutdown dengan `Ctrl+C`

---

**Happy Coding! 🎉**

