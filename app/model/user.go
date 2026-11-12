package model

import "time"

// User merepresentasikan entitas mahasiswa/pengguna pada tabel PostgreSQL users.
type User struct {
	ID           int       `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	FullName     string    `json:"full_name"`
	Phone        string    `json:"phone,omitempty"`
	Role         string    `json:"role"`
	TrustScore   int       `json:"trust_score"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// UpdateUserRequest memuat field profil pengguna yang dapat diperbarui.
type UpdateUserRequest struct {
	FullName string `json:"full_name" validate:"required,min=2,max=100"`
	Phone    string `json:"phone" validate:"omitempty,min=8,max=20"`
}

// UserReputationResponse menggabungkan profil skor PostgreSQL dengan riwayat log MongoDB.
type UserReputationResponse struct {
	User       User            `json:"user"`
	Score      int             `json:"score"`
	Badge      string          `json:"badge"`
	TotalFound int64           `json:"total_found"`
	History    []ReputationLog `json:"history"`
}


type CreateUserRequest struct {
	Username string `json:"username" validate:"required,min=3,max=50,username"`
	Email    string `json:"email" validate:"required,email,max=120"`
	Password string `json:"password" validate:"required,max=72,strongpassword"`
	FullName string `json:"full_name" validate:"required,min=2,max=100"`
	Phone    string `json:"phone" validate:"omitempty,min=8,max=20"`
	Role     string `json:"role" validate:"omitempty,oneof=admin staff user"`
}

type ReplaceUserRequest struct {
	Username string `json:"username" validate:"required,min=3,max=50,username"`
	Email    string `json:"email" validate:"required,email,max=120"`
	FullName string `json:"full_name" validate:"required,min=2,max=100"`
	Phone    string `json:"phone" validate:"omitempty,min=8,max=20"`
}

type PatchUserRequest struct {
	FullName *string `json:"full_name" validate:"omitempty,min=2,max=100"`
	Phone    *string `json:"phone" validate:"omitempty,min=8,max=20"`
}

type AssignRoleRequest struct {
	Role string `json:"role" validate:"required,oneof=admin staff user"`
}
