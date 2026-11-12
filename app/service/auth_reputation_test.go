package service_test

import (
	"context"
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

	"findit-api/app/model"
	"findit-api/app/repository"
	"findit-api/app/service"
	"findit-api/config"
	"findit-api/helper"
	"findit-api/middleware"
	"findit-api/route"

	"github.com/gofiber/fiber/v2"
)

type fakeUserRepo struct {
	mu     sync.Mutex
	nextID int
	users  map[int]model.User
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{
		nextID: 1,
		users:  make(map[int]model.User),
	}
}

func (r *fakeUserRepo) Create(_ context.Context, u model.User) (model.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, existing := range r.users {
		if strings.EqualFold(existing.Username, u.Username) || strings.EqualFold(existing.Email, u.Email) {
			return model.User{}, repository.ErrDuplicate
		}
	}

	now := time.Now().UTC()
	u.ID = r.nextID
	r.nextID++
	if u.Role == "" {
		u.Role = "user"
	}
	if u.TrustScore == 0 {
		u.TrustScore = 100
	}
	u.CreatedAt = now
	u.UpdatedAt = now
	r.users[u.ID] = u
	return u, nil
}

func (r *fakeUserRepo) FindByUsername(_ context.Context, username string) (model.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, u := range r.users {
		if strings.EqualFold(u.Username, username) {
			return u, nil
		}
	}
	return model.User{}, repository.ErrNotFound
}

func (r *fakeUserRepo) FindByEmail(_ context.Context, email string) (model.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, u := range r.users {
		if strings.EqualFold(u.Email, email) {
			return u, nil
		}
	}
	return model.User{}, repository.ErrNotFound
}

func (r *fakeUserRepo) FindByID(_ context.Context, id int) (model.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	u, ok := r.users[id]
	if !ok {
		return model.User{}, repository.ErrNotFound
	}
	return u, nil
}

func (r *fakeUserRepo) UpdateProfile(_ context.Context, id int, fullName, phone string) (model.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	u, ok := r.users[id]
	if !ok {
		return model.User{}, repository.ErrNotFound
	}
	u.FullName = fullName
	u.Phone = phone
	u.UpdatedAt = time.Now().UTC()
	r.users[id] = u
	return u, nil
}

func (r *fakeUserRepo) UpdateTrustScore(_ context.Context, _ repository.DBTX, id int, deltaPoints int) (model.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	u, ok := r.users[id]
	if !ok {
		return model.User{}, repository.ErrNotFound
	}
	u.TrustScore += deltaPoints
	if u.TrustScore < 0 {
		u.TrustScore = 0
	}
	u.UpdatedAt = time.Now().UTC()
	r.users[id] = u
	return u, nil
}

func (r *fakeUserRepo) FindAfterCursor(_ context.Context, q model.CursorQuery) ([]model.User, error) {
	return []model.User{}, nil
}

func (r *fakeUserRepo) Update(_ context.Context, id int, u model.User) (model.User, error) {
	return r.UpdateProfile(context.Background(), id, u.FullName, u.Phone)
}

func (r *fakeUserRepo) UpdateRole(_ context.Context, id int, role string) (model.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.users[id]
	if !ok {
		return model.User{}, repository.ErrNotFound
	}
	u.Role = role
	r.users[id] = u
	return u, nil
}

func (r *fakeUserRepo) Delete(_ context.Context, id int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.users, id)
	return nil
}

type fakeTokenRepo struct {
	mu     sync.Mutex
	tokens map[string]model.RefreshToken
}

func newFakeTokenRepo() *fakeTokenRepo {
	return &fakeTokenRepo{tokens: make(map[string]model.RefreshToken)}
}

func (r *fakeTokenRepo) Save(_ context.Context, t model.RefreshToken) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	t.ID = int64(len(r.tokens) + 1)
	t.CreatedAt = time.Now().UTC()
	r.tokens[t.TokenHash] = t
	return nil
}

