package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/redisx"
	"avtotest.uz/backend/internal/testdb"
)

func TestLoginNormalWrongAndTemporaryPassword(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	c := redisx.NewTest(t)
	svc := NewService(q, pool, Limiter{R: c}, SandboxSender{Log: zap.NewNop()}, []byte(handlerSecret), "test")
	ctx := context.Background()

	const phone = "901222333"
	const password = "normalpass"

	reg, err := svc.Register(ctx, RegisterInput{Phone: phone, Password: password, Name: "A"})
	if err != nil {
		t.Fatal(err)
	}
	if reg.Profile.MustChangePassword {
		t.Fatal("fresh register must not require password change")
	}

	login, err := svc.Login(ctx, LoginInput{Phone: phone, Password: password})
	if err != nil {
		t.Fatalf("normal login: %v", err)
	}
	if login.Profile.MustChangePassword {
		t.Fatal("normal login must_change=false")
	}

	if _, err := svc.Login(ctx, LoginInput{Phone: phone, Password: "wrongpass"}); !errors.Is(err, ErrInvalidCreds) {
		t.Fatalf("wrong password err=%v", err)
	}

	temp, err := RandomTempPassword()
	if err != nil {
		t.Fatal(err)
	}
	hash, err := HashPassword(temp)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.SetProfilePassword(ctx, sqlc.SetProfilePasswordParams{
		ID:                 reg.Profile.ID,
		PasswordHash:       pgtype.Text{String: hash, Valid: true},
		MustChangePassword: true,
	}); err != nil {
		t.Fatal(err)
	}

	tempLogin, err := svc.Login(ctx, LoginInput{Phone: phone, Password: temp})
	if err != nil {
		t.Fatalf("temp login: %v", err)
	}
	if !tempLogin.Profile.MustChangePassword {
		t.Fatal("temp login must set must_change_password")
	}
	if _, err := svc.Login(ctx, LoginInput{Phone: phone, Password: password}); !errors.Is(err, ErrInvalidCreds) {
		t.Fatalf("old password after temp reset err=%v", err)
	}

	// HTTP login response includes flag and never echoes password material.
	r := chi.NewRouter()
	(&Handler{Svc: svc}).Routes(r)
	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)
	status, env := postJSON(t, ts, "/auth/login", map[string]string{"phone": phone, "password": temp})
	if status != http.StatusOK {
		t.Fatalf("http login status=%d env=%+v", status, env)
	}
	raw := string(env.Data)
	if strings.Contains(raw, temp) || strings.Contains(raw, "password_hash") {
		t.Fatalf("login response leaked secret material: %s", raw)
	}
	var toks tokensResponse
	if err := json.Unmarshal(env.Data, &toks); err != nil {
		t.Fatal(err)
	}
	if !toks.MustChangePassword || toks.AccessToken == "" {
		t.Fatalf("toks=%+v", toks)
	}
}

func TestPasswordChangeAllowedPaths(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/me", nil)
	if !passwordChangeAllowed(req) {
		t.Fatal("GET /api/v1/me must be allowed")
	}
	req, _ = http.NewRequest(http.MethodPost, "/api/v1/me/password", nil)
	if !passwordChangeAllowed(req) {
		t.Fatal("POST /me/password must be allowed")
	}
	req, _ = http.NewRequest(http.MethodGet, "/api/v1/me/entitlement", nil)
	if passwordChangeAllowed(req) {
		t.Fatal("GET /me/entitlement must be gated")
	}
	req, _ = http.NewRequest(http.MethodPatch, "/api/v1/me", nil)
	if passwordChangeAllowed(req) {
		t.Fatal("PATCH /me must be gated")
	}
}
