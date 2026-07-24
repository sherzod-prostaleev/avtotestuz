package billing

import (
	"context"
	"encoding/base64"
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
	res, err := svc.StartCheckout(ctx, profileID, "gentra", "M1", "https://checkout.paycom.uz", "ru", "")
	if err != nil {
		t.Fatal(err)
	}

	// a payment row was created for this profile at the tariff price
	var amount int64
	var status string
	if err := pool.QueryRow(ctx,
		`SELECT amount_uzs, status FROM payment WHERE id = $1 AND profile_id = $2`,
		res.PaymentID, profileID).Scan(&amount, &status); err != nil {
		t.Fatal(err)
	}
	if amount != 59900 || status != "created" {
		t.Errorf("payment amount=%d status=%q, want 59900/created", amount, status)
	}

	// checkout URL encodes this payment id + tiyin amount
	dec, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(res.CheckoutURL, "https://checkout.paycom.uz/"))
	if !strings.Contains(string(dec), "ac.order_id="+res.PaymentID.String()) || !strings.Contains(string(dec), "a=5990000") {
		t.Errorf("checkout raw = %q", string(dec))
	}
}
