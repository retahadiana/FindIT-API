CREATE TABLE IF NOT EXISTS roles (
    name VARCHAR(20) PRIMARY KEY,
    description VARCHAR(150) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO roles (name, description) VALUES
    ('admin', 'Akses penuh terhadap seluruh data, reputasi, kategori, dan antrian'),
    ('staff', 'Boleh melihat data seluruh user, tetapi tidak boleh mengubah role'),
    ('user', 'Pengguna mahasiswa standar untuk laporan dan klaim')
ON CONFLICT (name) DO NOTHING;

CREATE TABLE IF NOT EXISTS permissions (
    name VARCHAR(50) PRIMARY KEY,
    description VARCHAR(150) NOT NULL
);

INSERT INTO permissions (name, description) VALUES
    ('user:list', 'Melihat daftar seluruh user'),
    ('user:read:any', 'Melihat profil user mana pun'),
    ('user:update:any', 'Mengubah data user mana pun'),
    ('user:delete', 'Menghapus user'),
    ('role:assign', 'Mengubah role milik user lain'),
    ('category:manage', 'Mengelola kategori barang'),
    ('queue:monitor', 'Melihat keadaan outbox dan queue task')
ON CONFLICT (name) DO NOTHING;

CREATE TABLE IF NOT EXISTS role_permissions (
    role_name VARCHAR(20) NOT NULL REFERENCES roles(name) ON DELETE CASCADE,
    permission_name VARCHAR(50) NOT NULL REFERENCES permissions(name) ON DELETE CASCADE,
    PRIMARY KEY (role_name, permission_name)
);

INSERT INTO role_permissions (role_name, permission_name) VALUES
    ('admin', 'user:list'), ('admin', 'user:read:any'), ('admin', 'user:update:any'),
    ('admin', 'user:delete'), ('admin', 'role:assign'), ('admin', 'category:manage'),
    ('admin', 'queue:monitor'), ('staff', 'user:list'), ('staff', 'user:read:any')
ON CONFLICT DO NOTHING;

UPDATE users SET role = 'user' WHERE role NOT IN (SELECT name FROM roles);
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_fkey;
ALTER TABLE users ADD CONSTRAINT users_role_fkey FOREIGN KEY (role) REFERENCES roles(name);
