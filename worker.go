package main

import (
	"context"
	"log/slog"
	"path/filepath"
	"time"

	"findit-api/app/model"
	"findit-api/app/repository"
	"findit-api/app/service"
	"findit-api/config"
	"findit-api/helper"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
)

func runWorker(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger) error {
	userRepo := repository.NewUserRepository(pool)
	outboxRepo := repository.NewOutboxRepository(pool)
	notifRepo := repository.NewNotificationRepository(pool)
	txMgr := repository.NewTxManager(pool)

	enqueuer := config.NewAsynqClient()
	defer enqueuer.Close()

	mailer := helper.NewLogMailer(
		filepath.Join("logs", "mail.log"),
		time.Duration(config.GetEnvInt("MAILER_DELAY_MS", 1500))*time.Millisecond,
		config.GetEnvInt("MAILER_FAIL_EVERY", 0),
	)
	worker := service.NewNotificationWorker(userRepo, notifRepo, mailer, logger)
	relay := service.NewOutboxRelay(txMgr, outboxRepo, enqueuer, logger,
		time.Duration(config.GetEnvInt("RELAY_INTERVAL_MS", 1000))*time.Millisecond,
		config.GetEnvInt("RELAY_BATCH", 100),
	)

	mux := asynq.NewServeMux()
	mux.HandleFunc(model.TaskEmailSend, worker.HandleDeliverEmail)

	srv := config.NewAsynqServer(logger)
	go relay.Run(ctx)
	go func() {
		<-ctx.Done()
		srv.Shutdown()
	}()
	return srv.Run(mux)
}
