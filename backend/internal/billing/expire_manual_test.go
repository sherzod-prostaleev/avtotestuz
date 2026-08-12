package billing

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

func TestExpireStaleManualPayments(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	q := sqlc.New(pool)
	svc := Service{Q: q, Pool: pool, Secret: []byte("test-secret-key-32-bytes-minimum!!")}
	enableManual(t, pool)

	_, _ = pool.Exec(ctx, `DELETE FROM manual_pay_event; DELETE FROM manual_pay_assignment; DELETE FROM manual_pay_card`)
	for i, last4 := range []string{"4042", "1111", "2222"} {
		seedManualCard(t, pool, q, last4, int32(i))
	}

	staleProfile := uuid.New()
	freshProfile := uuid.New()
	paidProfile := uuid.New()
	paymeProfile := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone) VALUES
		($1, '+998901110001'), ($2, '+998901110002'), ($3, '+998901110003'), ($4, '+998901110004')`,
		staleProfile, freshProfile, paidProfile, paymeProfile); err != nil {
		t.Fatal(err)
	}

	stale, err := svc.StartManualCheckout(ctx, staleProfile, "gentra", "")
	if err != nil {
		t.Fatalf("stale checkout: %v", err)
	}
	fresh, err := svc.StartManualCheckout(ctx, freshProfile, "gentra", "")
	if err != nil {
		t.Fatalf("fresh checkout: %v", err)
	}
	paid, err := svc.StartManualCheckout(ctx, paidProfile, "gentra", "")
	if err != nil {
		t.Fatalf("paid checkout: %v", err)
	}
	if _, err := svc.ConfirmManualPayment(ctx, paid.PaymentID, "admin"); err != nil {
		t.Fatalf("confirm paid: %v", err)
	}

	var tariffID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM tariff WHERE code='gentra'`).Scan(&tariffID); err != nil {
		t.Fatal(err)
	}
	paymeID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO payment (id, profile_id, tariff_id, amount_uzs, provider, status, idempotency_key,
		                     tariff_days_snapshot, tariff_price_uzs_snapshot, created_at)
		VALUES ($1, $2, $3, 59900, 'payme', 'created', $4, 30, 59900, now() - interval '8 hours')`,
		paymeID, paymeProfile, tariffID, "expire-payme-"+paymeID.String()); err != nil {
		t.Fatal(err)
	}

	if _, err := pool.Exec(ctx, `UPDATE payment SET created_at = now() - interval '5 hours' WHERE id = $1`, stale.PaymentID); err != nil {
		t.Fatal(err)
	}

	n, err := svc.ExpireStaleManualPayments(ctx)
	if err != nil {
		t.Fatalf("expire: %v", err)
	}
	if n != 1 {
		t.Fatalf("expired count=%d want 1", n)
	}

	assertStatus := func(id uuid.UUID, want string) {
		t.Helper()
		var got string
		if err := pool.QueryRow(ctx, `SELECT status FROM payment WHERE id=$1`, id).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("payment %s status=%s want %s", id, got, want)
		}
	}
	assertStatus(stale.PaymentID, "canceled")
	assertStatus(fresh.PaymentID, "pending")
	assertStatus(paid.PaymentID, "paid")
	assertStatus(paymeID, "created")

	var rows int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM payment WHERE id=$1`, stale.PaymentID).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Fatal("expired payment row was deleted")
	}

	var asgState string
	if err := pool.QueryRow(ctx, `SELECT manual_state FROM manual_pay_assignment WHERE payment_id=$1`, stale.PaymentID).Scan(&asgState); err != nil {
		t.Fatal(err)
	}
	if asgState != "rejected" {
		t.Fatalf("assignment state=%s want rejected", asgState)
	}

	if _, err := svc.ConfirmManualPayment(ctx, stale.PaymentID, "admin"); err != ErrManualAlreadyDone {
		t.Fatalf("confirm expired: err=%v want ErrManualAlreadyDone", err)
	}

	pushTime := time.Now().UTC().Add(-5 * time.Hour)
	local := pushTime.In(time.FixedZone("UZT", 5*3600))
	raw := fmt.Sprintf(`🎉 To'ldirish
➕ %s UZS
📍 CLICK P2P
💳 HUMOCARD *%s
🕓 %02d:%02d %02d.%02d.%04d
💰 9.100,00 UZS`,
		formatUzsDot(stale.AmountUzs), stale.PanLast4,
		local.Hour(), local.Minute(), local.Day(), int(local.Month()), local.Year(),
	)
	out, err := svc.IngestHumoPush(ctx, raw, 88001)
	if err != nil {
		t.Fatalf("ingest after expire: %v", err)
	}
	if fmt.Sprint(out["status"]) != "unmatched" {
		t.Fatalf("late push after expire matched: %v", out)
	}

	n, err = svc.ExpireStaleManualPayments(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("second expire count=%d want 0", n)
	}
}
