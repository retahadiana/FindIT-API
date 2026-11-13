package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"findit-api/app/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const userColumns = "id, username, email, password_hash, full_name, COALESCE(phone, ''), role, trust_score, created_at, updated_at"

// UserRepository adalah kontrak penyimpanan data pengguna.
type UserRepository interface {
	Create(ctx context.Context, u model.User) (model.User, error)
	FindByUsername(ctx context.Context, username string) (model.User, error)
	FindByEmail(ctx context.Context, email string) (model.User, error)
	FindByID(ctx context.Context, id int) (model.User, error)
	UpdateProfile(ctx context.Context, id int, fullName, phone string) (model.User, error)
	UpdateTrustScore(ctx context.Context, db DBTX, id int, deltaPoints int) (model.User, error)
	FindAfterCursor(ctx context.Context, q model.CursorQuery) ([]model.User, error)
	Update(ctx context.Context, id int, u model.User) (model.User, error)
	Delete(ctx context.Context, id int) error
	UpdateRole(ctx context.Context, id int, role string) (model.User, error)
}

type userPostgresRepository struct {
	pool *pgxpool.Pool
}

// NewUserRepository membuat implementasi PostgreSQL untuk UserRepository.
func NewUserRepository(pool *pgxpool.Pool) UserRepository {
	return &userPostgresRepository{pool: pool}
}

func scanUser(row pgx.Row) (model.User, error) {
	var u model.User
	err := row.Scan(
		&u.ID,
		&u.Username,
		&u.Email,
		&u.PasswordHash,
		&u.FullName,
		&u.Phone,
		&u.Role,
		&u.TrustScore,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	return u, err
}

func (r *userPostgresRepository) Create(ctx context.Context, u model.User) (model.User, error) {
	role := u.Role
	if role == "" {
		role = "user"
	}
	trustScore := u.TrustScore
	if trustScore == 0 {
		trustScore = 100
	}

	created, err := scanUser(r.pool.QueryRow(ctx,
		`INSERT INTO users (username, email, password_hash, full_name, phone, role, trust_score)
		 VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6, $7)
		 RETURNING `+userColumns,
		u.Username, u.Email, u.PasswordHash, u.FullName, u.Phone, role, trustScore,
	))
	if err != nil {
		if isUniqueViolation(err) {
			return model.User{}, ErrDuplicate
		}
		return model.User{}, fmt.Errorf("menyimpan user: %w", err)
	}
	return created, nil
}

func (r *userPostgresRepository) FindByUsername(ctx context.Context, username string) (model.User, error) {
	u, err := scanUser(r.pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE LOWER(username) = LOWER($1)`,
		username,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.User{}, ErrNotFound
		}
		return model.User{}, fmt.Errorf("mengambil user berdasarkan username: %w", err)
	}
	return u, nil
}

func (r *userPostgresRepository) FindByEmail(ctx context.Context, email string) (model.User, error) {
	u, err := scanUser(r.pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE LOWER(email) = LOWER($1)`,
		email,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.User{}, ErrNotFound
		}
		return model.User{}, fmt.Errorf("mengambil user berdasarkan email: %w", err)
	}
	return u, nil
}

func (r *userPostgresRepository) FindByID(ctx context.Context, id int) (model.User, error) {
	u, err := scanUser(r.pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE id = $1`,
		id,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.User{}, ErrNotFound
		}
		return model.User{}, fmt.Errorf("mengambil user berdasarkan id: %w", err)
	}
	return u, nil
}

func (r *userPostgresRepository) UpdateProfile(ctx context.Context, id int, fullName, phone string) (model.User, error) {
	updated, err := scanUser(r.pool.QueryRow(ctx,
		`UPDATE users
		 SET full_name = $1, phone = NULLIF($2, ''), updated_at = NOW()
		 WHERE id = $3
		 RETURNING `+userColumns,
		fullName, phone, id,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.User{}, ErrNotFound
		}
		return model.User{}, fmt.Errorf("memperbarui profil user: %w", err)
	}
	return updated, nil
}

func (r *userPostgresRepository) UpdateTrustScore(ctx context.Context, db DBTX, id int, deltaPoints int) (model.User, error) {
	queryer := db
	if queryer == nil {
		queryer = r.pool
	}
	updated, err := scanUser(queryer.QueryRow(ctx,
		`UPDATE users
		 SET trust_score = GREATEST(0, trust_score + $1), updated_at = NOW()
		 WHERE id = $2
		 RETURNING `+userColumns,
		deltaPoints, id,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.User{}, ErrNotFound
		}
		return model.User{}, fmt.Errorf("memperbarui trust score user: %w", err)
	}
	return updated, nil
}


func (r *userPostgresRepository) FindAfterCursor(ctx context.Context, q model.CursorQuery) ([]model.User, error) {
	conds := []string{"1=1"}
	args := []any{}
	idx := 1
	if q.Search != "" {
		conds = append(conds, fmt.Sprintf("(username ILIKE $%d OR full_name ILIKE $%d)", idx, idx))
		args = append(args, "%"+q.Search+"%")
		idx++
	}
	if q.After != nil {
		afterTime := time.Unix(0, q.After.CreatedAt).UTC()
		conds = append(conds, fmt.Sprintf("(created_at, id) < ($%d, $%d)", idx, idx+1))
		args = append(args, afterTime, q.After.ID)
		idx += 2
	}
	args = append(args, q.Limit+1)
	query := fmt.Sprintf(`SELECT `+userColumns+` FROM users WHERE %s ORDER BY created_at DESC, id DESC LIMIT $%d`,
		strings.Join(conds, " AND "), idx)
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, u)
	}
	return list, rows.Err()
}

func (r *userPostgresRepository) Update(ctx context.Context, id int, u model.User) (model.User, error) {
	updated, err := scanUser(r.pool.QueryRow(ctx,
		`UPDATE users SET username = $1, email = $2, full_name = $3, phone = NULLIF($4, ''), updated_at = NOW()
		 WHERE id = $5 RETURNING `+userColumns,
		u.Username, u.Email, u.FullName, u.Phone, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.User{}, ErrNotFound
		}
		if isUniqueViolation(err) {
			return model.User{}, ErrDuplicate
		}
		return model.User{}, err
	}
	return updated, nil
}

func (r *userPostgresRepository) UpdateRole(ctx context.Context, id int, role string) (model.User, error) {
	updated, err := scanUser(r.pool.QueryRow(ctx,
		`UPDATE users SET role = $1, updated_at = NOW() WHERE id = $2 RETURNING `+userColumns, role, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.User{}, ErrNotFound
		}
		return model.User{}, err
	}
	return updated, nil
}

func (r *userPostgresRepository) Delete(ctx context.Context, id int) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
