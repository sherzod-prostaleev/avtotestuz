package account_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/db/sqlc"
)

// seedTariff inserts a tariff row directly: sqlc has no CreateTariff query
// (tariffs are seed data, not written via the API), mirroring the pattern
// billing/payme's methods_test.go already uses for the same reason.
func seedTariff(t *testing.T, pool *pgxpool.Pool, code string, days int) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO tariff (id, code, days, price_uzs, sort_order, active) VALUES ($1, $2, $3, 59900, 1, true)`,
		id, code, days); err != nil {
		t.Fatalf("seed tariff: %v", err)
	}
	return id
}

func seedTariffTranslation(t *testing.T, pool *pgxpool.Pool, tariffID uuid.UUID, locale, name string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO tariff_translation (tariff_id, locale, name, description) VALUES ($1, $2, $3, '')`,
		tariffID, locale, name); err != nil {
		t.Fatalf("seed tariff_translation: %v", err)
	}
}

// seedPayment inserts a payment row directly so created_at/paid_at can be
// controlled precisely (ordering/nullability assertions need this; the
// CreatePayment sqlc query always defaults status='created' and leaves
// created_at/paid_at to the DB).
func seedPayment(t *testing.T, pool *pgxpool.Pool, profileID, tariffID uuid.UUID, amountUZS int64, status string, createdAt time.Time, paidAt *time.Time) uuid.UUID {
	t.Helper()
	id := uuid.New()
	var paid pgtype.Timestamptz
	if paidAt != nil {
		paid = pgtype.Timestamptz{Time: *paidAt, Valid: true}
	}
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO payment (id, profile_id, tariff_id, amount_uzs, provider, status, idempotency_key, created_at, paid_at)
		 VALUES ($1, $2, $3, $4, 'payme', $5, $6, $7, $8)`,
		id, profileID, tariffID, amountUZS, status, uuid.New().String(), createdAt, paid); err != nil {
		t.Fatalf("seed payment: %v", err)
	}
	return id
}

// TestListMyPaymentsOrderedWithTranslation covers brief cases 1 and 7:
// newest-first ordering, tariff_name resolved from a matching-locale
// translation, and paid_at being a real timestamp for a paid payment vs
// JSON null for a non-paid one.
func TestListMyPaymentsOrderedWithTranslation(t *testing.T) {
	ts, profile, pool := setup(t)
	tok, err := auth.IssueAccess([]byte(testSecret), profile.ID, "user", time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	tariffA := seedTariff(t, pool, "tarif-a", 7)
	seedTariffTranslation(t, pool, tariffA, "uz-Latn", "TarifA")
	tariffB := seedTariff(t, pool, "tarif-b", 30)
	seedTariffTranslation(t, pool, tariffB, "uz-Latn", "TarifB")

	now := time.Now().UTC().Truncate(time.Second)
	older := now.Add(-2 * time.Hour)
	paidAt := now
	seedPayment(t, pool, profile.ID, tariffA, 24900, "failed", older, nil)
	seedPayment(t, pool, profile.ID, tariffB, 59900, "paid", now, &paidAt)

	status, env := doReq(t, ts, http.MethodGet, "/me/payments", tok, nil)
	if status != http.StatusOK {
		t.Fatalf("status=%d want 200, body=%s", status, env.Data)
	}
	var out []struct {
		TariffCode string     `json:"tariff_code"`
		TariffName string     `json:"tariff_name"`
		Status     string     `json:"status"`
		PaidAt     *time.Time `json:"paid_at"`
	}
	if err := json.Unmarshal(env.Data, &out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("got %d payments, want 2", len(out))
	}
	if out[0].TariffCode != "tarif-b" || out[1].TariffCode != "tarif-a" {
		t.Fatalf("wrong order: %s, %s (want tarif-b then tarif-a)", out[0].TariffCode, out[1].TariffCode)
	}
	if out[0].TariffName != "TarifB" || out[1].TariffName != "TarifA" {
		t.Fatalf("wrong tariff_name: %s, %s", out[0].TariffName, out[1].TariffName)
	}
	if out[0].PaidAt == nil {
		t.Fatal("paid payment: paid_at = nil, want a timestamp")
	}
	if !out[0].PaidAt.Equal(paidAt) {
		t.Fatalf("paid_at = %v, want %v", out[0].PaidAt, paidAt)
	}
	if out[1].PaidAt != nil {
		t.Fatalf("failed payment: paid_at = %v, want nil", out[1].PaidAt)
	}
}

// TestListMyPaymentsFallsBackToUzLatn covers brief case 2: a tariff with no
// translation for the requested locale falls back to its uz-Latn name.
func TestListMyPaymentsFallsBackToUzLatn(t *testing.T) {
	ts, profile, pool := setup(t)
	tok, err := auth.IssueAccess([]byte(testSecret), profile.ID, "user", time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	tariffC := seedTariff(t, pool, "tarif-c", 30)
	seedTariffTranslation(t, pool, tariffC, "uz-Latn", "TarifC-UZ")
	seedPayment(t, pool, profile.ID, tariffC, 59900, "paid", time.Now().UTC(), nil)

	status, env := doReq(t, ts, http.MethodGet, "/me/payments?locale=ru", tok, nil)
	if status != http.StatusOK {
		t.Fatalf("status=%d want 200, body=%s", status, env.Data)
	}
	var out []struct {
		TariffName string `json:"tariff_name"`
	}
	if err := json.Unmarshal(env.Data, &out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("got %d payments, want 1", len(out))
	}
	if out[0].TariffName != "TarifC-UZ" {
		t.Fatalf("tariff_name = %q, want uz-Latn fallback %q", out[0].TariffName, "TarifC-UZ")
	}
}

// TestListMyPaymentsLimit covers brief case 3: ?limit=1 returns only the
// newest payment.
func TestListMyPaymentsLimit(t *testing.T) {
	ts, profile, pool := setup(t)
	tok, err := auth.IssueAccess([]byte(testSecret), profile.ID, "user", time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	tariff := seedTariff(t, pool, "tarif-limit", 30)
	now := time.Now().UTC().Truncate(time.Second)
	seedPayment(t, pool, profile.ID, tariff, 59900, "paid", now.Add(-2*time.Hour), nil)
	seedPayment(t, pool, profile.ID, tariff, 59900, "paid", now.Add(-1*time.Hour), nil)
	seedPayment(t, pool, profile.ID, tariff, 59900, "paid", now, nil)

	status, env := doReq(t, ts, http.MethodGet, "/me/payments?limit=1", tok, nil)
	if status != http.StatusOK {
		t.Fatalf("status=%d want 200, body=%s", status, env.Data)
	}
	var out []struct {
		CreatedAt time.Time `json:"created_at"`
	}
	if err := json.Unmarshal(env.Data, &out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("got %d payments, want 1", len(out))
	}
	if !out[0].CreatedAt.Equal(now) {
		t.Fatalf("returned payment created_at = %v, want newest %v", out[0].CreatedAt, now)
	}
}

// TestListMyPaymentsInvalidLimit covers brief case 4: a non-integer ?limit
// is rejected with 400 invalid_request.
func TestListMyPaymentsInvalidLimit(t *testing.T) {
	ts, profile, _ := setup(t)
	tok, err := auth.IssueAccess([]byte(testSecret), profile.ID, "user", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	status, env := doReq(t, ts, http.MethodGet, "/me/payments?limit=abc", tok, nil)
	if status != http.StatusBadRequest {
		t.Fatalf("status=%d want 400", status)
	}
	if env.Error == nil || env.Error.Code != "invalid_request" {
		t.Fatalf("error = %+v, want code invalid_request", env.Error)
	}
}

// TestListMyPaymentsProfileIsolation covers brief case 5: one profile never
// sees another profile's payments.
func TestListMyPaymentsProfileIsolation(t *testing.T) {
	ts, profile, pool := setup(t)
	tok, err := auth.IssueAccess([]byte(testSecret), profile.ID, "user", time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	q := sqlc.New(pool)
	otherProfile, err := q.CreateProfile(context.Background(), sqlc.CreateProfileParams{
		Phone: "+998907654321",
	})
	if err != nil {
		t.Fatalf("create other profile: %v", err)
	}

	tariff := seedTariff(t, pool, "tarif-iso", 30)
	seedPayment(t, pool, otherProfile.ID, tariff, 59900, "paid", time.Now().UTC(), nil)

	status, env := doReq(t, ts, http.MethodGet, "/me/payments", tok, nil)
	if status != http.StatusOK {
		t.Fatalf("status=%d want 200, body=%s", status, env.Data)
	}
	var out []json.RawMessage
	if err := json.Unmarshal(env.Data, &out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Fatalf("got %d payments, want 0 (other profile's payment must not leak)", len(out))
	}
}

// TestListMyPaymentsRequiresAuth covers brief case 6: no auth header → 401.
func TestListMyPaymentsRequiresAuth(t *testing.T) {
	ts, _, _ := setup(t)
	status, env := doReq(t, ts, http.MethodGet, "/me/payments", "", nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", status)
	}
	if env.Error == nil || env.Error.Code != "unauthorized" {
		t.Fatalf("error = %+v, want code unauthorized", env.Error)
	}
}
