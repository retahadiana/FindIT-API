// === FILE: app/repository/item_repository.go ===
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

var ErrConflictOCC = errors.New("konflik versi optimistik pada item")

type ItemRepository interface {
	FindAll(ctx context.Context, q model.ItemQuery) ([]model.Item, int, error)
	FindAfterCursor(ctx context.Context, q model.CursorQuery) ([]model.Item, error)
	FindByID(ctx context.Context, id int) (model.Item, error)
	Create(ctx context.Context, item model.Item) (model.Item, error)
	Update(ctx context.Context, id int, req model.UpdateItemRequest) (model.Item, error)
	UpdateStatusWithOCC(ctx context.Context, db DBTX, id int, status string, expectedVersion int) error
	Delete(ctx context.Context, id int) error
}

type itemPostgresRepository struct{ pool *pgxpool.Pool }

func NewItemRepository(pool *pgxpool.Pool) ItemRepository {
	return &itemPostgresRepository{pool: pool}
}

const itemCols = `i.id, i.user_id, i.category_id, c.name, i.title, i.description,
i.item_type, i.location, i.event_date, i.status, i.version, i.created_at,
i.updated_at`

func scanItem(row pgx.Row) (model.Item, error) {
	var it model.Item
	err := row.Scan(&it.ID, &it.UserID, &it.CategoryID, &it.Category, &it.Title,
		&it.Description,
		&it.ItemType, &it.Location, &it.EventDate, &it.Status, &it.Version, &it.CreatedAt,
		&it.UpdatedAt)
	return it, err
}

func (r *itemPostgresRepository) FindAll(ctx context.Context, q model.ItemQuery) ([]model.Item, int, error) {
	conds := []string{"1=1"}
	args := []any{}
	idx := 1
	if q.Search != "" {
		conds = append(conds, fmt.Sprintf("(i.title ILIKE $%d OR i.location ILIKE $%d)", idx, idx))
		args = append(args, "%"+q.Search+"%")
		idx++
	}
	if q.ItemType != "" {
		conds = append(conds, fmt.Sprintf("i.item_type = $%d", idx))
		args = append(args, q.ItemType)
		idx++
	}
	if q.Status != "" {
		conds = append(conds, fmt.Sprintf("i.status = $%d", idx))
		args = append(args, q.Status)
		idx++
	}
	if q.CategoryID > 0 {
		conds = append(conds, fmt.Sprintf("i.category_id = $%d", idx))
		args = append(args, q.CategoryID)
		idx++
	}
	where := strings.Join(conds, " AND ")
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM items i WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, q.Limit, q.Offset())
	query := fmt.Sprintf(`SELECT %s FROM items i JOIN categories c ON c.id = i.category_id
WHERE %s ORDER BY i.created_at DESC, i.id DESC LIMIT $%d OFFSET $%d`, itemCols, where, idx, idx+1)
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var list []model.Item
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, it)
	}
	return list, total, rows.Err()
}

func (r *itemPostgresRepository) FindAfterCursor(ctx context.Context, q model.CursorQuery) ([]model.Item, error) {
	conds := []string{"1=1"}
	args := []any{}
	idx := 1
	if q.Search != "" {
		conds = append(conds, fmt.Sprintf("(i.title ILIKE $%d OR i.location ILIKE $%d)", idx, idx))
		args = append(args, "%"+q.Search+"%")
		idx++
	}
	if q.After != nil {
		afterTime := time.Unix(0, q.After.CreatedAt).UTC()
		conds = append(conds, fmt.Sprintf("(i.created_at, i.id) < ($%d, $%d)", idx, idx+1))
		args = append(args, afterTime, q.After.ID)
		idx += 2
	}
	args = append(args, q.Limit+1)
	query := fmt.Sprintf(`SELECT %s FROM items i JOIN categories c ON c.id = i.category_id
WHERE %s ORDER BY i.created_at DESC, i.id DESC LIMIT $%d`, itemCols, strings.Join(conds, " AND "), idx)
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.Item
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, it)
	}
	return list, rows.Err()
}

func (r *itemPostgresRepository) FindByID(ctx context.Context, id int) (model.Item, error) {
	it, err := scanItem(r.pool.QueryRow(ctx,
		`SELECT `+itemCols+` FROM items i JOIN categories c ON c.id = i.category_id WHERE i.id = $1`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Item{}, ErrNotFound
		}
		return model.Item{}, err
	}
	return it, nil
}

func (r *itemPostgresRepository) Create(ctx context.Context, it model.Item) (model.Item, error) {
	var id int
	err := r.pool.QueryRow(ctx,
		`INSERT INTO items (user_id, category_id, title, description, item_type, location, event_date, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, 'open') RETURNING id`,
		it.UserID, it.CategoryID, it.Title, it.Description, it.ItemType, it.Location, it.EventDate,
	).Scan(&id)
	if err != nil {
		return model.Item{}, err
	}
	return r.FindByID(ctx, id)
}

func (r *itemPostgresRepository) Update(ctx context.Context, id int, req model.UpdateItemRequest) (model.Item, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE items SET category_id = $1, title = $2, description = $3, location = $4, status = $5,
version = version + 1, updated_at = NOW() WHERE id = $6`,
		req.CategoryID, req.Title, req.Description, req.Location, req.Status, id)
	if err != nil {
		return model.Item{}, err
	}
	if tag.RowsAffected() == 0 {
		return model.Item{}, ErrNotFound
	}
	return r.FindByID(ctx, id)
}

func (r *itemPostgresRepository) UpdateStatusWithOCC(ctx context.Context, db DBTX, id int, status string, expectedVersion int) error {
	queryer := db
	if queryer == nil {
		queryer = r.pool
	}
	tag, err := queryer.Exec(ctx,
		`UPDATE items SET status = $1, version = version + 1, updated_at = NOW()
WHERE id = $2 AND version = $3`,
		status, id, expectedVersion)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrConflictOCC
	}
	return nil
}

func (r *itemPostgresRepository) Delete(ctx context.Context, id int) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM items WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
