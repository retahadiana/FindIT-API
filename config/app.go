package config

import (
	"errors"
	"log/slog"

	"findit-api/app/model"
	"findit-api/helper"
	"findit-api/middleware"
	"findit-api/route"

	"github.com/gofiber/fiber/v2"
)

// NewApp merakit instance Fiber beserta ErrorHandler, middleware global, dan routing.
func NewApp(logger *slog.Logger, deps route.Dependencies, allowedOrigins string) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      GetEnv("APP_NAME", "FindIt API - Lost and Found Portal"),
		ErrorHandler: newErrorHandler(logger),
		BodyLimit:    1 * 1024 * 1024,
	})

	middleware.Register(app, logger, allowedOrigins)
	route.Register(app, deps)

	app.Use(func(c *fiber.Ctx) error {
		return helper.NotFound("endpoint tidak ditemukan")
	})

	return app
}

func newErrorHandler(logger *slog.Logger) fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		requestID := helper.RequestID(c)
		var appErr *helper.AppError

		switch {
		case errors.As(err, &appErr):
		case errors.Is(err, fiber.ErrRequestEntityTooLarge):
			appErr = &helper.AppError{
				Status:  fiber.StatusRequestEntityTooLarge,
				Code:    "PAYLOAD_TOO_LARGE",
				Message: "ukuran body melebihi batas yang diizinkan",
			}
		default:
			var fiberErr *fiber.Error
			if errors.As(err, &fiberErr) {
				appErr = &helper.AppError{
					Status:  fiberErr.Code,
					Code:    "HTTP_ERROR",
					Message: fiberErr.Message,
				}
			} else {
				appErr = helper.Internal(err)
			}
		}

		if appErr.Status >= fiber.StatusInternalServerError {
			logger.Error("request_failed",
				slog.String("request_id", requestID),
				slog.String("path", c.Path()),
				slog.String("code", appErr.Code),
				slog.Int("status", appErr.Status),
				slog.String("error", appErr.Error()),
			)
		} else {
			logger.Warn("request_rejected",
				slog.String("request_id", requestID),
				slog.String("path", c.Path()),
				slog.String("code", appErr.Code),
				slog.Int("status", appErr.Status),
			)
		}

		return c.Status(appErr.Status).JSON(model.ErrorResponse{
			Success:   false,
			Code:      appErr.Code,
			Message:   appErr.Message,
			Fields:    appErr.Fields,
			RequestID: requestID,
		})
	}
}
