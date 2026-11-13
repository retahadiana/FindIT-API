# 🎓 FindIt API - Lost & Found Portal Kampus

FindIt API adalah backend layanan pelaporan barang hilang (*lost*) dan temuan (*found*) di lingkungan kampus. Dibangun menggunakan arsitektur **Clean Architecture (Service-Centric)** tanpa controller terpisah, dengan pendekatan **Hybrid Database (PostgreSQL + MongoDB)** dan antrian asynchronous **Redis / Asynq**.

---

## 🚀 Fitur Utama & Status Modul

| Modul / Fase | Penanggung Jawab | Cakupan Fitur Utama | Status |
|---|---|---|---|
| **Fase 0 & 1** | Lifia | Autentikasi JWT & rotasi Refresh Token, RBAC dinamis (`roles`, `permissions`), Audit Log Reputasi & Trust Score (MongoDB `$facet`), Transactional Outbox Relay & Asynq Worker notifikasi, CLI `tools/gentoken`. | ✅ Selesai |
| **Fase 2** | Reta | CRUD Categories & Items, filter lost/found, Keyset Cursor Pagination (`?after=...`), ekspor data CSV (`Accept: text/csv`), Optimistic Concurrency Control (OCC kolom `version`). | ✅ Selesai |
| **Fase 3** | Zaskia | Pengajuan klaim barang temuan (TxManager atomik), verifikasi tanya-jawab ciri khusus rahasia di MongoDB (max 10 Q&A), OCC Approval pengembalian barang, script simulasi race condition `tools/loadtest_claim`. | ✅ Selesai |
| **Fase 4** | Lifia (Lead) | Finalisasi routing terpusat ([route/route.go](file:///d:/Go/FindIT-API/route/route.go)), multi-role entrypoint ([main.go](file:///d:/Go/FindIT-API/main.go)), dan dokumentasi interaktif **Swagger OpenAPI**. | ✅ Selesai |

---

## 🛠️ Prasyarat (Prerequisites)

- **Go**: Versi 1.22 atau lebih baru
- **PostgreSQL**: Versi 15+ (port default `5432`)
- **MongoDB**: Versi 6+ atau 7+ (port default `27017`)
- **Redis**: Versi 7+ (port default `6379`) — *diperlukan untuk mode worker notifikasi*

---

## 💻 Panduan Menjalankan di Lokal (Step-by-Step)

### 1. Unduh Dependensi
```bash
go mod download
```

### 2. Siapkan Berkas Konfigurasi Lingkungan (`.env`)
Salin `.env.example` menjadi `.env` lalu sesuaikan kredensial database lokal Anda:
```powershell
# Windows PowerShell
Copy-Item .env.example .env
```
Pastikan `DB_PASSWORD` sesuai dengan akun PostgreSQL lokal Anda:
```env
APP_PORT=3000
APP_ROLE=all
SWAGGER_ENABLED=true

DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=password_postgres_anda
DB_NAME=findit_db
DB_SSLMODE=disable

MONGO_URI=mongodb://localhost:27017
MONGO_DATABASE=findit_db

REDIS_ADDR=localhost:6379
JWT_SECRET=rahasia-minimal-32-karakter-harus-panjang-dan-aman!
```

### 3. Setup Database & Migrasi SQL
Buat database `findit_db` di PostgreSQL, lalu jalankan seluruh migrasi secara berurutan:
```bash
# Buat database (jika belum ada)
psql -U postgres -h localhost -p 5432 -c "CREATE DATABASE findit_db;"

# Jalankan skrip migrasi 001 s/d 006
psql -U postgres -h localhost -p 5432 -d findit_db -f migrations/001_create_users_and_tokens.sql
psql -U postgres -h localhost -p 5432 -d findit_db -f migrations/002_create_rbac.sql
psql -U postgres -h localhost -p 5432 -d findit_db -f migrations/003_create_outbox_and_notifications.sql
psql -U postgres -h localhost -p 5432 -d findit_db -f migrations/004_create_categories.sql
psql -U postgres -h localhost -p 5432 -d findit_db -f migrations/005_create_items.sql
psql -U postgres -h localhost -p 5432 -d findit_db -f migrations/006_create_claims.sql
```
*(Collection dan indeks MongoDB dibuat otomatis oleh aplikasi saat pertama kali start).*

### 4. Menjalankan Server Aplikasi & Akses Swagger UI

Pilih salah satu mode server sesuai kebutuhan:

- **Mode 1: Full Server (HTTP API + Background Worker) — Default**
  ```bash
  go run .
  # atau
  go run . -role=all
  ```
  *Membutuhkan Redis aktif.*

- **Mode 2: Hanya HTTP API (Tanpa Redis)**
  ```bash
  go run . -role=api
  ```
  *Gunakan mode ini jika Anda hanya ingin menguji endpoint HTTP & Swagger tanpa menyalakan Redis.*

- **Mode 3: Hanya Worker Notifikasi Latar Belakang**
  ```bash
  go run . -role=worker
  ```

#### Akses Dokumentasi Interaktif Swagger UI
Setelah server berjalan, dokumentasi interaktif seluruh endpoint API dapat langsung diakses melalui browser:
- **Swagger UI**: [http://localhost:3000/swagger/index.html](http://localhost:3000/swagger/index.html)
- **OpenAPI Spec (JSON)**: [http://localhost:3000/swagger/doc.json](http://localhost:3000/swagger/doc.json)

> **Catatan:**
> - Fitur Swagger dikendalikan melalui `SWAGGER_ENABLED=true` pada berkas `.env` (di lingkungan *production*, sebaiknya disetel ke `false` demi keamanan).
> - Untuk memperbarui dokumentasi setelah mengubah anotasi godoc pada layer service:
>   ```bash
>   swag init -g main.go -o docs
>   ```

### 5. Verifikasi Status Server (Health Check)
```bash
curl http://localhost:3000/api/v1/health
```
Respons sukses:
```json
{
  "success": true,
  "message": "server, PostgreSQL, dan MongoDB berjalan",
  "data": {
    "postgres": "connected",
    "mongodb": "connected"
  }
}
```

---

## 🧪 Pengujian & Utilitas CLI

### A. Menjalankan Unit Test
```bash
go test -v -cover ./...
```

### B. Generate Token Pengujian Instan (`tools/gentoken`)
Generate token JWT tanpa perlu registrasi manual di Postman:
```bash
# Token Admin (user_id: 1)
go run ./tools/gentoken -user=1 -role=admin

# Token Mahasiswa Biasa (user_id: 2)
go run ./tools/gentoken -user=2 -role=user
```
Gunakan token pada header request: `Authorization: Bearer <TOKEN>`.

### C. Concurrency Load Test OCC (`tools/loadtest_claim`)
Uji ketahanan race condition Optimistic Concurrency Control (OCC) saat beberapa worker meng-approve klaim secara bersamaan:
```bash
go run ./tools/loadtest_claim -item=2 -workers=10
```
