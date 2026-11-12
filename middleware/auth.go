package middleware

import (
	"errors"
	"strings"
	"time"

	"findit-api/helper"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

// RequireAuth memvalidasi header Authorization: Bearer <token> dan menyimpan
// identitas pengguna ke dalam Locals.
func RequireAuth(jwtManager *helper.JWTManager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		token, err := bearerToken(c)
		if err != nil {
			c.Set("WWW-Authenticate", `Bearer realm="api"`)
			return helper.Unauthorized("header Authorization tidak ada atau salah bentuk")
		}
		authUser, err := jwtManager.Parse(token)
		if err != nil {
			c.Set("WWW-Authenticate", `Bearer realm="api"`)
			if errors.Is(err, helper.ErrExpiredToken) {
				return helper.Unauthorized("access token kedaluwarsa")
			}
			return helper.Unauthorized("access token tidak valid")
		}
		c.Locals(helper.LocalsAuthUser, authUser)
		return c.Next()
	}
}

// RequireRole membatasi akses endpoint hanya untuk role tertentu (mis. "admin").
func RequireRole(roles ...string) fiber.Handler {
	allowed := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		allowed[role] = struct{}{}
	}

	return func(c *fiber.Ctx) error {
		user, ok := helper.CurrentUser(c)
		if !ok {
			return helper.Unauthorized("belum terautentikasi")
		}
		if _, granted := allowed[user.Role]; !granted {
			return helper.Forbidden("role Anda tidak berhak mengakses endpoint ini")
		}
		return c.Next()
	}
}

// LoginRateLimiter membatasi percobaan login maksimal 5 kali per menit per IP.
func LoginRateLimiter() fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        5,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			c.Set("Retry-After", "60")
			return helper.TooManyRequests("terlalu banyak percobaan login, coba lagi dalam satu menit")
		},
	})
}

func bearerToken(c *fiber.Ctx) (string, error) {
	header := c.Get(fiber.HeaderAuthorization)
	if header == "" {
		return "", errors.New("header kosong")
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", errors.New("format bukan Bearer")
	}
	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", errors.New("token kosong")
	}
	return token, nil
}
