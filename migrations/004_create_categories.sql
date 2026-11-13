-- === FILE: migrations/004_create_categories.sql ===
CREATE TABLE IF NOT EXISTS categories (
    id SERIAL PRIMARY KEY,
    name VARCHAR(80) UNIQUE NOT NULL,
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO categories (name, description) VALUES
('Elektronik', 'Laptop, handphone, charger, flashdisk, TWS'),
('Dokumen', 'KTM, KTP, SIM, STNK, dompet'),
('Kendaraan & Kunci', 'Kunci motor, kunci kos, helm, kartu parkir'),
('Aksesoris Pribadi', 'Jam tangan, kacamata, botol minum, payung'),
('Buku & Alat Tulis', 'Buku catatan, modul kuliah, kalkulator')
ON CONFLICT (name) DO NOTHING;

