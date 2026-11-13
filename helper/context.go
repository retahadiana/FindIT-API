package helper

import (
	"findit-api/app/model"

	"github.com/gofiber/fiber/v2"
)

// LocalsAuthUser adalah kunci penyimpanan identitas pengguna di Fiber Locals.
const LocalsAuthUser = "authUser"

func CurrentUser(c *fiber.Ctx) (model.AuthUser, bool) {
	user, ok := c.Locals(LocalsAuthUser).(model.AuthUser)
	return user, ok
}
