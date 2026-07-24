package billing

import (
	"context"
	"errors"
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
		INSERT INTO payment (id, profile_id, tariff_id, amount_uzs, provider, status, paid_at, idempotency_key)
		VALUES ($1, $2, (SELECT id FROM tariff WHERE code = 'gentra'), 59900, 'payme', 'paid', NOW(), $3)
	`, paymentID, referee, uuid.New().String()); err != nil {
		t.Fatalf("create payment failed: %v", err)
	}

	// Process payment grant (should trigger referral reward)
	if err := svc.ProcessPaymentGrant(ctx, paymentID); err != nil {
		t.Fatalf("ProcessPaymentGrant failed: %v", err)
	}

	// Check referrer entitlement (should have 7 days VIP)
	active, until, err := svc.Status(ctx, referrer)
	if err != nil {
		t.Fatalf("check referrer status failed: %v", err)
	}
	if !active || until == nil {
		t.Fatalf("referrer should have active entitlement")
	}

	// Check referrer stats (total_rewarded should be 1, bonus_days_earned = 7)
	stats, err := svc.GetReferralStats(ctx, referrer)
	if err != nil {
		t.Fatal(err)
	}
	if stats.TotalRewarded != 1 || stats.BonusDaysEarned != 7 {
		t.Errorf("got rewarded=%d bonusDays=%d, want 1 and 7", stats.TotalRewarded, stats.BonusDaysEarned)
	}

	// Second payment by same referee does not double grant referral reward
	payment2ID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO payment (id, profile_id, tariff_id, amount_uzs, provider, status, paid_at, idempotency_key)
		VALUES ($1, $2, (SELECT id FROM tariff WHERE code = 'gentra'), 59900, 'payme', 'paid', NOW(), $3)
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
	if stats2.TotalRewarded != 1 || stats2.BonusDaysEarned != 7 {
		t.Errorf("got rewarded=%d bonusDays=%d on second payment, want 1 and 7", stats2.TotalRewarded, stats2.BonusDaysEarned)
	}
}
