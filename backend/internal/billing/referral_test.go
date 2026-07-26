package billing

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

func TestReferralCodeGenerationAndLookup(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	userA := uuid.New()

	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone) VALUES ($1, '+998901110001')`, userA); err != nil {
		t.Fatal(err)
	}

	svc := Service{Q: sqlc.New(pool)}
	code1, err := svc.GetOrCreateReferralCode(ctx, userA)
	if err != nil {
		t.Fatalf("unexpected error creating referral code: %v", err)
	}
	if len(code1) < 5 {
		t.Errorf("code too short: %s", code1)
	}

	// Repeated call returns same code
	code2, err := svc.GetOrCreateReferralCode(ctx, userA)
	if err != nil {
		t.Fatalf("unexpected error getting referral code: %v", err)
	}
	if code1 != code2 {
		t.Errorf("got %s, want %s", code2, code1)
	}

	// Stats for new user
	stats, err := svc.GetReferralStats(ctx, userA)
	if err != nil {
		t.Fatalf("unexpected error getting stats: %v", err)
	}
	if stats.ReferralCode != code1 {
		t.Errorf("got code %s, want %s", stats.ReferralCode, code1)
	}
	if stats.TotalInvited != 0 || stats.TotalRewarded != 0 || stats.BonusDaysEarned != 0 {
		t.Errorf("expected 0 stats, got %+v", stats)
	}
}

func TestApplyReferralCode_Success(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	referrer := uuid.New()
	referee := uuid.New()

	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone) VALUES ($1, '+998901110002')`, referrer); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone) VALUES ($1, '+998901110003')`, referee); err != nil {
		t.Fatal(err)
	}

	svc := Service{Q: sqlc.New(pool)}
	code, err := svc.GetOrCreateReferralCode(ctx, referrer)
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.ApplyReferralCode(ctx, referee, code); err != nil {
		t.Fatalf("unexpected error applying referral code: %v", err)
	}

	// Check referrer stats: invited = 1, rewarded = 0
	stats, err := svc.GetReferralStats(ctx, referrer)
	if err != nil {
		t.Fatal(err)
	}
	if stats.TotalInvited != 1 || stats.TotalRewarded != 0 {
		t.Errorf("got invited=%d rewarded=%d, want 1 and 0", stats.TotalInvited, stats.TotalRewarded)
	}
}

func TestApplyReferralCode_SelfReferral(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	userA := uuid.New()

	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone) VALUES ($1, '+998901110004')`, userA); err != nil {
		t.Fatal(err)
	}

	svc := Service{Q: sqlc.New(pool)}
	code, err := svc.GetOrCreateReferralCode(ctx, userA)
	if err != nil {
		t.Fatal(err)
	}

	err = svc.ApplyReferralCode(ctx, userA, code)
	if !errors.Is(err, ErrReferralSelf) {
		t.Errorf("got error %v, want ErrReferralSelf", err)
	}
}

