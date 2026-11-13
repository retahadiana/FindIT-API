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

type UserService struct {
	users repository.UserRepository
	perms *helper.PermissionSet
}

func NewUserService(users repository.UserRepository, perms *helper.PermissionSet) *UserService {
	return &UserService{users: users, perms: perms}
}

// List godoc
//
// @Summary Daftar pengguna terpaginasi atau CSV
// @Description Menampilkan daftar pengguna dengan cursor pagination atau ekspor CSV melalui header Accept.
// @Tags users
// @Produce json
// @Produce text/csv
// @Security BearerAuth
// @Param cursor query string false "Cursor untuk navigasi halaman berikutnya"
// @Param limit query int false "Batas jumlah data per halaman (default 20, maks 100)"
// @Param search query string false "Pencarian nama atau username"
// @Success 200 {object} model.WebResponse{data=[]model.User}
// @Failure 401 {object} model.ErrorResponse "Belum terautentikasi"
// @Failure 403 {object} model.ErrorResponse "Tidak memiliki izin user:list"
// @Router /users [get]
func (s *UserService) List(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	format, err := helper.NegotiateFormat(c)
	if err != nil {
		return err
	}
	cq, err := helper.ParseCursorQuery(c)
	if err != nil {
		return err
	}
	users, err := s.users.FindAfterCursor(ctx, cq)
	if err != nil {
		return helper.Internal(err)
	}
	hasMore := len(users) > cq.Limit
	if hasMore {
		users = users[:cq.Limit]
	}
	if format == helper.FormatCSV {
		rows := make([][]string, 0, len(users))
		for _, u := range users {
			rows = append(rows, []string{strconv.Itoa(u.ID), u.Username, u.Email, u.FullName, u.Role, strconv.Itoa(u.TrustScore)})
		}
		return helper.SendCSV(c, "users.csv", []string{"id", "username", "email", "full_name", "role", "trust_score"}, rows)
	}
	var nextCursor string
	if hasMore && len(users) > 0 {
		last := users[len(users)-1]
		nextCursor = helper.EncodeCursor(last.CreatedAt, last.ID)
	}
	return c.Status(fiber.StatusOK).JSON(model.WebResponse{
		Success: true,
		Message: "daftar user berhasil diambil",
		Data:    users,
		Meta:    model.CursorMeta{Limit: cq.Limit, NextCursor: nextCursor, HasMore: hasMore},
	})
}

// GetByID godoc
//
// @Summary Ambil data pengguna berdasarkan ID
// @Description Mengambil informasi profil pengguna tertentu berdasarkan ID.
// @Tags users
// @Produce json
// @Security BearerAuth
// @Param id path int true "ID Pengguna"
// @Success 200 {object} model.WebResponse{data=model.User}
// @Failure 400 {object} model.ErrorResponse "ID user tidak valid"
// @Failure 401 {object} model.ErrorResponse "Belum terautentikasi"
// @Failure 403 {object} model.ErrorResponse "Hanya boleh melihat profil sendiri"
// @Failure 404 {object} model.ErrorResponse "User tidak ditemukan"
// @Router /users/{id} [get]
func (s *UserService) GetByID(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	actor, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}
	id, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id user harus berupa angka positif")
	}
	if !CanAccessUser(actor, id, s.perms, "user:read:any") {
		return helper.Forbidden("hanya boleh melihat profil sendiri")
	}
	user, err := s.users.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return helper.NotFound("user tidak ditemukan")
		}
		return helper.Internal(err)
	}
	return helper.Success(c, fiber.StatusOK, "user ditemukan", user)
}

// Create godoc
//
// @Summary Buat pengguna baru oleh admin/staff
// @Description Menambahkan pengguna baru secara langsung ke database.
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body model.CreateUserRequest true "Data pengguna baru"
// @Success 201 {object} model.WebResponse{data=model.User}
// @Failure 400 {object} model.ErrorResponse "Body harus JSON valid"
// @Failure 401 {object} model.ErrorResponse "Belum terautentikasi"
// @Failure 403 {object} model.ErrorResponse "Tidak memiliki izin"
// @Failure 409 {object} model.ErrorResponse "Username atau email sudah dipakai"
// @Failure 422 {object} model.ErrorResponse "Validasi gagal"
// @Router /users [post]
func (s *UserService) Create(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	var req model.CreateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus JSON valid")
	}
	if errs := helper.ValidateStruct(req); len(errs) > 0 {
		return helper.Validation(errs)
	}
	hashed, err := helper.HashPassword(req.Password)
	if err != nil {
		return helper.Internal(err)
	}
	role := req.Role
	if role == "" {
		role = "user"
	}
	created, err := s.users.Create(ctx, model.User{
		Username: strings.TrimSpace(req.Username), Email: strings.TrimSpace(req.Email),
		PasswordHash: hashed, FullName: strings.TrimSpace(req.FullName), Phone: strings.TrimSpace(req.Phone), Role: role,
	})
	if err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return helper.Conflict("username atau email sudah dipakai")
		}
		return helper.Internal(err)
	}
	return helper.Created(c, "user berhasil dibuat", created, "/api/v1/users/"+strconv.Itoa(created.ID))
}

// UpdateProfile godoc
//
// @Summary Perbarui profil diri sendiri
// @Description Mengubah nama lengkap dan nomor telepon akun yang sedang login.
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body model.UpdateUserRequest true "Data pembaruan profil"
// @Success 200 {object} model.WebResponse{data=model.User}
// @Failure 400 {object} model.ErrorResponse "Body bukan JSON valid"
// @Failure 401 {object} model.ErrorResponse "Belum terautentikasi"
// @Failure 422 {object} model.ErrorResponse "Validasi gagal"
// @Router /users/profile [put]
func (s *UserService) UpdateProfile(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}
	var req model.UpdateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus JSON valid")
	}
	if errs := helper.ValidateStruct(req); len(errs) > 0 {
		return helper.Validation(errs)
	}
	updated, err := s.users.UpdateProfile(ctx, current.UserID, strings.TrimSpace(req.FullName), strings.TrimSpace(req.Phone))
	if err != nil {
		return helper.Internal(err)
	}
	return helper.Success(c, fiber.StatusOK, "profil berhasil diperbarui", updated)
}

