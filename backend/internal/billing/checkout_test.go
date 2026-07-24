package billing

import (
	"context"
	"encoding/base64"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

func TestBuildPaymeURL(t *testing.T) {
	got := BuildPaymeURL("https://checkout.paycom.uz/", "M1", "ORD", 59900, "ru", "")
	const prefix = "https://checkout.paycom.uz/"
	if !strings.HasPrefix(got, prefix) {
		t.Fatalf("url = %q, want prefix %q", got, prefix)
	}
	dec, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(got, prefix))
	if err != nil {
		t.Fatal(err)
	}
	// amount must be in tiyin (×100) and account uses order_id
	if string(dec) != "m=M1;ac.order_id=ORD;a=5990000;l=ru" {
		t.Errorf("decoded = %q", string(dec))
	}
}

func TestBuildClickURL(t *testing.T) {
	got := BuildClickURL("SID", "MID", "ORD", 59900, "https://app.example/return")
	const prefix = "https://my.click.uz/services/pay?"
	if !strings.HasPrefix(got, prefix) {
		t.Fatalf("url = %q, want prefix %q", got, prefix)
	}
	u, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	cases := map[string]string{
		"service_id":        "SID",
		"merchant_id":       "MID",
		"amount":            "59900",
		"transaction_param": "ORD",
		"return_url":        "https://app.example/return",
	}
	for k, want := range cases {
		if got := q.Get(k); got != want {
			t.Errorf("query[%q] = %q, want %q", k, got, want)
		}
	}
}

func TestStartCheckout(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	profileID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	if _, err := pool.Exec(ctx,
		`INSERT INTO profile (id, phone) VALUES ($1, '+998900000000')`, profileID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO tariff (code, days, price_uzs, sort_order, active) VALUES ('gentra', 30, 59900, 1, true)`); err != nil {
		t.Fatal(err)
	}

	svc := Service{Q: sqlc.New(pool)}
	res, err := svc.StartCheckout(ctx, profileID, "gentra", "payme", CheckoutConfig{PaymeMerchantID: "M1", PaymeCheckoutHost: "https://checkout.paycom.uz"}, "ru", "", "")
	if err != nil {
		t.Fatal(err)
	}

	// a payment row was created for this profile at the tariff price
	var amount int64
	var status string
	var provider string
	if err := pool.QueryRow(ctx,
		`SELECT amount_uzs, status, provider FROM payment WHERE id = $1 AND profile_id = $2`,
		res.PaymentID, profileID).Scan(&amount, &status, &provider); err != nil {
		t.Fatal(err)
	}
	if amount != 59900 || status != "created" || provider != "payme" {
		t.Errorf("payment amount=%d status=%q provider=%q, want 59900/created/payme", amount, status, provider)
	}

	// checkout URL encodes this payment id + tiyin amount
	dec, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(res.CheckoutURL, "https://checkout.paycom.uz/"))
	if !strings.Contains(string(dec), "ac.order_id="+res.PaymentID.String()) || !strings.Contains(string(dec), "a=5990000") {
		t.Errorf("checkout raw = %q", string(dec))
	}
}

func TestStartCheckoutClick(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	profileID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	if _, err := pool.Exec(ctx,
		`INSERT INTO profile (id, phone) VALUES ($1, '+998900000001')`, profileID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO tariff (code, days, price_uzs, sort_order, active) VALUES ('gentra', 30, 59900, 1, true)`); err != nil {
		t.Fatal(err)
	}

	svc := Service{Q: sqlc.New(pool)}
	res, err := svc.StartCheckout(ctx, profileID, "gentra", "click", CheckoutConfig{ClickServiceID: "S1", ClickMerchantID: "M1"}, "ru", "", "")
	if err != nil {
		t.Fatal(err)
	}

	var provider string
	if err := pool.QueryRow(ctx,
		`SELECT provider FROM payment WHERE id = $1 AND profile_id = $2`,
		res.PaymentID, profileID).Scan(&provider); err != nil {
		t.Fatal(err)
	}
	if provider != "click" {
		t.Errorf("payment provider = %q, want click", provider)
	}

	if !strings.Contains(res.CheckoutURL, "my.click.uz") {
		t.Errorf("checkout url = %q, want it to contain my.click.uz", res.CheckoutURL)
	}
	if !strings.Contains(res.CheckoutURL, "transaction_param="+res.PaymentID.String()) {
		t.Errorf("checkout url = %q, want transaction_param=%s", res.CheckoutURL, res.PaymentID.String())
	}
}

