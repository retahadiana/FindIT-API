package service

import (
	"errors"
	"strings"

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
// @Param body body model.CreateVerificationSessionRequest true "Daftar pertanyaan verifikasi"
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
	if err != nil {
		return helper.NotFound("barang tidak ditemukan")
	}
	if item.UserID != actor.UserID && actor.Role != "admin" {
		return helper.Forbidden("hanya penemu barang atau admin yang dapat membuat sesi verifikasi")
	}
	var req model.CreateVerificationSessionRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus JSON valid")
	}
	if errs := helper.ValidateStruct(req); len(errs) > 0 {
		return helper.Validation(errs)
	}
	var questions []model.QuestionAnswer
	for _, q := range req.Questions {
		text := strings.TrimSpace(q.Question)
		if len(text) < 3 {
			return helper.BadRequest("setiap pertanyaan minimal harus memiliki panjang 3 karakter")
		}
		questions = append(questions, model.QuestionAnswer{Question: text})
	}
	if len(questions) == 0 {
		return helper.BadRequest("daftar pertanyaan verifikasi wajib diisi (minimal 1 pertanyaan)")
	}
	created, err := s.verif.CreateSession(ctx, model.VerificationSession{
		ClaimID: claim.ID, ItemID: item.ID, FinderID: item.UserID,
		ClaimantID: claim.ClaimantID, Questions: questions,
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
// @Description Mengambil data sesi tanya-jawab ciri khusus barang dari MongoDB berdasarkan ID klaim. Hanya penemu, pengklaim, atau admin yang berhak mengakses.
// @Tags verifications
// @Produce json
// @Security BearerAuth
// @Param id path int true "ID Klaim"
// @Success 200 {object} model.WebResponse{data=model.VerificationSession}
// @Failure 400 {object} model.ErrorResponse "ID klaim tidak valid"
// @Failure 401 {object} model.ErrorResponse "Belum terautentikasi"
// @Failure 403 {object} model.ErrorResponse "Tidak berhak mengakses sesi verifikasi ini"
// @Failure 404 {object} model.ErrorResponse "Sesi verifikasi tidak ditemukan"
// @Router /claims/{id}/verification [get]
func (s *VerificationService) GetSession(c *fiber.Ctx) error {
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

	// Otorisasi: Hanya penemu barang, pengklaim sah, atau admin yang boleh mengakses
	if session.FinderID != actor.UserID && session.ClaimantID != actor.UserID && actor.Role != "admin" {
		return helper.Forbidden("tidak berhak mengakses sesi verifikasi ini")
	}

	// Privasi: Jika pengklaim yang membaca, sembunyikan catatan rahasia milik penemu
	if actor.Role != "admin" && session.FinderID != actor.UserID {
		session.ConfidentialNotes = ""
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
// @Param body body model.SubmitAnswersRequest true "Jawaban atas pertanyaan verifikasi"
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
	claim, err := s.claims.FindByID(ctx, claimID)
	if err != nil {
		return helper.NotFound("klaim tidak ditemukan")
	}
	if claim.Status != "pending" {
		return helper.Conflict("sesi verifikasi tidak dapat diubah karena status klaim bukan lagi pending")
	}
	session, err := s.verif.FindByClaimID(ctx, claimID)
	if err != nil {
		return helper.NotFound("sesi verifikasi tidak ditemukan")
	}
	if session.ClaimantID != actor.UserID {
		return helper.Forbidden("hanya pengklaim yang boleh mengisi jawaban")
	}
	if session.IsVerified {
		return helper.Conflict("sesi verifikasi sudah selesai dinilai dan tidak dapat diubah lagi")
	}
	var req model.SubmitAnswersRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus JSON valid")
	}
	if errs := helper.ValidateStruct(req); len(errs) > 0 {
		return helper.Validation(errs)
	}
	rawAnswers := req.Answers
	if len(rawAnswers) == 0 {
		return helper.BadRequest("jawaban verifikasi wajib diisi (gunakan field 'answers')")
	}
	if len(rawAnswers) != len(session.Questions) {
		return helper.BadRequest("jumlah jawaban harus sesuai dengan jumlah pertanyaan penemu")
	}
	answers := make([]model.QuestionAnswer, len(session.Questions))
	for i, q := range session.Questions {
		ans := strings.TrimSpace(rawAnswers[i])
		if ans == "" {
			return helper.BadRequest("setiap pertanyaan verifikasi wajib dijawab")
		}
		answers[i] = model.QuestionAnswer{
			Question: q.Question,
			Answer:   ans,
		}
	}
	updated, err := s.verif.UpdateAnswers(ctx, claimID, answers)
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
	claim, err := s.claims.FindByID(ctx, claimID)
	if err != nil {
		return helper.NotFound("klaim tidak ditemukan")
	}
	if claim.Status != "pending" {
		return helper.Conflict("sesi verifikasi tidak dapat ditinjau karena status klaim bukan lagi pending")
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
	if errs := helper.ValidateStruct(req); len(errs) > 0 {
		return helper.Validation(errs)
	}
	if req.IsVerified == nil && req.Action == "" {
		return helper.BadRequest("status penilaian 'is_verified' (true/false) wajib diisi")
	}
	isVerified := false
	if req.IsVerified != nil {
		isVerified = *req.IsVerified
	} else if strings.EqualFold(req.Action, "pass") || strings.EqualFold(req.Action, "approved") {
		isVerified = true
	}

	notes := strings.TrimSpace(req.ConfidentialNotes)
	if notes == "" {
		notes = strings.TrimSpace(req.Notes)
	}

	updated, err := s.verif.Review(ctx, claimID, isVerified, notes)
	if err != nil {
		return helper.Internal(err)
	}
	return helper.Success(c, fiber.StatusOK, "hasil verifikasi disimpan", updated)
}
