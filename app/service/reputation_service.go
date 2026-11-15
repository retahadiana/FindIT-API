package service

import (
	"errors"
	"strings"
	"time"

	"findit-api/app/model"
	"findit-api/app/repository"
	"findit-api/helper"

	"github.com/gofiber/fiber/v2"
)

// ReputationService mengelola perhitungan Trust Score (PostgreSQL) dan audit log MongoDB.
type ReputationService struct {
	users       repository.UserRepository
	reputations repository.ReputationRepository
	claims      repository.ClaimRepository
	items       repository.ItemRepository
	outbox      repository.OutboxRepository
}

// NewReputationService membuat instance ReputationService.
func NewReputationService(
	users repository.UserRepository,
	reputations repository.ReputationRepository,
	claims repository.ClaimRepository,
	items repository.ItemRepository,
	outbox repository.OutboxRepository,
) *ReputationService {
	return &ReputationService{
		users:       users,
		reputations: reputations,
		claims:      claims,
		items:       items,
		outbox:      outbox,
	}
}

// CalculatePointsFromRating menentukan poin reputasi berdasarkan rating kepuasan ulasan (1-5).
// Rating 5: +35 poin (Sangat memuaskan, penemu langsung dipromosikan ke badge Silver)
// Rating 4: +20 poin (Baik & kooperatif)
// Rating 3: +10 poin (Cukup / netral)
// Rating 2: +5 poin  (Kurang memuaskan)
// Rating 1: +0 poin  (Sangat buruk)
func CalculatePointsFromRating(rating int) int {
	switch rating {
	case 5:
		return 35
	case 4:
		return 20
	case 3:
		return 10
	case 2:
		return 5
	case 1:
		return 0
	default:
		return 10
	}
}

// BadgeForScore memetakan nilai trust_score ke tingkatan lencana kejujuran.
func BadgeForScore(score int) string {
	switch {
	case score >= 200:
		return "Gold Samaritan"
	case score >= 130:
		return "Silver"
	default:
		return "Bronze"
	}
}

// GetUserReputation godoc
//
// @Summary Ambil skor reputasi dan riwayat log pengguna
// @Description Mengambil skor trust score, badge level, total barang dikembalikan, dan riwayat ulasan dari MongoDB.
// @Tags reputation
// @Produce json
// @Param id path int true "ID Pengguna"
// @Success 200 {object} model.WebResponse{data=model.UserReputationResponse}
// @Failure 400 {object} model.ErrorResponse "ID user harus berupa angka positif"
// @Failure 404 {object} model.ErrorResponse "User tidak ditemukan"
// @Router /users/{id}/reputation [get]
func (s *ReputationService) GetUserReputation(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	userID, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id user harus berupa angka positif")
	}

	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return helper.NotFound("user tidak ditemukan")
		}
		return helper.Internal(err)
	}

	history, err := s.reputations.FindByUserID(ctx, userID)
	if err != nil {
		return helper.Internal(err)
	}

	totalFound, err := s.reputations.GetTotalReturnedCount(ctx, userID)
	if err != nil {
		return helper.Internal(err)
	}

	return helper.Success(c, fiber.StatusOK, "reputasi user berhasil diambil", model.UserReputationResponse{
		User:       model.ToUserPublic(user),
		Score:      user.TrustScore,
		Badge:      BadgeForScore(user.TrustScore),
		TotalFound: totalFound,
		History:    history,
	})
}

