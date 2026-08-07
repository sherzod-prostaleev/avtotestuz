package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/redisx"
	"avtotest.uz/backend/internal/testdb"
)

func TestAdminUsersListDetailBlockSessions(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := Store{Pool: pool}
	secret := []byte("test-admin-secret-at-least-32-bytes!!")

	adminID, err := store.EnsureSuperadmin(t.Context(), "ops@example.uz", "password123", "Ops")
	if err != nil {
		t.Fatal(err)
	}
	_ = adminID

	profileID := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO profile (id, phone, name, referral_code, status)
		 VALUES ($1, $2, $3, $4, 'active')`,
		profileID, "+998901112233", "Ali Valiyev", "ALI123"); err != nil {
		t.Fatal(err)
	}
	sessionID := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO refresh_token (id, profile_id, token_hash, expires_at)
		 VALUES ($1, $2, $3, $4)`,
		sessionID, profileID, "hash-active-1", time.Now().Add(24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO streak (profile_id, current, best, last_active_date, daily_goal, today_done)
		 VALUES ($1, 5, 10, CURRENT_DATE, 10, 3)`, profileID); err != nil {
		t.Fatal(err)
	}

	svc := Service{Store: store, Secret: secret}
	h := &Handler{Svc: svc, Pool: pool, Secret: secret}
	r := chi.NewRouter()
	r.Route("/admin/v1", h.Routes)

	access := loginAccess(t, r, "ops@example.uz", "password123")

	t.Run("list search by phone name", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/v1/users?q=Ali&page=1&limit=20", nil)
		req.Header.Set("Authorization", "Bearer "+access)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		var env struct {
			Data ListLearnersResult `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		if env.Data.Total != 1 || len(env.Data.Items) != 1 {
			t.Fatalf("got %+v", env.Data)
		}
		if env.Data.Items[0].Phone != "+998901112233" {
			t.Fatalf("list must show full phone for staff, got %q", env.Data.Items[0].Phone)
		}
		if env.Data.Items[0].PhoneMasked == "+998901112233" {
			t.Fatal("phone_masked must still be masked")
		}
		if env.Data.Items[0].Streak != 5 || env.Data.Items[0].Status != "active" {
			t.Fatalf("row=%+v", env.Data.Items[0])
		}
	})

	t.Run("detail", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/v1/users/"+profileID.String(), nil)
		req.Header.Set("Authorization", "Bearer "+access)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		var env struct {
			Data LearnerDetail `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		if env.Data.Phone != "+998901112233" || env.Data.ReferralCode != "ALI123" {
			t.Fatalf("detail=%+v", env.Data)
		}
		raw := w.Body.String()
		if containsStr(raw, "password_hash") || containsStr(raw, `"password":`) {
			t.Fatal("detail must never expose password or hash")
		}
	})

	t.Run("grant vip days", func(t *testing.T) {
		body := bytes.NewReader([]byte(`{"days":30,"note":"promo"}`))
		req := httptest.NewRequest(http.MethodPost, "/admin/v1/users/"+profileID.String()+"/grant", body)
		req.Header.Set("Authorization", "Bearer "+access)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		var env struct {
			Data struct {
				User  LearnerDetail `json:"user"`
				Days  int           `json:"days"`
				Until string        `json:"until"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		if !env.Data.User.VIPActive || env.Data.Days != 30 || env.Data.Until == "" {
			t.Fatalf("grant=%+v", env.Data)
		}
		if len(env.Data.User.Entitlements) < 1 {
			t.Fatal("expected entitlement history")
		}
	})

	t.Run("sessions list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/v1/users/"+profileID.String()+"/sessions", nil)
		req.Header.Set("Authorization", "Bearer "+access)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		var env struct {
			Data []LearnerSessionRow `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		if len(env.Data) != 1 || !env.Data[0].Active {
			t.Fatalf("sessions=%+v", env.Data)
		}
	})

	t.Run("block requires reason", func(t *testing.T) {
		body := bytes.NewReader([]byte(`{"reason":""}`))
		req := httptest.NewRequest(http.MethodPost, "/admin/v1/users/"+profileID.String()+"/block", body)
		req.Header.Set("Authorization", "Bearer "+access)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status=%d want 400", w.Code)
		}
	})

	t.Run("block and revoke sessions", func(t *testing.T) {
		body := bytes.NewReader([]byte(`{"reason":"abuse"}`))
		req := httptest.NewRequest(http.MethodPost, "/admin/v1/users/"+profileID.String()+"/block", body)
		req.Header.Set("Authorization", "Bearer "+access)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		var status string
		if err := pool.QueryRow(context.Background(),
			`SELECT status FROM profile WHERE id = $1`, profileID).Scan(&status); err != nil {
			t.Fatal(err)
		}
		if status != "banned" {
			t.Fatalf("status=%q want banned", status)
		}
		var active int
		if err := pool.QueryRow(context.Background(),
			`SELECT COUNT(*)::int FROM refresh_token WHERE profile_id = $1 AND revoked_at IS NULL`,
			profileID).Scan(&active); err != nil {
			t.Fatal(err)
		}
		if active != 0 {
			t.Fatalf("active sessions=%d", active)
		}
		var auditCount int
		if err := pool.QueryRow(context.Background(),
			`SELECT COUNT(*)::int FROM admin_audit_log WHERE action = 'users.block' AND entity_id = $1`,
			profileID.String()).Scan(&auditCount); err != nil {
			t.Fatal(err)
		}
		if auditCount < 1 {
			t.Fatal("expected audit_log for block")
		}
	})

	t.Run("unblock", func(t *testing.T) {
		body := bytes.NewReader([]byte(`{"reason":"appeal accepted"}`))
		req := httptest.NewRequest(http.MethodPost, "/admin/v1/users/"+profileID.String()+"/unblock", body)
		req.Header.Set("Authorization", "Bearer "+access)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		var status string
		if err := pool.QueryRow(context.Background(),
			`SELECT status FROM profile WHERE id = $1`, profileID).Scan(&status); err != nil {
			t.Fatal(err)
		}
		if status != "active" {
			t.Fatalf("status=%q", status)
		}
	})

	t.Run("revoke single session", func(t *testing.T) {
		sid := uuid.New()
		if _, err := pool.Exec(context.Background(),
			`INSERT INTO refresh_token (id, profile_id, token_hash, expires_at)
			 VALUES ($1, $2, $3, $4)`,
			sid, profileID, "hash-active-2", time.Now().Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost,
			"/admin/v1/users/"+profileID.String()+"/sessions/"+sid.String()+"/revoke", nil)
		req.Header.Set("Authorization", "Bearer "+access)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("permission gated without users.read", func(t *testing.T) {
		// Create support-less admin: editor role only
		editorID := uuid.New()
		hash, err := auth.HashPassword("password123")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(context.Background(),
			`INSERT INTO admin_user (id, email, display_name, password_hash, status)
			 VALUES ($1, $2, 'Ed', $3, 'active')`,
			editorID, "editor@example.uz", hash); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(context.Background(),
			`INSERT INTO admin_user_role (admin_user_id, role_id)
			 SELECT $1, id FROM admin_role WHERE code = 'editor'`, editorID); err != nil {
			t.Fatal(err)
		}
		edAccess := loginAccess(t, r, "editor@example.uz", "password123")
		req := httptest.NewRequest(http.MethodGet, "/admin/v1/users", nil)
		req.Header.Set("Authorization", "Bearer "+edAccess)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("status=%d want 403", w.Code)
		}
	})
}

func TestAdminUserHardDelete(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	ctx := context.Background()
	store := Store{Pool: pool}
	secret := []byte("test-admin-secret-at-least-32-bytes!!")

	adminID, err := store.EnsureSuperadmin(ctx, "delete@example.uz", "password123", "Delete Ops")
	if err != nil {
		t.Fatal(err)
	}
	h := &Handler{Svc: Service{Store: store, Secret: secret}, Pool: pool, Secret: secret}
	r := chi.NewRouter()
	r.Route("/admin/v1", h.Routes)
	access := loginAccess(t, r, "delete@example.uz", "password123")

	// The learner and staff namespaces are separate, but matching UUIDs are
	// still rejected so an accidentally linked current-admin profile is safe.
	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone, name) VALUES ($1, '+998900000001', 'Current admin profile')`, adminID); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodDelete, "/admin/v1/users/"+adminID.String(),
		bytes.NewBufferString(`{"confirm":"DELETE"}`))
	req.Header.Set("Authorization", "Bearer "+access)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("self delete status=%d body=%s", w.Code, w.Body.String())
	}

	passwordHash, err := auth.HashPassword("learner-password-123")
	if err != nil {
		t.Fatal(err)
	}
	targetID := uuid.New()
	const targetPhone = "+998901234500"
	if _, err := pool.Exec(ctx, `
		INSERT INTO profile (id, phone, name, password_hash)
		VALUES ($1, $2, 'Delete Me', $3)`, targetID, targetPhone, passwordHash); err != nil {
		t.Fatal(err)
	}
	otherID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO profile (id, phone, name, referred_by)
		VALUES ($1, '+998901234501', 'Keep Me', $2)`, otherID, targetID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO refresh_token (profile_id, token_hash, expires_at)
		VALUES ($1, 'hard-delete-refresh', now() + interval '1 day')`, targetID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO event (profile_id, name) VALUES ($1, 'hard_delete_test')`, targetID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO entitlement (profile_id, source, starts_at, ends_at, created_by, note)
		VALUES ($1, 'admin', now(), now() + interval '40 days', $2, 'unrelated keep')`, otherID, targetID); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range []struct {
		query string
		args  []any
	}{
		{
			`INSERT INTO otp_challenge (phone, code_hash, channel, expires_at)
			 VALUES ($1, 'delete-otp', 'sandbox', now()+interval '10 minutes')`,
			[]any{targetPhone},
		},
		{
			`INSERT INTO support_ticket (profile_id, contact_phone, subject, body)
			 VALUES ($1, $2, 'linked', 'delete linked support')`,
			[]any{targetID, targetPhone},
		},
		{
			`INSERT INTO support_ticket (contact_phone, subject, body, source)
			 VALUES ($1, 'public', 'delete phone-bound support', 'public')`,
			[]any{targetPhone},
		},
		{
			`INSERT INTO audit_log (action, entity, entity_id)
			 VALUES ('profile.updated', 'profile', $1)`,
			[]any{targetID},
		},
	} {
		if _, err := pool.Exec(ctx, fixture.query, fixture.args...); err != nil {
			t.Fatal(err)
		}
	}

	// Keep representative production content in the database and prove that
	// privacy deletion never reaches questions or variants.
	if _, err := pool.Exec(ctx, `
		WITH c AS (
		  INSERT INTO category (code) VALUES ('HARD_DELETE_KEEP_CATEGORY') RETURNING id
		)
		INSERT INTO question (source_ext_id, category_id, content_hash, source)
		SELECT 'hard-delete-keep-question', id, 'hard-delete-keep-hash', 'test' FROM c;
		INSERT INTO variant (number) VALUES (910001)`); err != nil {
		t.Fatal(err)
	}
	var questionsBefore, variantsBefore int
	if err := pool.QueryRow(ctx, `SELECT (SELECT COUNT(*) FROM question)::int, (SELECT COUNT(*) FROM variant)::int`).Scan(
		&questionsBefore, &variantsBefore,
	); err != nil {
		t.Fatal(err)
	}

	var tariffID uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO tariff (code, days, price_uzs) VALUES ('DELETE-USER-TARIFF', 30, 100000)
		RETURNING id`).Scan(&tariffID); err != nil {
		t.Fatal(err)
	}
	var paymentID uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO payment
		  (profile_id, tariff_id, amount_uzs, provider, status, idempotency_key,
		   tariff_days_snapshot, tariff_price_uzs_snapshot)
		VALUES ($1, $2, 100000, 'sandbox', 'paid', 'hard-delete-payment', 30, 100000)
		RETURNING id`, targetID, tariffID).Scan(&paymentID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO entitlement (profile_id, source, starts_at, ends_at, payment_id, note)
		VALUES ($1, 'purchase', now(), now() + interval '30 days', $2, 'target purchase')`, targetID, paymentID); err != nil {
		t.Fatal(err)
	}

	// An ordinary admin has broad write/grant powers but no destructive right.
	limitedID := uuid.New()
	limitedHash, err := auth.HashPassword("password123")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO admin_user (id, email, display_name, password_hash, status)
		VALUES ($1, 'limited-delete@example.uz', 'Limited', $2, 'active')`, limitedID, limitedHash); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO admin_user_role (admin_user_id, role_id)
		SELECT $1, id FROM admin_role WHERE code = 'admin'`, limitedID); err != nil {
		t.Fatal(err)
	}
	limitedAccess := loginAccess(t, r, "limited-delete@example.uz", "password123")
	req = httptest.NewRequest(http.MethodDelete, "/admin/v1/users/"+targetID.String(),
		bytes.NewBufferString(`{"confirm":"DELETE"}`))
	req.Header.Set("Authorization", "Bearer "+limitedAccess)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("permission status=%d body=%s", w.Code, w.Body.String())
	}

	// Wrong type-to-confirm is rejected without a partial delete or audit.
	req = httptest.NewRequest(http.MethodDelete, "/admin/v1/users/"+targetID.String(),
		bytes.NewBufferString(`{"confirm":"wrong"}`))
	req.Header.Set("Authorization", "Bearer "+access)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("confirmation status=%d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/admin/v1/users/"+targetID.String(),
		bytes.NewBufferString(`{"confirm":"`+targetPhone+`"}`))
	req.Header.Set("Authorization", "Bearer "+access)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("delete status=%d body=%s", w.Code, w.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/admin/v1/users/"+targetID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+access)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("detail after delete status=%d body=%s", w.Code, w.Body.String())
	}

	for table, check := range map[string]struct {
		query string
		args  []any
	}{
		"profile":       {`SELECT COUNT(*)::int FROM profile WHERE id=$1`, []any{targetID}},
		"refresh_token": {`SELECT COUNT(*)::int FROM refresh_token WHERE profile_id=$1`, []any{targetID}},
		"entitlement":   {`SELECT COUNT(*)::int FROM entitlement WHERE profile_id=$1`, []any{targetID}},
		"payment":       {`SELECT COUNT(*)::int FROM payment WHERE profile_id=$1`, []any{targetID}},
		"event":         {`SELECT COUNT(*)::int FROM event WHERE profile_id=$1`, []any{targetID}},
		"otp_challenge": {`SELECT COUNT(*)::int FROM otp_challenge WHERE phone=$1`, []any{targetPhone}},
		"support_ticket": {
			`SELECT COUNT(*)::int FROM support_ticket WHERE profile_id=$1 OR contact_phone=$2`,
			[]any{targetID, targetPhone},
		},
		"learner_audit": {
			`SELECT COUNT(*)::int FROM audit_log WHERE actor_id=$1 OR (entity='profile' AND entity_id=$1)`,
			[]any{targetID},
		},
	} {
		var n int
		if err := pool.QueryRow(ctx, check.query, check.args...).Scan(&n); err != nil {
			t.Fatalf("%s count: %v", table, err)
		}
		if n != 0 {
			t.Fatalf("%s rows after hard delete=%d", table, n)
		}
	}
	var otherEntitlements int
	var createdBy *uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM entitlement WHERE profile_id=$1`, otherID).Scan(&otherEntitlements); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT created_by FROM entitlement WHERE profile_id=$1 LIMIT 1`, otherID).Scan(&createdBy); err != nil {
		t.Fatal(err)
	}
	if otherEntitlements != 1 || createdBy != nil {
		t.Fatalf("unrelated entitlement count=%d created_by=%v", otherEntitlements, createdBy)
	}
	var referredBy *uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT referred_by FROM profile WHERE id=$1`, otherID).Scan(&referredBy); err != nil {
		t.Fatal(err)
	}
	if referredBy != nil {
		t.Fatalf("referred_by still points at deleted profile: %v", referredBy)
	}
	var questionsAfter, variantsAfter int
	if err := pool.QueryRow(ctx, `SELECT (SELECT COUNT(*) FROM question)::int, (SELECT COUNT(*) FROM variant)::int`).Scan(
		&questionsAfter, &variantsAfter,
	); err != nil {
		t.Fatal(err)
	}
	if questionsAfter != questionsBefore || variantsAfter != variantsBefore {
		t.Fatalf("content counts changed: questions %d->%d variants %d->%d",
			questionsBefore, questionsAfter, variantsBefore, variantsAfter)
	}
	var auditCount int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*)::int FROM admin_audit_log
		WHERE action='users.hard_delete' AND admin_user_id=$1 AND entity_id=$2
		  AND before_json->>'phone'=$3`, adminID, targetID.String(), targetPhone).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 {
		t.Fatalf("attributed hard-delete audit rows=%d", auditCount)
	}

	// Credentials are gone, so password login resolves to invalid credentials.
	learnerAuth := auth.NewService(
		sqlc.New(pool), pool, auth.Limiter{R: redisx.NewTest(t)}, auth.DisabledSender{},
		[]byte("learner-test-secret"), "test",
	)
	if _, err := learnerAuth.Login(ctx, auth.LoginInput{Phone: targetPhone, Password: "learner-password-123"}); !errors.Is(err, auth.ErrInvalidCreds) {
		t.Fatalf("login after hard delete err=%v want invalid credentials", err)
	}
}

func loginAccess(t *testing.T, r chi.Router, email, password string) string {
	t.Helper()
	body := bytes.NewReader([]byte(`{"email":"` + email + `","password":"` + password + `"}`))
	req := httptest.NewRequest(http.MethodPost, "/admin/v1/auth/login", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("login status=%d body=%s", w.Code, w.Body.String())
	}
	var env struct {
		Data struct {
			Tokens TokenPair `json:"tokens"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	return env.Data.Tokens.AccessToken
}

func containsStr(s, sub string) bool {
	return bytes.Contains([]byte(s), []byte(sub))
}
