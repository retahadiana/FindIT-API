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

// List godoc
//
// @Summary Daftar seluruh kategori barang
// @Description Mengambil semua daftar kategori barang yang tersedia.
// @Tags categories
// @Produce json
// @Success 200 {object} model.WebResponse{data=[]model.Category}
// @Failure 500 {object} model.ErrorResponse "Terjadi kesalahan internal server"
// @Router /categories [get]
func (s *CategoryService) List(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	list, err := s.repo.FindAll(ctx)
	if err != nil {
		return helper.Internal(err)
	}
	return helper.Success(c, fiber.StatusOK, "daftar kategori berhasil diambil", list)
}

// Create godoc
//
// @Summary Buat kategori baru (Admin)
// @Description Membuat kategori barang baru di database. Memerlukan permission category:manage.
// @Tags categories
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body model.CreateCategoryRequest true "Data kategori baru"
// @Success 201 {object} model.WebResponse{data=model.Category}
// @Failure 400 {object} model.ErrorResponse "Body bukan JSON valid"
// @Failure 401 {object} model.ErrorResponse "Belum terautentikasi"
// @Failure 403 {object} model.ErrorResponse "Tidak memiliki izin category:manage"
// @Failure 409 {object} model.ErrorResponse "Nama kategori sudah ada"
// @Failure 422 {object} model.ErrorResponse "Validasi gagal"
// @Router /categories [post]
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