func (r *fakeTokenRepo) FindByHash(_ context.Context, tokenHash string) (model.RefreshToken, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	t, ok := r.tokens[tokenHash]
	if !ok || t.RevokedAt != nil || !t.ExpiresAt.After(time.Now()) {
		return model.RefreshToken{}, repository.ErrNotFound
	}
	return t, nil
}

func (r *fakeTokenRepo) Revoke(_ context.Context, tokenHash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	t, ok := r.tokens[tokenHash]
	if !ok || t.RevokedAt != nil {
		return nil
	}
	now := time.Now().UTC()
	t.RevokedAt = &now
	r.tokens[tokenHash] = t
	return nil
}

func (r *fakeTokenRepo) RevokeAllUserTokens(_ context.Context, userID int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	for hash, t := range r.tokens {
		if t.UserID == userID && t.RevokedAt == nil {
			t.RevokedAt = &now
			r.tokens[hash] = t
		}
	}
	return nil
}

type fakeReputationRepo struct {
	mu   sync.Mutex
	logs []model.ReputationLog
}

func newFakeReputationRepo() *fakeReputationRepo {
	return &fakeReputationRepo{}
}

func (r *fakeReputationRepo) EnsureIndexes(_ context.Context) error { return nil }

func (r *fakeReputationRepo) CreateLog(_ context.Context, entry model.ReputationLog) (model.ReputationLog, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, existing := range r.logs {
		if existing.ClaimID == entry.ClaimID && existing.UserID == entry.UserID {
			return model.ReputationLog{}, repository.ErrDuplicate
		}
	}
	entry.ID = "67330000000000000000000" + strconv.Itoa(len(r.logs)+1)
	r.logs = append(r.logs, entry)
	return entry, nil
}

func (r *fakeReputationRepo) FindByUserID(_ context.Context, userID int) ([]model.ReputationLog, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]model.ReputationLog, 0)
	for i := len(r.logs) - 1; i >= 0; i-- {
		if r.logs[i].UserID == userID {
			out = append(out, r.logs[i])
		}
	}
	return out, nil
}

func (r *fakeReputationRepo) GetTotalReturnedCount(_ context.Context, userID int) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var count int64
	for _, entry := range r.logs {
		if entry.UserID == userID {
			count++
		}
	}
	return count, nil
}

func (r *fakeReputationRepo) SummaryByUserID(_ context.Context, userID int) (model.ReputationSummary, error) {
	return model.ReputationSummary{UserID: userID, Distribution: map[string]int{"1": 0, "2": 0, "3": 0, "4": 0, "5": 0}}, nil
}

type testEnv struct {
	app   *fiber.App
	jwt   *helper.JWTManager
	users *fakeUserRepo
}

func newTestEnv(t *testing.T) testEnv {
	t.Helper()

	users := newFakeUserRepo()
	tokens := newFakeTokenRepo()
	reputations := newFakeReputationRepo()
	jwtManager := helper.NewJWTManager("secret-findit-api-untuk-test-minimal-32-karakter", "findit-api", time.Hour)

	perms := helper.DefaultPermissions()
	app := config.NewApp(slog.New(slog.NewTextHandler(io.Discard, nil)), route.Dependencies{
		JWT:               jwtManager,
		Permissions:       perms,
		AuthService:       service.NewAuthService(users, tokens, jwtManager, perms, 7*24*time.Hour),
		UserService:       service.NewUserService(users, perms),
		ReputationService: service.NewReputationService(users, reputations, nil),
	}, "*")

	return testEnv{app: app, jwt: jwtManager, users: users}
}

func (e testEnv) do(t *testing.T, bearerToken, method, path, body string) (int, map[string]any) {
	t.Helper()

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	}

	resp, err := e.app.Test(req, -1)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()

	var parsed map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		t.Fatalf("%s %s: response bukan JSON: %v", method, path, err)
	}
	return resp.StatusCode, parsed
}

