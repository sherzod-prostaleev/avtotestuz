package account_test

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
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"avtotest.uz/backend/internal/account"
	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/redisx"
	"avtotest.uz/backend/internal/testdb"
)

func setupPasswordServer(t *testing.T) (*httptest.Server, *auth.Service, *sqlc.Queries, *pgxpool.Pool) {
	t.Helper()
	pool := testdb.New(t)
	q := sqlc.New(pool)
	c := redisx.NewTest(t)
	svc := auth.NewService(q, pool, auth.Limiter{R: c}, auth.SandboxSender{Log: zap.NewNop()}, []byte(testSecret), "test")

	r := chi.NewRouter()
	h := &account.Handler{Q: q, Billing: billing.Service{Q: q, Pool: pool}}
	authed := r.With(
		auth.Required([]byte(testSecret)),
		auth.RejectBanned(q),
		auth.RequirePasswordChanged(q),
	)
	h.Routes(authed)

	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)
	return ts, svc, q, pool
}

func registerLearner(t *testing.T, svc *auth.Service, phone, password string) auth.VerifyResult {
	t.Helper()
	res, err := svc.Register(context.Background(), auth.RegisterInput{
		Phone: phone, Password: password, Name: "Learner",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	return res
}

func TestPasswordChangeFlow(t *testing.T) {
	ts, svc, q, pool := setupPasswordServer(t)
	ctx := context.Background()
	const phone = "901110001"
	const oldPass = "oldpass12"
	const newPass = "newpass99"

	reg := registerLearner(t, svc, phone, oldPass)

	t.Run("wrong_current_password", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"current_password": "wrongpass",
			"new_password":     newPass,
			"confirm_password": newPass,
		})
		status, env := doReq(t, ts, http.MethodPost, "/me/password", reg.Access, body)
		if status != http.StatusUnauthorized || env.Error == nil || env.Error.Code != "invalid_current_password" {
			t.Fatalf("status=%d env=%+v", status, env)
		}
	})

	t.Run("mismatched_confirmation", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"current_password": oldPass,
			"new_password":     newPass,
			"confirm_password": "different1",
		})
		status, env := doReq(t, ts, http.MethodPost, "/me/password", reg.Access, body)
		if status != http.StatusBadRequest || env.Error == nil || env.Error.Code != "password_mismatch" {
			t.Fatalf("status=%d env=%+v", status, env)
		}
	})

	t.Run("weak_password", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"current_password": oldPass,
			"new_password":     "short",
			"confirm_password": "short",
		})
		status, env := doReq(t, ts, http.MethodPost, "/me/password", reg.Access, body)
		if status != http.StatusBadRequest || env.Error == nil || env.Error.Code != "weak_password" {
			t.Fatalf("status=%d env=%+v", status, env)
		}
	})

	t.Run("successful_change_clears_must_change_and_revokes_sessions", func(t *testing.T) {
		// Force must_change and an extra refresh session.
		if _, err := q.SetProfilePassword(ctx, sqlc.SetProfilePasswordParams{
			ID:                 reg.Profile.ID,
			PasswordHash:       reg.Profile.PasswordHash,
			MustChangePassword: true,
		}); err != nil {
			// PasswordHash may be empty on VerifyResult profile snapshot — reload.
			p, err := q.GetProfileByID(ctx, reg.Profile.ID)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := q.SetProfilePassword(ctx, sqlc.SetProfilePasswordParams{
				ID:                 p.ID,
				PasswordHash:       p.PasswordHash,
				MustChangePassword: true,
			}); err != nil {
				t.Fatal(err)
			}
		}
		if err := q.CreateRefreshToken(ctx, sqlc.CreateRefreshTokenParams{
			ProfileID: reg.Profile.ID,
			TokenHash: "extra-session-hash",
			ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true},
		}); err != nil {
			t.Fatal(err)
		}

		body, _ := json.Marshal(map[string]string{
			"current_password": oldPass,
			"new_password":     newPass,
			"confirm_password": newPass,
		})
		status, env := doReq(t, ts, http.MethodPost, "/me/password", reg.Access, body)
		if status != http.StatusOK {
			t.Fatalf("status=%d env=%+v", status, env)
		}
		if strings.Contains(string(env.Data), oldPass) || strings.Contains(string(env.Data), newPass) {
			t.Fatal("response must not leak passwords")
		}
		var out struct {
			OK                 bool `json:"ok"`
			MustChangePassword bool `json:"must_change_password"`
		}
		if err := json.Unmarshal(env.Data, &out); err != nil {
			t.Fatal(err)
		}
		if !out.OK || out.MustChangePassword {
			t.Fatalf("out=%+v", out)
		}

		p, err := q.GetProfileByID(ctx, reg.Profile.ID)
		if err != nil {
			t.Fatal(err)
		}
		if p.MustChangePassword {
			t.Fatal("must_change_password should be false")
		}
		if auth.CheckPassword(p.PasswordHash.String, oldPass) {
			t.Fatal("old password must no longer work")
		}
		if !auth.CheckPassword(p.PasswordHash.String, newPass) {
			t.Fatal("new password must work")
		}

		var active int
		if err := pool.QueryRow(ctx, `
			SELECT COUNT(*)::int FROM refresh_token
			WHERE profile_id=$1 AND revoked_at IS NULL`, reg.Profile.ID).Scan(&active); err != nil {
			t.Fatal(err)
		}
		if active != 0 {
			t.Fatalf("active sessions=%d want 0", active)
		}
	})

	t.Run("old_password_login_fails_new_works", func(t *testing.T) {
		if _, err := svc.Login(ctx, auth.LoginInput{Phone: phone, Password: oldPass}); !errors.Is(err, auth.ErrInvalidCreds) {
			t.Fatalf("old password login err=%v want ErrInvalidCreds", err)
		}
		login, err := svc.Login(ctx, auth.LoginInput{Phone: phone, Password: newPass})
		if err != nil {
			t.Fatalf("login with new password: %v", err)
		}
		if login.Profile.MustChangePassword {
			t.Fatal("must_change should be clear after change")
		}
	})

	t.Run("idor_other_user_password_unaffected", func(t *testing.T) {
		other := registerLearner(t, svc, "901110002", "otherpass1")
		attacker := registerLearner(t, svc, "901110003", "attacker1")
		body, _ := json.Marshal(map[string]string{
			"current_password": "attacker1",
			"new_password":     "attacker2",
			"confirm_password": "attacker2",
		})
		status, _ := doReq(t, ts, http.MethodPost, "/me/password", attacker.Access, body)
		if status != http.StatusOK {
			t.Fatalf("attacker change status=%d", status)
		}
		// Victim still authenticates with original password — no IDOR path exists.
		if _, err := svc.Login(ctx, auth.LoginInput{Phone: "901110002", Password: "otherpass1"}); err != nil {
			t.Fatalf("victim password changed unexpectedly: %v", err)
		}
		p, err := q.GetProfileByID(ctx, other.Profile.ID)
		if err != nil {
			t.Fatal(err)
		}
		if !auth.CheckPassword(p.PasswordHash.String, "otherpass1") {
			t.Fatal("other user's password must be unchanged")
		}
	})
}

