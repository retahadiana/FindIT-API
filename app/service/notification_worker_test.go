package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"

	"findit-api/app/model"
	"findit-api/app/repository"
	"findit-api/helper"

	"github.com/hibiken/asynq"
)

type stubWorkerUsers struct {
	repository.UserRepository
	users map[int]model.User
}

func (s *stubWorkerUsers) FindByID(_ context.Context, id int) (model.User, error) {
	u, ok := s.users[id]
	if !ok {
		return model.User{}, repository.ErrNotFound
	}
	return u, nil
}

type stubWorkerNotifs struct {
	repository.NotificationRepository
	seen map[int64]bool
}

func (s *stubWorkerNotifs) CreateIfNotExists(_ context.Context, n model.Notification) (bool, error) {
	if s.seen[n.OutboxID] {
		return false, nil
	}
	s.seen[n.OutboxID] = true
	return true, nil
}

type stubMailer struct {
	sent int
	err  error
}

func (m *stubMailer) Send(_ context.Context, _, _, _ string) error {
	if m.err != nil {
		return m.err
	}
	m.sent++
	return nil
}

func TestNotificationWorker_DeliverAndIdempotent(t *testing.T) {
	users := &stubWorkerUsers{users: map[int]model.User{7: {ID: 7, Email: "mhs@kampus.ac.id"}}}
	notifs := &stubWorkerNotifs{seen: map[int64]bool{}}
	mailer := &stubMailer{}
	w := NewNotificationWorker(users, notifs, mailer, slog.New(slog.NewTextHandler(io.Discard, nil)))

	payload, _ := json.Marshal(model.EmailDeliveryPayload{
		OutboxID: 42, EventType: model.EventClaimSubmitted, UserID: 7,
		Subject: "Klaim Baru", Body: "Silakan cek verifikasi",
	})
	task := asynq.NewTask(model.TaskEmailSend, payload)

	if err := w.HandleDeliverEmail(context.Background(), task); err != nil {
		t.Fatalf("eksekusi pertama gagal: %v", err)
	}
	if err := w.HandleDeliverEmail(context.Background(), task); err != nil {
		t.Fatalf("eksekusi duplikat harus tetap sukses idempoten: %v", err)
	}

	mailer.err = helper.ErrMailerTransient
	if err := w.HandleDeliverEmail(context.Background(), task); !errors.Is(err, helper.ErrMailerTransient) {
		t.Fatalf("transient error harus diteruskan untuk retry Asynq, dapat: %v", err)
	}
}
