package billing

import (
	"testing"
	"time"
)

func TestParseHumoCardBotPush_AmountFromPlusNotBalance(t *testing.T) {
	raw := `🎉 To'ldirish
➕ 6.000,00 UZS
📍 CLICK P2P U2H ER>TAS
💳 HUMOCARD *4042
🕓 00:29 27.07.2026
💰 9.100,00 UZS`
	p, ok := ParseHumoCardBotPush(raw)
	if !ok {
		t.Fatal("expected parse ok")
	}
	if p.AmountUzs != 6000 {
		t.Fatalf("amount=%d want 6000 (from ➕, not 💰)", p.AmountUzs)
	}
	if p.PanLast4 != "4042" {
		t.Fatalf("last4=%q", p.PanLast4)
	}
	if !p.HasBalance || p.BalanceUzs != 9100 {
		t.Fatalf("balance=%d has=%v", p.BalanceUzs, p.HasBalance)
	}
	loc := time.FixedZone("UZT", 5*3600)
	want := time.Date(2026, 7, 27, 0, 29, 0, 0, loc)
	if !p.TransferAt.Equal(want) {
		t.Fatalf("transfer_at=%v want %v", p.TransferAt, want)
	}
}

func TestParseHumoCardBotPush_RejectsNonToldirish(t *testing.T) {
	raw := `🔹 HUMOCARD OCTOBANK *4042
💵 0.00 UZS`
	if _, ok := ParseHumoCardBotPush(raw); ok {
		t.Fatal("card list should not parse as deposit")
	}
}

func TestNormalizeCardPAN(t *testing.T) {
	full, last4, err := NormalizeCardPAN("9860 1234 5678 4042")
	if err != nil {
		t.Fatal(err)
	}
	if full != "9860123456784042" || last4 != "4042" {
		t.Fatalf("full=%s last4=%s", full, last4)
	}
}
