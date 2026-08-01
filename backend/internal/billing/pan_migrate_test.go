package billing

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/testdb"
)

func TestEncryptStoredPANsPreservesRowsAndIsIdempotent(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	ctx := context.Background()
	secret := []byte("test-pan-migration-secret-at-least-32-bytes")

	profileID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone) VALUES ($1, '+998901400001')`, profileID); err != nil {
		t.Fatal(err)
	}
	cardID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO manual_pay_card (id, pan_full, pan_last4, holder_name, sort_order, enabled)
		VALUES ($1, '9860123456784042', '4042', 'Migration Test', 1, true)`, cardID); err != nil {
		t.Fatal(err)
	}
	payoutID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO referral_payout (id, profile_id, amount_uzs, card_number, card_network)
		VALUES ($1, $2, 10000, '8600123456789012', 'uzcard')`, payoutID, profileID); err != nil {
		t.Fatal(err)
	}

	result, err := EncryptStoredPANs(ctx, pool, secret)
	if err != nil {
		t.Fatal(err)
	}
	if result.ManualCards != 1 || result.ReferralPayouts != 1 {
		t.Fatalf("result=%+v want 1/1", result)
	}

	var storedCard, storedPayout string
	if err := pool.QueryRow(ctx, `SELECT pan_full FROM manual_pay_card WHERE id=$1`, cardID).Scan(&storedCard); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT card_number FROM referral_payout WHERE id=$1`, payoutID).Scan(&storedPayout); err != nil {
		t.Fatal(err)
	}
	for label, stored := range map[string]string{"manual": storedCard, "payout": storedPayout} {
		if !strings.HasPrefix(stored, panCipherPrefix) {
			t.Fatalf("%s PAN was not encrypted: %q", label, stored)
		}
		if strings.Contains(stored, "12345678") {
			t.Fatalf("%s ciphertext contains plaintext PAN fragment", label)
		}
	}
	svc := Service{Secret: secret}
	if got, err := svc.DecryptPAN(storedCard); err != nil || got != "9860123456784042" {
		t.Fatalf("manual decrypt=%q err=%v", got, err)
	}
	if got, err := svc.DecryptPAN(storedPayout); err != nil || got != "8600123456789012" {
		t.Fatalf("payout decrypt=%q err=%v", got, err)
	}

	second, err := EncryptStoredPANs(ctx, pool, secret)
	if err != nil {
		t.Fatal(err)
	}
	if second.ManualCards != 0 || second.ReferralPayouts != 0 {
		t.Fatalf("second run changed encrypted rows: %+v", second)
	}
	var cardCount, payoutCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM manual_pay_card WHERE id=$1`, cardID).Scan(&cardCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM referral_payout WHERE id=$1`, payoutID).Scan(&payoutCount); err != nil {
		t.Fatal(err)
	}
	if cardCount != 1 || payoutCount != 1 {
		t.Fatalf("rows were not preserved: cards=%d payouts=%d", cardCount, payoutCount)
	}
}

func TestEncryptStoredPANsRollsBackAllRowsOnInvalidLegacyPAN(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	ctx := context.Background()
	profileID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone) VALUES ($1, '+998901400002')`, profileID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO manual_pay_card (pan_full, pan_last4, holder_name, sort_order, enabled)
		VALUES ('9860123456784042', '4042', 'Valid', 1, true)`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO referral_payout (profile_id, amount_uzs, card_number, card_network)
		VALUES ($1, 10000, 'invalid-pan', 'uzcard')`, profileID); err != nil {
		t.Fatal(err)
	}

	if _, err := EncryptStoredPANs(ctx, pool, []byte("test-pan-migration-secret-at-least-32-bytes")); err == nil {
		t.Fatal("expected invalid legacy PAN to abort migration")
	}
	var stored string
	if err := pool.QueryRow(ctx, `SELECT pan_full FROM manual_pay_card WHERE pan_last4='4042'`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != "9860123456784042" {
		t.Fatalf("transaction did not roll back, stored=%q", stored)
	}
}
