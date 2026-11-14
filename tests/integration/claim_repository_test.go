//go:build integration

package integration_test

import (
	"context"
	"errors"
	"testing"

	"findit-api/app/model"
	"findit-api/app/repository"

	"github.com/jackc/pgx/v5"
)

func seedUserAndItem(t *testing.T) (finderID, claimantID, itemID int) {
	t.Helper()
	ctx := context.Background()

	_, _ = testPool.Exec(ctx, `TRUNCATE notifications, outbox, claims, items, refresh_tokens, users RESTART IDENTITY CASCADE`)
	_, _ = testPool.Exec(ctx, `INSERT INTO categories (name) VALUES ('Elektronik') ON CONFLICT (name) DO NOTHING`)
	_ = testPool.QueryRow(ctx, `INSERT INTO users (username, email, password_hash, full_name) VALUES ('finder_test','finder@test.id','hash','Finder') RETURNING id`).Scan(&finderID)
	_ = testPool.QueryRow(ctx, `INSERT INTO users (username, email, password_hash, full_name) VALUES ('claimant_test','claim@test.id','hash','Claimant') RETURNING id`).Scan(&claimantID)

	var catID int
	_ = testPool.QueryRow(ctx, `SELECT id FROM categories LIMIT 1`).Scan(&catID)
	_ = testPool.QueryRow(ctx, `
		INSERT INTO items (user_id, category_id, item_type, title, description, location, event_date, status, version)
		VALUES ($1, $2, 'found', 'Laptop Lab', 'Abu-abu stiker kampus', 'Gedung A', '2026-11-10', 'open', 1)
		RETURNING id`, finderID, catID).Scan(&itemID)

	return finderID, claimantID, itemID
}

func TestItemOCC_And_TxRollback(t *testing.T) {
	ctx := context.Background()
	_, claimantID, itemID := seedUserAndItem(t)

	itemRepo := repository.NewItemRepository(testPool)
	claimRepo := repository.NewClaimRepository(testPool)
	outboxRepo := repository.NewOutboxRepository(testPool)
	txMgr := repository.NewTxManager(testPool)

	// 1. Uji WithTxOnce rollback atomik: error di tengah harus membatalkan insert claims & outbox
	errStop := errors.New("simulasi rollback")
	err := txMgr.WithTxOnce(ctx, func(tx pgx.Tx) error {
		if _, err := claimRepo.Create(ctx, tx, model.Claim{ItemID: itemID, ClaimantID: claimantID, ProofDescription: "Serial 123"}); err != nil {
			return err
		}
		if _, err := outboxRepo.Insert(ctx, tx, model.EventClaimSubmitted, model.ClaimEvent{ItemID: itemID, ClaimantID: claimantID}); err != nil {
			return err
		}
		return errStop
	})

	if !errors.Is(err, errStop) {
		t.Fatalf("harus rollback dengan errStop, dapat: %v", err)
	}

	var claimCount int
	_ = testPool.QueryRow(ctx, `SELECT COUNT(*) FROM claims`).Scan(&claimCount)
	if claimCount != 0 {
		t.Fatalf("claims seharusnya ikut rollback (0 baris), dapat %d", claimCount)
	}

	// 2. Uji OCC UpdateStatusWithOCC (version=1 sukses, pengulangan version=1 -> ErrConflictOCC)
	if err := itemRepo.UpdateStatusWithOCC(ctx, nil, itemID, "returned", 1); err != nil {
		t.Fatalf("OCC pertama gagal: %v", err)
	}

	got, _ := itemRepo.FindByID(ctx, itemID)
	if got.Version != 2 || got.Status != "returned" {
		t.Fatalf("version harus naik jadi 2, dapat ver=%d status=%s", got.Version, got.Status)
	}

	if err := itemRepo.UpdateStatusWithOCC(ctx, nil, itemID, "returned", 1); !errors.Is(err, repository.ErrConflictOCC) {
		t.Fatalf("OCC versi lama harus mengembalikan ErrConflictOCC, dapat: %v", err)
	}
}

