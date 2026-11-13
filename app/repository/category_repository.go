// === FILE: app/repository/category_repository.go ===
package repository

import (
	"context"
	"errors"
	"fmt"

	"findit-api/app/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CategoryRepository interface {
	FindAll(ctx context.Context) ([]model.Category, error)
	FindByID(ctx context.Context, id int) (model.Category, error)
	Create(ctx context.Context, req model.CreateCategoryRequest) (model.Category, error)
}

type categoryPostgresRepository struct{ pool *pgxpool.Pool }

func NewCategoryRepository(pool *pgxpool.Pool) CategoryRepository {
	return &categoryPostgresRepository{pool: pool}
}

func (r *categoryPostgresRepository) FindAll(ctx context.Context) ([]model.Category, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name, COALESCE(description, ''), created_at FROM categories ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("mengambil kategori: %w", err)
	}
	defer rows.Close()

	var list []model.Category
	for rows.Next() {
		var c model.Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

func (r *categoryPostgresRepository) FindByID(ctx context.Context, id int) (model.Category, error) {
	var c model.Category
	err := r.pool.QueryRow(ctx, `SELECT id, name, COALESCE(description, ''), created_at FROM categories WHERE id = $1`, id).
		Scan(&c.ID, &c.Name, &c.Description, &c.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Category{}, ErrNotFound
		}
		return model.Category{}, err
	}
	return c, nil
}

func (r *categoryPostgresRepository) Create(ctx context.Context, req model.CreateCategoryRequest) (model.Category, error) {
	var c model.Category
	err := r.pool.QueryRow(ctx,
		`INSERT INTO categories (name, description) VALUES ($1, NULLIF($2, ''))
RETURNING id, name, COALESCE(description, ''), created_at`,
		req.Name, req.Description,
	).Scan(&c.ID, &c.Name, &c.Description, &c.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return model.Category{}, ErrDuplicate
		}
		return model.Category{}, err
	}
	return c, nil
}
