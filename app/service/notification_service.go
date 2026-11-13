package service

import (
	"findit-api/app/model"
	"findit-api/app/repository"
	"findit-api/helper"

	"github.com/gofiber/fiber/v2"
	"github.com/hibiken/asynq"
)

type NotificationService struct {
	notifications repository.NotificationRepository
	outbox        repository.OutboxRepository
	inspector     *asynq.Inspector
}

func NewNotificationService(notifications repository.NotificationRepository, outbox repository.OutboxRepository, inspector *asynq.Inspector) *NotificationService {
	return &NotificationService{notifications: notifications, outbox: outbox, inspector: inspector}
}

// Mine godoc
//
// @Summary Daftar notifikasi milik pengguna yang login
// @Description Menampilkan riwayat notifikasi email/in-app yang diterima oleh pengguna aktif.
// @Tags notifications
// @Produce json
// @Security BearerAuth
// @Param limit query int false "Batas jumlah notifikasi (default 20)"
// @Success 200 {object} model.WebResponse{data=[]model.Notification}
// @Failure 401 {object} model.ErrorResponse "Belum terautentikasi"
// @Router /notifications/mine [get]
func (s *NotificationService) Mine(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	user, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}
	limit := c.QueryInt("limit", 20)
	list, err := s.notifications.FindByUser(ctx, user.UserID, limit)
	if err != nil {
		return helper.Internal(err)
	}
	return helper.Success(c, fiber.StatusOK, "notifikasi berhasil diambil", list)
}

// Monitor godoc
//
// @Summary Monitor status antrian Asynq dan transactional outbox
// @Description Memantau statistik antrian Redis Asynq dan baris unpublished tabel outbox PostgreSQL.
// @Tags notifications
// @Produce json
// @Security BearerAuth
// @Success 200 {object} model.WebResponse{data=model.QueueMonitorResponse}
// @Failure 401 {object} model.ErrorResponse "Belum terautentikasi"
// @Failure 403 {object} model.ErrorResponse "Tidak memiliki izin queue:monitor"
// @Router /admin/queue [get]
func (s *NotificationService) Monitor(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	outboxStats, err := s.outbox.Stats(ctx)
	if err != nil {
		return helper.Internal(err)
	}
	totalSent, err := s.notifications.CountAll(ctx)
	if err != nil {
		return helper.Internal(err)
	}
	var qStats []model.QueueStats
	if s.inspector != nil {
		for _, q := range []string{model.QueueCritical, model.QueueDefault} {
			if info, err := s.inspector.GetQueueInfo(q); err == nil {
				qStats = append(qStats, model.QueueStats{
					Queue: q, Pending: info.Pending, Active: info.Active,
					Scheduled: info.Scheduled, Retry: info.Retry, Archived: info.Archived,
				})
			}
		}
	}
	return helper.Success(c, fiber.StatusOK, "status antrian worker", model.QueueMonitorResponse{
		Outbox: outboxStats, Queues: qStats, Notifications: totalSent,
	})
}
