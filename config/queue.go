package config

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"findit-api/app/model"

	"github.com/hibiken/asynq"
)

func RedisOpt() asynq.RedisClientOpt {
	return asynq.RedisClientOpt{
		Addr:     GetEnv("REDIS_ADDR", "localhost:6379"),
		Password: GetEnv("REDIS_PASSWORD", ""),
		DB:       GetEnvInt("REDIS_DB", 0),
	}
}

func NewAsynqClient() *asynq.Client       { return asynq.NewClient(RedisOpt()) }
func NewAsynqInspector() *asynq.Inspector { return asynq.NewInspector(RedisOpt()) }

func NewAsynqServer(logger *slog.Logger) *asynq.Server {
	concurrency := GetEnvInt("WORKER_CONCURRENCY", 5)
	return asynq.NewServer(RedisOpt(), asynq.Config{
		Concurrency: concurrency,
		Queues:      map[string]int{model.QueueCritical: 6, model.QueueDefault: 4},
		ErrorHandler: asynq.ErrorHandlerFunc(func(_ context.Context, task *asynq.Task, err error) {
			logger.Error("asynq_task_failed", slog.String("type", task.Type()), slog.String("error", err.Error()))
		}),
		RetryDelayFunc: func(n int, _ error, _ *asynq.Task) time.Duration {
			return time.Duration(n+1) * 5 * time.Second
		},
	})
}

func PingRedis(ctx context.Context) error {
	insp := NewAsynqInspector()
	defer insp.Close()
	if _, err := insp.Queues(); err != nil {
		return fmt.Errorf("redis/asynq ping gagal: %w", err)
	}
	return nil
}
