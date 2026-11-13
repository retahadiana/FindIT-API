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
	CategoryService     *service.CategoryService     // [Reta]
	ItemService         *service.ItemService         // [Reta]
	ClaimService        *service.ClaimService        // [Zaskia]
	VerificationService *service.VerificationService // [Zaskia]
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

	// Fase 2 — Categories & Items (Reta)
	if deps.CategoryService != nil {
		cat := api.Group("/categories")
		cat.Get("/", deps.CategoryService.List)
		cat.Post("/", middleware.RequireAuth(deps.JWT),
			middleware.RequirePermission(deps.Permissions, "category:manage"),
			middleware.RequireJSON, deps.CategoryService.Create)
	}

	if deps.ItemService != nil {
		items := api.Group("/items")
		items.Get("/", deps.ItemService.List)
		items.Get("/:id", deps.ItemService.Get)
		items.Post("/", middleware.RequireAuth(deps.JWT), middleware.RequireJSON,
			deps.ItemService.Create)
		items.Put("/:id", middleware.RequireAuth(deps.JWT), middleware.RequireJSON,
			deps.ItemService.Update)
		items.Delete("/:id", middleware.RequireAuth(deps.JWT), deps.ItemService.Delete)
	}

	// Fase 3 — Claims & Verification (Zaskia)
	if deps.ClaimService != nil {
		claims := api.Group("/claims", middleware.RequireAuth(deps.JWT))
		claims.Get("/me", deps.ClaimService.MyClaims)
		claims.Get("/:id", deps.ClaimService.Get)
		claims.Patch("/:id/status", middleware.RequireJSON, deps.ClaimService.UpdateStatus)

		api.Post("/items/:id/claims", middleware.RequireAuth(deps.JWT), middleware.RequireJSON, deps.ClaimService.Create)
		api.Get("/items/:id/claims", middleware.RequireAuth(deps.JWT), deps.ClaimService.ListByItem)
	}

	if deps.VerificationService != nil {
		verif := api.Group("/claims/:id/verification", middleware.RequireAuth(deps.JWT))
		verif.Get("/", deps.VerificationService.GetSession)
		verif.Post("/", middleware.RequireJSON, deps.VerificationService.CreateSession)
		verif.Put("/answers", middleware.RequireJSON, deps.VerificationService.AnswerQuestions)
		verif.Post("/review", middleware.RequireJSON, deps.VerificationService.ReviewVerifications)
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
