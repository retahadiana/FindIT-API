package repository

import (
	"context"
	"errors"

	"findit-api/app/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ClaimRepository interface {
	Create(ctx context.Context, db DBTX, c model.Claim) (model.Claim, error)
	FindByID(ctx context.Context, id int) (model.Claim, error)
	FindByItemID(ctx context.Context, itemID int) ([]model.Claim, error)
	FindByClaimant(ctx context.Context, claimantID int) ([]model.Claim, error)
	UpdateStatus(ctx context.Context, db DBTX, id int, status string) (model.Claim, error)
}

type claimPostgresRepository struct{ pool *pgxpool.Pool }

func NewClaimRepository(pool *pgxpool.Pool) ClaimRepository {
	return &claimPostgresRepository{pool: pool}
}

func scanClaim(row pgx.Row) (model.Claim, error) {
	var c model.Claim
	err := row.Scan(&c.ID, &c.ItemID, &c.ClaimantID, &c.ProofDescription, &c.Status, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

func (r *claimPostgresRepository) Create(ctx context.Context, db DBTX, c model.Claim) (model.Claim, error) {
	q := db
	if q == nil {
		q = r.pool
	}
	created, err := scanClaim(q.QueryRow(ctx,
		`INSERT INTO claims (item_id, claimant_id, proof_description, status)
 VALUES ($1, $2, $3, 'pending')
 RETURNING id, item_id, claimant_id, proof_description, status, created_at, updated_at`,
		c.ItemID, c.ClaimantID, c.ProofDescription))
	if err != nil {
		if isUniqueViolation(err) {
			return model.Claim{}, ErrDuplicate
		}
		return model.Claim{}, err
	}
	return created, nil
}

func (r *claimPostgresRepository) FindByID(ctx context.Context, id int) (model.Claim, error) {
	c, err := scanClaim(r.pool.QueryRow(ctx,
		`SELECT id, item_id, claimant_id, proof_description, status, created_at, updated_at FROM claims WHERE id = $1`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Claim{}, ErrNotFound
		}
		return model.Claim{}, err
	}
	return c, nil
}

func (r *claimPostgresRepository) FindByItemID(ctx context.Context, itemID int) ([]model.Claim, error) {
	return r.queryMany(ctx, `SELECT id, item_id, claimant_id, proof_description, status, created_at, updated_at FROM claims WHERE item_id = $1 ORDER BY created_at DESC`, itemID)
}

func (r *claimPostgresRepository) FindByClaimant(ctx context.Context, claimantID int) ([]model.Claim, error) {
	return r.queryMany(ctx, `SELECT id, item_id, claimant_id, proof_description, status, created_at, updated_at FROM claims WHERE claimant_id = $1 ORDER BY created_at DESC`, claimantID)
}

func (r *claimPostgresRepository) queryMany(ctx context.Context, sql string, arg any) ([]model.Claim, error) {
	rows, err := r.pool.Query(ctx, sql, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.Claim
	for rows.Next() {
		c, err := scanClaim(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

func (r *claimPostgresRepository) UpdateStatus(ctx context.Context, db DBTX, id int, status string) (model.Claim, error) {
	q := db
	if q == nil {
		q = r.pool
	}
	updated, err := scanClaim(q.QueryRow(ctx,
		`UPDATE claims SET status = $1, updated_at = NOW() WHERE id = $2
 RETURNING id, item_id, claimant_id, proof_description, status, created_at, updated_at`, status, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Claim{}, ErrNotFound
		}
		return model.Claim{}, err
	}
	return updated, nil
}