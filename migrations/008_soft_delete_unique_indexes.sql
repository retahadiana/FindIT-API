-- Migrasi 008: Menyesuaikan constraint UNIQUE users agar mendukung soft delete
-- Menggantikan constraint UNIQUE global dengan partial unique index (WHERE deleted_at IS NULL)
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_username_key;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_email_key;
DROP INDEX IF EXISTS idx_users_username_lower;
DROP INDEX IF EXISTS idx_users_email_lower;

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username_active ON users (LOWER(username)) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_active ON users (LOWER(email)) WHERE deleted_at IS NULL;
