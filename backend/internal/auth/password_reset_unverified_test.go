package auth

import (
	"context"
	"testing"

	"avtotest.uz/backend/internal/db/sqlc"
)

func phoneVerified(t *testing.T, svc *Service, tgID int64) bool {
	t.Helper()
	var ok bool
	if err := svc.Pool.QueryRow(context.Background(),
		`SELECT phone_verified_at IS NOT NULL FROM telegram_account WHERE tg_user_id=$1`, tgID).Scan(&ok); err != nil {
		t.Fatal(err)
	}
	return ok
}

// Account takeover through the bot reset: a legacy (/start <token>) link has
// no phone proof and an intruder can plant one on the victim's profile. It
// must not earn the linked shortcut; the intruder has to prove the phone.
func TestBeginPasswordResetIgnoresPlantedLegacyLink(t *testing.T) {
	svc, _ := resetTestService(t)
	ctx := context.Background()
	const attackerTg = 9101
	victim, err := svc.Register(ctx, RegisterInput{Phone: "901160001", Password: "victimpass1", Name: "V"})
	if err != nil {
		t.Fatal(err)
	}
	legacyLink(t, svc, victim.Profile.ID, attackerTg)

	start, err := svc.StartPasswordReset(ctx, "901160001", "6.6.6.6", "AvtoTestBot")
	if err != nil {
		t.Fatal(err)
	}
	raw := parseResetRaw(t, start.BotURL)

	begin, err := svc.BeginTelegramPasswordReset(ctx, raw, attackerTg)
	if err != nil {
		t.Fatal(err)
	}
	if begin.Outcome != TelegramResetNeedContact || begin.ConfirmNonce != "" {
		t.Fatalf("planted legacy link got %+v, want need_contact and no nonce", begin)
	}
	// The attacker can only share their own number.
	res, err := svc.ConfirmTelegramPasswordResetContact(ctx, attackerTg, attackerTg, "+998901160099")
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != TelegramResetInvalid || res.ConfirmNonce != "" {
		t.Fatalf("attacker contact=%+v want invalid", res)
	}
	assertResetState(t, svc, raw, ResetStatePending)
	if phoneVerified(t, svc, attackerTg) {
		t.Fatal("attacker's link became phone-verified")
	}
}

// The real owner holding a legacy link goes through the contact step, and
// that proof upgrades the link.
func TestBeginPasswordResetUnverifiedOwnerProvesPhoneThenLinkIsVerified(t *testing.T) {
	svc, _ := resetTestService(t)
	ctx := context.Background()
	const tg = 9102
	owner, err := svc.Register(ctx, RegisterInput{Phone: "901160002", Password: "ownerpass1", Name: "O"})
	if err != nil {
		t.Fatal(err)
	}
	legacyLink(t, svc, owner.Profile.ID, tg)
	start, err := svc.StartPasswordReset(ctx, "901160002", "6.6.6.7", "AvtoTestBot")
	if err != nil {
		t.Fatal(err)
	}
	raw := parseResetRaw(t, start.BotURL)
	if b, err := svc.BeginTelegramPasswordReset(ctx, raw, tg); err != nil || b.Outcome != TelegramResetNeedContact {
		t.Fatalf("begin=%+v err=%v want need_contact", b, err)
	}
	c, err := svc.ConfirmTelegramPasswordResetContact(ctx, tg, tg, "+998901160002")
	if err != nil || c.Outcome != TelegramResetNeedConfirm {
		t.Fatalf("contact=%+v err=%v", c, err)
	}
	if y, err := svc.AnswerTelegramPasswordResetConfirm(ctx, tg, c.ConfirmNonce, true); err != nil || y.Outcome != TelegramResetVerified {
		t.Fatalf("yes=%+v err=%v", y, err)
	}
	if !phoneVerified(t, svc, tg) {
		t.Fatal("contact path did not upgrade the legacy link")
	}
}

// A Telegram user linked to a different profile gets no shortcut either.
func TestBeginPasswordResetLinkToOtherProfileNeedsContact(t *testing.T) {
	svc, q := resetTestService(t)
	ctx := context.Background()
	victim, err := svc.Register(ctx, RegisterInput{Phone: "901160003", Password: "victimpass1", Name: "V"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := svc.Register(ctx, RegisterInput{Phone: "901160004", Password: "otherpass12", Name: "X"})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.UpsertTelegramAccount(ctx, sqlc.UpsertTelegramAccountParams{
		ProfileID: other.Profile.ID, TgUserID: 9103, Username: "x", PhoneVerified: true,
	}); err != nil {
		t.Fatal(err)
	}
	_ = victim
	start, err := svc.StartPasswordReset(ctx, "901160003", "6.6.6.8", "AvtoTestBot")
	if err != nil {
		t.Fatal(err)
	}
	raw := parseResetRaw(t, start.BotURL)
	if b, err := svc.BeginTelegramPasswordReset(ctx, raw, 9103); err != nil || b.Outcome != TelegramResetNeedContact {
		t.Fatalf("begin=%+v err=%v want need_contact", b, err)
	}
	assertResetState(t, svc, raw, ResetStatePending)
}
