package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"findit-api/app/model"
	"findit-api/app/repository"
	"findit-api/helper"

	"github.com/gofiber/fiber/v2"
)

const refreshTokenBytes = 32

// AuthService menangani registrasi, login, rotasi refresh token, logout, dan profil sesi aktif.
type AuthService struct {
	users      repository.UserRepository
	tokens     repository.TokenRepository
	jwt        *helper.JWTManager
	perms      *helper.PermissionSet
	refreshTTL time.Duration
}

// NewAuthService membuat instance AuthService.
func NewAuthService(
	users repository.UserRepository,
	tokens repository.TokenRepository,
	jwtManager *helper.JWTManager,
	perms *helper.PermissionSet,
	refreshTTL time.Duration,
) *AuthService {
	return &AuthService{
		users:      users,
		tokens:     tokens,
		jwt:        jwtManager,
		perms:      perms,
		refreshTTL: refreshTTL,
	}
}

// Register godoc
//
// @Summary Mendaftarkan akun baru
// @Description Mendaftarkan pengguna mahasiswa baru dengan role default user dan trust score 100.
// @Tags auth
// @Accept json
// @Produce json
// @Param body body model.RegisterRequest true "Data pendaftaran pengguna"
// @Success 201 {object} model.WebResponse{data=model.User}
// @Failure 400 {object} model.ErrorResponse "Body bukan JSON valid"
// @Failure 409 {object} model.ErrorResponse "Username atau email sudah digunakan"
// @Failure 422 {object} model.ErrorResponse "Validasi gagal"
// @Router /auth/register [post]
func (s *AuthService) Register(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	var req model.RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)
	req.FullName = strings.TrimSpace(req.FullName)
	req.Phone = strings.TrimSpace(req.Phone)

	if errs := helper.ValidateStruct(req); len(errs) > 0 {
		return helper.Validation(errs)
	}

	hashed, err := helper.HashPassword(req.Password)
	if err != nil {
		return helper.Internal(err)
	}

	// Role dan trust_score awal selalu ditentukan oleh server.
	created, err := s.users.Create(ctx, model.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: hashed,
		FullName:     req.FullName,
		Phone:        req.Phone,
		Role:         "user",
		TrustScore:   100,
	})
	if err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return helper.Conflict("username atau email sudah dipakai")
		}
		return helper.Internal(err)
	}

	return helper.Created(c, "pendaftaran berhasil", created, "/api/v1/users/"+strconv.Itoa(created.ID))
}

// Login godoc
//
// @Summary Login dan mendapatkan token
// @Description Masuk dengan username dan password untuk memperoleh Access Token dan Refresh Token.
// @Tags auth
// @Accept json
// @Produce json
// @Param body body model.LoginRequest true "Kredensial login"
// @Success 200 {object} model.WebResponse{data=model.TokenPair}
// @Failure 400 {object} model.ErrorResponse "Body bukan JSON valid"
// @Failure 401 {object} model.ErrorResponse "Username atau password salah"
// @Failure 422 {object} model.ErrorResponse "Validasi gagal"
// @Router /auth/login [post]
func (s *AuthService) Login(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	var req model.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}
	req.Username = strings.TrimSpace(req.Username)
	if errs := helper.ValidateStruct(req); len(errs) > 0 {
		return helper.Validation(errs)
	}

	user, err := s.users.FindByUsername(ctx, req.Username)
	if err != nil {
		// Tetap jalankan perbandingan hash palsu untuk mencegah timing attack.
		helper.VerifyDummyPassword(req.Password)
		return helper.Unauthorized("username atau password salah")
	}
	if !helper.VerifyPassword(user.PasswordHash, req.Password) {
		return helper.Unauthorized("username atau password salah")
	}

	pair, err := s.issueTokenPair(ctx, user)
	if err != nil {
		return helper.Internal(err)
	}
	return helper.Success(c, fiber.StatusOK, "login berhasil", pair)
}