func TestAuthLifecycle_RegisterLoginRefreshLogoutMe(t *testing.T) {
	env := newTestEnv(t)

	// 1. Register
	regBody := `{"username":"lifia_dev","email":"lifia@kampus.ac.id","password":"RahasiaFind1t!","full_name":"Lifia Putri","phone":"081234567890"}`
	status, body := env.do(t, "", http.MethodPost, "/api/v1/auth/register", regBody)
	if status != http.StatusCreated {
		t.Fatalf("register status = %d, want 201 (body: %v)", status, body)
	}
	userData := body["data"].(map[string]any)
	if _, hasPwd := userData["password_hash"]; hasPwd {
		t.Fatal("password_hash tidak boleh bocor di JSON response")
	}
	if userData["role"] != "user" || userData["trust_score"] != float64(100) {
		t.Fatalf("unexpected default user fields: %v", userData)
	}

	// Duplicate register -> 409
	status, _ = env.do(t, "", http.MethodPost, "/api/v1/auth/register", regBody)
	if status != http.StatusConflict {
		t.Fatalf("duplicate register status = %d, want 409", status)
	}

	// 2. Login
	loginBody := `{"username":"lifia_dev","password":"RahasiaFind1t!"}`
	status, body = env.do(t, "", http.MethodPost, "/api/v1/auth/login", loginBody)
	if status != http.StatusOK {
		t.Fatalf("login status = %d, want 200 (body: %v)", status, body)
	}
	pair := body["data"].(map[string]any)
	accessToken := pair["access_token"].(string)
	refreshToken := pair["refresh_token"].(string)

	// 3. Me
	status, body = env.do(t, accessToken, http.MethodGet, "/api/v1/auth/me", "")
	if status != http.StatusOK {
		t.Fatalf("me status = %d, want 200 (body: %v)", status, body)
	}
	if body["data"].(map[string]any)["username"] != "lifia_dev" {
		t.Fatalf("unexpected me username: %v", body["data"])
	}

	// 4. Refresh (rotasi token)
	status, body = env.do(t, "", http.MethodPost, "/api/v1/auth/refresh", `{"refresh_token":"`+refreshToken+`"}`)
	if status != http.StatusOK {
		t.Fatalf("refresh status = %d, want 200 (body: %v)", status, body)
	}
	newPair := body["data"].(map[string]any)
	newRefresh := newPair["refresh_token"].(string)
	if newRefresh == refreshToken {
		t.Fatal("refresh token baru harus berbeda setelah rotasi")
	}

	// Reuse old refresh token -> 401
	status, _ = env.do(t, "", http.MethodPost, "/api/v1/auth/refresh", `{"refresh_token":"`+refreshToken+`"}`)
	if status != http.StatusUnauthorized {
		t.Fatalf("reused refresh token status = %d, want 401", status)
	}

	// 5. Logout revokes newRefresh
	status, _ = env.do(t, "", http.MethodPost, "/api/v1/auth/logout", `{"refresh_token":"`+newRefresh+`"}`)
	if status != http.StatusOK {
		t.Fatalf("logout status = %d, want 200", status)
	}
	status, _ = env.do(t, "", http.MethodPost, "/api/v1/auth/refresh", `{"refresh_token":"`+newRefresh+`"}`)
	if status != http.StatusUnauthorized {
		t.Fatalf("revoked refresh token status = %d, want 401", status)
	}
}

