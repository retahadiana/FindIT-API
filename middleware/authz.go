package middleware

import (
	"findit-api/helper"
	"github.com/gofiber/fiber/v2"
)

func RequirePermission(perms *helper.PermissionSet, permission string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		user, ok := helper.CurrentUser(c)
		if !ok {
			return helper.Unauthorized("belum terautentikasi")
		}
		if !perms.Has(user.Role, permission) {
			return helper.Forbidden("akses ditolak: permission " + permission + " diperlukan")
		}
		return c.Next()
	}
}
