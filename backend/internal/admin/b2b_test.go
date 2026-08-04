package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/b2b"
	"avtotest.uz/backend/internal/testdb"
)

func TestAdminB2B(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := Store{Pool: pool}
	secret := []byte("test-admin-secret-at-least-32-bytes!!")

	if _, err := store.EnsureSuperadmin(t.Context(), "ops@example.uz", "password123", "Ops"); err != nil {
		t.Fatal(err)
	}

	profileID := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO profile (id, phone, name) VALUES ($1, $2, $3)`,
		profileID, "+998903334455", "Student"); err != nil {
		t.Fatal(err)
	}

	h := &Handler{Svc: Service{Store: store, Secret: secret}, Pool: pool, Secret: secret}
	r := chi.NewRouter()
	r.Route("/admin/v1", h.Routes)
	access := loginAccess(t, r, "ops@example.uz", "password123")

	var orgID uuid.UUID
	t.Run("create org license member grant", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/admin/v1/b2b/orgs",
			bytes.NewBufferString(`{"name":"Avtomaktab Demo"}`))
		req.Header.Set("Authorization", "Bearer "+access)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("create org status=%d body=%s", w.Code, w.Body.String())
		}
		var orgEnv struct {
			Data B2BOrgRow `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &orgEnv); err != nil {
			t.Fatal(err)
		}
		orgID = orgEnv.Data.ID

		req = httptest.NewRequest(http.MethodPost, "/admin/v1/b2b/orgs/"+orgID.String()+"/licenses",
			bytes.NewBufferString(`{"seats":2,"home_seats":2,"days":30,"note":"pilot"}`))
		req.Header.Set("Authorization", "Bearer "+access)
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("license status=%d body=%s", w.Code, w.Body.String())
		}

		req = httptest.NewRequest(http.MethodPost, "/admin/v1/b2b/orgs/"+orgID.String()+"/members",
			bytes.NewBufferString(`{"profile_id":"`+profileID.String()+`","role":"student"}`))
		req.Header.Set("Authorization", "Bearer "+access)
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("member status=%d body=%s", w.Code, w.Body.String())
		}

		req = httptest.NewRequest(http.MethodPost,
			"/admin/v1/b2b/orgs/"+orgID.String()+"/members/"+profileID.String()+"/grant",
			bytes.NewBufferString(`{"days":14,"note":"class A"}`))
		req.Header.Set("Authorization", "Bearer "+access)
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("grant status=%d body=%s", w.Code, w.Body.String())
		}

		var source string
		if err := pool.QueryRow(context.Background(),
			`SELECT source FROM entitlement WHERE profile_id=$1 ORDER BY created_at DESC LIMIT 1`,
			profileID).Scan(&source); err != nil {
			t.Fatal(err)
		}
		if source != "b2b" {
			t.Fatalf("source=%s", source)
		}

		req = httptest.NewRequest(http.MethodGet, "/admin/v1/b2b/orgs/"+orgID.String(), nil)
		req.Header.Set("Authorization", "Bearer "+access)
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("detail status=%d", w.Code)
		}
		var detailEnv struct {
			Data B2BOrgDetail `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &detailEnv); err != nil {
			t.Fatal(err)
		}
		if len(detailEnv.Data.Members) != 1 || detailEnv.Data.Org.Seats < 2 || detailEnv.Data.HomeSeatsUsed < 1 {
			t.Fatalf("detail=%+v", detailEnv.Data)
		}

		req = httptest.NewRequest(http.MethodGet, "/admin/v1/b2b/orgs/"+orgID.String()+"/stats", nil)
		req.Header.Set("Authorization", "Bearer "+access)
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("stats status=%d body=%s", w.Code, w.Body.String())
		}

		req = httptest.NewRequest(http.MethodGet, "/admin/v1/b2b/orgs/"+orgID.String()+"/export.csv", nil)
		req.Header.Set("Authorization", "Bearer "+access)
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("csv status=%d", w.Code)
		}

		req = httptest.NewRequest(http.MethodPatch,
			"/admin/v1/b2b/orgs/"+orgID.String()+"/members/"+profileID.String(),
			bytes.NewBufferString(`{"role":"teacher"}`))
		req.Header.Set("Authorization", "Bearer "+access)
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("role status=%d body=%s", w.Code, w.Body.String())
		}

		req = httptest.NewRequest(http.MethodPost, "/admin/v1/b2b/orgs/"+orgID.String()+"/invites",
			bytes.NewBufferString(`{"phone":"+998909998877","role":"student"}`))
		req.Header.Set("Authorization", "Bearer "+access)
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("invite status=%d body=%s", w.Code, w.Body.String())
		}

		req = httptest.NewRequest(http.MethodDelete,
			"/admin/v1/b2b/orgs/"+orgID.String()+"/members/"+profileID.String(), nil)
		req.Header.Set("Authorization", "Bearer "+access)
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("remove status=%d body=%s", w.Code, w.Body.String())
		}
	})
}

func TestAdminB2BHardDelete(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	ctx := context.Background()
	store := Store{Pool: pool}
	secret := []byte("test-admin-secret-at-least-32-bytes!!")
	adminID, err := store.EnsureSuperadmin(ctx, "b2b-delete@example.uz", "password123", "B2B Delete")
	if err != nil {
		t.Fatal(err)
	}
	h := &Handler{Svc: Service{Store: store, Secret: secret}, Pool: pool, Secret: secret}
	r := chi.NewRouter()
	r.Route("/admin/v1", h.Routes)
	access := loginAccess(t, r, "b2b-delete@example.uz", "password123")

	memberID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO profile (id, phone, name) VALUES ($1, '+998907770001', 'School Student')`, memberID); err != nil {
		t.Fatal(err)
	}
	org, err := store.CreateB2BOrg(ctx, "Delete School")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddB2BMember(ctx, org.ID, memberID, "student"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateB2BLicense(ctx, org.ID, 2, 30, 1, "delete test", "test"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO b2b_invite (token, org_id, phone, role, expires_at, created_by)
		VALUES ('delete-org-invite', $1, '+998907770002', 'student', now()+interval '7 days', $2)`, org.ID, memberID); err != nil {
		t.Fatal(err)
	}
	b2bStore := b2b.Store{Pool: pool}
	if _, err := pool.Exec(ctx, `
		INSERT INTO b2b_station (org_id, label, status, activated_by, public_key, hwid_hash)
		VALUES ($1, 'Class-1', 'active', 'test', $2, 'delete-school-hwid')`,
		org.ID, []byte("delete-school-pubkey")); err != nil {
		t.Fatal(err)
	}
	promo, err := b2bStore.CreatePartnerPromo(ctx, org.ID, "DELETE20", "percent", 20, 90)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO entitlement (profile_id, source, starts_at, ends_at, note)
		VALUES
		  ($1, 'b2b', now(), now()+interval '30 days', $2),
		  ($1, 'admin', now(), now()+interval '60 days', 'keep unrelated VIP')`,
		memberID, "b2b_org="+org.ID.String()+"; kind=home; delete test"); err != nil {
		t.Fatal(err)
	}

	var tariffID uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO tariff (code, days, price_uzs) VALUES ('DELETE-ORG-TARIFF', 30, 100000)
		RETURNING id`).Scan(&tariffID); err != nil {
		t.Fatal(err)
	}
	var paymentID uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO payment
		  (profile_id, tariff_id, amount_uzs, provider, status, idempotency_key, promo_code_id,
		   tariff_days_snapshot, tariff_price_uzs_snapshot)
		VALUES ($1, $2, 80000, 'sandbox', 'paid', 'delete-org-payment', $3, 30, 100000)
		RETURNING id`, memberID, tariffID, promo.ID).Scan(&paymentID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO promo_redemption (promo_code_id, profile_id, payment_id)
		VALUES ($1, $2, $3)`, promo.ID, memberID, paymentID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		WITH c AS (
		  INSERT INTO category (code) VALUES ('ORG_DELETE_KEEP_CATEGORY') RETURNING id
		)
		INSERT INTO question (source_ext_id, category_id, content_hash, source)
		SELECT 'org-delete-keep-question', id, 'org-delete-keep-hash', 'test' FROM c;
		INSERT INTO variant (number) VALUES (910002)`); err != nil {
		t.Fatal(err)
	}
	var questionsBefore, variantsBefore int
	if err := pool.QueryRow(ctx, `SELECT (SELECT COUNT(*) FROM question)::int, (SELECT COUNT(*) FROM variant)::int`).Scan(
		&questionsBefore, &variantsBefore,
	); err != nil {
		t.Fatal(err)
	}

	limitedID := uuid.New()
	limitedHash, err := auth.HashPassword("password123")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO admin_user (id, email, display_name, password_hash, status)
		VALUES ($1, 'b2b-limited@example.uz', 'Limited', $2, 'active')`, limitedID, limitedHash); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO admin_user_role (admin_user_id, role_id)
		SELECT $1, id FROM admin_role WHERE code='admin'`, limitedID); err != nil {
		t.Fatal(err)
	}
	limitedAccess := loginAccess(t, r, "b2b-limited@example.uz", "password123")
	req := httptest.NewRequest(http.MethodDelete, "/admin/v1/b2b/orgs/"+org.ID.String(),
		bytes.NewBufferString(`{"confirm":"Delete School"}`))
	req.Header.Set("Authorization", "Bearer "+limitedAccess)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("permission status=%d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/admin/v1/b2b/orgs/"+org.ID.String(),
		bytes.NewBufferString(`{"confirm":"wrong"}`))
	req.Header.Set("Authorization", "Bearer "+access)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("confirmation status=%d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/admin/v1/b2b/orgs/"+org.ID.String(),
		bytes.NewBufferString(`{"confirm":"Delete School"}`))
	req.Header.Set("Authorization", "Bearer "+access)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("delete status=%d body=%s", w.Code, w.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/admin/v1/b2b/orgs/"+org.ID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+access)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("detail after delete status=%d body=%s", w.Code, w.Body.String())
	}

	for table, query := range map[string]string{
		"org":           `SELECT COUNT(*)::int FROM b2b_org WHERE id=$1`,
		"member":        `SELECT COUNT(*)::int FROM b2b_org_member WHERE org_id=$1`,
		"license":       `SELECT COUNT(*)::int FROM b2b_org_license WHERE org_id=$1`,
		"invite":        `SELECT COUNT(*)::int FROM b2b_invite WHERE org_id=$1`,
		"station":       `SELECT COUNT(*)::int FROM b2b_station WHERE org_id=$1`,
		"partner_promo": `SELECT COUNT(*)::int FROM promo_code WHERE partner_org_id=$1`,
	} {
		var n int
		if err := pool.QueryRow(ctx, query, org.ID).Scan(&n); err != nil {
			t.Fatalf("%s count: %v", table, err)
		}
		if n != 0 {
			t.Fatalf("%s rows after hard delete=%d", table, n)
		}
	}
	var profileCount, entitlementCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM profile WHERE id=$1`, memberID).Scan(&profileCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM entitlement WHERE profile_id=$1`, memberID).Scan(&entitlementCount); err != nil {
		t.Fatal(err)
	}
	if profileCount != 1 || entitlementCount != 1 {
		t.Fatalf("member profile=%d entitlements=%d, want profile and unrelated VIP preserved", profileCount, entitlementCount)
	}
	var keptPromoID *uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT promo_code_id FROM payment WHERE id=$1`, paymentID).Scan(&keptPromoID); err != nil {
		t.Fatalf("payment must be preserved: %v", err)
	}
	if keptPromoID != nil {
		t.Fatalf("payment promo reference not detached: %v", keptPromoID)
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
	rows, err := store.ListB2BOrgs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.ID == org.ID {
			t.Fatal("deleted org still present in list")
		}
	}
	var auditCount int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*)::int FROM admin_audit_log
		WHERE action='b2b.orgs.hard_delete' AND admin_user_id=$1 AND entity_id=$2
		  AND before_json->>'name'='Delete School'`, adminID, org.ID.String()).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 {
		t.Fatalf("attributed org hard-delete audit rows=%d", auditCount)
	}
}
