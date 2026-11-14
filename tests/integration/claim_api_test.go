//go:build integration

package integration_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"findit-api/app/repository"
	"findit-api/app/service"
	"findit-api/config"
	"findit-api/helper"
	"findit-api/route"
)

func TestClaimApproveConcurrency_OutboxExactOnce(t *testing.T) {
	finderID, claimantID, itemID := seedUserAndItem(t)

	jwtMgr := helper.NewJWTManager("secret-integration-test-minimal-32-karakter", "findit-test", time.Hour)
	perms := helper.DefaultPermissions()

	claimRepo := repository.NewClaimRepository(testPool)
	itemRepo := repository.NewItemRepository(testPool)
	outboxRepo := repository.NewOutboxRepository(testPool)
	txMgr := repository.NewTxManager(testPool)

	app := config.NewApp(slog.New(slog.NewTextHandler(io.Discard, nil)), route.Dependencies{
		Pool:         testPool,
		JWT:          jwtMgr,
		Permissions:  perms,
		ClaimService: service.NewClaimService(claimRepo, itemRepo, outboxRepo, txMgr),
	}, "*")

	claimantTok, _ := jwtMgr.GenerateAccess(claimantID, "claimant_test", "user")
	finderTok, _ := jwtMgr.GenerateAccess(finderID, "finder_test", "user")

	// 1. Ajukan klaim -> wajib menulis tepat 1 outbox 'claim.submitted'
	req := httptest.NewRequest(http.MethodPost, "/api/v1/items/"+strconv.Itoa(itemID)+"/claims", strings.NewReader(`{"proof_description":"Ada stiker lab di pojok kiri laptop"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+claimantTok)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("submit claim status = %d, want 201", resp.StatusCode)
	}

	var parsed struct {
		Data struct {
			ID int `json:"id"`
		} `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&parsed)
	resp.Body.Close()

	// 2. Uji 5 goroutine approve klaim bersamaan: hanya 1 boleh 200 OK, sisanya 409 Conflict
	var wg sync.WaitGroup
	var okCount, conflictCount int
	var mu sync.Mutex

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := httptest.NewRequest(http.MethodPatch, "/api/v1/claims/"+strconv.Itoa(parsed.Data.ID)+"/status", strings.NewReader(`{"status":"approved"}`))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Authorization", "Bearer "+finderTok)
			rs, err := app.Test(r, -1)
			if err != nil {
				return
			}
			rs.Body.Close()

			mu.Lock()
			if rs.StatusCode == http.StatusOK {
				okCount++
			} else if rs.StatusCode == http.StatusConflict {
				conflictCount++
			}
			mu.Unlock()
		}()
	}

	wg.Wait()

	if okCount != 1 || conflictCount != 4 {
		t.Fatalf("OCC concurrency salah: 200=%d, 409=%d (harus 1 dan 4)", okCount, conflictCount)
	}
}

