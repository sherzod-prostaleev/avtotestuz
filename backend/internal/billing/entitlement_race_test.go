package billing

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

// TestProcessPaymentGrantSerialPromoBypass is the regression test for the
// critical finding that promo max_uses / per_user_limit were bypassable with no
// concurrency whatsoever. The promo_redemption row those limits are counted
// from is not written at checkout — it is written when the payment completes —
// so CountPromoRedemptions stayed at 0 for as long as nothing completed, and a
// 'created' payment never expires. An auditor therefore made five SEQUENTIAL
// StartCheckout calls against a max_uses=1, per_user_limit=1 code, minutes
// apart if it liked, and then completed all five: five discounted grants, five
// redemption rows.
//
// The fix re-validates under the promo_code row lock inside
// ProcessPaymentGrant. Only the first completion may redeem; the rest are
// pro-rated to the fraction of the list price actually paid, so banking extra
// discounted checkouts buys nothing. Here: gentra is 30 days at 59 900 UZS and
// the code is 50% off, so a rejected completion grants 15 days for the 29 950
// paid, and the total must be 30 + 4×15 = 90 days rather than 5×30 = 150.
func TestProcessPaymentGrantSerialPromoBypass(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	profileID := uuid.New()

	if _, err := pool.Exec(ctx,
		`INSERT INTO profile (id, phone) VALUES ($1, '+998900000021')`, profileID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO tariff (code, days, price_uzs, sort_order, active)
		 VALUES ('gentra', 30, 59900, 1, true)`); err != nil {
		t.Fatal(err)
	}
	var promoID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO promo_code (code, kind, value, max_uses, per_user_limit, active)
		 VALUES ('HALFOFF', 'percent', 50, 1, 1, true) RETURNING id`).Scan(&promoID); err != nil {
		t.Fatal(err)
	}

	svc := Service{Q: sqlc.New(pool), Pool: pool}
	cfg := CheckoutConfig{PaymeMerchantID: "M1", PaymeCheckoutHost: "https://checkout.paycom.uz"}

	// Phase 1: bank five discounted checkouts, strictly one after another.
	// These still succeed by design — nothing has been redeemed yet, so
	// refusing them at this point would be wrong.
	const n = 5
	paymentIDs := make([]uuid.UUID, 0, n)
	for i := 0; i < n; i++ {
		res, err := svc.StartCheckout(ctx, profileID, "gentra", "payme", cfg, "ru", "", "HALFOFF")
		if err != nil {
			t.Fatalf("serial checkout %d: %v", i, err)
		}
		paymentIDs = append(paymentIDs, res.PaymentID)
	}

	// Phase 2: complete all five, each in its own transaction, exactly as
	// payme.performTransaction / click.confirmAndGrant do.
	for i, pid := range paymentIDs {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		q := sqlc.New(tx)
		if err := q.MarkPaymentPaid(ctx, pid); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatalf("mark paid %d: %v", i, err)
		}
		if err := (Service{Q: q}).ProcessPaymentGrant(ctx, pid); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatalf("process grant %d: %v", i, err)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatalf("commit %d: %v", i, err)
		}
	}

	var redemptions int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM promo_redemption WHERE promo_code_id = $1`, promoID).Scan(&redemptions); err != nil {
		t.Fatal(err)
	}
	if redemptions != 1 {
		t.Errorf("promo_redemption rows = %d, want exactly 1 (max_uses=1)", redemptions)
	}

	// Entitlement days actually granted, measured end-to-end: the grants stack,
	// so the final active end minus the first start is the total term.
	var totalDays float64
	if err := pool.QueryRow(ctx,
		`SELECT EXTRACT(EPOCH FROM (MAX(ends_at) - MIN(starts_at))) / 86400
		 FROM entitlement WHERE profile_id = $1`, profileID).Scan(&totalDays); err != nil {
		t.Fatal(err)
	}
	if got, want := int(totalDays+0.5), 30+4*15; got != want {
		t.Errorf("total granted days = %d, want %d (1 honored 30-day grant + 4 pro-rated 15-day grants)", got, want)
	}

	// The pro-rated grants must say why, so support can reconcile them.
	var noted int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM entitlement WHERE profile_id = $1 AND note LIKE '%not redeemable at completion%'`,
		profileID).Scan(&noted); err != nil {
		t.Fatal(err)
	}
	if noted != 4 {
		t.Errorf("entitlements carrying a pro-rata note = %d, want 4", noted)
	}
}

