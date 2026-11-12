package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"findit-api/app/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

type OutboxRepository interface {
	Insert(ctx context.Context, db DBTX, eventType string, payload any) (int64, error)
	FetchPending(ctx context.Context, db DBTX, limit int) ([]model.OutboxMessage, error)
	MarkPublished(ctx context.Context, db DBTX, ids []int64) error
	Stats(ctx context.Context) (model.OutboxStats, error)
}

type outboxPostgresRepository struct{ pool *pgxpool.Pool }

func NewOutboxRepository(pool *pgxpool.Pool) OutboxRepository {
	return &outboxPostgresRepository{pool: pool}
}

func (r *outboxPostgresRepository) Insert(ctx context.Context, db DBTX, eventType string, payload any) (int64, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("marshal outbox payload: %w", err)
	}
	queryer := db
	if queryer == nil {
		queryer = r.pool
	}
	var id int64
	err = queryer.QueryRow(ctx,
		`INSERT INTO outbox (event_type, payload) VALUES ($1, $2) RETURNING id`,
		eventType, body,
	).Scan(&id)
	return id, err
}

func (r *outboxPostgresRepository) FetchPending(ctx context.Context, db DBTX, limit int) ([]model.OutboxMessage, error) {
	queryer := db
	if queryer == nil {
		queryer = r.pool
	}
	rows, err := queryer.Query(ctx, `
		SELECT id, event_type, payload, created_at
		FROM outbox
		WHERE published_at IS NULL
		ORDER BY id ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.OutboxMessage
	for rows.Next() {
		var m model.OutboxMessage
		if err := rows.Scan(&m.ID, &m.EventType, &m.Payload, &m.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (r *outboxPostgresRepository) MarkPublished(ctx context.Context, db DBTX, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	queryer := db
	if queryer == nil {
		queryer = r.pool
	}
	_, err := queryer.Exec(ctx, `UPDATE outbox SET published_at = NOW() WHERE id = ANY($1)`, ids)
	return err
}

func (r *outboxPostgresRepository) Stats(ctx context.Context) (model.OutboxStats, error) {
	var s model.OutboxStats
	err := r.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE published_at IS NULL),
			COUNT(*) FILTER (WHERE published_at IS NOT NULL)
		FROM outbox`).Scan(&s.Pending, &s.Published)
	return s, err
}
