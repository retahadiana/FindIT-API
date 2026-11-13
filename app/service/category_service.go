// === FILE: app/service/category_service.go ===
package service

import (
	"errors"
	"strconv"
	"strings"

	"findit-api/app/model"
	"findit-api/app/repository"
	"findit-api/helper"

	"github.com/gofiber/fiber/v2"
)

type CategoryService struct{ repo repository.CategoryRepository }

func NewCategoryService(repo repository.CategoryRepository) *CategoryService {
	return &CategoryService{repo: repo}
}

func (s *CategoryService) List(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	list, err := s.repo.FindAll(ctx)
	if err != nil {
		return helper.Internal(err)
	}
	return helper.Success(c, fiber.StatusOK, "daftar kategori berhasil diambil", list)
}

func (s *CategoryService) Create(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	var req model.CreateCategoryRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	if errs := helper.ValidateStruct(req); len(errs) > 0 {
		return helper.Validation(errs)
	}
	created, err := s.repo.Create(ctx, req)
	if err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return helper.Conflict("nama kategori sudah ada")
		}
		return helper.Internal(err)
	}
	return helper.Created(c, "kategori berhasil dibuat", created,
		"/api/v1/categories/"+strconv.Itoa(created.ID))
}