// Replace godoc
//
// @Summary Ganti seluruh atribut data pengguna (PUT)
// @Description Mengganti atribut profil pengguna secara utuh berdasarkan ID.
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "ID Pengguna"
// @Param body body model.ReplaceUserRequest true "Data lengkap pengguna"
// @Success 200 {object} model.WebResponse{data=model.User}
// @Failure 400 {object} model.ErrorResponse "ID atau body tidak valid"
// @Failure 401 {object} model.ErrorResponse "Belum terautentikasi"
// @Failure 403 {object} model.ErrorResponse "Tidak memiliki izin"
// @Failure 404 {object} model.ErrorResponse "User tidak ditemukan"
// @Failure 409 {object} model.ErrorResponse "Username atau email sudah dipakai"
// @Router /users/{id} [put]
func (s *UserService) Replace(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	id, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id user tidak valid")
	}
	var req model.ReplaceUserRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus JSON valid")
	}
	if errs := helper.ValidateStruct(req); len(errs) > 0 {
		return helper.Validation(errs)
	}
	updated, err := s.users.Update(ctx, id, model.User{
		Username: req.Username, Email: req.Email, FullName: req.FullName, Phone: req.Phone,
	})
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return helper.NotFound("user tidak ditemukan")
		}
		if errors.Is(err, repository.ErrDuplicate) {
			return helper.Conflict("username atau email sudah dipakai")
		}
		return helper.Internal(err)
	}
	return helper.Success(c, fiber.StatusOK, "user berhasil diperbarui", updated)
}

// Patch godoc
//
// @Summary Perbarui sebagian data pengguna (PATCH)
// @Description Memperbarui sebagian field profil pengguna (partial update).
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "ID Pengguna"
// @Param body body model.PatchUserRequest true "Field yang akan diperbarui"
// @Success 200 {object} model.WebResponse{data=model.User}
// @Failure 400 {object} model.ErrorResponse "Minimal satu field harus diisi atau ID tidak valid"
// @Failure 401 {object} model.ErrorResponse "Belum terautentikasi"
// @Failure 403 {object} model.ErrorResponse "Tidak memiliki izin"
// @Failure 404 {object} model.ErrorResponse "User tidak ditemukan"
// @Router /users/{id} [patch]
func (s *UserService) Patch(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	id, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id user tidak valid")
	}
	var req model.PatchUserRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus JSON valid")
	}
	if IsEmptyPatch(req) {
		return helper.BadRequest("minimal satu field harus diisi")
	}
	if errs := helper.ValidateStruct(req); len(errs) > 0 {
		return helper.Validation(errs)
	}
	existing, err := s.users.FindByID(ctx, id)
	if err != nil {
		return helper.NotFound("user tidak ditemukan")
	}
	patched := ApplyPatch(existing, req)
	updated, err := s.users.UpdateProfile(ctx, id, patched.FullName, patched.Phone)
	if err != nil {
		return helper.Internal(err)
	}
	return helper.Success(c, fiber.StatusOK, "user berhasil di-patch", updated)
}

// AssignRole godoc
//
// @Summary Ubah peran pengguna (Assign Role)
// @Description Mengubah level kewenangan role pengguna (admin/staff/user).
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "ID Pengguna"
// @Param body body model.AssignRoleRequest true "Peran baru"
// @Success 200 {object} model.WebResponse{data=model.User}
// @Failure 400 {object} model.ErrorResponse "Body bukan JSON valid atau role tidak dikenal"
// @Failure 401 {object} model.ErrorResponse "Belum terautentikasi"
// @Failure 403 {object} model.ErrorResponse "Tidak memiliki izin role:assign atau tidak boleh mengubah role diri sendiri"
// @Failure 404 {object} model.ErrorResponse "User tidak ditemukan"
// @Router /users/{id}/role [patch]
func (s *UserService) AssignRole(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	actor, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}
	id, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id user tidak valid")
	}
	var req model.AssignRoleRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus JSON valid")
	}
	if err := ValidateAssignRole(actor, id, req.Role); err != nil {
		return err
	}
	updated, err := s.users.UpdateRole(ctx, id, req.Role)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return helper.NotFound("user tidak ditemukan")
		}
		return helper.Internal(err)
	}
	return helper.Success(c, fiber.StatusOK, "role user berhasil diubah", updated)
}

// Delete godoc
//
// @Summary Hapus pengguna
// @Description Menghapus data akun pengguna dari sistem secara permanen.
// @Tags users
// @Security BearerAuth
// @Param id path int true "ID Pengguna"
// @Success 204 "Berhasil dihapus tanpa body"
// @Failure 400 {object} model.ErrorResponse "ID user tidak valid"
// @Failure 401 {object} model.ErrorResponse "Belum terautentikasi"
// @Failure 403 {object} model.ErrorResponse "Tidak memiliki izin user:delete"
// @Failure 404 {object} model.ErrorResponse "User tidak ditemukan"
// @Router /users/{id} [delete]
func (s *UserService) Delete(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	id, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id user tidak valid")
	}
	if err := s.users.Delete(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return helper.NotFound("user tidak ditemukan")
		}
		return helper.Internal(err)
	}
	return helper.NoContent(c)
}
