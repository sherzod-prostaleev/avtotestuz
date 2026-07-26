package billing

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

func seedManualCard(t *testing.T, pool *pgxpool.Pool, q *sqlc.Queries, last4 string, sort int32) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	_, _ = pool.Exec(ctx, `DELETE FROM manual_pay_event`)
	_, _ = pool.Exec(ctx, `DELETE FROM manual_pay_assignment`)
	_, _ = pool.Exec(ctx, `DELETE FROM manual_pay_card WHERE pan_last4 = $1`, last4)
	pan := "986012345678" + last4
	row, err := q.InsertManualPayCard(ctx, sqlc.InsertManualPayCardParams{
		Network:    "humo",
		PanFull:    pan,
		PanLast4:   last4,
		HolderName: "TEST HOLDER",
		SortOrder:  sort,
		Enabled:    true,
	})
	if err != nil {
		t.Fatalf("insert card: %v", err)
	}
	return row.ID
}

func enableManual(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO payment_provider_status (provider, enabled) VALUES ('manual', true)
		ON CONFLICT (provider) DO UPDATE SET enabled = true`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO feature_flag (key, type, value_json, description)
		VALUES ('checkout_manual', 'boolean', 'true'::jsonb, '')
		ON CONFLICT (key) DO UPDATE SET value_json = 'true'::jsonb`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO tariff (code, days, price_uzs, sort_order, active) VALUES ('gentra', 30, 59900, 1, true) ON CONFLICT (code) DO UPDATE SET active = true, price_uzs = 59900, days = 30`); err != nil {
		t.Fatal(err)
	}
}

func TestManualPay_AssignConfirmIdempotentAndLatePush(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	q := sqlc.New(pool)
	svc := Service{Q: q, Pool: pool, Secret: []byte("test-secret-key-32-bytes-minimum!!")}
	enableManual(t, pool)

	_, _ = pool.Exec(ctx, `DELETE FROM manual_pay_event; DELETE FROM manual_pay_assignment; DELETE FROM manual_pay_card`)
	for i, last4 := range []string{"1111", "2222", "3333", "4042"} {
		seedManualCard(t, pool, q, last4, int32(i))
	}

	profileID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone) VALUES ($1, '+998901112233')`, profileID); err != nil {
		t.Fatal(err)
	}

	info, err := svc.StartManualCheckout(ctx, profileID, "gentra", "")
	if err != nil {
		t.Fatalf("StartManualCheckout: %v", err)
	}
	if info.AmountUzs <= 0 || info.PanLast4 == "" {
		t.Fatalf("bad info: %+v", info)
	}

	if _, err = svc.ClaimManualPayment(ctx, profileID, info.PaymentID); err != nil {
		t.Fatalf("claim: %v", err)
	}

	_, err = pool.Exec(ctx, `UPDATE manual_pay_assignment SET hold_until = now() - interval '1 minute' WHERE payment_id = $1`, info.PaymentID)
	if err != nil {
		t.Fatal(err)
	}
	if err := q.ReleaseExpiredManualHolds(ctx); err != nil {
		t.Fatal(err)
	}
	st, err := svc.GetManualPaymentStatus(ctx, profileID, info.PaymentID)
	if err != nil {
		t.Fatal(err)
	}
	if st.ManualState != "awaiting_review" {
		t.Fatalf("state=%s want awaiting_review", st.ManualState)
	}

	assigned := info.HoldUntil.Add(-ManualHoldDuration)
	pushTime := assigned.In(time.FixedZone("UZT", 5*3600)).Add(2 * time.Minute)
	raw := fmt.Sprintf(`🎉 To'ldirish
➕ %s UZS
📍 CLICK P2P
💳 HUMOCARD *%s
🕓 %02d:%02d %02d.%02d.%04d
💰 9.100,00 UZS`,
		formatUzsDot(info.AmountUzs),
		info.PanLast4,
		pushTime.Hour(), pushTime.Minute(),
		pushTime.Day(), int(pushTime.Month()), pushTime.Year(),
	)

	out, err := svc.IngestHumoPush(ctx, raw, 42)
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if out["status"] != "matched" {
		t.Fatalf("ingest out=%v", out)
	}

	already, err := svc.ConfirmManualPayment(ctx, info.PaymentID, "admin")
	if err != nil {
		t.Fatalf("admin reconfirm: %v", err)
	}
	if !already {
		t.Fatal("expected already confirmed after bot")
	}

	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM payment WHERE id=$1`, info.PaymentID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "paid" {
		t.Fatalf("payment status=%s", status)
	}
}

func TestManualPay_NoDoubleGrantAdminThenBot(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	q := sqlc.New(pool)
	svc := Service{Q: q, Pool: pool, Secret: []byte("test-secret-key-32-bytes-minimum!!")}
	enableManual(t, pool)
	_, _ = pool.Exec(ctx, `DELETE FROM manual_pay_event; DELETE FROM manual_pay_assignment; DELETE FROM manual_pay_card`)
	seedManualCard(t, pool, q, "5555", 0)
	profileID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone) VALUES ($1, '+998901112244')`, profileID); err != nil {
		t.Fatal(err)
	}

	info, err := svc.StartManualCheckout(ctx, profileID, "gentra", "")
	if err != nil {
		t.Fatal(err)
	}
	already, err := svc.ConfirmManualPayment(ctx, info.PaymentID, "admin")
	if err != nil || already {
		t.Fatalf("admin confirm: already=%v err=%v", already, err)
	}
	already, err = svc.ConfirmManualPayment(ctx, info.PaymentID, "bot")
	if err != nil || !already {
		t.Fatalf("bot confirm: already=%v err=%v", already, err)
	}
}

func TestManualPay_CardPoolExhausted(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	q := sqlc.New(pool)
	svc := Service{Q: q, Pool: pool, Secret: []byte("test-secret-key-32-bytes-minimum!!")}
	enableManual(t, pool)
	_, _ = pool.Exec(ctx, `DELETE FROM manual_pay_event; DELETE FROM manual_pay_assignment; DELETE FROM manual_pay_card`)
	seedManualCard(t, pool, q, "6666", 0)
	p1, p2 := uuid.New(), uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone) VALUES ($1, '+998901112255'), ($2, '+998901112266')`, p1, p2); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.StartManualCheckout(ctx, p1, "gentra", ""); err != nil {
		t.Fatal(err)
	}
	_, err := svc.StartManualCheckout(ctx, p2, "gentra", "")
	if err == nil || err != ErrManualNoCardAvailable {
		t.Fatalf("want ErrManualNoCardAvailable got %v", err)
	}
}

func formatUzsDot(v int64) string {
	s := fmt.Sprintf("%d", v)
	var out []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, '.')
		}
		out = append(out, byte(c))
	}
	return string(out) + ",00"
}
