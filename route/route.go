package route

import (
	"context"
	"time"

	"findit-api/helper"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// Dependencies menampung seluruh dependensi service yang disuntikkan ke router.
type Dependencies struct {
	Pool  *pgxpool.Pool
	Mongo *mongo.Client
}

// Register mendaftarkan seluruh endpoint API FindIt ke Fiber app.
func Register(app *fiber.App, deps Dependencies) {
	api := app.Group("/api/v1")
	api.Get("/health", healthCheck(deps.Pool, deps.Mongo))
}

func healthCheck(pool *pgxpool.Pool, mongoClient *mongo.Client) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.UserContext(), 2*time.Second)
		defer cancel()

		if pool == nil {
			return helper.ServiceUnavailable("PostgreSQL belum terkonfigurasi")
		}
		if err := pool.Ping(ctx); err != nil {
			return helper.ServiceUnavailable("PostgreSQL tidak dapat dihubungi")
		}
		if mongoClient == nil {
			return helper.ServiceUnavailable("MongoDB belum terkonfigurasi")
		}
		if err := mongoClient.Ping(ctx, nil); err != nil {
			return helper.ServiceUnavailable("MongoDB tidak dapat dihubungi")
		}

		return helper.Success(c, fiber.StatusOK, "server, PostgreSQL, dan MongoDB berjalan", fiber.Map{
			"postgres": "connected",
			"mongodb":  "connected",
		})
	}
}
