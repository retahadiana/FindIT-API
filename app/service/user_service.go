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
