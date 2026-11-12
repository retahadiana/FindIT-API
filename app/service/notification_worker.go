package service

import (
	"context"
	"encoding/json"
	"log/slog"

	"findit-api/app/model"
	"findit-api/app/repository"
	"findit-api/helper"

	"github.com/hibiken/asynq"
)

type NotificationWorker struct {
	users         repository.UserRepository
	notifications repository.NotificationRepository
	mailer        helper.Mailer
	logger        *slog.Logger
}

func NewNotificationWorker(users repository.UserRepository, notifications repository.NotificationRepository, mailer helper.Mailer, logger *slog.Logger) *NotificationWorker {
	return &NotificationWorker{users: users, notifications: notifications, mailer: mailer, logger: logger}
}

func (w *NotificationWorker) HandleDeliverEmail(ctx context.Context, task *asynq.Task) error {
	var p model.EmailDeliveryPayload
	if err := json.Unmarshal(task.Payload(), &p); err != nil {
		return err
	}
	u, err := w.users.FindByID(ctx, p.UserID)
	if err != nil {
		return err
	}
	if err := w.mailer.Send(ctx, u.Email, p.Subject, p.Body); err != nil {
		return err
	}
	_, err = w.notifications.CreateIfNotExists(ctx, model.Notification{
		OutboxID: p.OutboxID, UserID: p.UserID, Channel: "email", Subject: p.Subject, Body: p.Body,
	})
	return err
}
