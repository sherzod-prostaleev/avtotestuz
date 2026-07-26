package billing

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

func TestBuyerReferralBonusDays(t *testing.T) {
	cases := []struct {
		days int32
		want int
	}{
		{7, 3},
		{30, 7},
		{75, 10},
		{14, 0},
	}
	for _, tc := range cases {
		if got := BuyerReferralBonusDays(tc.days); got != tc.want {
			t.Errorf("BuyerReferralBonusDays(%d)=%d, want %d", tc.days, got, tc.want)
		}
	}
}

func TestCommissionUZS(t *testing.T) {
	if got := CommissionUZS(59900, 20); got != 11980 {
		t.Errorf("20%% of 59900 = %d, want 11980", got)
	}
	if got := CommissionUZS(59900, 50); got != 29950 {
		t.Errorf("50%% of 59900 = %d, want 29950", got)
	}
	if got := CommissionUZS(100, 0); got != 0 {
		t.Errorf("0%% = %d, want 0", got)
	}
}

func TestPayoutHoldAndReject(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	referrer := uuid.New()
	referee := uuid.New()
	paymentID := uuid.New()

	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone) VALUES ($1, '+998901120001')`, referrer); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone, referral_commission_percent) VALUES ($1, '+998901120002', 50)`, referee); err != nil {
		t.Fatal(err)
	}
	// Put 50% on referrer
	if _, err := pool.Exec(ctx, `UPDATE profile SET referral_commission_percent = 50 WHERE id = $1`, referrer); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO tariff (code, days, price_uzs, sort_order, active) VALUES ('nexia', 7, 24900, 0, true) ON CONFLICT (code) DO UPDATE SET active = true, price_uzs = 24900, days = 7`); err != nil {
		t.Fatal(err)
	}

	svc := Service{Q: sqlc.New(pool), Pool: pool}
	code, err := svc.GetOrCreateReferralCode(ctx, referrer)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.ApplyReferralCode(ctx, referee, code); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO payment (id, profile_id, tariff_id, amount_uzs, provider, status, paid_at, idempotency_key,
		                     tariff_days_snapshot, tariff_price_uzs_snapshot)
		SELECT $1, $2, t.id, 24900, 'payme', 'paid', NOW(), $3, t.days, t.price_uzs
		FROM tariff t WHERE t.code = 'nexia'
	`, paymentID, referee, uuid.New().String()); err != nil {
		t.Fatal(err)
	}
	if err := svc.ProcessPaymentGrant(ctx, paymentID); err != nil {
		t.Fatal(err)
	}

	want := int64(24900 * 50 / 100)
	bal, err := svc.ReferralBalance(ctx, referrer)
	if err != nil {
		t.Fatal(err)
	}
	if bal != want {
		t.Fatalf("balance=%d, want %d", bal, want)
	}

	// Buyer bonus for nexia = +3 days
	var bonusDays float64
	if err := pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(EXTRACT(EPOCH FROM (ends_at - starts_at)) / 86400), 0)
		FROM entitlement WHERE profile_id = $1 AND source = 'referral_buyer'
	`, referee).Scan(&bonusDays); err != nil {
		t.Fatal(err)
	}
	if bonusDays < 2.99 || bonusDays > 3.01 {
		t.Errorf("buyer bonus days=%v, want 3", bonusDays)
	}

	payout, err := svc.RequestReferralPayout(ctx, referrer, want, "8600123456789012", "uzcard")
	if err != nil {
		t.Fatalf("payout: %v", err)
	}
	balAfter, _ := svc.ReferralBalance(ctx, referrer)
	if balAfter != 0 {
		t.Errorf("balance after hold=%d, want 0", balAfter)
	}

	adminSvc := struct{}{}
	_ = adminSvc
	// Reject via ledger release using admin service pattern inline
	q := sqlc.New(pool)
	if _, err := q.MarkReferralPayoutRejected(ctx, sqlc.MarkReferralPayoutRejectedParams{
		ID:          payout.ID,
		ProcessedBy: uuid.NullUUID{},
		AdminNote:   "test reject",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := q.InsertReferralLedger(ctx, sqlc.InsertReferralLedgerParams{
		ProfileID: referrer,
		EntryType: "payout_reject_release",
		AmountUzs: want,
		PaymentID: uuid.NullUUID{},
		PayoutID:  uuid.NullUUID{UUID: payout.ID, Valid: true},
		Meta:      []byte(`{}`),
	}); err != nil {
		t.Fatal(err)
	}
	balRestored, _ := svc.ReferralBalance(ctx, referrer)
	if balRestored != want {
		t.Errorf("balance after reject=%d, want %d", balRestored, want)
	}
}

func TestNoCommissionWithoutAttribution(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	buyer := uuid.New()
	paymentID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone) VALUES ($1, '+998901120003')`, buyer); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO tariff (code, days, price_uzs, sort_order, active) VALUES ('gentra', 30, 59900, 1, true) ON CONFLICT (code) DO UPDATE SET active = true`); err != nil {
		t.Fatal(err)
	}
	svc := Service{Q: sqlc.New(pool), Pool: pool}
	if _, err := pool.Exec(ctx, `
		INSERT INTO payment (id, profile_id, tariff_id, amount_uzs, provider, status, paid_at, idempotency_key,
		                     tariff_days_snapshot, tariff_price_uzs_snapshot)
		SELECT $1, $2, t.id, 59900, 'payme', 'paid', NOW(), $3, t.days, t.price_uzs
		FROM tariff t WHERE t.code = 'gentra'
	`, paymentID, buyer, uuid.New().String()); err != nil {
		t.Fatal(err)
	}
	if err := svc.ProcessPaymentGrant(ctx, paymentID); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM referral_ledger`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("ledger rows=%d, want 0 without attribution", n)
	}
}
