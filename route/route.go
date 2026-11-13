package route

import (
	"context"
	"time"

	"findit-api/app/service"
	"findit-api/helper"
	"findit-api/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// Dependencies menampung seluruh dependensi service yang disuntikkan ke router.
type Dependencies struct {
	Pool                *pgxpool.Pool
	Mongo               *mongo.Client
	JWT                 *helper.JWTManager
	Permissions         *helper.PermissionSet        // [Lifia]
	AuthService         *service.AuthService         // [Lifia]
	UserService         *service.UserService         // [Lifia]
	ReputationService   *service.ReputationService   // [Lifia]
	NotificationService *service.NotificationService // [Lifia]
}

// Register mendaftarkan seluruh endpoint API FindIt modul Fase 1 ke Fiber app.
func Register(app *fiber.App, deps Dependencies) {
	api := app.Group("/api/v1")
	api.Get("/health", healthCheck(deps.Pool, deps.Mongo))

	if deps.AuthService != nil {
		auth := api.Group("/auth", middleware.RequireJSON)
		auth.Post("/register", deps.AuthService.Register)
		auth.Post("/login", middleware.LoginRateLimiter(), deps.AuthService.Login)
		auth.Post("/refresh", deps.AuthService.Refresh)
		auth.Post("/logout", deps.AuthService.Logout)
		auth.Get("/me", middleware.RequireAuth(deps.JWT), deps.AuthService.Me)
	}

	if deps.UserService != nil || deps.ReputationService != nil {
		users := api.Group("/users", middleware.RequireAuth(deps.JWT))
		if deps.UserService != nil {
			users.Get("/", middleware.RequirePermission(deps.Permissions, "user:list"), deps.UserService.List)
			users.Post("/", middleware.RequirePermission(deps.Permissions, "user:update:any"), middleware.RequireJSON, deps.UserService.Create)
			users.Put("/profile", middleware.RequireJSON, deps.UserService.UpdateProfile)
			users.Get("/:id", deps.UserService.GetByID)
			users.Put("/:id", middleware.RequirePermission(deps.Permissions, "user:update:any"), middleware.RequireJSON, deps.UserService.Replace)
			users.Patch("/:id", middleware.RequirePermission(deps.Permissions, "user:update:any"), middleware.RequireJSON, deps.UserService.Patch)
			users.Delete("/:id", middleware.RequirePermission(deps.Permissions, "user:delete"), deps.UserService.Delete)
			users.Patch("/:id/role", middleware.RequirePermission(deps.Permissions, "role:assign"), middleware.RequireJSON, deps.UserService.AssignRole)
		}
		if deps.ReputationService != nil {
			users.Get("/:id/reputation", deps.ReputationService.GetUserReputation)
			users.Get("/:id/reputation/summary", deps.ReputationService.Summary)
			users.Post("/:id/reward", middleware.RequireJSON, deps.ReputationService.AddReward)
		}
	}

	if deps.NotificationService != nil {
		api.Get("/notifications/mine", middleware.RequireAuth(deps.JWT), deps.NotificationService.Mine)
		api.Get("/admin/queue", middleware.RequireAuth(deps.JWT), middleware.RequirePermission(deps.Permissions, "queue:monitor"), deps.NotificationService.Monitor)
	}
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
