# 🎓 FindIt API - Sistem Portal Lost & Found Kampus Terintegrasi

Backend layanan pelaporan barang hilang (*lost*) dan temuan (*found*) di lingkungan kampus yang dibangun menggunakan **Go (Golang)** dan **Fiber v2** dengan mengadopsi arsitektur **Go Clean Architecture (Service-Centric)**, persistensi **Hybrid Database (PostgreSQL & MongoDB)**, jaminan konsistensi konkurensi **Optimistic Concurrency Control (OCC)**, antrean pesan asinkron **Transactional Outbox Pattern & Redis / Asynq**, serta orkestrasi kontainer **Docker Multi-Container**.

---

## 👥 Tim Pengembang & Identitas Akademik

**Program Studi D4 Teknik Informatika**  
**Fakultas Vokasi, Universitas Airlangga (2026)**  
Mata Kuliah: *Praktikum Pemrograman Backend Lanjut*

| No | Nama Anggota | Branch Git | Modul & Lingkup Tanggung Jawab Teknis |
|:---:|---|:---:|---|
| 1 | **Alifia Putri Marizka**<br>*(Git Master & Lead)* | `alifia` | **Fase 0, 1 & 4A**: Inisialisasi arsitektur bersama, modul Auth (Dual Token JWT Bcrypt), Dynamic RBAC, Reputation Engine (MongoDB `$facet`), `TxManager`, Transactional Outbox Relay & Asynq Notification Worker, CLI `tools/gentoken`, serta integrasi dokumentasi interaktif **Swagger OpenAPI**. |
| 2 | **Reta Hadiana Unggula** | `reta` | **Fase 2 & 4B**: Modul master Categories & Items (CRUD, status `open/claimed/returned`), Keyset Cursor Pagination (`?after=...`), Streaming Content Negotiation CSV (`Accept: text/csv`), OCC kolom `version`, Testing Suite (Unit & Integration), dan orkestrasi **Docker Multi-Container**. |
| 3 | **Zaskia Rania** | `zaskia` | **Fase 3**: Modul pengajuan klaim barang temuan (TxManager atomik), verifikasi tanya-jawab ciri khusus rahasia di MongoDB (kuota maksimal 10 butir Q&A), persetujuan klaim atomik OCC, serta pengujian beban *race condition* `tools/loadtest_claim`. |

---

## 🏛️ Arsitektur Sistem (Hybrid Persistence & Event-Driven)

```text
                               ┌────────────────────────────────────────────────────────┐
                               │             Klien (Postman / Swagger UI)              │
                               └───────────────────────────┬────────────────────────────┘
                                                           │ HTTP JSON / CSV (Port 3000)
                                                           ▼
                               ┌────────────────────────────────────────────────────────┐
                               │                  Go Fiber Web Server                   │
                               │        (Clean Architecture: Service-Centric)           │
                               │  Middleware: RequireAuth, Dynamic RBAC, Rate Limiter   │
                               └──────────────┬─────────────────────────┬───────────────┘
                                              │                         │
                   ┌──────────────────────────┴──────────┐              │
                   ▼                                     ▼              ▼
     ┌───────────────────────────┐         ┌───────────────────────────┐   ┌───────────────────────────┐
     │   PostgreSQL (Port 5432)  │         │    MongoDB (Port 27017)   │   │     Redis (Port 6379)     │
     ├───────────────────────────┤         ├───────────────────────────┤   ├───────────────────────────┤
     │ • users & refresh_tokens  │         │ • item_verifications      │   │ • Asynq Message Queue     │
     │ • roles & permissions     │         │   (Array Q&A ciri rahasia │   │ • Task Type:              │
     │ • categories & items      │         │    kuota maks 10 butir)   │   │   "email:deliver"         │
     │   (OCC kolom version)     │         │ • user_reputations        │   └─────────────┬─────────────┘
     │ • claims (Atomic OCC)     │         │   (Audit log reward &     │                 │
     │ • transactional outbox    │         │    agregasi $facet badge) │                 ▼
     │ • in-app notifications    │         └───────────────────────────┘   ┌───────────────────────────┐
     └─────────────┬─────────────┘                                         │    Asynq Background       │
                   │                                                       │         Worker            │
                   └───────────────── Transactional Outbox Relay ──────────► (Mengirim notifikasi &   │
                                                                           │  mencatat logs/mail.log)  │
                                                                           └───────────────────────────┘
```

---

## 🛠️ Prasyarat Perangkat Lunak (Prerequisites)

- **Go (Golang)**: Versi 1.22 atau lebih baru (`go version`)
- **PostgreSQL**: Versi 15+ (port default `5432`)
- **MongoDB**: Versi 6+ atau 7+ (port default `27017`)
- **Redis**: Versi 7+ (port default `6379`) — *diperlukan untuk mode worker notifikasi*
- **Docker & Docker Compose**: *(Opsional)* jika ingin menjalankan via kontainerisasi otomatis