func TestMustChangePasswordGate(t *testing.T) {
	ts, svc, q, _ := setupPasswordServer(t)
	ctx := context.Background()
	reg := registerLearner(t, svc, "901110010", "tempgate1")

	p, err := q.GetProfileByID(ctx, reg.Profile.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.SetProfilePassword(ctx, sqlc.SetProfilePasswordParams{
		ID:                 p.ID,
		PasswordHash:       p.PasswordHash,
		MustChangePassword: true,
	}); err != nil {
		t.Fatal(err)
	}

	// GET /me allowed
	status, env := doReq(t, ts, http.MethodGet, "/me", reg.Access, nil)
	if status != http.StatusOK {
		t.Fatalf("GET /me status=%d", status)
	}
	if !bytes.Contains(env.Data, []byte(`"must_change_password":true`)) {
		t.Fatalf("me payload missing must_change_password: %s", env.Data)
	}

	// Other learner routes blocked
	status, env = doReq(t, ts, http.MethodGet, "/me/entitlement", reg.Access, nil)
	if status != http.StatusForbidden || env.Error == nil || env.Error.Code != "password_change_required" {
		t.Fatalf("gate status=%d env=%+v", status, env)
	}

	// Password change allowed and clears gate
	body, _ := json.Marshal(map[string]string{
		"current_password": "tempgate1",
		"new_password":     "permanen1",
		"confirm_password": "permanen1",
	})
	status, _ = doReq(t, ts, http.MethodPost, "/me/password", reg.Access, body)
	if status != http.StatusOK {
		t.Fatalf("change status=%d", status)
	}
	status, env = doReq(t, ts, http.MethodGet, "/me/entitlement", reg.Access, nil)
	if status != http.StatusOK {
		t.Fatalf("after change entitlement status=%d env=%+v", status, env)
	}
}

func TestChangePasswordRequiresAuth(t *testing.T) {
	ts, _, _, _ := setupPasswordServer(t)
	body, _ := json.Marshal(map[string]string{
		"current_password": "x",
		"new_password":     "abcdefgh",
		"confirm_password": "abcdefgh",
	})
	status, _ := doReq(t, ts, http.MethodPost, "/me/password", "", body)
	if status != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", status)
	}
	// Random UUID in token belonging to nobody still can't target another id via body.
	tok, err := auth.IssueAccess([]byte(testSecret), uuid.New(), "user", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	status, _ = doReq(t, ts, http.MethodPost, "/me/password", tok, body)
	if status == http.StatusOK {
		t.Fatal("unknown profile must not succeed")
	}
}
