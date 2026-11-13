package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"findit-api/app/repository"
	"findit-api/app/service"
	"findit-api/config"
	"findit-api/database"
	"findit-api/helper"
	"findit-api/route"
)

// @title FindIt API - Lost & Found Kampus
// @version 1.0
// @description API Lost and Found Kampus untuk pelaporan dan pencarian barang hilang/temuan, verifikasi klaim, reputasi, dan notifikasi outbox.
// @BasePath /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Masukkan token dengan format: Bearer <access_token>
func main() {
	config.LoadEnv()
	logger := config.NewLogger()
	role := flag.String("role", config.GetEnv("APP_ROLE", "all"), "api | worker | all")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.NewPool(ctx)
	if err != nil {
		logger.Error("postgres gagal terhubung", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer pool.Close()

	if *role == "worker" || *role == "all" {
		go func() {
			if err := runWorker(ctx, pool, logger); err != nil {
				logger.Error("worker berhenti", slog.String("error", err.Error()))
			}
		}()
	}

	if *role == "api" || *role == "all" {
		mongoClient, err := database.NewMongoClient(ctx)
		if err != nil {
			logger.Error("mongodb gagal terhubung", slog.String("error", err.Error()))
			os.Exit(1)
		}
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = mongoClient.Disconnect(shutdownCtx)
		}()

		mongoDB := mongoClient.Database(config.GetEnv("MONGO_DATABASE", "findit_db"))

		userRepo := repository.NewUserRepository(pool)
		tokenRepo := repository.NewTokenRepository(pool)
		roleRepo := repository.NewRoleRepository(pool)
		outboxRepo := repository.NewOutboxRepository(pool)
		notifRepo := repository.NewNotificationRepository(pool)
		catRepo := repository.NewCategoryRepository(pool)
		itemRepo := repository.NewItemRepository(pool)
		claimRepo := repository.NewClaimRepository(pool)
		verifRepo := repository.NewVerificationRepository(mongoDB)
		_ = verifRepo.EnsureIndexes(ctx)
		txMgr := repository.NewTxManager(pool)
		repRepo := repository.NewReputationRepository(mongoDB)
		_ = repRepo.EnsureIndexes(ctx)

		perms, err := roleRepo.LoadPermissions(ctx)
		if err != nil {
			logger.Warn("gagal load permissions dari db, fallback default", slog.String("error", err.Error()))
			perms = helper.DefaultPermissions()
		}

		inspector := config.NewAsynqInspector()
		defer inspector.Close()

		jwtTTL := time.Duration(config.GetEnvInt("JWT_ACCESS_TTL_MINUTES", 60)) * time.Minute
		refreshTTL := time.Duration(config.GetEnvInt("JWT_REFRESH_TTL_DAYS", 7)) * 24 * time.Hour
		jwtMgr := helper.NewJWTManager(config.GetEnv("JWT_SECRET", ""), config.GetEnv("JWT_ISSUER", "findit-api"), jwtTTL)

		app := config.NewApp(logger, route.Dependencies{
			Pool:                pool,
			Mongo:               mongoClient,
			JWT:                 jwtMgr,
			Permissions:         perms,
			AuthService:         service.NewAuthService(userRepo, tokenRepo, jwtMgr, perms, refreshTTL),
			UserService:         service.NewUserService(userRepo, perms),
			ReputationService:   service.NewReputationService(userRepo, repRepo, outboxRepo),
			NotificationService: service.NewNotificationService(notifRepo, outboxRepo, inspector),
			CategoryService:     service.NewCategoryService(catRepo),
			ItemService:         service.NewItemService(itemRepo, catRepo),
			ClaimService:        service.NewClaimService(claimRepo, itemRepo, outboxRepo, txMgr),
			VerificationService: service.NewVerificationService(verifRepo, claimRepo, itemRepo),
			EnableSwagger:       config.GetEnv("SWAGGER_ENABLED", "true") == "true",
		}, config.GetEnv("ALLOWED_ORIGINS", "*"))

		port := config.GetEnv("APP_PORT", "3000")
		go func() {
			logger.Info("server berjalan", slog.String("port", port), slog.String("role", *role))
			if err := app.Listen(":" + port); err != nil {
				logger.Error("server berhenti", slog.String("error", err.Error()))
			}
		}()

		<-ctx.Done()
		_ = app.ShutdownWithTimeout(10 * time.Second)
		return
	}

	<-ctx.Done()
}
