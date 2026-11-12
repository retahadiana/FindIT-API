package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"findit-api/app/model"
	"findit-api/app/repository"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
)

type OutboxRelay struct {
	tx       *repository.TxManager
	outbox   repository.OutboxRepository
	enqueuer *asynq.Client
	logger   *slog.Logger
	interval time.Duration
	batch    int
}

func NewOutboxRelay(tx *repository.TxManager, outbox repository.OutboxRepository, enqueuer *asynq.Client, logger *slog.Logger, interval time.Duration, batch int) *OutboxRelay {
	return &OutboxRelay{tx: tx, outbox: outbox, enqueuer: enqueuer, logger: logger, interval: interval, batch: batch}
}

func (r *OutboxRelay) Run(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		if err := r.ProcessOnce(ctx); err != nil {
			r.logger.Error("outbox_relay_error", slog.String("error", err.Error()))
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return
		}
	}
}

func (r *OutboxRelay) ProcessOnce(ctx context.Context) error {
	return r.tx.WithTxOnce(ctx, func(tx pgx.Tx) error {
		msgs, err := r.outbox.FetchPending(ctx, tx, r.batch)
		if err != nil || len(msgs) == 0 {
			return err
		}
		var published []int64
		for _, msg := range msgs {
			queue, payload, err := buildEmailPayload(msg)
			if err != nil {
				continue
			}
			body, _ := json.Marshal(payload)
			task := asynq.NewTask(model.TaskEmailSend, body, asynq.TaskID(fmt.Sprintf("outbox:%d", msg.ID)))
			if _, err := r.enqueuer.EnqueueContext(ctx, task, asynq.Queue(queue), asynq.MaxRetry(5)); err != nil {
				return err
			}
			published = append(published, msg.ID)
		}
		return r.outbox.MarkPublished(ctx, tx, published)
	})
}

func buildEmailPayload(msg model.OutboxMessage) (string, model.EmailDeliveryPayload, error) {
	switch msg.EventType {
	case model.EventReputationRewarded:
		var ev model.ReputationRewardedEvent
		_ = json.Unmarshal(msg.Payload, &ev)
		return model.QueueDefault, model.EmailDeliveryPayload{
			OutboxID: msg.ID, EventType: msg.EventType, UserID: ev.UserID,
			Subject: "Terima Kasih! Anda Mendapat Poin Reputasi FindIt",
			Body:    fmt.Sprintf("+%d trust score karena: %s", ev.Points, ev.Reason),
		}, nil
	case model.EventClaimSubmitted:
		var ev model.ClaimEvent
		_ = json.Unmarshal(msg.Payload, &ev)
		return model.QueueCritical, model.EmailDeliveryPayload{
			OutboxID: msg.ID, EventType: msg.EventType, UserID: ev.FinderID,
			Subject: "Klaim Baru Masuk untuk Barang: " + ev.ItemTitle,
			Body:    fmt.Sprintf("Klaim #%d membutuhkan verifikasi Anda.", ev.ClaimID),
		}, nil
	case model.EventClaimDecided, model.EventItemReturned:
		var ev model.ClaimEvent
		_ = json.Unmarshal(msg.Payload, &ev)
		return model.QueueCritical, model.EmailDeliveryPayload{
			OutboxID: msg.ID, EventType: msg.EventType, UserID: ev.ClaimantID,
			Subject: fmt.Sprintf("Status Klaim #%d: %s", ev.ClaimID, ev.Status),
			Body:    fmt.Sprintf("Barang '%s' kini berstatus %s.", ev.ItemTitle, ev.Status),
		}, nil
	default:
		return model.QueueDefault, model.EmailDeliveryPayload{}, fmt.Errorf("unknown event: %s", msg.EventType)
	}
}