---

## 🚀 Panduan Instalasi & Menjalankan Lokal

### 1. Unduh Dependensi Modul Go
```bash
go mod download
```

### 2. Konfigurasi Lingkungan (`.env` dan `.env.test`)
Salin berkas contoh konfigurasi:
```bash
# Windows PowerShell
Copy-Item .env.example .env
```
Sesuaikan nilai konfigurasi di `.env` dengan kredensial database lokal Anda:
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
JWT_SECRET=kunci-rahasia-minimal-32-karakter-harus-panjang-dan-aman!
```

### 3. Setup Database PostgreSQL & Migrasi SQL
Buat database `findit_db`, kemudian jalankan seluruh 6 skrip migrasi berurutan:
```bash
# Buat database PostgreSQL
psql -U postgres -h localhost -p 5432 -c "CREATE DATABASE findit_db;"

# Jalankan skrip migrasi 001 s/d 006
psql -U postgres -h localhost -p 5432 -d findit_db -f migrations/001_create_users_and_tokens.sql
psql -U postgres -h localhost -p 5432 -d findit_db -f migrations/002_create_rbac.sql
psql -U postgres -h localhost -p 5432 -d findit_db -f migrations/003_create_outbox_and_notifications.sql
psql -U postgres -h localhost -p 5432 -d findit_db -f migrations/004_create_categories.sql
psql -U postgres -h localhost -p 5432 -d findit_db -f migrations/005_create_items.sql
psql -U postgres -h localhost -p 5432 -d findit_db -f migrations/006_create_claims.sql
```
*(Catatan: Koleksi dan indeks MongoDB akan diinisialisasi otomatis oleh aplikasi saat pertama kali dijalankan).*

---

## 💻 Pilihan Mode Menjalankan Server

### Opsi A: Menggunakan Go CLI Langsung

1. **Mode 1: Full Server (HTTP API + Background Worker) — Default**
   ```bash
   go run .
   # atau
   go run . -role=all
   ```
   *(Membutuhkan Redis aktif di port 6379).*

2. **Mode 2: Hanya HTTP API (Tanpa Redis)**
   ```bash
   go run . -role=api
   ```
   *Gunakan mode ini jika Anda hanya ingin menguji endpoint HTTP tanpa menyalakan antrean Redis.*

3. **Mode 3: Hanya Worker Latar Belakang**
   ```bash
   go run . -role=worker
   ```

---

### Opsi B: Menggunakan Docker Compose (Satu Perintah Instan)

Seluruh ekosistem (PostgreSQL 17, MongoDB 8, Redis 7, HTTP API, dan Background Worker) dapat dijalankan sekaligus tanpa instalasi database lokal:
```bash
# Jalankan seluruh multi-kontainer di latar belakang
docker compose up -d

# Periksa status seluruh kontainer
docker compose ps
```
Layanan API akan langsung aktif di `http://localhost:3000`.

---

## 📖 Dokumentasi Interaktif Swagger OpenAPI UI