func TestUserAndReputationFlow(t *testing.T) {
	env := newTestEnv(t)

	_, _ = env.do(t, "", http.MethodPost, "/api/v1/auth/register",
		`{"username":"finder1","email":"finder@kampus.ac.id","password":"RahasiaFind1t!","full_name":"Finder Satu"}`)
	_, _ = env.do(t, "", http.MethodPost, "/api/v1/auth/register",
		`{"username":"owner1","email":"owner@kampus.ac.id","password":"RahasiaFind1t!","full_name":"Owner Satu"}`)

	finderToken, err := env.jwt.GenerateAccess(1, "finder1", "user")
	if err != nil {
		t.Fatal(err)
	}
	ownerToken, err := env.jwt.GenerateAccess(2, "owner1", "user")
	if err != nil {
		t.Fatal(err)
	}

	// Update profile finder1
	status, body := env.do(t, finderToken, http.MethodPut, "/api/v1/users/profile", `{"full_name":"Finder Terverifikasi","phone":"089988776655"}`)
	if status != http.StatusOK {
		t.Fatalf("update profile status = %d, want 200 (body: %v)", status, body)
	}
	if body["data"].(map[string]any)["full_name"] != "Finder Terverifikasi" {
		t.Fatalf("full_name tidak terupdate: %v", body["data"])
	}

	// Self-reward forbidden
	rewardPayload := `{"claim_id":10,"item_id":5,"points":35,"reason":"Mengembalikan laptop dengan aman","feedback":{"rating":5,"comment":"Sangat jujur dan cepat!"}}`
	status, _ = env.do(t, finderToken, http.MethodPost, "/api/v1/users/1/reward", rewardPayload)
	if status != http.StatusForbidden {
		t.Fatalf("self reward status = %d, want 403", status)
	}

	// Owner rewards Finder (+35 -> 135 -> Silver)
	status, body = env.do(t, ownerToken, http.MethodPost, "/api/v1/users/1/reward", rewardPayload)
	if status != http.StatusCreated {
		t.Fatalf("add reward status = %d, want 201 (body: %v)", status, body)
	}
	data := body["data"].(map[string]any)
	if data["trust_score"] != float64(135) || data["badge"] != "Silver" {
		t.Fatalf("unexpected score/badge: %v", data)
	}

	// Duplicate reward for same claim -> 409
	status, _ = env.do(t, ownerToken, http.MethodPost, "/api/v1/users/1/reward", rewardPayload)
	if status != http.StatusConflict {
		t.Fatalf("duplicate claim reward status = %d, want 409", status)
	}

	// GetUserReputation
	status, body = env.do(t, ownerToken, http.MethodGet, "/api/v1/users/1/reputation", "")
	if status != http.StatusOK {
		t.Fatalf("get reputation status = %d, want 200 (body: %v)", status, body)
	}
	rep := body["data"].(map[string]any)
	if rep["score"] != float64(135) || rep["badge"] != "Silver" || rep["total_found"] != float64(1) {
		t.Fatalf("unexpected reputation summary: %v", rep)
	}
}

func TestRequireRoleMiddleware(t *testing.T) {
	jwtManager := helper.NewJWTManager("secret-findit-api-untuk-test-minimal-32-karakter", "findit-api", time.Hour)
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			if appErr, ok := err.(*helper.AppError); ok {
				return c.Status(appErr.Status).JSON(model.ErrorResponse{
					Success: false,
					Code:    appErr.Code,
					Message: appErr.Message,
				})
			}
			return c.SendStatus(fiber.StatusInternalServerError)
		},
	})
	app.Get("/admin-only", middleware.RequireAuth(jwtManager), middleware.RequireRole("admin"), func(c *fiber.Ctx) error {
		return helper.Success(c, fiber.StatusOK, "ok", nil)
	})

	userTok, _ := jwtManager.GenerateAccess(1, "mhs", "user")
	adminTok, _ := jwtManager.GenerateAccess(2, "adm", "admin")

	reqUser := httptest.NewRequest(http.MethodGet, "/admin-only", nil)
	reqUser.Header.Set("Authorization", "Bearer "+userTok)
	respUser, err := app.Test(reqUser, -1)
	if err != nil {
		t.Fatal(err)
	}
	if respUser.StatusCode != http.StatusForbidden {
		t.Fatalf("user role status = %d, want 403", respUser.StatusCode)
	}

	reqAdmin := httptest.NewRequest(http.MethodGet, "/admin-only", nil)
	reqAdmin.Header.Set("Authorization", "Bearer "+adminTok)
	respAdmin, err := app.Test(reqAdmin, -1)
	if err != nil {
		t.Fatal(err)
	}
	if respAdmin.StatusCode != http.StatusOK {
		t.Fatalf("admin role status = %d, want 200", respAdmin.StatusCode)
	}
}
