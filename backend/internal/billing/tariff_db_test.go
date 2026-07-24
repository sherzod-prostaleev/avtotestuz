package billing

import (
	"context"
	"testing"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

// testdb.Truncate wipes the migration seed, so ListTariffs behaviour is
// verified against a test-owned fixture inserted below.
func TestListTariffs(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()

	_, err := pool.Exec(ctx, `
		INSERT INTO tariff (code, days, price_uzs, old_price_uzs, badge, sort_order, active) VALUES
			('t7',  7,  24900, 34900, NULL,      1, true),
			('t30', 30, 59900, 99900, 'popular', 2, true),
			('t0',  30, 50000, NULL,  NULL,      3, true),
			('tOff',10, 10000, NULL,  NULL,      0, false);
		INSERT INTO tariff_translation (tariff_id, locale, name, description)
		SELECT t.id, v.locale, v.name, v.description FROM tariff t JOIN (VALUES
			('t7', 'uz-Latn'::locale_code, 'Seven',  'desc7 uz'),
			('t30','uz-Latn'::locale_code, 'Thirty', 'desc30 uz'),
			('t30','ru'::locale_code,      'Thirty', 'desc30 ru'),
			('t0', 'uz-Latn'::locale_code, 'Zero',   'desc0 uz')
		) AS v(code, locale, name, description) ON v.code = t.code;`)
	if err != nil {
		t.Fatal(err)
	}

	svc := Service{Q: sqlc.New(pool)}

	got, err := svc.ListTariffs(ctx, "uz-Latn")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 { // tOff is inactive → excluded
		t.Fatalf("got %d tariffs, want 3 (active only)", len(got))
	}
	if got[0].Code != "t7" || got[1].Code != "t30" || got[2].Code != "t0" {
		t.Fatalf("wrong sort_order: %s %s %s", got[0].Code, got[1].Code, got[2].Code)
	}

	// t7: computed fields + no badge
	if got[0].PricePerDayUZS != 3557 || got[0].DiscountPercent != 29 {
		t.Errorf("t7 computed wrong: perDay=%d disc=%d", got[0].PricePerDayUZS, got[0].DiscountPercent)
	}
	if got[0].Badge != nil {
		t.Errorf("t7 badge = %v, want nil", got[0].Badge)
	}
	if got[0].OldPriceUZS == nil || *got[0].OldPriceUZS != 34900 {
		t.Errorf("t7 old price = %v, want 34900", got[0].OldPriceUZS)
	}

	// t30: badge + discount
	if got[1].DiscountPercent != 40 || got[1].Badge == nil || *got[1].Badge != "popular" {
		t.Errorf("t30 wrong: disc=%d badge=%v", got[1].DiscountPercent, got[1].Badge)
	}

	// t0: null old price → nil + 0% discount
	if got[2].OldPriceUZS != nil || got[2].DiscountPercent != 0 {
		t.Errorf("t0 wrong: old=%v disc=%d", got[2].OldPriceUZS, got[2].DiscountPercent)
	}

	// ru locale: t30 has ru, t7 falls back to uz-Latn
	ru, err := svc.ListTariffs(ctx, "ru")
	if err != nil {
		t.Fatal(err)
	}
	if ru[1].Description != "desc30 ru" {
		t.Errorf("t30 ru desc = %q, want 'desc30 ru'", ru[1].Description)
	}
	if ru[0].Description != "desc7 uz" {
		t.Errorf("t7 ru desc = %q, want uz-Latn fallback 'desc7 uz'", ru[0].Description)
	}

	// unknown locale (kaa): everything falls back to uz-Latn
	kaa, err := svc.ListTariffs(ctx, "kaa")
	if err != nil {
		t.Fatal(err)
	}
	if kaa[1].Description != "desc30 uz" {
		t.Errorf("t30 kaa fallback = %q, want 'desc30 uz'", kaa[1].Description)
	}
}