Setelah server berjalan dengan `SWAGGER_ENABLED=true`, dokumentasi interaktif visual dapat diakses langsung via peramban:
- **Swagger UI**: [http://localhost:3000/swagger/index.html](http://localhost:3000/swagger/index.html)
- **OpenAPI JSON Spec**: [http://localhost:3000/swagger/doc.json](http://localhost:3000/swagger/doc.json)

> **Catatan Pengembang:**  
> Untuk memperbarui dokumentasi OpenAPI setelah mengubah anotasi godoc pada layer service:
> ```bash
> swag init -g main.go -o docs
> ```

---

## 📋 Katalog Lengkap 36 Rute Endpoint RESTful API

Format request body (`POST`, `PUT`, `PATCH`) wajib menyertakan header `Content-Type: application/json`.  
Endpoint terproteksi wajib menyertakan header `Authorization: Bearer <access_token>`.

| No | Metode | Endpoint URL | Hak Akses / Middleware | Deskripsi Fungsional Fitur |
|:---:|:---:|---|---|---|
| 1 | `GET` | `/swagger/*` | Publik | Dokumentasi visual interaktif Swagger OpenAPI UI |
| 2 | `GET` | `/api/v1/health` | Publik | Health check kesiapan server, PostgreSQL & MongoDB secara simultan |
| 3 | `POST` | `/api/v1/auth/register` | RequireJSON | Registrasi akun pengguna baru (Trust score default 100, role `user`) |
| 4 | `POST` | `/api/v1/auth/login` | RateLimit (5/m), JSON | Login pengguna, anti-brute force, terbit Access & Refresh Token |
| 5 | `POST` | `/api/v1/auth/refresh` | RequireJSON | Rotasi token baru; refresh token lama langsung dicabut (*revoked*) |
| 6 | `POST` | `/api/v1/auth/logout` | RequireAuth | Pencabutan (*revoke*) status aktif refresh token di database PostgreSQL |
| 7 | `GET` | `/api/v1/auth/me` | RequireAuth | Mengambil informasi profil lengkap akun pengguna yang sedang login |
| 8 | `GET` | `/api/v1/users` | `user:list` (Admin) | Katalog seluruh pengguna dengan keyset cursor pagination (`?limit=&after=`) |
| 9 | `POST` | `/api/v1/users` | `user:update:any` (Admin) | Pembuatan akun pengguna baru secara manual oleh Administrator |
| 10 | `PUT` | `/api/v1/users/profile` | RequireAuth, JSON | Pembaruan mandiri nama lengkap dan nomor telepon profil pengguna |
| 11 | `GET` | `/api/v1/users/:id` | Pemilik / `user:read:any` | Mengambil data profil spesifik satu pengguna berdasarkan ID |
| 12 | `PUT` | `/api/v1/users/:id` | `user:update:any` (Admin) | Penggantian seluruh data profil pengguna secara utuh oleh Administrator |
| 13 | `PATCH` | `/api/v1/users/:id` | `user:update:any` (Admin) | Pembaruan parsial atribut pengguna (`ApplyPatch` & sanitasi string) |
| 14 | `DELETE`| `/api/v1/users/:id` | `user:delete` (Admin) | Penghapusan akun pengguna dari basis data oleh Administrator |
| 15 | `PATCH` | `/api/v1/users/:id/role`| `role:assign` (Admin) | Penugasan wewenang dinamis (*Self-Role Modification Guard* 403) |
| 16 | `GET` | `/api/v1/users/:id/reputation` | RequireAuth | Melihat skor reputasi, riwayat badge, dan total reward penemuan |
| 17 | `GET` | `/api/v1/users/:id/reputation/summary`| RequireAuth | Ringkasan agregasi MongoDB `$facet` (rata-rata skor, distribusi ulasan) |
| 18 | `POST` | `/api/v1/users/:id/reward` | RequireAuth, JSON | Pemilik memberi reward trust score (+35) ke penemu (Anti-Self & Anti-Duplikat) |
| 19 | `GET` | `/api/v1/notifications/mine` | RequireAuth | Feed notifikasi masuk milik pengguna dalam aplikasi (*In-App Feed*) |
| 20 | `GET` | `/api/v1/admin/queue` | `queue:monitor` (Admin)| Inspeksi metrik pemantauan antrian notifikasi Asynq Redis oleh Admin |
| 21 | `GET` | `/api/v1/categories` | Publik | Melihat seluruh daftar kategori master barang temuan/hilang |
| 22 | `POST` | `/api/v1/categories` | `category:manage` (Admin)| Menambahkan master kategori barang baru ke PostgreSQL |
| 23 | `GET` | `/api/v1/items` | Publik, Content-Negotiate| Katalog barang (filter multi-param, keyset cursor, & streaming CSV) |
| 24 | `GET` | `/api/v1/items/:id` | Publik | Melihat rincian lengkap informasi barang temuan atau kehilangan |
| 25 | `POST` | `/api/v1/items` | RequireAuth, JSON | Publikasi barang temuan/hilang baru (Inisialisasi versi OCC `version=1`) |
| 26 | `PUT` | `/api/v1/items/:id` | RequireAuth, JSON | Pembaruan data barang (*Ownership Guard*: hanya pemilik sah atau Admin) |
| 27 | `DELETE`| `/api/v1/items/:id` | RequireAuth | Soft delete barang (*Ownership Guard* & proteksi 409 jika status *claimed*) |
| 28 | `POST` | `/api/v1/items/:id/claims` | RequireAuth, JSON | Pengajuan klaim barang (*Anti-Self-Claiming Guard* & outbox atomik) |
| 29 | `GET` | `/api/v1/items/:id/claims` | Pemilik Barang / Admin | Penemu melihat daftar pengajuan klaim yang masuk pada barang miliknya |
| 30 | `GET` | `/api/v1/claims/me` | RequireAuth | Riwayat dan status pengajuan klaim milik pengguna saat ini (*My Claims*) |
| 31 | `GET` | `/api/v1/claims/:id` | Stakeholder Terkait | Melihat detail status dan deskripsi bukti pengajuan klaim tertentu |
| 32 | `PATCH` | `/api/v1/claims/:id/status`| RequireAuth, JSON | Persetujuan (*approval*) atau penolakan klaim transaksi atomik OCC |
| 33 | `GET` | `/api/v1/claims/:id/verification`| Stakeholder Terkait | Mengambil sesi daftar pertanyaan verifikasi ciri rahasia dari MongoDB |
| 34 | `POST` | `/api/v1/claims/:id/verification`| RequireAuth, JSON | Penemu membuat butir pertanyaan ciri rahasia di MongoDB (Maks 10 butir) |
| 35 | `PUT` | `/api/v1/claims/:id/verification/answers` | RequireAuth, JSON | Pengklaim mengirimkan jawaban atas pertanyaan ciri rahasia ke MongoDB |
| 36 | `POST` | `/api/v1/claims/:id/verification/review` | RequireAuth, JSON | Penemu/Admin menilai kebenaran jawaban verifikasi (`is_verified`) |

---

## 🧪 Pengujian Otomatis & Kakas CLI (`tools/`)

### A. Eksekusi Unit Test Service Layer (Mock & Pure Logic)
Menguji logika bisnis service tanpa ketergantungan database fisik eksternal:
```bash
go test -v -cover ./app/service/...
```
*Cakupan:* 7 fungsi unit test komprehensif (`authz_rules_test.go`, `auth_reputation_test.go`, `notification_worker_test.go`) dengan capaian code coverage **88.9%**.

---

### B. Eksekusi Integration Test Riil Database PostgreSQL
Menguji transaksi atomik, rollback, outbox event, dan kueri OCC pada database PostgreSQL fisik nyata:
```bash
# Pastikan database findit_db_test aktif sesuai konfigurasi .env.test
go test -v -tags=integration ./tests/integration/...
```
*Cakupan:*
1. `TestItemOCC_And_TxRollback`: Menguji rollback atomik `TxManager.WithTxOnce` dan deteksi `ErrConflictOCC` pada update versi usang.
2. `TestClaimApproveConcurrency_OutboxExactOnce`: Menguji penerbitan event outbox *exactly-once* dan simulasi 5 goroutine approval serentak (tepat 1x `200 OK`, 4x `409 Conflict`).

---

### C. Concurrency Race Condition Load Test OCC (`tools/loadtest_claim`)
Kakas CLI untuk mensimulasikan uji beban persetujuan klaim secara serentak menggunakan goroutine dan `sync/atomic`:
```bash
go run ./tools/loadtest_claim/main.go -url http://localhost:3000/api/v1/claims/1/status -token <FINDER_BEARER_TOKEN> -n 20
```
*Hasil Empiris:*
- **200 OK**: Tepat 1 request (5%) — Versi item naik dari 1 menjadi 2 (*First-Win*).
- **409 Conflict**: Tepat 19 request (95%) — Ditolak aman oleh OCC (*klausul WHERE version = 1 tidak lagi terpenuhi*).
- **500 Error / Crash**: 0 request (0%) — Sistem 100% konsisten tanpa deadlock.

---

### D. Generator Token Pengujian Cepat (`tools/gentoken`)
Menerbitkan Access Token JWT instan untuk mempermudah pengujian di Postman tanpa harus melakukan registrasi/login manual berulang:
```bash
# Generate token untuk user 'lifia' (Role: user)
go run ./tools/gentoken/main.go -user lifia

# Generate token untuk akun dengan username 'admin' (Role: admin)
go run ./tools/gentoken/main.go -user admin
```
Gunakan token yang dicetak pada terminal ke dalam header Postman:  
`Authorization: Bearer <TOKEN>`

---

## 🔒 Standar Keamanan & Pertahanan Sistem

1. **Pencegahan SQL Injection**: Seluruh query relasional menggunakan *parameterized queries* bawaan driver `pgxpool`.
2. **Pencegahan NoSQL Injection**: Seluruh input MongoDB divalidasi ketat (*strongly-typed struct*) di gerbang API Fiber.
3. **Pencegahan Race Condition**: Menggunakan Optimistic Concurrency Control (OCC) pada kolom `version` dan transaksi atomik `pgx.Tx`.
4. **Keamanan Dual Token JWT**: Access Token berumur singkat (15 menit), sedangkan Refresh Token (7 hari) disimpan dalam bentuk hash kriptografis SHA-256 di basis data dan mendukung rotasi otomatis serta pencabutan sesi logout.
5. **Proteksi Wewenang Dinamis**: Middleware `RequirePermission` dan *Self-Role Modification Guard* melindungi akun administrator dari kesalahan konfigurasi wewenang sendiri.

---

## 📄 Berkas Laporan Akademik

Format penyusunan laporan akhir tugas besar, rekapitulasi 56 tangkapan layar pengujian, dan matriks UAT lengkap terdokumentasi di:
- **[laporan.docx](file:///d:/Go/FindIT-API/laporan.docx)** (Laporan Tugas Besar Resmi - Format Microsoft Word)