// TestProcessPaymentGrantUsesCheckoutTimeTariffTerms pins the tariff snapshot
// added in migration 0019.
//
// Everything downstream of checkout used to re-read tariff.days and
// tariff.price_uzs from the live row. Those are mutable and a 'created' payment
// never expires, so the window between checkout and webhook completion is
// unbounded: editing a tariff silently rewrote what an in-flight purchase was
// worth. Here the tariff is doubled in price and halved in term *after*
// checkout — the customer must still receive the 30 days they bought, and the
// pro-rating ratio must still be measured against the price they were quoted.
func TestProcessPaymentGrantUsesCheckoutTimeTariffTerms(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	profileID := uuid.New()

	if _, err := pool.Exec(ctx,
		`INSERT INTO profile (id, phone) VALUES ($1, '+998900000026')`, profileID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO tariff (code, days, price_uzs, sort_order, active)
		 VALUES ('gentra', 30, 59900, 1, true)`); err != nil {
		t.Fatal(err)
	}

	svc := Service{Q: sqlc.New(pool), Pool: pool}
	cfg := CheckoutConfig{PaymeMerchantID: "M1", PaymeCheckoutHost: "https://checkout.paycom.uz"}
	res, err := svc.StartCheckout(ctx, profileID, "gentra", "payme", cfg, "ru", "", "")
	if err != nil {
		t.Fatal(err)
	}

	// The tariff changes while the payment sits in 'created' — a price rise and
	// a shorter term, i.e. the direction that would shortchange the customer.
	if _, err := pool.Exec(ctx,
		`UPDATE tariff SET days = 15, price_uzs = 119800 WHERE code = 'gentra'`); err != nil {
		t.Fatal(err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	q := sqlc.New(tx)
	if err := q.MarkPaymentPaid(ctx, res.PaymentID); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := (Service{Q: q}).ProcessPaymentGrant(ctx, res.PaymentID); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	var grantedDays float64
	if err := pool.QueryRow(ctx,
		`SELECT EXTRACT(EPOCH FROM (MAX(ends_at) - MIN(starts_at))) / 86400
		 FROM entitlement WHERE profile_id = $1`, profileID).Scan(&grantedDays); err != nil {
		t.Fatal(err)
	}
	if got := int(grantedDays + 0.5); got != 30 {
		t.Errorf("granted days = %d, want 30 (the term sold at checkout, not the tariff's current 15)", got)
	}
}

// TestProcessPaymentGrantHonorsFirstPromoUse pins the other side of the same
// branch: a promo used within its limits must still grant the full term and
// write its redemption row. Without this, "pro-rate everything" would pass the
// bypass test above while quietly breaking every legitimate discount.
func TestProcessPaymentGrantHonorsFirstPromoUse(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	profileID := uuid.New()

	if _, err := pool.Exec(ctx,
		`INSERT INTO profile (id, phone) VALUES ($1, '+998900000022')`, profileID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO tariff (code, days, price_uzs, sort_order, active)
		 VALUES ('gentra', 30, 59900, 1, true)`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO promo_code (code, kind, value, max_uses, per_user_limit, active)
		 VALUES ('HALFOFF', 'percent', 50, 1, 1, true)`); err != nil {
		t.Fatal(err)
	}

	svc := Service{Q: sqlc.New(pool), Pool: pool}
	cfg := CheckoutConfig{PaymeMerchantID: "M1", PaymeCheckoutHost: "https://checkout.paycom.uz"}
	res, err := svc.StartCheckout(ctx, profileID, "gentra", "payme", cfg, "ru", "", "HALFOFF")
	if err != nil {
		t.Fatal(err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	q := sqlc.New(tx)
	if err := q.MarkPaymentPaid(ctx, res.PaymentID); err != nil {
		t.Fatal(err)
	}
	if err := (Service{Q: q}).ProcessPaymentGrant(ctx, res.PaymentID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	var redemptions int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM promo_redemption WHERE profile_id = $1`, profileID).Scan(&redemptions); err != nil {
		t.Fatal(err)
	}
	if redemptions != 1 {
		t.Errorf("promo_redemption rows = %d, want 1", redemptions)
	}

	var days float64
	var note string
	if err := pool.QueryRow(ctx,
		`SELECT EXTRACT(EPOCH FROM (ends_at - starts_at)) / 86400, note
		 FROM entitlement WHERE profile_id = $1`, profileID).Scan(&days, &note); err != nil {
		t.Fatal(err)
	}
	if got := int(days + 0.5); got != 30 {
		t.Errorf("granted days = %d, want the full 30", got)
	}
	if note != "" {
		t.Errorf("note = %q, want empty (the promo was honored, nothing to explain)", note)
	}
}

