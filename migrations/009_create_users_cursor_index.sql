-- Migrasi 009: Penambahan indeks komposit keyset cursor pagination untuk tabel users
CREATE INDEX IF NOT EXISTS idx_users_cursor ON users(created_at DESC, id DESC);
