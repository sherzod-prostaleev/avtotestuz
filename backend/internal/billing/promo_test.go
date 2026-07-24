package billing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

func TestValidatePromoPercent(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	profileID := uuid.New()

	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone) VALUES ($1, '+998902000001') ON CONFLICT (phone) DO UPDATE SET id = EXCLUDED.id`, profileID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO tariff (code, days, price_uzs, sort_order, active) VALUES ('gentra', 30, 59900, 1, true) ON CONFLICT (code) DO UPDATE SET active = true, price_uzs = 59900, days = 30`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO promo_code (code, kind, value, active) VALUES ('SUMMER20', 'percent', 20, true) ON CONFLICT (code) DO UPDATE SET active = true, value = 20`); err != nil {
		t.Fatal(err)
	}

	svc := Service{Q: sqlc.New(pool)}
	res, err := svc.ValidatePromo(ctx, profileID, "summer20", "gentra")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Code != "SUMMER20" {
		t.Errorf("got code %q, want SUMMER20", res.Code)
	}
	if res.DiscountUzs != 11980 {
		t.Errorf("got discount %d, want 11980", res.DiscountUzs)
	}
	if res.FinalAmountUzs != 47920 {
		t.Errorf("got final amount %d, want 47920", res.FinalAmountUzs)
	}
}

func TestValidatePromoFixed(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	profileID := uuid.New()

	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone) VALUES ($1, '+998902000002') ON CONFLICT (phone) DO UPDATE SET id = EXCLUDED.id`, profileID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO tariff (code, days, price_uzs, sort_order, active) VALUES ('gentra', 30, 59900, 1, true) ON CONFLICT (code) DO UPDATE SET active = true, price_uzs = 59900, days = 30`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO promo_code (code, kind, value, active) VALUES ('FIXED10K', 'fixed', 10000, true) ON CONFLICT (code) DO UPDATE SET active = true, value = 10000`); err != nil {
		t.Fatal(err)
	}

	svc := Service{Q: sqlc.New(pool)}
	res, err := svc.ValidatePromo(ctx, profileID, "FIXED10K", "gentra")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.DiscountUzs != 10000 {
		t.Errorf("got discount %d, want 10000", res.DiscountUzs)
	}
	if res.FinalAmountUzs != 49900 {
		t.Errorf("got final amount %d, want 49900", res.FinalAmountUzs)
	}
}

func TestValidatePromoDays(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	profileID := uuid.New()

	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone) VALUES ($1, '+998902000003') ON CONFLICT (phone) DO UPDATE SET id = EXCLUDED.id`, profileID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO tariff (code, days, price_uzs, sort_order, active) VALUES ('gentra', 30, 59900, 1, true) ON CONFLICT (code) DO UPDATE SET active = true, price_uzs = 59900, days = 30`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO promo_code (code, kind, value, active) VALUES ('FREE7DAYS', 'days', 7, true) ON CONFLICT (code) DO UPDATE SET active = true, value = 7`); err != nil {
		t.Fatal(err)
	}

	svc := Service{Q: sqlc.New(pool)}
	res, err := svc.ValidatePromo(ctx, profileID, "FREE7DAYS", "gentra")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.FinalAmountUzs != 0 {
		t.Errorf("got final amount %d, want 0", res.FinalAmountUzs)
	}
	if res.BonusDays != 7 {
		t.Errorf("got bonus days %d, want 7", res.BonusDays)
	}
}

func TestValidatePromoNotFoundAndExpired(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	profileID := uuid.New()

	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone) VALUES ($1, '+998902000004') ON CONFLICT (phone) DO UPDATE SET id = EXCLUDED.id`, profileID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO tariff (code, days, price_uzs, sort_order, active) VALUES ('gentra', 30, 59900, 1, true) ON CONFLICT (code) DO UPDATE SET active = true, price_uzs = 59900, days = 30`); err != nil {
		t.Fatal(err)
	}

	svc := Service{Q: sqlc.New(pool)}

	// Not found
	_, err := svc.ValidatePromo(ctx, profileID, "NONEXISTENT", "gentra")
	if !errors.Is(err, ErrPromoNotFound) {
		t.Errorf("got error %v, want ErrPromoNotFound", err)
	}

	// Expired
	past := time.Now().Add(-24 * time.Hour)
	if _, err := pool.Exec(ctx, `INSERT INTO promo_code (code, kind, value, valid_to, active) VALUES ('EXPIRED', 'percent', 10, $1, true) ON CONFLICT (code) DO UPDATE SET valid_to = EXCLUDED.valid_to, active = true`, past); err != nil {
		t.Fatal(err)
	}

	_, err = svc.ValidatePromo(ctx, profileID, "EXPIRED", "gentra")
	if !errors.Is(err, ErrPromoExpired) {
		t.Errorf("got error %v, want ErrPromoExpired", err)
	}
}

func TestValidatePromoLimits(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	profileID1 := uuid.New()
	profileID2 := uuid.New()

	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone) VALUES ($1, '+998902000005'), ($2, '+998902000006') ON CONFLICT (phone) DO UPDATE SET id = EXCLUDED.id`, profileID1, profileID2); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO tariff (code, days, price_uzs, sort_order, active) VALUES ('gentra', 30, 59900, 1, true) ON CONFLICT (code) DO UPDATE SET active = true, price_uzs = 59900, days = 30`); err != nil {
		t.Fatal(err)
	}

	// Promo with max_uses = 1
	var promoID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO promo_code (code, kind, value, max_uses, per_user_limit, active) VALUES ('MAX1', 'fixed', 5000, 1, 1, true) ON CONFLICT (code) DO UPDATE SET max_uses = 1, active = true RETURNING id`).Scan(&promoID); err != nil {
		t.Fatal(err)
	}

	svc := Service{Q: sqlc.New(pool)}

	// User 1 uses promo MAX1 once
	if _, err := pool.Exec(ctx, `INSERT INTO promo_redemption (promo_code_id, profile_id) VALUES ($1, $2)`, promoID, profileID1); err != nil {
		t.Fatal(err)
	}

	// User 1 tries again -> ErrPromoUserLimitReached
	_, err := svc.ValidatePromo(ctx, profileID1, "MAX1", "gentra")
	if !errors.Is(err, ErrPromoUserLimitReached) {
		t.Errorf("user 1 got error %v, want ErrPromoUserLimitReached", err)
	}

	// User 2 tries -> ErrPromoLimitReached (because total max_uses = 1 reached)
	_, err = svc.ValidatePromo(ctx, profileID2, "MAX1", "gentra")
	if !errors.Is(err, ErrPromoLimitReached) {
		t.Errorf("user 2 got error %v, want ErrPromoLimitReached", err)
	}
}
