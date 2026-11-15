// === FILE: app/service/item_service.go ===
package service

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"findit-api/app/model"
	"findit-api/app/repository"
	"findit-api/helper"

	"github.com/gofiber/fiber/v2"
)

type ItemService struct {
	items      repository.ItemRepository
	categories repository.CategoryRepository
}

func NewItemService(items repository.ItemRepository, categories repository.CategoryRepository) *ItemService {
	return &ItemService{items: items, categories: categories}
}

// List godoc
//
// @Summary Daftar laporan barang (Lost / Found)
// @Description Menampilkan daftar laporan barang dengan filter tipe (lost/found), status (open/resolved/returned), kategori, serta mendukung pagination berbasis halaman, cursor, maupun ekspor CSV.
// @Tags items
// @Produce json
// @Produce text/csv
// @Param type query string false "Filter tipe: lost atau found"
// @Param status query string false "Filter status: open, claimed, returned, resolved"
// @Param category_id query int false "Filter berdasarkan ID kategori"
// @Param page query int false "Nomor halaman (default 1)"
// @Param limit query int false "Batas item per halaman (default 20, maks 100)"
// @Param search query string false "Kata kunci judul atau deskripsi"
// @Param after query string false "Cursor untuk cursor-based pagination"
// @Success 200 {object} model.WebResponse{data=[]model.Item}
// @Router /items [get]
func (s *ItemService) List(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	format, err := helper.NegotiateFormat(c)
	if err != nil {
		return err
	}

	if c.Query("after") != "" || format == helper.FormatCSV {
		cq, err := helper.ParseCursorQuery(c)
		if err != nil {
			return err
		}
		itemCQ := model.ItemCursorQuery{
			CursorQuery: cq,
			ItemType:    strings.TrimSpace(c.Query("type")),
			Status:      strings.TrimSpace(c.Query("status")),
			CategoryID:  c.QueryInt("category_id", 0),
		}
		items, err := s.items.FindAfterCursor(ctx, itemCQ)
		if err != nil {
			return helper.Internal(err)
		}
		hasMore := len(items) > cq.Limit
		if hasMore {
			items = items[:cq.Limit]
		}
		if format == helper.FormatCSV {
			rows := make([][]string, 0, len(items))
			for _, it := range items {
				rows = append(rows, []string{
					strconv.Itoa(it.ID), it.ItemType, it.Status, it.Category, it.Title, it.Location,
				})
			}
			return helper.SendCSV(c, "items.csv", []string{"id", "type", "status", "category", "title", "location"}, rows)
		}
		var nextCursor string
		if hasMore && len(items) > 0 {
			last := items[len(items)-1]
			nextCursor = helper.EncodeCursor(last.CreatedAt, last.ID)
		}
		return c.Status(fiber.StatusOK).JSON(model.WebResponse{
			Success: true,
			Message: "daftar barang berhasil diambil",
			Data:    items,
			Meta:    model.CursorMeta{Limit: cq.Limit, NextCursor: nextCursor, HasMore: hasMore},
		})
	}

	q := model.ItemQuery{
		ListQuery:  helper.ParseListQuery(c),
		ItemType:   strings.TrimSpace(c.Query("type")),
		Status:     strings.TrimSpace(c.Query("status")),
		CategoryID: c.QueryInt("category_id", 0),
	}
	items, total, err := s.items.FindAll(ctx, q)
	if err != nil {
		return helper.Internal(err)
	}
	totalPages := (total + q.Limit - 1) / q.Limit
	return helper.SuccessList(c, "daftar barang berhasil diambil", items, &model.Meta{
		Page: q.Page, Limit: q.Limit, Total: total, TotalPages: totalPages,
	})
}

// Get godoc
//
// @Summary Detail laporan barang
// @Description Mengambil rincian informasi satu barang berdasarkan ID.
// @Tags items
// @Produce json
// @Param id path int true "ID Barang"
// @Success 200 {object} model.WebResponse{data=model.Item}
// @Failure 400 {object} model.ErrorResponse "ID barang tidak valid"
// @Failure 404 {object} model.ErrorResponse "Barang tidak ditemukan"
// @Router /items/{id} [get]
func (s *ItemService) Get(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	id, ok := helper.ParamID(c)
	if !ok {
		return helper.BadRequest("id barang tidak valid")
	}
	item, err := s.items.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return helper.NotFound("barang tidak ditemukan")
		}
		return helper.Internal(err)
	}
	return helper.Success(c, fiber.StatusOK, "detail barang ditemukan", item)
}

