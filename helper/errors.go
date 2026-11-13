package helper

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
)

// Kode error stabil sebagai bagian dari kontrak API.
const (
	CodeValidation         = "VALIDATION_ERROR"
	CodeBadRequest         = "BAD_REQUEST"
	CodeUnauthorized       = "UNAUTHORIZED"
	CodeForbidden          = "FORBIDDEN"
	CodeNotFound           = "NOT_FOUND"
	CodeConflict           = "CONFLICT"
	CodeNotAcceptable      = "NOT_ACCEPTABLE"
	CodeUnsupportedMedia   = "UNSUPPORTED_MEDIA_TYPE"
	CodeTooManyRequests    = "TOO_MANY_REQUESTS"
	CodeInternal           = "INTERNAL_ERROR"
	CodeServiceUnavailable = "SERVICE_UNAVAILABLE"
)

// AppError merepresentasikan error terstruktur aplikasi tanpa menyentuh *fiber.Ctx.
type AppError struct {
	Status  int
	Code    string
	Message string
	Fields  map[string]string
	cause   error
}

func (e *AppError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error { return e.cause }

func BadRequest(message string) *AppError {
	return &AppError{Status: fiber.StatusBadRequest, Code: CodeBadRequest, Message: message}
}

func Unauthorized(message string) *AppError {
	return &AppError{Status: fiber.StatusUnauthorized, Code: CodeUnauthorized, Message: message}
}

func Forbidden(message string) *AppError {
	return &AppError{Status: fiber.StatusForbidden, Code: CodeForbidden, Message: message}
}

func NotFound(message string) *AppError {
	return &AppError{Status: fiber.StatusNotFound, Code: CodeNotFound, Message: message}
}

func Conflict(message string) *AppError {
	return &AppError{Status: fiber.StatusConflict, Code: CodeConflict, Message: message}
}

func Validation(fields map[string]string) *AppError {
	return &AppError{
		Status:  fiber.StatusUnprocessableEntity,
		Code:    CodeValidation,
		Message: "validasi gagal",
		Fields:  fields,
	}
}

func UnsupportedMediaType(message string) *AppError {
	return &AppError{Status: fiber.StatusUnsupportedMediaType, Code: CodeUnsupportedMedia, Message: message}
}

func NotAcceptable(message string) *AppError {
	return &AppError{Status: fiber.StatusNotAcceptable, Code: CodeNotAcceptable, Message: message}
}

func TooManyRequests(message string) *AppError {
	return &AppError{Status: fiber.StatusTooManyRequests, Code: CodeTooManyRequests, Message: message}
}

func ServiceUnavailable(message string) *AppError {
	return &AppError{Status: fiber.StatusServiceUnavailable, Code: CodeServiceUnavailable, Message: message}
}

func Internal(cause error) *AppError {
	return &AppError{
		Status:  fiber.StatusInternalServerError,
		Code:    CodeInternal,
		Message: "terjadi kesalahan pada server",
		cause:   cause,
	}
}

func NewError(status int, code, message string) *AppError {
	return &AppError{Status: status, Code: code, Message: message}
}
