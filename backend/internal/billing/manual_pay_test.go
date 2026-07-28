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

// TestManualPay_AmountsAreUniquePerCard pins the fix for a hijack that
// needed no race to win. Amounts used to be the tariff price verbatim, and
// matching keyed on (card, amount) then preferred the NEWEST assignment on
// that card. With only a handful of cards and three prices, an attacker
// could loop checkouts so his assignment was always the newest, and a
// genuine payer's late transfer confirmed HIS payment instead. Unique
// amounts per card make the two assignments distinguishable, so a transfer
// can only ever match the person who was told to send exactly that sum.
func TestManualPay_AmountsAreUniquePerCard(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	q := sqlc.New(pool)
	svc := Service{Q: q, Pool: pool, Secret: []byte("test-secret-key-32-bytes-minimum!!")}
	enableManual(t, pool)

	_, _ = pool.Exec(ctx, `DELETE FROM manual_pay_event; DELETE FROM manual_pay_assignment; DELETE FROM manual_pay_card`)
	// One card only: every checkout lands on it, which is exactly the
	// collision the old code could not tell apart.
	seedManualCard(t, pool, q, "4042", 0)

	const buyers = 5
	amounts := make(map[int64]uuid.UUID, buyers)
	for i := 0; i < buyers; i++ {
		profileID := uuid.New()
		if _, err := pool.Exec(ctx,
			`INSERT INTO profile (id, phone) VALUES ($1, $2)`,
			profileID, fmt.Sprintf("+9989011%05d", i)); err != nil {
			t.Fatal(err)
		}
		info, err := svc.StartManualCheckout(ctx, profileID, "gentra", "")
		if err != nil {
			t.Fatalf("checkout %d: %v", i, err)
		}
		// Expire this buyer's hold so the card returns to the pool for the
		// next one. The released assignment stays matchable ("late push OK"),
		// which is precisely how several claimable assignments pile up on one
		// card — the situation the old (card, amount) key could not resolve.
		if _, err := pool.Exec(ctx,
			`UPDATE manual_pay_assignment SET hold_until = now() - interval '1 minute' WHERE payment_id = $1`,
			info.PaymentID); err != nil {
			t.Fatal(err)
		}
		if err := q.ReleaseExpiredManualHolds(ctx); err != nil {
			t.Fatal(err)
		}
		if prev, clash := amounts[info.AmountUzs]; clash {
			t.Fatalf("checkout %d reused amount %d already held by payment %s: a transfer could confirm the wrong buyer",
				i, info.AmountUzs, prev)
		}
		amounts[info.AmountUzs] = info.PaymentID
	}

	// The nudge must stay small enough to be invisible to the payer.
	var base int64
	if err := pool.QueryRow(ctx, `SELECT price_uzs FROM tariff WHERE code = 'gentra'`).Scan(&base); err != nil {
		t.Fatal(err)
	}
	for amt := range amounts {
		if amt < base || amt >= base+manualAmountSlots {
			t.Errorf("amount %d outside [%d, %d): the suffix should be a few som, not a price change",
				amt, base, base+manualAmountSlots)
		}
	}
}

// TestManualPay_SameMinuteTransferMatches covers the other half of the
// matching fix. The bank push carries only HH:MM, so transfer_at arrives
// with its seconds zeroed and can read slightly BEFORE assigned_at. The old
// predicate required transfer_at >= assigned_at, so anyone who paid in the
// same minute they checked out silently failed to auto-confirm — they had
// paid, and nothing was granted until a human intervened.
func TestManualPay_SameMinuteTransferMatches(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	q := sqlc.New(pool)
	svc := Service{Q: q, Pool: pool, Secret: []byte("test-secret-key-32-bytes-minimum!!")}
	enableManual(t, pool)

	_, _ = pool.Exec(ctx, `DELETE FROM manual_pay_event; DELETE FROM manual_pay_assignment; DELETE FROM manual_pay_card`)
	seedManualCard(t, pool, q, "4042", 0)

	profileID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone) VALUES ($1, '+998901119999')`, profileID); err != nil {
		t.Fatal(err)
	}
	info, err := svc.StartManualCheckout(ctx, profileID, "gentra", "")
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}

	// Pin assigned_at to a known instant mid-minute, then have the push
	// report that same minute — i.e. a transfer_at 30s "before" it.
	assigned := time.Date(2026, 7, 28, 12, 0, 30, 0, time.UTC)
	if _, err := pool.Exec(ctx,
		`UPDATE manual_pay_assignment
		 SET assigned_at = $2::timestamptz, hold_until = $2::timestamptz + interval '10 minutes'
		 WHERE payment_id = $1`,
		info.PaymentID, assigned); err != nil {
		t.Fatal(err)
	}

	local := assigned.In(time.FixedZone("UZT", 5*3600))
	raw := fmt.Sprintf(`🎉 To'ldirish
➕ %s UZS
📍 CLICK P2P
💳 HUMOCARD *%s
🕓 %02d:%02d %02d.%02d.%04d
💰 9.100,00 UZS`,
		formatUzsDot(info.AmountUzs), info.PanLast4,
		local.Hour(), local.Minute(), local.Day(), int(local.Month()), local.Year(),
	)

	res, err := svc.IngestHumoPush(ctx, raw, 987001)
	if err != nil {
		t.Fatalf("IngestHumoPush: %v", err)
	}
	if got := fmt.Sprint(res["status"]); got != "matched" {
		t.Fatalf("status=%v, want matched: a same-minute payer must auto-confirm (res=%v)", got, res)
	}

	var payStatus string
	if err := pool.QueryRow(ctx, `SELECT status FROM payment WHERE id = $1`, info.PaymentID).Scan(&payStatus); err != nil {
		t.Fatal(err)
	}
	if payStatus != "paid" {
		t.Errorf("payment.status=%q, want paid", payStatus)
	}
}
