-- === FILE: migrations/007_soft_delete.sql ===
-- Migrasi penambahan kolom deleted_at untuk skema Soft Delete pada items dan users

ALTER TABLE items ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP WITH TIME ZONE DEFAULT NULL;
CREATE INDEX IF NOT EXISTS idx_items_deleted_at ON items(deleted_at);

ALTER TABLE users ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP WITH TIME ZONE DEFAULT NULL;
CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users(deleted_at);
