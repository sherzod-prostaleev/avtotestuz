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

// TestStartCheckoutConcurrentPromoRedemption is the regression test for the
// confirmed critical bug: StartCheckout's promo validation used to run a
// plain, non-transactional, non-locking SELECT COUNT(*) to check
// max_uses/per_user_limit, so N concurrent requests against the same
// max_uses=1/per_user_limit=1 promo code could all read "0 used" and all
// pass the check before any of them committed a redemption row. A reviewer
// proved this exploitable: 10 concurrent StartCheckout calls against a
// one-time promo yielded 10/10 successful free-VIP redemptions for one
// profile.
//
// This fires N=10 real concurrent goroutines, each calling StartCheckout
// against the shared test pool for the SAME profile and the SAME
// max_uses=1/per_user_limit=1 promo code (kind="days", 100% off — the
// zero-amount branch that immediately marks the payment paid and grants
// entitlement, exactly the path the reviewer exploited). With the fix
// (promo_code row locked via SELECT ... FOR UPDATE for the whole
// validate+redeem transaction), exactly one call may succeed; every other
// call must fail with a promo-limit domain error, and exactly one
// promo_redemption row (and one entitlement row) must exist afterward.
func TestStartCheckoutConcurrentPromoRedemption(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	profileID := uuid.MustParse("55555555-5555-5555-5555-555555555555")

	if _, err := pool.Exec(ctx,
		`INSERT INTO profile (id, phone) VALUES ($1, '+998900000005')`, profileID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO tariff (code, days, price_uzs, sort_order, active) VALUES ('gentra', 30, 59900, 1, true)`); err != nil {
		t.Fatal(err)
	}
	var promoID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO promo_code (code, kind, value, max_uses, per_user_limit, active)
		 VALUES ('ONETIME', 'days', 30, 1, 1, true) RETURNING id`).Scan(&promoID); err != nil {
		t.Fatal(err)
	}

	const n = 10
	svc := Service{Q: sqlc.New(pool), Pool: pool}

	results := make([]CheckoutResult, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			cfg := CheckoutConfig{PaymeMerchantID: "M1", PaymeCheckoutHost: "https://checkout.paycom.uz"}
			results[i], errs[i] = svc.StartCheckout(ctx, profileID, "gentra", "payme", cfg, "ru", "", "ONETIME")
		}(i)
	}
	wg.Wait()

	successes := 0
	for i := 0; i < n; i++ {
		if errs[i] == nil {
			successes++
			if !results[i].Free {
				t.Errorf("call %d: succeeded but Free=false", i)
			}
			continue
		}
		if !errors.Is(errs[i], ErrPromoLimitReached) && !errors.Is(errs[i], ErrPromoUserLimitReached) {
			t.Errorf("call %d: unexpected error %v, want ErrPromoLimitReached or ErrPromoUserLimitReached", i, errs[i])
		}
	}
	if successes != 1 {
		t.Errorf("successes = %d, want exactly 1", successes)
	}

	var redemptionCount int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM promo_redemption WHERE promo_code_id = $1 AND profile_id = $2`,
		promoID, profileID).Scan(&redemptionCount); err != nil {
		t.Fatal(err)
	}
	if redemptionCount != 1 {
		t.Errorf("promo_redemption rows = %d, want exactly 1", redemptionCount)
	}

	var entitlementCount int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM entitlement WHERE profile_id = $1`, profileID).Scan(&entitlementCount); err != nil {
		t.Fatal(err)
	}
	if entitlementCount != 1 {
		t.Errorf("entitlement rows = %d, want exactly 1 (no double-grant)", entitlementCount)
	}

	var paidPayments int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM payment WHERE profile_id = $1 AND promo_code_id = $2 AND status = 'paid'`,
		profileID, promoID).Scan(&paidPayments); err != nil {
		t.Fatal(err)
	}
	if paidPayments != 1 {
		t.Errorf("paid payments referencing the promo = %d, want exactly 1", paidPayments)
	}
}

// TestStartCheckoutConcurrentPromoRedemption_MultiUser is the multi-user
// variant of the same race: max_uses=1 but per_user_limit=5, so the limit
// that must stop the race is the total-use cap, not the per-user cap. 10
// different profiles fire StartCheckout concurrently against the same
// code; exactly one may win.
func TestStartCheckoutConcurrentPromoRedemption_MultiUser(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()

	if _, err := pool.Exec(ctx,
		`INSERT INTO tariff (code, days, price_uzs, sort_order, active) VALUES ('gentra', 30, 59900, 1, true)`); err != nil {
		t.Fatal(err)
	}
	var promoID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO promo_code (code, kind, value, max_uses, per_user_limit, active)
		 VALUES ('ONETIMEMULTI', 'days', 30, 1, 5, true) RETURNING id`).Scan(&promoID); err != nil {
		t.Fatal(err)
	}

	const n = 10
	profileIDs := make([]uuid.UUID, n)
	for i := 0; i < n; i++ {
		profileIDs[i] = uuid.New()
		if _, err := pool.Exec(ctx,
			`INSERT INTO profile (id, phone) VALUES ($1, $2)`,
			profileIDs[i], "+99890000100"+string(rune('0'+i))); err != nil {
			t.Fatal(err)
		}
	}

	svc := Service{Q: sqlc.New(pool), Pool: pool}
	results := make([]CheckoutResult, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			cfg := CheckoutConfig{PaymeMerchantID: "M1", PaymeCheckoutHost: "https://checkout.paycom.uz"}
			results[i], errs[i] = svc.StartCheckout(ctx, profileIDs[i], "gentra", "payme", cfg, "ru", "", "ONETIMEMULTI")
		}(i)
	}
	wg.Wait()

	successes := 0
	for i := 0; i < n; i++ {
		if errs[i] == nil {
			successes++
			if !results[i].Free {
				t.Errorf("call %d: succeeded but Free=false", i)
			}
			continue
		}
		if !errors.Is(errs[i], ErrPromoLimitReached) && !errors.Is(errs[i], ErrPromoUserLimitReached) {
			t.Errorf("call %d: unexpected error %v, want ErrPromoLimitReached or ErrPromoUserLimitReached", i, errs[i])
		}
	}
	if successes != 1 {
		t.Errorf("successes = %d, want exactly 1", successes)
	}

	var redemptionCount int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM promo_redemption WHERE promo_code_id = $1`, promoID).Scan(&redemptionCount); err != nil {
		t.Fatal(err)
	}
	if redemptionCount != 1 {
		t.Errorf("promo_redemption rows = %d, want exactly 1", redemptionCount)
	}
}
