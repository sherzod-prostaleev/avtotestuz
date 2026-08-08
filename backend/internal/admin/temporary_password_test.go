package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/redisx"
	"avtotest.uz/backend/internal/testdb"
)

func TestIssueTemporaryPassword(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	ctx := context.Background()
	store := Store{Pool: pool}
	secret := []byte("test-admin-secret-at-least-32-bytes!!")

	if _, err := store.EnsureSuperadmin(ctx, "ops@example.uz", "password123", "Ops"); err != nil {
		t.Fatal(err)
	}
	h := &Handler{Svc: Service{Store: store, Secret: secret}, Pool: pool, Secret: secret}
	r := chi.NewRouter()
	r.Route("/admin/v1", h.Routes)
	access := loginAccess(t, r, "ops@example.uz", "password123")

	oldHash, err := auth.HashPassword("original-pass")
	if err != nil {
		t.Fatal(err)
	}
	learnerID := uuid.New()
	const learnerPhone = "+998901234567"
	if _, err := pool.Exec(ctx, `
		INSERT INTO profile (id, phone, name, password_hash)
		VALUES ($1, $2, 'Learner', $3)`, learnerID, learnerPhone, oldHash); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO refresh_token (profile_id, token_hash, expires_at)
		VALUES ($1, 'live-session', now() + interval '1 day')`, learnerID); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/v1/users/"+learnerID.String()+"/temporary-password", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}

	var env struct {
		Data struct {
			TemporaryPassword  string `json:"temporary_password"`
			MustChangePassword bool   `json:"must_change_password"`
			SessionsRevoked    int64  `json:"sessions_revoked"`
			User               struct {
				MustChangePassword bool `json:"must_change_password"`
				HasPassword        bool `json:"has_password"`
			} `json:"user"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	plain := env.Data.TemporaryPassword
	if plain == "" || len(plain) < 8 {
		t.Fatalf("temporary_password missing/short: %q", plain)
	}
	if !env.Data.MustChangePassword || !env.Data.User.MustChangePassword || !env.Data.User.HasPassword {
		t.Fatalf("flags=%+v", env.Data)
	}
	if env.Data.SessionsRevoked < 1 {
		t.Fatalf("sessions_revoked=%d", env.Data.SessionsRevoked)
	}

	// Plaintext must not be persisted in DB columns or audit JSON.
	var storedHash string
	var mustChange bool
	if err := pool.QueryRow(ctx, `
		SELECT password_hash, must_change_password FROM profile WHERE id=$1`, learnerID).
		Scan(&storedHash, &mustChange); err != nil {
		t.Fatal(err)
	}
	if storedHash == "" || storedHash == plain || strings.Contains(storedHash, plain) {
		t.Fatalf("DB must store bcrypt only, got %q", storedHash)
	}
	if !mustChange {
		t.Fatal("must_change_password should be true")
	}
	if !auth.CheckPassword(storedHash, plain) {
		t.Fatal("temp password must verify against stored hash")
	}
	if auth.CheckPassword(storedHash, "original-pass") {
		t.Fatal("old password must be replaced")
	}

	var auditBlob string
	if err := pool.QueryRow(ctx, `
		SELECT COALESCE(after_json::text, '') FROM admin_audit_log
		WHERE action='users.temporary_password' AND entity_id=$1
		ORDER BY created_at DESC LIMIT 1`, learnerID.String()).Scan(&auditBlob); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(auditBlob, plain) || strings.Contains(auditBlob, storedHash) {
		t.Fatalf("audit must not contain password material: %s", auditBlob)
	}
	if strings.Contains(w.Body.String(), "password_hash") {
		t.Fatal("API response must not include password_hash")
	}

	var active int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*)::int FROM refresh_token
		WHERE profile_id=$1 AND revoked_at IS NULL`, learnerID).Scan(&active); err != nil {
		t.Fatal(err)
	}
	if active != 0 {
		t.Fatalf("active sessions=%d want 0", active)
	}

	// Learner can login with temporary password and sees must_change.
	learnerAuth := auth.NewService(
		sqlc.New(pool), pool, auth.Limiter{R: redisx.NewTest(t)}, auth.DisabledSender{},
		[]byte("learner-test-secret"), "test",
	)
	login, err := learnerAuth.Login(ctx, auth.LoginInput{Phone: learnerPhone, Password: plain})
	if err != nil {
		t.Fatalf("temp login: %v", err)
	}
	if !login.Profile.MustChangePassword {
		t.Fatal("login profile must_change_password=true")
	}

	t.Run("permission_gated", func(t *testing.T) {
		editorID := uuid.New()
		hash, err := auth.HashPassword("password123")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO admin_user (id, email, display_name, password_hash, status)
			 VALUES ($1, $2, 'Ed', $3, 'active')`,
			editorID, "editor-temp@example.uz", hash); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO admin_user_role (admin_user_id, role_id)
			 SELECT $1, id FROM admin_role WHERE code = 'editor'`, editorID); err != nil {
			t.Fatal(err)
		}
		edAccess := loginAccess(t, r, "editor-temp@example.uz", "password123")
		req := httptest.NewRequest(http.MethodPost, "/admin/v1/users/"+learnerID.String()+"/temporary-password", nil)
		req.Header.Set("Authorization", "Bearer "+edAccess)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("status=%d want 403", w.Code)
		}
	})
}

func TestTemporaryPasswordLoginAndChange(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	ctx := context.Background()
	q := sqlc.New(pool)
	store := Store{Pool: pool}
	secret := []byte("test-admin-secret-at-least-32-bytes!!")
	if _, err := store.EnsureSuperadmin(ctx, "ops2@example.uz", "password123", "Ops"); err != nil {
		t.Fatal(err)
	}
	h := &Handler{Svc: Service{Store: store, Secret: secret}, Pool: pool, Secret: secret}
	r := chi.NewRouter()
	r.Route("/admin/v1", h.Routes)
	access := loginAccess(t, r, "ops2@example.uz", "password123")

	hash, err := auth.HashPassword("before-temp")
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO profile (id, phone, name, password_hash)
		VALUES ($1, '+998909998877', 'T', $2)`, id, hash); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/v1/users/"+id.String()+"/temporary-password", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var env struct {
		Data struct {
			TemporaryPassword string `json:"temporary_password"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &env)

	learnerAuth := auth.NewService(
		q, pool, auth.Limiter{R: redisx.NewTest(t)}, auth.DisabledSender{},
		[]byte("learner-test-secret"), "test",
	)
	login, err := learnerAuth.Login(ctx, auth.LoginInput{Phone: "+998909998877", Password: env.Data.TemporaryPassword})
	if err != nil {
		t.Fatal(err)
	}
	if !login.Profile.MustChangePassword {
		t.Fatal("expected must_change after temp password")
	}

	newHash, err := auth.HashPassword("freshpass1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.SetProfilePassword(ctx, sqlc.SetProfilePasswordParams{
		ID:                 id,
		PasswordHash:       pgtype.Text{String: newHash, Valid: true},
		MustChangePassword: false,
	}); err != nil {
		t.Fatal(err)
	}
	if err := q.CreateRefreshToken(ctx, sqlc.CreateRefreshTokenParams{
		ProfileID: id,
		TokenHash: "should-be-revoked-later",
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true},
	}); err != nil {
		t.Fatal(err)
	}

	// Wrong password still fails.
	if _, err := learnerAuth.Login(ctx, auth.LoginInput{Phone: "+998909998877", Password: "wrong-pass"}); !errors.Is(err, auth.ErrInvalidCreds) {
		t.Fatalf("wrong password err=%v", err)
	}
	okLogin, err := learnerAuth.Login(ctx, auth.LoginInput{Phone: "+998909998877", Password: "freshpass1"})
	if err != nil {
		t.Fatal(err)
	}
	if okLogin.Profile.MustChangePassword {
		t.Fatal("must_change should be false after set")
	}

	// Ensure response body from admin never logged plaintext into after_json.
	var n int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*)::int FROM admin_audit_log
		WHERE action='users.temporary_password'
		  AND (after_json::text LIKE '%' || $1 || '%' OR before_json::text LIKE '%' || $1 || '%')`,
		env.Data.TemporaryPassword).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatal("plaintext temporary password leaked into audit log")
	}
}

func TestIssueTemporaryPasswordUnauthorized(t *testing.T) {
	pool := testdb.New(t)
	store := Store{Pool: pool}
	secret := []byte("test-admin-secret-at-least-32-bytes!!")
	h := &Handler{Svc: Service{Store: store, Secret: secret}, Pool: pool, Secret: secret}
	r := chi.NewRouter()
	r.Route("/admin/v1", h.Routes)

	req := httptest.NewRequest(http.MethodPost, "/admin/v1/users/"+uuid.New().String()+"/temporary-password", bytes.NewReader(nil))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", w.Code)
	}
}