// Create godoc
//
// @Summary Buat laporan barang hilang atau temuan
// @Description Mendaftarkan laporan kehilangan (lost) atau penemuan barang (found).
// @Tags items
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body model.CreateItemRequest true "Data laporan barang"
// @Success 201 {object} model.WebResponse{data=model.Item}
// @Failure 400 {object} model.ErrorResponse "Body bukan JSON valid atau kategori tidak ditemukan"
// @Failure 401 {object} model.ErrorResponse "Belum terautentikasi"
// @Failure 422 {object} model.ErrorResponse "Validasi gagal"
// @Router /items [post]
func (s *ItemService) Create(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	user, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	var req model.CreateItemRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Description = strings.TrimSpace(req.Description)
	req.Location = strings.TrimSpace(req.Location)
	if errs := helper.ValidateStruct(req); len(errs) > 0 {
		return helper.Validation(errs)
	}
	eventDate := req.EventDate.Time()
	if eventDate.After(time.Now().Add(24 * time.Hour)) {
		return helper.BadRequest("tanggal kejadian tidak boleh di masa depan")
	}
	if _, err := s.categories.FindByID(ctx, req.CategoryID); err != nil {
		return helper.BadRequest("kategori tidak ditemukan")
	}
	created, err := s.items.Create(ctx, model.Item{
		UserID:      user.UserID,
		CategoryID:  req.CategoryID,
		Title:       req.Title,
		Description: req.Description,
		ItemType:    req.ItemType,
		Location:    req.Location,
		EventDate:   eventDate,
	})
	if err != nil {
		return helper.Internal(err)
	}
	return helper.Created(c, "laporan barang berhasil dibuat", created,
		"/api/v1/items/"+strconv.Itoa(created.ID))
}

// Update godoc
//
// @Summary Perbarui laporan barang
// @Description Memperbarui informasi laporan barang milik sendiri atau oleh admin.
// @Tags items
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "ID Barang"
// @Param body body model.UpdateItemRequest true "Data pembaruan laporan"
// @Success 200 {object} model.WebResponse{data=model.Item}
// @Failure 400 {object} model.ErrorResponse "Data atau ID tidak valid"
// @Failure 401 {object} model.ErrorResponse "Belum terautentikasi"
// @Failure 403 {object} model.ErrorResponse "Bukan pemilik atau admin"
// @Failure 404 {object} model.ErrorResponse "Barang tidak ditemukan"
// @Router /items/{id} [put]
func (s *ItemService) Update(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	user, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}
	id, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id barang tidak valid")
	}
	existing, err := s.items.FindByID(ctx, id)
	if err != nil {
		return helper.NotFound("barang tidak ditemukan")
	}
	if existing.UserID != user.UserID && user.Role != "admin" {
		return helper.Forbidden("hanya pemilik postingan atau admin yang boleh mengubah")
	}
	var req model.UpdateItemRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}
	if errs := helper.ValidateStruct(req); len(errs) > 0 {
		return helper.Validation(errs)
	}
	if req.Status == "" {
		req.Status = existing.Status
	}
	if _, err := s.categories.FindByID(ctx, req.CategoryID); err != nil {
		return helper.BadRequest("kategori tidak ditemukan")
	}
	if existing.Status == "returned" {
		return helper.Conflict("barang yang sudah berstatus returned tidak dapat diubah")
	}
	if req.Status == "claimed" && existing.Status != "claimed" {
		return helper.Conflict("status claimed hanya dapat diubah oleh sistem melalui proses klaim")
	}
	if existing.Status != req.Status {
		if existing.Status == "claimed" && req.Status != "closed" {
			return helper.Conflict("barang yang sedang dalam proses klaim tidak dapat diubah statusnya secara langsung")
		}
		if req.Status == "returned" {
			return helper.Conflict("status returned hanya dapat diberikan melalui proses persetujuan klaim resmi")
		}
	}
	updated, err := s.items.Update(ctx, id, req)
	if err != nil {
		return helper.Internal(err)
	}
	return helper.Success(c, fiber.StatusOK, "laporan barang berhasil diperbarui", updated)
}

// Delete godoc
//
// @Summary Hapus laporan barang (Soft Delete)
// @Description Menghapus postingan barang secara soft delete oleh pemilik atau admin.
// @Tags items
// @Security BearerAuth
// @Param id path int true "ID Barang"
// @Success 204 "Berhasil dihapus tanpa body"
// @Failure 400 {object} model.ErrorResponse "ID barang tidak valid"
// @Failure 401 {object} model.ErrorResponse "Belum terautentikasi"
// @Failure 403 {object} model.ErrorResponse "Tidak berhak menghapus"
// @Failure 404 {object} model.ErrorResponse "Barang tidak ditemukan"
// @Failure 409 {object} model.ErrorResponse "Barang berstatus claimed tidak dapat dihapus"
// @Router /items/{id} [delete]
func (s *ItemService) Delete(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	user, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}
	id, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id barang tidak valid")
	}
	existing, err := s.items.FindByID(ctx, id)
	if err != nil {
		return helper.NotFound("barang tidak ditemukan")
	}
	if existing.UserID != user.UserID && user.Role != "admin" {
		return helper.Forbidden("tidak berhak menghapus laporan barang ini")
	}
	if existing.Status != "open" {
		return helper.Conflict("hanya barang berstatus open yang dapat dihapus")
	}
	hasPending, err := s.items.HasPendingClaims(ctx, id)
	if err != nil {
		return helper.Internal(err)
	}
	if hasPending {
		return helper.Conflict("barang tidak dapat dihapus karena masih memiliki klaim aktif yang sedang diproses")
	}
	if err := s.items.Delete(ctx, id); err != nil {
		return helper.Internal(err)
	}
	return helper.NoContent(c)
}
