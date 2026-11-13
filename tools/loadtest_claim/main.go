package main

import (
"bytes"
"flag"
"fmt"
"net/http"
"sync"
"sync/atomic"
)

func main() {
url := flag.String("url", "http://localhost:3000/api/v1/claims/1/status", "endpoint approve klaim")
token := flag.String("token", "", "Bearer token penemu barang")
workers := flag.Int("n", 20, "jumlah goroutine serentak")
flag.Parse()

var wg sync.WaitGroup
var ok200, conflict409, other atomic.Int32
client := &http.Client{}
payload := []byte(`{"status":"approved"}`)

for i := 0; i < *workers; i++ {
wg.Add(1)
go func() {
defer wg.Done()
req, _ := http.NewRequest(http.MethodPatch, *url, bytes.NewReader(payload))
req.Header.Set("Content-Type", "application/json")
req.Header.Set("Authorization", "Bearer "+*token)
resp, err := client.Do(req)
if err != nil {
other.Add(1)
return
}
resp.Body.Close()
switch resp.StatusCode {
case http.StatusOK:
ok200.Add(1)
case http.StatusConflict:
conflict409.Add(1)
default:
other.Add(1)
}
}()
}
wg.Wait()
fmt.Printf("Hasil OCC Load Test -> 200 OK: %d | 409 Conflict: %d | Lainnya: %d\n",
ok200.Load(), conflict409.Load(), other.Load())
}
// === UPDATE route/route.go (Tambahkan Field & Rute Zaskia di Branch 'zaskia') ===
type Dependencies struct {
    // ... dependensi Lifia (Fase 1) & Reta (Fase 2) hasil pull dari main ...
    ClaimService        *service.ClaimService        // [Zaskia]
    VerificationService *service.VerificationService // [Zaskia]
}
// Di Register(app, deps):
claims := api.Group("/claims", middleware.RequireAuth(deps.JWT))
claims.Get("/me", deps.ClaimService.MyClaims)
claims.Get("/:id", deps.ClaimService.Get)
claims.Patch("/:id/status", middleware.RequireJSON, deps.ClaimService.UpdateStatus)
api.Post("/items/:id/claims", middleware.RequireAuth(deps.JWT), middleware.RequireJSON, deps.ClaimService.Create)
api.Get("/items/:id/claims", middleware.RequireAuth(deps.JWT), deps.ClaimService.ListByItem)

verif := api.Group("/claims/:id/verification", middleware.RequireAuth(deps.JWT))
verif.Get("/", deps.VerificationService.GetSession)
verif.Post("/", middleware.RequireJSON, deps.VerificationService.CreateSession)
verif.Put("/answers", middleware.RequireJSON, deps.VerificationService.AnswerQuestions)
verif.Post("/review", middleware.RequireJSON, deps.VerificationService.ReviewVerifications)

// === UPDATE main.go (Inisialisasi Modul Zaskia di Branch 'zaskia') ===
txMgr := repository.NewTxManager(pool)
claimRepo := repository.NewClaimRepository(pool)
verifRepo := repository.NewVerificationRepository(mongoDB)
_ = verifRepo.EnsureIndexes(ctx)
app := config.NewApp(logger, route.Dependencies{
    // ... field Lifia & Reta ...
    ClaimService:        service.NewClaimService(claimRepo, itemRepo, outboxRepo, txMgr),      // [Zaskia]
    VerificationService: service.NewVerificationService(verifRepo, claimRepo, itemRepo),      // [Zaskia]
}, config.GetEnv("ALLOWED_ORIGINS", "*"))