func TestApplyReferralCode_AlreadyApplied(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	referrer1 := uuid.New()
	referrer2 := uuid.New()
	referee := uuid.New()

	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone) VALUES ($1, '+998901110005')`, referrer1); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone) VALUES ($1, '+998901110006')`, referrer2); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone) VALUES ($1, '+998901110007')`, referee); err != nil {
		t.Fatal(err)
	}

	svc := Service{Q: sqlc.New(pool)}
	code1, _ := svc.GetOrCreateReferralCode(ctx, referrer1)
	code2, _ := svc.GetOrCreateReferralCode(ctx, referrer2)

	if err := svc.ApplyReferralCode(ctx, referee, code1); err != nil {
		t.Fatalf("first apply failed: %v", err)
	}

	err := svc.ApplyReferralCode(ctx, referee, code2)
	if !errors.Is(err, ErrReferralAlreadyApplied) {
		t.Errorf("got error %v, want ErrReferralAlreadyApplied", err)
	}
}

func TestApplyReferralCode_NotEligiblePaid(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	referrer := uuid.New()
	referee := uuid.New()

	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone) VALUES ($1, '+998901110101')`, referrer); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone) VALUES ($1, '+998901110102')`, referee); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO tariff (code, days, price_uzs, sort_order, active) VALUES ('gentra', 30, 59900, 1, true) ON CONFLICT (code) DO UPDATE SET active = true`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO payment (id, profile_id, tariff_id, amount_uzs, provider, status, paid_at, idempotency_key,
		                     tariff_days_snapshot, tariff_price_uzs_snapshot)
		SELECT $1, $2, t.id, 59900, 'payme', 'paid', NOW(), $3, t.days, t.price_uzs
		FROM tariff t WHERE t.code = 'gentra'
	`, uuid.New(), referee, uuid.New().String()); err != nil {
		t.Fatal(err)
	}

	svc := Service{Q: sqlc.New(pool)}
	code, err := svc.GetOrCreateReferralCode(ctx, referrer)
	if err != nil {
		t.Fatal(err)
	}
	err = svc.ApplyReferralCode(ctx, referee, code)
	if !errors.Is(err, ErrReferralNotEligiblePaid) {
		t.Fatalf("got %v, want ErrReferralNotEligiblePaid", err)
	}
}

func TestApplyReferralCode_AbandonedCheckoutStillEligible(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	referrer := uuid.New()
	referee := uuid.New()

	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone) VALUES ($1, '+998901110103')`, referrer); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone) VALUES ($1, '+998901110104')`, referee); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO tariff (code, days, price_uzs, sort_order, active) VALUES ('gentra', 30, 59900, 1, true) ON CONFLICT (code) DO UPDATE SET active = true`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO payment (id, profile_id, tariff_id, amount_uzs, provider, status, idempotency_key,
		                     tariff_days_snapshot, tariff_price_uzs_snapshot)
		SELECT $1, $2, t.id, 59900, 'payme', 'created', $3, t.days, t.price_uzs
		FROM tariff t WHERE t.code = 'gentra'
	`, uuid.New(), referee, uuid.New().String()); err != nil {
		t.Fatal(err)
	}

	svc := Service{Q: sqlc.New(pool)}
	code, err := svc.GetOrCreateReferralCode(ctx, referrer)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.ApplyReferralCode(ctx, referee, code); err != nil {
		t.Fatalf("abandoned checkout should still allow attach: %v", err)
	}
}

func TestApplyReferralCode_WindowClosed(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	referrer := uuid.New()
	referee := uuid.New()

	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone) VALUES ($1, '+998901110105')`, referrer); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO profile (id, phone, created_at)
		VALUES ($1, '+998901110106', NOW() - INTERVAL '31 days')
	`, referee); err != nil {
		t.Fatal(err)
	}

	svc := Service{Q: sqlc.New(pool)}
	code, err := svc.GetOrCreateReferralCode(ctx, referrer)
	if err != nil {
		t.Fatal(err)
	}
	err = svc.ApplyReferralCode(ctx, referee, code)
	if !errors.Is(err, ErrReferralWindowClosed) {
		t.Fatalf("got %v, want ErrReferralWindowClosed", err)
	}
}

func TestProcessReferralRewardOnPayment(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	referrer := uuid.New()
	referee := uuid.New()
	paymentID := uuid.New()

	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone) VALUES ($1, '+998901110008')`, referrer); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone) VALUES ($1, '+998901110009')`, referee); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO tariff (code, days, price_uzs, sort_order, active) VALUES ('gentra', 30, 59900, 1, true) ON CONFLICT (code) DO UPDATE SET active = true, price_uzs = 59900, days = 30`); err != nil {
		t.Fatal(err)
	}

	svc := Service{Q: sqlc.New(pool)}
	code, err := svc.GetOrCreateReferralCode(ctx, referrer)
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.ApplyReferralCode(ctx, referee, code); err != nil {
		t.Fatal(err)
	}

	// Create paid payment for referee
	if _, err := pool.Exec(ctx, `
		INSERT INTO payment (id, profile_id, tariff_id, amount_uzs, provider, status, paid_at, idempotency_key,
		                     tariff_days_snapshot, tariff_price_uzs_snapshot)
		SELECT $1, $2, t.id, 59900, 'payme', 'paid', NOW(), $3, t.days, t.price_uzs
		FROM tariff t WHERE t.code = 'gentra'
	`, paymentID, referee, uuid.New().String()); err != nil {
		t.Fatalf("create payment failed: %v", err)
	}

	// Process payment grant (should trigger referral cash + buyer bonus days)
	if err := svc.ProcessPaymentGrant(ctx, paymentID); err != nil {
		t.Fatalf("ProcessPaymentGrant failed: %v", err)
	}

	// Referrer gets cash, not VIP days
	active, _, err := svc.Status(ctx, referrer)
	if err != nil {
		t.Fatalf("check referrer status failed: %v", err)
	}
	if active {
		t.Fatalf("referrer should not receive VIP days from referral")
	}
	wantCommission := int64(59900 * 20 / 100)
	bal, err := svc.ReferralBalance(ctx, referrer)
	if err != nil {
		t.Fatal(err)
	}
	if bal != wantCommission {
		t.Errorf("referrer balance=%d, want %d", bal, wantCommission)
	}

	// Buyer gets +7 days referral_buyer bonus on top of purchase
	var buyerBonus int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM entitlement WHERE profile_id = $1 AND source = 'referral_buyer'`,
		referee,
	).Scan(&buyerBonus); err != nil {
		t.Fatal(err)
	}
	if buyerBonus != 1 {
		t.Errorf("buyer referral_buyer grants=%d, want 1", buyerBonus)
	}

	stats, err := svc.GetReferralStats(ctx, referrer)
	if err != nil {
		t.Fatal(err)
	}
	if stats.TotalRewarded != 1 || stats.EarnedUzs != wantCommission {
		t.Errorf("got rewarded=%d earned=%d, want 1 and %d", stats.TotalRewarded, stats.EarnedUzs, wantCommission)
	}

	// Second payment by same referee does not double grant referral reward
	payment2ID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO payment (id, profile_id, tariff_id, amount_uzs, provider, status, paid_at, idempotency_key,
		                     tariff_days_snapshot, tariff_price_uzs_snapshot)
		SELECT $1, $2, t.id, 59900, 'payme', 'paid', NOW(), $3, t.days, t.price_uzs
		FROM tariff t WHERE t.code = 'gentra'
	`, payment2ID, referee, uuid.New().String()); err != nil {
		t.Fatalf("create second payment failed: %v", err)
	}

	if err := svc.ProcessPaymentGrant(ctx, payment2ID); err != nil {
		t.Fatalf("ProcessPaymentGrant 2 failed: %v", err)
	}

	stats2, err := svc.GetReferralStats(ctx, referrer)
	if err != nil {
		t.Fatal(err)
	}
	if stats2.TotalRewarded != 1 || stats2.EarnedUzs != wantCommission {
		t.Errorf("got rewarded=%d earned=%d on second payment, want 1 and %d", stats2.TotalRewarded, stats2.EarnedUzs, wantCommission)
	}
	bal2, err := svc.ReferralBalance(ctx, referrer)
	if err != nil {
		t.Fatal(err)
	}
	if bal2 != wantCommission {
		t.Errorf("balance after second payment=%d, want %d (no double credit)", bal2, wantCommission)
	}
}

