package recon

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/testdb"
)

func TestRunFlagsPaidMissingPerform(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()

	profileID := uuid.New()
	phone := fmt.Sprintf("+9989%08d", int(profileID.ID())%100000000)
	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone) VALUES ($1, $2)`, profileID, phone); err != nil {
		t.Fatal(err)
	}
	tariffID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO tariff (id, code, days, price_uzs, sort_order, active) VALUES ($1, 'gentra', 30, 59900, 1, true)`,
		tariffID); err != nil {
		t.Fatal(err)
	}
	paymentID := uuid.New()
	created := time.Now().UTC().Add(-time.Hour)
	if _, err := pool.Exec(ctx, `
		INSERT INTO payment (id, profile_id, tariff_id, amount_uzs, provider, status, idempotency_key,
		                     tariff_days_snapshot, tariff_price_uzs_snapshot, created_at)
		VALUES ($1, $2, $3, 59900, 'payme', 'paid', $4, 30, 59900, $5)`,
		paymentID, profileID, tariffID, "recon-"+paymentID.String(), created); err != nil {
		t.Fatal(err)
	}

	res, err := Run(ctx, pool, Options{
		From: created.Add(-time.Minute),
		To:   time.Now().UTC().Add(time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.ScannedPay != 1 {
		t.Fatalf("scanned pay=%d", res.ScannedPay)
	}
	found := false
	for _, f := range res.Findings {
		if f.Code == "paid_missing_perform" && f.PaymentID == paymentID {
			found = true
		}
	}
	if !found {
		t.Fatalf("want paid_missing_perform, got %+v", res.Findings)
	}
}

func TestRunCleanPaidWithPerform(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()

	profileID := uuid.New()
	phone := fmt.Sprintf("+9989%08d", int(profileID.ID())%100000000)
	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone) VALUES ($1, $2)`, profileID, phone); err != nil {
		t.Fatal(err)
	}
	tariffID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO tariff (id, code, days, price_uzs, sort_order, active) VALUES ($1, 'gentra', 30, 59900, 1, true)`,
		tariffID); err != nil {
		t.Fatal(err)
	}
	paymentID := uuid.New()
	created := time.Now().UTC().Add(-time.Hour)
	if _, err := pool.Exec(ctx, `
		INSERT INTO payment (id, profile_id, tariff_id, amount_uzs, provider, status, idempotency_key,
		                     tariff_days_snapshot, tariff_price_uzs_snapshot, created_at)
		VALUES ($1, $2, $3, 59900, 'payme', 'paid', $4, 30, 59900, $5)`,
		paymentID, profileID, tariffID, "recon-ok-"+paymentID.String(), created); err != nil {
		t.Fatal(err)
	}
	createMs := created.UnixMilli()
	if _, err := pool.Exec(ctx, `
		INSERT INTO payme_transaction (payme_id, payment_id, amount_tiyin, state, create_time, perform_time)
		VALUES ($1, $2, 5990000, 2, $3, $3)`,
		"payme-recon-1", paymentID, createMs); err != nil {
		t.Fatal(err)
	}

	res, err := Run(ctx, pool, Options{
		From: created.Add(-time.Minute),
		To:   time.Now().UTC().Add(time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range res.Findings {
		if f.Severity == SeverityError {
			t.Fatalf("unexpected error finding: %+v", f)
		}
	}
}

// TestRunFallbackLookupPaymentOutsideWindow covers the batched fallback
// lookup added for PERF-3: a payme_transaction whose create_time is inside
// the recon window but whose payment.created_at is not, so the payment
// never appears in listPayments' result and statusByPayment must fall back
// to fillPaymentStatuses. This is the "found via fallback" clean case.
func TestRunFallbackLookupPaymentOutsideWindow(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()

	profileID := uuid.New()
	phone := fmt.Sprintf("+9989%08d", int(profileID.ID())%100000000)
	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone) VALUES ($1, $2)`, profileID, phone); err != nil {
		t.Fatal(err)
	}
	tariffID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO tariff (id, code, days, price_uzs, sort_order, active) VALUES ($1, 'gentra', 30, 59900, 1, true)`,
		tariffID); err != nil {
		t.Fatal(err)
	}
	paymentID := uuid.New()
	// created_at is well before the recon window below.
	paymentCreated := time.Now().UTC().Add(-48 * time.Hour)
	if _, err := pool.Exec(ctx, `
		INSERT INTO payment (id, profile_id, tariff_id, amount_uzs, provider, status, idempotency_key,
		                     tariff_days_snapshot, tariff_price_uzs_snapshot, created_at)
		VALUES ($1, $2, $3, 59900, 'payme', 'paid', $4, 30, 59900, $5)`,
		paymentID, profileID, tariffID, "recon-fallback-"+paymentID.String(), paymentCreated); err != nil {
		t.Fatal(err)
	}
	// create_time (payme txn clock) is inside the window even though
	// payment.created_at is not.
	txnCreateMs := time.Now().UTC().Add(-time.Hour).UnixMilli()
	if _, err := pool.Exec(ctx, `
		INSERT INTO payme_transaction (payme_id, payment_id, amount_tiyin, state, create_time, perform_time)
		VALUES ($1, $2, 5990000, 2, $3, $3)`,
		"payme-fallback-1", paymentID, txnCreateMs); err != nil {
		t.Fatal(err)
	}

	res, err := Run(ctx, pool, Options{
		From: time.Now().UTC().Add(-2 * time.Hour),
		To:   time.Now().UTC().Add(time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.ScannedPay != 0 {
		t.Fatalf("payment should be outside the listPayments window: scanned_pay=%d", res.ScannedPay)
	}
	for _, f := range res.Findings {
		if f.PaymentID == paymentID {
			t.Fatalf("unexpected finding for paid payment resolved via fallback lookup: %+v", f)
		}
	}
}

// TestRunFallbackLookupFlagsMismatchOutsideWindow is the same setup as
// above but with a payment status that does not reconcile, proving the
// fallback lookup's result is actually used for the paid/refunded check
// (not just silently treated as "found").
func TestRunFallbackLookupFlagsMismatchOutsideWindow(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()

	profileID := uuid.New()
	phone := fmt.Sprintf("+9989%08d", int(profileID.ID())%100000000)
	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone) VALUES ($1, $2)`, profileID, phone); err != nil {
		t.Fatal(err)
	}
	tariffID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO tariff (id, code, days, price_uzs, sort_order, active) VALUES ($1, 'gentra', 30, 59900, 1, true)`,
		tariffID); err != nil {
		t.Fatal(err)
	}
	paymentID := uuid.New()
	paymentCreated := time.Now().UTC().Add(-48 * time.Hour)
	if _, err := pool.Exec(ctx, `
		INSERT INTO payment (id, profile_id, tariff_id, amount_uzs, provider, status, idempotency_key,
		                     tariff_days_snapshot, tariff_price_uzs_snapshot, created_at)
		VALUES ($1, $2, $3, 59900, 'payme', 'pending', $4, 30, 59900, $5)`,
		paymentID, profileID, tariffID, "recon-fallback-bad-"+paymentID.String(), paymentCreated); err != nil {
		t.Fatal(err)
	}
	txnCreateMs := time.Now().UTC().Add(-time.Hour).UnixMilli()
	if _, err := pool.Exec(ctx, `
		INSERT INTO payme_transaction (payme_id, payment_id, amount_tiyin, state, create_time, perform_time)
		VALUES ($1, $2, 5990000, 2, $3, $3)`,
		"payme-fallback-2", paymentID, txnCreateMs); err != nil {
		t.Fatal(err)
	}

	res, err := Run(ctx, pool, Options{
		From: time.Now().UTC().Add(-2 * time.Hour),
		To:   time.Now().UTC().Add(time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range res.Findings {
		if f.Code == "perform_payment_not_paid" && f.PaymentID == paymentID {
			found = true
			if f.Detail != "payme state=2 but payment.status=pending" {
				t.Fatalf("detail=%q", f.Detail)
			}
		}
	}
	if !found {
		t.Fatalf("want perform_payment_not_paid via fallback lookup, got %+v", res.Findings)
	}
}

func TestRunStalePending(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()

	profileID := uuid.New()
	phone := fmt.Sprintf("+9989%08d", int(profileID.ID())%100000000)
	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone) VALUES ($1, $2)`, profileID, phone); err != nil {
		t.Fatal(err)
	}
	tariffID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO tariff (id, code, days, price_uzs, sort_order, active) VALUES ($1, 'gentra', 30, 59900, 1, true)`,
		tariffID); err != nil {
		t.Fatal(err)
	}
	paymentID := uuid.New()
	created := time.Now().UTC().Add(-20 * time.Hour)
	if _, err := pool.Exec(ctx, `
		INSERT INTO payment (id, profile_id, tariff_id, amount_uzs, provider, status, idempotency_key,
		                     tariff_days_snapshot, tariff_price_uzs_snapshot, created_at)
		VALUES ($1, $2, $3, 59900, 'payme', 'pending', $4, 30, 59900, $5)`,
		paymentID, profileID, tariffID, "recon-stale-"+paymentID.String(), created); err != nil {
		t.Fatal(err)
	}

	res, err := Run(ctx, pool, Options{
		From:              created.Add(-time.Minute),
		To:                time.Now().UTC().Add(time.Minute),
		StalePendingAfter: 12 * time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range res.Findings {
		if f.Code == "stale_pending" {
			found = true
		}
	}
	if !found {
		t.Fatalf("want stale_pending, got %+v", res.Findings)
	}
}
