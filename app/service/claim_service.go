package service

import (
	"errors"
	"strconv"
	"strings"

	"findit-api/app/model"
	"findit-api/app/repository"
	"findit-api/helper"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type ClaimService struct {
	claims repository.ClaimRepository
	items  repository.ItemRepository
	outbox repository.OutboxRepository
	tx     *repository.TxManager
}

func NewClaimService(claims repository.ClaimRepository, items repository.ItemRepository, outbox repository.OutboxRepository, tx *repository.TxManager) *ClaimService {
	return &ClaimService{claims: claims, items: items, outbox: outbox, tx: tx}
}

func (s *ClaimService) Create(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	actor, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}
	itemID, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id barang tidak valid")
	}
	item, err := s.items.FindByID(ctx, itemID)
	if err != nil {
		return helper.NotFound("barang tidak ditemukan")
	}
	if item.ItemType != "found" || item.Status != "open" {
		return helper.Conflict("hanya barang temuan berstatus open yang dapat diklaim")
	}
	if item.UserID == actor.UserID {
		return helper.Forbidden("tidak boleh mengklaim barang temuan sendiri")
	}
	var req model.CreateClaimRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus JSON valid")
	}
	if errs := helper.ValidateStruct(req); len(errs) > 0 {
		return helper.Validation(errs)
	}
	var created model.Claim
	err = s.tx.WithTxOnce(ctx, func(tx pgx.Tx) error {
		var err error
		created, err = s.claims.Create(ctx, tx, model.Claim{
			ItemID: itemID, ClaimantID: actor.UserID, ProofDescription: strings.TrimSpace(req.ProofDescription),
		})
		if err != nil {
			return err
		}
		_, err = s.outbox.Insert(ctx, tx, model.EventClaimSubmitted, model.ClaimEvent{
			ClaimID: created.ID, ItemID: item.ID, ItemTitle: item.Title,
			FinderID: item.UserID, ClaimantID: actor.UserID, Status: "pending",
		})
		return err
	})
	if err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return helper.Conflict("Anda sudah mengajukan klaim pada barang ini")
		}
		return helper.Internal(err)
	}
	return helper.Created(c, "klaim berhasil diajukan", created, "/api/v1/claims/"+strconv.Itoa(created.ID))
}

func (s *ClaimService) Get(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	id, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id klaim tidak valid")
	}
	claim, err := s.claims.FindByID(ctx, id)
	if err != nil {
		return helper.NotFound("klaim tidak ditemukan")
	}
	return helper.Success(c, fiber.StatusOK, "detail klaim ditemukan", claim)
}

func (s *ClaimService) ListByItem(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	actor, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}
	itemID, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id barang tidak valid")
	}
	item, err := s.items.FindByID(ctx, itemID)
	if err != nil {
		return helper.NotFound("barang tidak ditemukan")
	}
	if item.UserID != actor.UserID && actor.Role != "admin" {
		return helper.Forbidden("hanya penemu barang atau admin yang berhak melihat klaim masuk")
	}
	list, err := s.claims.FindByItemID(ctx, itemID)
	if err != nil {
		return helper.Internal(err)
	}
	return helper.Success(c, fiber.StatusOK, "daftar klaim berhasil diambil", list)
}

func (s *ClaimService) MyClaims(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	actor, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}
	list, err := s.claims.FindByClaimant(ctx, actor.UserID)
	if err != nil {
		return helper.Internal(err)
	}
	return helper.Success(c, fiber.StatusOK, "riwayat klaim Anda", list)
}

func (s *ClaimService) UpdateStatus(c *fiber.Ctx) error {
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
	var req model.UpdateClaimStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus JSON valid")
	}
	if errs := helper.ValidateStruct(req); len(errs) > 0 {
		return helper.Validation(errs)
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
	if req.Status == "cancelled" && claim.ClaimantID != actor.UserID {
		return helper.Forbidden("hanya pengklaim yang boleh membatalkan klaim")
	}
	if (req.Status == "approved" || req.Status == "rejected") && item.UserID != actor.UserID && actor.Role != "admin" {
		return helper.Forbidden("hanya penemu barang atau admin yang boleh memproses klaim")
	}

	var updated model.Claim
	err = s.tx.WithTx(ctx, func(tx pgx.Tx) error {
		var err error
		updated, err = s.claims.UpdateStatus(ctx, tx, claimID, req.Status)
		if err != nil {
			return err
		}
		eventType := model.EventClaimDecided
		if req.Status == "approved" {
			eventType = model.EventItemReturned
			if err := s.items.UpdateStatusWithOCC(ctx, tx, item.ID, "returned", item.Version); err != nil {
				return err
			}
		}
		_, err = s.outbox.Insert(ctx, tx, eventType, model.ClaimEvent{
			ClaimID: claim.ID, ItemID: item.ID, ItemTitle: item.Title,
			FinderID: item.UserID, ClaimantID: claim.ClaimantID, Status: req.Status,
		})
		return err
	})
	if err != nil {
		if errors.Is(err, repository.ErrConflictOCC) {
			return helper.Conflict("konflik versi optimistik: barang sudah diselesaikan oleh klaim lain")
		}
		return helper.Internal(err)
	}
	return helper.Success(c, fiber.StatusOK, "status klaim diperbarui", updated)
}