// TestProcessReferralRewardOnPayment_ConcurrentDoubleGrant is a genuine
// concurrency regression test for the double-grant race: two different
// payments for the same referee reaching ProcessPaymentGrant at nearly the
// same time must NOT both credit the referrer's commission.
func TestProcessReferralRewardOnPayment_ConcurrentDoubleGrant(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	referrer := uuid.New()
	referee := uuid.New()

	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone) VALUES ($1, '+998901110010')`, referrer); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone) VALUES ($1, '+998901110011')`, referee); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO tariff (code, days, price_uzs, sort_order, active) VALUES ('gentra', 30, 59900, 1, true) ON CONFLICT (code) DO UPDATE SET active = true, price_uzs = 59900, days = 30`); err != nil {
		t.Fatal(err)
	}

	svc := Service{Q: sqlc.New(pool)}
	code, err := svc.GetOrCreateReferralCode(ctx, referrer)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.ApplyReferralCode(ctx, referee, code); err != nil {
		t.Fatal(err)
	}

	// Simulate several payments for the same referee racing to trigger the
	// referral reward at once. A shared *pgxpool.Pool (as returned by
	// testdb.New) is safe for concurrent use, so this exercises the real
	// race window in the database, not just in Go code.
	const concurrency = 8
	paymentIDs := make([]uuid.UUID, concurrency)
	for i := range paymentIDs {
		paymentIDs[i] = uuid.New()
		if _, err := pool.Exec(ctx, `
			INSERT INTO payment (id, profile_id, tariff_id, amount_uzs, provider, status, paid_at, idempotency_key,
			                     tariff_days_snapshot, tariff_price_uzs_snapshot)
			SELECT $1, $2, t.id, 59900, 'payme', 'paid', NOW(), $3, t.days, t.price_uzs
			FROM tariff t WHERE t.code = 'gentra'
		`, paymentIDs[i], referee, uuid.New().String()); err != nil {
			t.Fatalf("create payment %d failed: %v", i, err)
		}
	}

	ready := make(chan struct{})
	errs := make([]error, concurrency)
	var wg sync.WaitGroup
	for i := range paymentIDs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-ready // barrier: release all goroutines at once
			errs[i] = svc.ProcessPaymentGrant(ctx, paymentIDs[i])
		}(i)
	}
	close(ready)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("ProcessPaymentGrant[%d] failed: %v", i, err)
		}
	}

	var commissionRows int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM referral_ledger WHERE profile_id = $1 AND entry_type = 'commission'`,
		referrer,
	).Scan(&commissionRows); err != nil {
		t.Fatal(err)
	}
	if commissionRows != 1 {
		t.Errorf("got %d commission rows, want exactly 1 (double-credit race)", commissionRows)
	}

	wantCommission := int64(59900 * 20 / 100)
	bal, err := svc.ReferralBalance(ctx, referrer)
	if err != nil {
		t.Fatal(err)
	}
	if bal != wantCommission {
		t.Errorf("balance=%d, want %d", bal, wantCommission)
	}

	stats, err := svc.GetReferralStats(ctx, referrer)
	if err != nil {
		t.Fatal(err)
	}
	if stats.TotalRewarded != 1 || stats.EarnedUzs != wantCommission {
		t.Errorf("got rewarded=%d earned=%d, want exactly 1 and %d", stats.TotalRewarded, stats.EarnedUzs, wantCommission)
	}

	var referralStatus string
	if err := pool.QueryRow(ctx,
		`SELECT status FROM referral WHERE referrer_id = $1 AND referee_id = $2`,
		referrer, referee,
	).Scan(&referralStatus); err != nil {
		t.Fatal(err)
	}
	if referralStatus != "rewarded" {
		t.Errorf("got referral status %q, want %q", referralStatus, "rewarded")
	}
}