// AddReward godoc
//
// @Summary Berikan reward poin reputasi dan feedback
// @Description Menambahkan poin reputasi kejujuran dan ulasan bintang/komentar atas pengembalian barang.
// @Tags reputation
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "ID Pengguna Penerima Reward"
// @Param body body model.AddReputationRequest true "Detail poin dan feedback"
// @Success 201 {object} model.WebResponse
// @Failure 400 {object} model.ErrorResponse "Data tidak valid"
// @Failure 401 {object} model.ErrorResponse "Belum terautentikasi"
// @Failure 403 {object} model.ErrorResponse "Tidak boleh memberi reward ke diri sendiri"
// @Failure 404 {object} model.ErrorResponse "User tidak ditemukan"
// @Failure 409 {object} model.ErrorResponse "Reward untuk klaim ini sudah pernah diberikan"
// @Router /users/{id}/reward [post]
func (s *ReputationService) AddReward(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	targetUserID, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id user harus berupa angka positif")
	}
	if current.UserID == targetUserID {
		return helper.Forbidden("tidak boleh memberikan poin reputasi kepada diri sendiri")
	}

	var req model.AddReputationRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}
	// Poin reputasi dihitung secara otoritatif oleh sistem berdasarkan rating feedback untuk mencegah manipulasi poin.
	if req.Feedback != nil && req.Feedback.Rating > 0 {
		req.Points = CalculatePointsFromRating(req.Feedback.Rating)
	} else if current.Role == "admin" && req.Points > 0 {
		// Khusus admin dapat menentukan poin reward kustom
	} else {
		req.Points = 10 // Poin default otomatis
	}
	req.Reason = strings.TrimSpace(req.Reason)
	var logFeedback *model.Feedback
	if req.Feedback != nil {
		req.Feedback.Comment = strings.TrimSpace(req.Feedback.Comment)
		logFeedback = &model.Feedback{
			Rating:     req.Feedback.Rating,
			Comment:    req.Feedback.Comment,
			FromUserID: current.UserID,
		}
	}

	if errs := helper.ValidateStruct(req); len(errs) > 0 {
		return helper.Validation(errs)
	}

	if _, err := s.users.FindByID(ctx, targetUserID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return helper.NotFound("user tidak ditemukan")
		}
		return helper.Internal(err)
	}

	// Validasi integritas klaim & anti-fraud reward
	if s.claims != nil {
		claim, err := s.claims.FindByID(ctx, req.ClaimID)
		if err != nil {
			return helper.NotFound("klaim tidak ditemukan")
		}
		// Selalu gunakan ItemID resmi dari data klaim untuk mencegah manipulasi ID barang
		req.ItemID = claim.ItemID
		if claim.ClaimantID != current.UserID && current.Role != "admin" {
			return helper.Forbidden("hanya pengklaim sah penerima barang yang boleh memberikan reward")
		}
		if claim.Status != "approved" {
			return helper.Conflict("reward hanya dapat diberikan untuk klaim yang sudah disetujui (approved)")
		}
		if s.items != nil {
			item, err := s.items.FindByID(ctx, req.ItemID)
			if err != nil {
				return helper.NotFound("barang tidak ditemukan")
			}
			if item.UserID != targetUserID {
				return helper.BadRequest("penerima reward bukan penemu sah dari barang ini")
			}
		}
	}

	// Dual-write: 1. Update trust score di PostgreSQL
	updatedUser, err := s.users.UpdateTrustScore(ctx, nil, targetUserID, req.Points)
	if err != nil {
		return helper.Internal(err)
	}

	// Dual-write: 2. Catat audit log ke MongoDB
	createdLog, err := s.reputations.CreateLog(ctx, model.ReputationLog{
		UserID:    targetUserID,
		ClaimID:   req.ClaimID,
		ItemID:    req.ItemID,
		Points:    req.Points,
		Reason:    req.Reason,
		Feedback:  logFeedback,
		CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		// Kompensasi rollback PostgreSQL jika MongoDB gagal/duplikat
		_, _ = s.users.UpdateTrustScore(ctx, nil, targetUserID, -req.Points)
		if errors.Is(err, repository.ErrDuplicate) {
			return helper.Conflict("reward reputasi untuk klaim ini sudah diberikan")
		}
		return helper.Internal(err)
	}

	if s.outbox != nil {
		_, _ = s.outbox.Insert(ctx, nil, model.EventReputationRewarded, model.ReputationRewardedEvent{
			UserID: targetUserID, ClaimID: req.ClaimID, ItemID: req.ItemID, Points: req.Points, Reason: req.Reason,
		})
	}

	return helper.Success(c, fiber.StatusCreated, "poin reputasi berhasil ditambahkan", fiber.Map{
		"user_id":     updatedUser.ID,
		"trust_score": updatedUser.TrustScore,
		"badge":       BadgeForScore(updatedUser.TrustScore),
		"log":         createdLog,
	})
}

// Summary godoc
//
// @Summary Ringkasan distribusi rating reputasi
// @Description Mengambil ringkasan statistik rating bintang pengguna via MongoDB aggregation $facet.
// @Tags reputation
// @Produce json
// @Param id path int true "ID Pengguna"
// @Success 200 {object} model.WebResponse{data=model.ReputationSummary}
// @Failure 400 {object} model.ErrorResponse "ID user harus berupa angka positif"
// @Router /users/{id}/reputation/summary [get]
func (s *ReputationService) Summary(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	userID, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id user harus berupa angka positif")
	}
	summary, err := s.reputations.SummaryByUserID(ctx, userID)
	if err != nil {
		return helper.Internal(err)
	}
	return helper.Success(c, fiber.StatusOK, "ringkasan reputasi berhasil diambil", summary)
}