func TestStartCheckoutWithPromo(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	profileID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone) VALUES ($1, '+998900000003')`, profileID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO tariff (code, days, price_uzs, sort_order, active) VALUES ('gentra', 30, 59900, 1, true)`); err != nil {
		t.Fatal(err)
	}
	var promoID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO promo_code (code, kind, value, active) VALUES ('DISCOUNT10K', 'fixed', 10000, true) RETURNING id`).Scan(&promoID); err != nil {
		t.Fatal(err)
	}

	svc := Service{Q: sqlc.New(pool)}
	res, err := svc.StartCheckout(ctx, profileID, "gentra", "payme", CheckoutConfig{PaymeMerchantID: "M1", PaymeCheckoutHost: "https://checkout.paycom.uz"}, "ru", "", "DISCOUNT10K")
	if err != nil {
		t.Fatal(err)
	}

	var amount int64
	var status string
	var dbPromoID uuid.NullUUID
	if err := pool.QueryRow(ctx, `SELECT amount_uzs, status, promo_code_id FROM payment WHERE id = $1`, res.PaymentID).Scan(&amount, &status, &dbPromoID); err != nil {
		t.Fatal(err)
	}
	if amount != 49900 {
		t.Errorf("amount = %d, want 49900 (59900 - 10000)", amount)
	}
	if !dbPromoID.Valid || dbPromoID.UUID != promoID {
		t.Errorf("promo_code_id = %v, want %v", dbPromoID, promoID)
	}
}

func TestStartCheckoutZeroAmountFree(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	profileID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone) VALUES ($1, '+998900000004')`, profileID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO tariff (code, days, price_uzs, sort_order, active) VALUES ('gentra', 30, 59900, 1, true)`); err != nil {
		t.Fatal(err)
	}
	var promoID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO promo_code (code, kind, value, active) VALUES ('FREEPASS', 'days', 30, true) RETURNING id`).Scan(&promoID); err != nil {
		t.Fatal(err)
	}

	svc := Service{Q: sqlc.New(pool)}
	res, err := svc.StartCheckout(ctx, profileID, "gentra", "payme", CheckoutConfig{PaymeMerchantID: "M1", PaymeCheckoutHost: "https://checkout.paycom.uz"}, "ru", "", "FREEPASS")
	if err != nil {
		t.Fatal(err)
	}

	if !res.Free {
		t.Error("res.Free = false, want true")
	}
	if res.CheckoutURL != "" {
		t.Errorf("res.CheckoutURL = %q, want empty for free checkout", res.CheckoutURL)
	}

	// Verify payment marked paid
	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM payment WHERE id = $1`, res.PaymentID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "paid" {
		t.Errorf("payment status = %q, want paid", status)
	}

	// Verify entitlement active
	active, _, err := svc.Status(ctx, profileID)
	if err != nil || !active {
		t.Fatalf("svc.Status active=%v err=%v, want active=true", active, err)
	}

	// Verify promo redemption recorded
	var redemptionCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM promo_redemption WHERE promo_code_id = $1 AND profile_id = $2`, promoID, profileID).Scan(&redemptionCount); err != nil {
		t.Fatal(err)
	}
	if redemptionCount != 1 {
		t.Errorf("redemptionCount = %d, want 1", redemptionCount)
	}
}
