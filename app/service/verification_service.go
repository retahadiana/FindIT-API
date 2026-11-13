package service

import (
	"errors"
	"findit-api/app/model"
	"findit-api/app/repository"
	"findit-api/helper"
	"github.com/gofiber/fiber/v2"
)

type VerificationService struct {
	verif  repository.VerificationRepository
	claims repository.ClaimRepository
	items  repository.ItemRepository
}

func NewVerificationService(verif repository.VerificationRepository, claims repository.ClaimRepository, items repository.ItemRepository) *VerificationService {
	return &VerificationService{verif: verif, claims: claims, items: items}
}

// CreateSession godoc
//
// @Summary Buat sesi pertanyaan verifikasi rahasia (Penemu)
// @Description Penemu barang membuat daftar pertanyaan ciri khusus rahasia (maksimal 10 pertanyaan disimpan di MongoDB) untuk dijawab oleh pengklaim.
// @Tags verifications
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "ID Klaim"
// @Param body body model.SubmitQARequest true "Daftar pertanyaan verifikasi"
// @Success 201 {object} model.WebResponse{data=model.VerificationSession}
// @Failure 400 {object} model.ErrorResponse "ID klaim atau payload tidak valid"
// @Failure 401 {object} model.ErrorResponse "Belum terautentikasi"
// @Failure 403 {object} model.ErrorResponse "Hanya penemu barang yang berhak membuat sesi"
// @Failure 404 {object} model.ErrorResponse "Klaim tidak ditemukan"
// @Failure 409 {object} model.ErrorResponse "Sesi verifikasi untuk klaim ini sudah ada atau klaim bukan pending"
// @Router /claims/{id}/verification [post]
func (s *VerificationService) CreateSession(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	actor, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}
	claimID, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id klaim tidak valid")
	}
	claim, err := s.claims.FindByID(ctx, claimID)
	if err != nil {
		return helper.NotFound("klaim tidak ditemukan")
	}
	if claim.Status != "pending" {
		return helper.Conflict("klaim sudah diproses sebelumnya")
	}
	item, err := s.items.FindByID(ctx, claim.ItemID)
	if err != nil || item.UserID != actor.UserID {
		return helper.Forbidden("hanya penemu barang yang dapat membuat sesi verifikasi")
	}
	var req model.SubmitQARequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus JSON valid")
	}
	if errs := helper.ValidateStruct(req); len(errs) > 0 {
		return helper.Validation(errs)
	}
	created, err := s.verif.CreateSession(ctx, model.VerificationSession{
		ClaimID: claim.ID, ItemID: item.ID, FinderID: item.UserID,
		ClaimantID: claim.ClaimantID, Questions: req.Questions,
	})
	if err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return helper.Conflict("sesi verifikasi untuk klaim ini sudah ada")
		}
		return helper.Internal(err)
	}
	return helper.Created(c, "sesi verifikasi dibuat", created, "")
}

// GetSession godoc
//
// @Summary Ambil sesi verifikasi tanya-jawab ciri khusus
// @Description Mengambil data sesi tanya-jawab ciri khusus barang dari MongoDB berdasarkan ID klaim.
// @Tags verifications
// @Produce json
// @Security BearerAuth
// @Param id path int true "ID Klaim"
// @Success 200 {object} model.WebResponse{data=model.VerificationSession}
// @Failure 400 {object} model.ErrorResponse "ID klaim tidak valid"
// @Failure 404 {object} model.ErrorResponse "Sesi verifikasi tidak ditemukan"
// @Router /claims/{id}/verification [get]
func (s *VerificationService) GetSession(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	claimID, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id klaim tidak valid")
	}
	session, err := s.verif.FindByClaimID(ctx, claimID)
	if err != nil {
		return helper.NotFound("sesi verifikasi tidak ditemukan")
	}
	return helper.Success(c, fiber.StatusOK, "sesi verifikasi ditemukan", session)
}

// AnswerQuestions godoc
//
// @Summary Jawab pertanyaan verifikasi (Pengklaim)
// @Description Pengklaim menjawab pertanyaan-pertanyaan ciri khusus yang diajukan oleh penemu.
// @Tags verifications
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "ID Klaim"
// @Param body body model.SubmitQARequest true "Jawaban atas pertanyaan verifikasi"
// @Success 200 {object} model.WebResponse{data=model.VerificationSession}
// @Failure 400 {object} model.ErrorResponse "ID klaim atau payload tidak valid"
// @Failure 401 {object} model.ErrorResponse "Belum terautentikasi"
// @Failure 403 {object} model.ErrorResponse "Hanya pengklaim yang berhak menjawab pertanyaan"
// @Failure 404 {object} model.ErrorResponse "Sesi verifikasi tidak ditemukan"
// @Router /claims/{id}/verification/answers [put]
func (s *VerificationService) AnswerQuestions(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	actor, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}
	claimID, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id klaim tidak valid")
	}
	session, err := s.verif.FindByClaimID(ctx, claimID)
	if err != nil {
		return helper.NotFound("sesi verifikasi tidak ditemukan")
	}
	if session.ClaimantID != actor.UserID {
		return helper.Forbidden("hanya pengklaim yang boleh mengisi jawaban")
	}
	var req model.SubmitQARequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus JSON valid")
	}
	if errs := helper.ValidateStruct(req); len(errs) > 0 {
		return helper.Validation(errs)
	}
	updated, err := s.verif.UpdateAnswers(ctx, claimID, req.Questions)
	if err != nil {
		return helper.Internal(err)
	}
	return helper.Success(c, fiber.StatusOK, "jawaban verifikasi disimpan", updated)
}

// ReviewVerifications godoc
//
// @Summary Tinjau dan nilai hasil jawaban verifikasi (Penemu / Admin)
// @Description Penemu atau admin menilai kebenaran jawaban pengklaim (is_verified) dan memberikan catatan rahasia.
// @Tags verifications
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "ID Klaim"
// @Param body body model.ReviewVerificationRequest true "Hasil penilaian verifikasi"
// @Success 200 {object} model.WebResponse{data=model.VerificationSession}
// @Failure 400 {object} model.ErrorResponse "ID klaim atau payload tidak valid"
// @Failure 401 {object} model.ErrorResponse "Belum terautentikasi"
// @Failure 403 {object} model.ErrorResponse "Hanya penemu atau admin yang berhak meninjau"
// @Failure 404 {object} model.ErrorResponse "Sesi verifikasi tidak ditemukan"
// @Router /claims/{id}/verification/review [post]
func (s *VerificationService) ReviewVerifications(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	actor, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}
	claimID, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id klaim tidak valid")
	}
	session, err := s.verif.FindByClaimID(ctx, claimID)
	if err != nil {
		return helper.NotFound("sesi verifikasi tidak ditemukan")
	}
	if session.FinderID != actor.UserID && actor.Role != "admin" {
		return helper.Forbidden("hanya penemu atau admin yang berhak meninjau verifikasi")
	}
	var req model.ReviewVerificationRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus JSON valid")
	}
	updated, err := s.verif.Review(ctx, claimID, req.IsVerified, req.ConfidentialNotes)
	if err != nil {
		return helper.Internal(err)
	}
	return helper.Success(c, fiber.StatusOK, "hasil verifikasi disimpan", updated)
}
