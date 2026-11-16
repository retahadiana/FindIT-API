package helper

import (
	"context"
	"strconv"
	"strings"
	"time"

	"findit-api/app/model"

	"github.com/gofiber/fiber/v2"
)

// RequestContext memberikan batas waktu 5 detik untuk setiap operasi database.
func RequestContext(c *fiber.Ctx) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.UserContext(), 5*time.Second)
}

// RequestID membaca Request ID dari context yang dipasang middleware requestid.
func RequestID(c *fiber.Ctx) string {
	id, _ := c.Locals("requestid").(string)
	return id
}

// ParamID membaca parameter :id dari URL dan memastikan bernilai positif.
func ParamID(c *fiber.Ctx) (int, bool) {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id < 1 {
		return 0, false
	}
	return id, true
}

var allowedSort = map[string]bool{
	"id": true, "title": true, "event_date": true, "created_at": true,
}

// ParseListQuery membaca query string paginasi dengan nilai default yang aman.
func ParseListQuery(c *fiber.Ctx) model.ListQuery {
	q := model.ListQuery{
		Page:   c.QueryInt("page", 1),
		Limit:  c.QueryInt("limit", 10),
		Search: strings.TrimSpace(c.Query("search")),
		Sort:   c.Query("sort", "created_at"),
		Order:  strings.ToLower(c.Query("order", "desc")),
	}
	if q.Page < 1 {
		q.Page = 1
	}
	if q.Limit < 1 {
		q.Limit = 10
	}
	if q.Limit > 100 {
		q.Limit = 100
	}
	if !allowedSort[q.Sort] {
		q.Sort = "created_at"
	}
	if q.Order != "asc" {
		q.Order = "desc"
	}
	return q
}

// ParseCursorQuery membaca parameter keyset pagination (?after=...&limit=...&search=... atau ?cursor=...).
func ParseCursorQuery(c *fiber.Ctx) (model.CursorQuery, error) {
	q := model.CursorQuery{
		Limit:  c.QueryInt("limit", 10),
		Search: strings.TrimSpace(c.Query("search")),
	}
	if q.Limit < 1 {
		q.Limit = 10
	}
	if q.Limit > 100 {
		q.Limit = 100
	}
	afterRaw := strings.TrimSpace(c.Query("after"))
	if afterRaw == "" {
		afterRaw = strings.TrimSpace(c.Query("cursor"))
	}
	if afterRaw != "" {
		cursor, err := DecodeCursor(afterRaw)
		if err != nil {
			return model.CursorQuery{}, BadRequest("parameter cursor tidak valid")
		}
		q.After = cursor
	}
	return q, nil
}