// TestGrantDaysConcurrentSameProfile is the regression test for GrantDays being
// an unlocked read-modify-write. ActiveEntitlementEnd then InsertEntitlement
// share no row, so under READ COMMITTED two overlapping transactions both read
// the same pre-existing end and both write the same interval — the second
// grant's days vanish. An auditor measured exactly this: two grants of 7 days
// each landed as 7 effective days, not 14.
//
// Two goroutines, two real transactions, same profile. Both must commit and the
// stacked total must be the full 14 days.
func TestGrantDaysConcurrentSameProfile(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	profileID := uuid.New()

	if _, err := pool.Exec(ctx,
		`INSERT INTO profile (id, phone) VALUES ($1, '+998900000023')`, profileID); err != nil {
		t.Fatal(err)
	}

	const n = 2
	errs := make([]error, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			tx, err := pool.Begin(ctx)
			if err != nil {
				errs[i] = err
				return
			}
			defer func() { _ = tx.Rollback(ctx) }()
			svc := Service{Q: sqlc.New(tx)}
			if _, err := svc.GrantDays(ctx, profileID, 7, "referral", "concurrent grant", uuid.NullUUID{}); err != nil {
				errs[i] = err
				return
			}
			errs[i] = tx.Commit(ctx)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("grant %d: %v", i, err)
		}
	}

	var rows int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM entitlement WHERE profile_id = $1`, profileID).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != n {
		t.Fatalf("entitlement rows = %d, want %d", rows, n)
	}

	var effectiveDays float64
	if err := pool.QueryRow(ctx,
		`SELECT EXTRACT(EPOCH FROM (MAX(ends_at) - now())) / 86400
		 FROM entitlement WHERE profile_id = $1`, profileID).Scan(&effectiveDays); err != nil {
		t.Fatal(err)
	}
	// Allow a little slack for the wall-clock time the transactions take.
	if effectiveDays < 13.9 || effectiveDays > 14.1 {
		t.Errorf("effective entitlement = %.2f days, want ~14 (both grants must stack, not collapse)", effectiveDays)
	}
}

// TestReferralRewardTwoRefereesPayingConcurrently reproduces the auditor's
// exact scenario end-to-end: one referrer, two different referees, both first
// payments completing at the same moment. ClaimPendingReferralForReferee
// serializes per referee row, and two referees are two different rows, so
// nothing upstream serialized the shared GrantDays(referrer) — both referrals
// were permanently marked 'rewarded' while the referrer banked 7 days instead
// of 14. The profile lock inside GrantDays is what closes it.
func TestReferralRewardTwoRefereesPayingConcurrently(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()

	referrerID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO profile (id, phone) VALUES ($1, '+998900000030')`, referrerID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO tariff (code, days, price_uzs, sort_order, active)
		 VALUES ('gentra', 30, 59900, 1, true)`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO user_referral_code (user_id, code) VALUES ($1, 'REF-TEST01')`, referrerID); err != nil {
		t.Fatal(err)
	}

	const n = 2
	paymentIDs := make([]uuid.UUID, n)
	svc := Service{Q: sqlc.New(pool), Pool: pool}
	cfg := CheckoutConfig{PaymeMerchantID: "M1", PaymeCheckoutHost: "https://checkout.paycom.uz"}
	for i := 0; i < n; i++ {
		refereeID := uuid.New()
		if _, err := pool.Exec(ctx,
			`INSERT INTO profile (id, phone) VALUES ($1, $2)`,
			refereeID, "+99890000004"+string(rune('0'+i))); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO referral (referrer_id, referee_id, referral_code, status)
			 VALUES ($1, $2, 'REF-TEST01', 'pending')`, referrerID, refereeID); err != nil {
			t.Fatal(err)
		}
		res, err := svc.StartCheckout(ctx, refereeID, "gentra", "payme", cfg, "ru", "", "")
		if err != nil {
			t.Fatal(err)
		}
		paymentIDs[i] = res.PaymentID
	}

	errs := make([]error, n)
	var wg sync.WaitGroup
	wg.Add(n)
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			<-start
			tx, err := pool.Begin(ctx)
			if err != nil {
				errs[i] = err
				return
			}
			defer func() { _ = tx.Rollback(ctx) }()
			q := sqlc.New(tx)
			if err := q.MarkPaymentPaid(ctx, paymentIDs[i]); err != nil {
				errs[i] = err
				return
			}
			if err := (Service{Q: q}).ProcessPaymentGrant(ctx, paymentIDs[i]); err != nil {
				errs[i] = err
				return
			}
			errs[i] = tx.Commit(ctx)
		}(i)
	}
	close(start)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("payment %d: %v", i, err)
		}
	}

	var rewarded int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM referral WHERE referrer_id = $1 AND status = 'rewarded'`,
		referrerID).Scan(&rewarded); err != nil {
		t.Fatal(err)
	}
	if rewarded != n {
		t.Fatalf("rewarded referrals = %d, want %d", rewarded, n)
	}

	var referrerDays float64
	if err := pool.QueryRow(ctx,
		`SELECT COALESCE(EXTRACT(EPOCH FROM (MAX(ends_at) - now())) / 86400, 0)
		 FROM entitlement WHERE profile_id = $1`, referrerID).Scan(&referrerDays); err != nil {
		t.Fatal(err)
	}
	if referrerDays < 13.9 || referrerDays > 14.1 {
		t.Errorf("referrer entitlement = %.2f days, want ~14 (7 per rewarded referral, both must stack)", referrerDays)
	}
}

func TestProRatedDays(t *testing.T) {
	tests := []struct {
		name       string
		paid, list int64
		days, want int
	}{
		{"full price grants the full term", 59900, 59900, 30, 30},
		{"overpaid still grants the full term", 60000, 59900, 30, 30},
		{"half price grants half the days", 29950, 59900, 30, 15},
		{"floors rather than rounds up", 29949, 59900, 30, 14},
		{"never returns zero for a completed payment", 1, 109900, 75, 1},
		{"free tariff cannot be pro-rated", 0, 0, 7, 7},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := proRatedDays(tc.paid, tc.list, tc.days); got != tc.want {
				t.Errorf("proRatedDays(%d, %d, %d) = %d, want %d", tc.paid, tc.list, tc.days, got, tc.want)
			}
		})
	}
}
