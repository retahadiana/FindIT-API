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
