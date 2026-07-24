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
	orderID := uuid.New().String()
	got := BuildPaymeURL("https://checkout.paycom.uz", "660000000000000000000000", orderID, 59900, "ru", "https://avtotest.uz/checkout/success")

	if !strings.HasPrefix(got, "https://checkout.paycom.uz/") {
		t.Fatalf("prefix = %q", got)
	}

	b64 := strings.TrimPrefix(got, "https://checkout.paycom.uz/")
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("decode b64: %v", err)
	}

	rawStr := string(raw)
	if !strings.Contains(rawStr, "m=660000000000000000000000") {
		t.Errorf("merchant id missing: %s", rawStr)
	}
	if !strings.Contains(rawStr, "ac.order_id="+orderID) {
		t.Errorf("order_id missing: %s", rawStr)
	}
	// 59900 UZS -> 5990000 tiyin
	if !strings.Contains(rawStr, "a=5990000") {
		t.Errorf("tiyin amount missing: %s", rawStr)
	}
	if !strings.Contains(rawStr, "l=ru") {
		t.Errorf("locale missing: %s", rawStr)
	}
	if !strings.Contains(rawStr, "c=https://avtotest.uz/checkout/success") {
		t.Errorf("callback missing: %s", rawStr)
	}
}

func TestBuildClickURL(t *testing.T) {
	orderID := uuid.New().String()
	got := BuildClickURL("12345", "67890", orderID, 59900, "https://avtotest.uz/checkout/success")

	if !strings.HasPrefix(got, "https://my.click.uz/services/pay?") {
		t.Fatalf("prefix = %q", got)
	}

	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}

	q := parsed.Query()
	cases := map[string]string{
		"service_id":        "12345",
		"merchant_id":       "67890",
		"amount":            "59900",
		"transaction_param": orderID,
		"return_url":        "https://avtotest.uz/checkout/success",
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
	profileID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO profile (id, phone) VALUES ($1, '+998901000001') ON CONFLICT (phone) DO UPDATE SET id = EXCLUDED.id`, profileID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO tariff (code, days, price_uzs, sort_order, active) VALUES ('gentra', 30, 59900, 1, true) ON CONFLICT (code) DO UPDATE SET active = true, price_uzs = 59900, days = 30`); err != nil {
		t.Fatal(err)
	}

	svc := Service{Q: sqlc.New(pool), Pool: pool}
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
	profileID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone) VALUES ($1, '+998901000002') ON CONFLICT (phone) DO UPDATE SET id = EXCLUDED.id`, profileID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO tariff (code, days, price_uzs, sort_order, active) VALUES ('gentra', 30, 59900, 1, true) ON CONFLICT (code) DO UPDATE SET active = true, price_uzs = 59900, days = 30`); err != nil {
		t.Fatal(err)
	}

	svc := Service{Q: sqlc.New(pool), Pool: pool}
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
	profileID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone) VALUES ($1, '+998901000003') ON CONFLICT (phone) DO UPDATE SET id = EXCLUDED.id`, profileID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO tariff (code, days, price_uzs, sort_order, active) VALUES ('gentra', 30, 59900, 1, true) ON CONFLICT (code) DO UPDATE SET active = true, price_uzs = 59900, days = 30`); err != nil {
		t.Fatal(err)
	}
	var promoID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO promo_code (code, kind, value, active) VALUES ('DISCOUNT10K', 'fixed', 10000, true) ON CONFLICT (code) DO UPDATE SET active = true, value = 10000 RETURNING id`).Scan(&promoID); err != nil {
		t.Fatal(err)
	}

	svc := Service{Q: sqlc.New(pool), Pool: pool}
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
	profileID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone) VALUES ($1, '+998901000004') ON CONFLICT (phone) DO UPDATE SET id = EXCLUDED.id`, profileID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO tariff (code, days, price_uzs, sort_order, active) VALUES ('gentra', 30, 59900, 1, true) ON CONFLICT (code) DO UPDATE SET active = true, price_uzs = 59900, days = 30`); err != nil {
		t.Fatal(err)
	}
	var promoID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO promo_code (code, kind, value, active) VALUES ('FREEPASS', 'days', 30, true) ON CONFLICT (code) DO UPDATE SET active = true RETURNING id`).Scan(&promoID); err != nil {
		t.Fatal(err)
	}

	svc := Service{Q: sqlc.New(pool), Pool: pool}
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
