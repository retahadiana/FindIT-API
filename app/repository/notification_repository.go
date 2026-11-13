package repository

import (
	"context"
	"findit-api/app/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

type NotificationRepository interface {
	CreateIfNotExists(ctx context.Context, n model.Notification) (inserted bool, err error)
	FindByUser(ctx context.Context, userID, limit int) ([]model.Notification, error)
	CountAll(ctx context.Context) (int64, error)
}

type notificationPostgresRepository struct{ pool *pgxpool.Pool }

func NewNotificationRepository(pool *pgxpool.Pool) NotificationRepository {
	return &notificationPostgresRepository{pool: pool}
}

func (r *notificationPostgresRepository) CreateIfNotExists(ctx context.Context, n model.Notification) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO notifications (outbox_id, user_id, channel, subject, body)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (outbox_id, user_id) DO NOTHING`,
		n.OutboxID, n.UserID, n.Channel, n.Subject, n.Body,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (r *notificationPostgresRepository) FindByUser(ctx context.Context, userID, limit int) ([]model.Notification, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, outbox_id, user_id, channel, subject, body, sent_at
		FROM notifications WHERE user_id = $1 ORDER BY outbox_id DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.Notification
	for rows.Next() {
		var n model.Notification
		if err := rows.Scan(&n.ID, &n.OutboxID, &n.UserID, &n.Channel, &n.Subject, &n.Body, &n.SentAt); err != nil {
			return nil, err
		}
		list = append(list, n)
	}
	return list, rows.Err()
}

func (r *notificationPostgresRepository) CountAll(ctx context.Context) (int64, error) {
	var total int64
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM notifications`).Scan(&total)
	return total, err
}
