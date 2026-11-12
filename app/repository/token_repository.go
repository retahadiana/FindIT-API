package repository

import (
	"context"
	"errors"
	"fmt"

	"findit-api/app/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TokenRepository adalah kontrak penyimpanan refresh token.
type TokenRepository interface {
	Save(ctx context.Context, t model.RefreshToken) error
	FindByHash(ctx context.Context, tokenHash string) (model.RefreshToken, error)
	Revoke(ctx context.Context, tokenHash string) error
	RevokeAllUserTokens(ctx context.Context, userID int) error
}

type tokenPostgresRepository struct {
	pool *pgxpool.Pool
}

// NewTokenRepository membuat implementasi PostgreSQL untuk TokenRepository.
func NewTokenRepository(pool *pgxpool.Pool) TokenRepository {
	return &tokenPostgresRepository{pool: pool}
}

func (r *tokenPostgresRepository) Save(ctx context.Context, t model.RefreshToken) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		 VALUES ($1, $2, $3)`,
		t.UserID, t.TokenHash, t.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("menyimpan refresh token: %w", err)
	}
	return nil
}

// FindByHash hanya mengembalikan refresh token yang belum dicabut dan belum kedaluwarsa.
func (r *tokenPostgresRepository) FindByHash(ctx context.Context, tokenHash string) (model.RefreshToken, error) {
	var t model.RefreshToken
	err := r.pool.QueryRow(ctx,
		`SELECT id, user_id, token_hash, expires_at, revoked_at, created_at
		 FROM refresh_tokens
		 WHERE token_hash = $1
		   AND revoked_at IS NULL
		   AND expires_at > NOW()`,
		tokenHash,
	).Scan(&t.ID, &t.UserID, &t.TokenHash, &t.ExpiresAt, &t.RevokedAt, &t.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.RefreshToken{}, ErrNotFound
		}
		return model.RefreshToken{}, fmt.Errorf("mengambil refresh token: %w", err)
	}
	return t, nil
}

func (r *tokenPostgresRepository) Revoke(ctx context.Context, tokenHash string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE refresh_tokens
		 SET revoked_at = NOW()
		 WHERE token_hash = $1 AND revoked_at IS NULL`,
		tokenHash,
	)
	if err != nil {
		return fmt.Errorf("mencabut refresh token: %w", err)
	}
	return nil
}

func (r *tokenPostgresRepository) RevokeAllUserTokens(ctx context.Context, userID int) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE refresh_tokens
		 SET revoked_at = NOW()
		 WHERE user_id = $1 AND revoked_at IS NULL`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("mencabut seluruh refresh token user: %w", err)
	}
	return nil
}
