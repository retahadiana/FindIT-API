package model

import "time"

// RegisterRequest memuat payload pendaftaran akun mahasiswa baru.
type RegisterRequest struct {
	Username string `json:"username" validate:"required,min=3,max=50,username"`
	Email    string `json:"email" validate:"required,email,max=120"`
	Password string `json:"password" validate:"required,max=72,strongpassword"`
	FullName string `json:"full_name" validate:"required,min=2,max=100"`
	Phone    string `json:"phone" validate:"omitempty,min=8,max=20"`
}

// LoginRequest memuat kredensial masuk pengguna.
type LoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

// RefreshRequest memuat refresh token untuk rotasi atau pencabutan sesi.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// TokenPair adalah pasangan access token dan refresh token yang dikirim ke klien.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

// RefreshToken merepresentasikan baris pada tabel refresh_tokens PostgreSQL.
// Nilai yang disimpan adalah TokenHash (SHA-256 hex), bukan token mentah.
type RefreshToken struct {
	ID        int64
	UserID    int
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
}

// AuthUser adalah identitas minimal yang disimpan dalam klaim JWT dan Fiber Locals.
type AuthUser struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}


type UserProfile struct {
	User
	Permissions []string `json:"permissions"`
}