// Refresh godoc
//
// @Summary Memperbarui access token
// @Description Menukar refresh token valid dengan pasangan access token dan refresh token baru (token rotation).
// @Tags auth
// @Accept json
// @Produce json
// @Param body body model.RefreshRequest true "Refresh token saat ini"
// @Success 200 {object} model.WebResponse{data=model.TokenPair}
// @Failure 400 {object} model.ErrorResponse "Body bukan JSON valid"
// @Failure 401 {object} model.ErrorResponse "Refresh token tidak valid atau kedaluwarsa"
// @Failure 422 {object} model.ErrorResponse "Validasi gagal"
// @Router /auth/refresh [post]
func (s *AuthService) Refresh(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	var req model.RefreshRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}
	req.RefreshToken = strings.TrimSpace(req.RefreshToken)
	if errs := helper.ValidateStruct(req); len(errs) > 0 {
		return helper.Validation(errs)
	}

	hash := helper.SHA256Hex(req.RefreshToken)
	stored, err := s.tokens.FindByHash(ctx, hash)
	if err != nil {
		return helper.Unauthorized("refresh token tidak valid atau sudah kedaluwarsa")
	}

	user, err := s.users.FindByID(ctx, stored.UserID)
	if err != nil {
		return helper.Unauthorized("akun tidak dapat dipakai")
	}

	// Rotasi token: refresh token lama dicabut sebelum menerbitkan pasangan baru.
	if err := s.tokens.Revoke(ctx, hash); err != nil {
		return helper.Internal(err)
	}

	pair, err := s.issueTokenPair(ctx, user)
	if err != nil {
		return helper.Internal(err)
	}
	return helper.Success(c, fiber.StatusOK, "token berhasil diperbarui", pair)
}

// Logout godoc
//
// @Summary Logout sesi pengguna
// @Description Mencabut refresh token aktif dari database.
// @Tags auth
// @Accept json
// @Produce json
// @Param body body model.RefreshRequest true "Refresh token yang akan dicabut"
// @Success 200 {object} model.WebResponse
// @Failure 400 {object} model.ErrorResponse "Body bukan JSON valid"
// @Router /auth/logout [post]
func (s *AuthService) Logout(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	var req model.RefreshRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}

	if token := strings.TrimSpace(req.RefreshToken); token != "" {
		_ = s.tokens.Revoke(ctx, helper.SHA256Hex(token))
	}
	return helper.Success(c, fiber.StatusOK, "logout berhasil", nil)
}

// Me godoc
//
// @Summary Profil pengguna aktif
// @Description Mengambil detail profil dan daftar permission pengguna yang sedang login.
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} model.WebResponse{data=model.UserProfile}
// @Failure 401 {object} model.ErrorResponse "Belum terautentikasi"
// @Router /auth/me [get]
func (s *AuthService) Me(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	authUser, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	user, err := s.users.FindByID(ctx, authUser.UserID)
	if err != nil {
		return helper.Unauthorized("user tidak ditemukan")
	}
	return helper.Success(c, fiber.StatusOK, "profil berhasil diambil", model.UserProfile{
		User:        user,
		Permissions: s.perms.ListForRole(user.Role),
	})
}

func (s *AuthService) issueTokenPair(ctx context.Context, user model.User) (model.TokenPair, error) {
	accessToken, err := s.jwt.GenerateAccess(user.ID, user.Username, user.Role)
	if err != nil {
		return model.TokenPair{}, err
	}
	refreshToken, err := helper.RandomToken(refreshTokenBytes)
	if err != nil {
		return model.TokenPair{}, err
	}

	err = s.tokens.Save(ctx, model.RefreshToken{
		UserID:    user.ID,
		TokenHash: helper.SHA256Hex(refreshToken),
		ExpiresAt: time.Now().Add(s.refreshTTL),
	})
	if err != nil {
		return model.TokenPair{}, err
	}

	return model.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(s.jwt.AccessTTL().Seconds()),
	}, nil
}
