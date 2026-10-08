package auth

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/db/sqlc"
)

// Regression tests for audit-2 C1: only a link whose phone Telegram itself
// vouched for may sign in to the Mini App without a password, and a password
// reset drops every link it cannot attribute to the Telegram user who
// confirmed it.

func phoneVerifiedOf(t *testing.T, svc *Service, profileID uuid.UUID) (tgID int64, verified, ok bool) {
	t.Helper()
	tgID, ok = tgLinkOf(t, svc, profileID)
	if !ok {
		return 0, false, false
	}
	if err := svc.Pool.QueryRow(context.Background(),
		`SELECT phone_verified_at IS NOT NULL FROM telegram_account WHERE profile_id=$1`, profileID).Scan(&verified); err != nil {
		t.Fatal(err)
	}
	return tgID, verified, true
}

func activeSessions(t *testing.T, svc *Service, profileID uuid.UUID) int {
	t.Helper()
	var n int
	if err := svc.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*)::int FROM refresh_token WHERE profile_id=$1 AND revoked_at IS NULL`, profileID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// legacyLink is what bot.LinkService.RedeemLinkToken writes for /start
// <token>: a link with no phone proof (bot imports auth, so the test writes
// the same row through the same query).
func legacyLink(t *testing.T, svc *Service, profileID uuid.UUID, tgID int64) {
	t.Helper()
	if err := svc.Q.UpsertTelegramAccount(context.Background(), sqlc.UpsertTelegramAccountParams{
		ProfileID: profileID, TgUserID: tgID, Username: "legacy",
	}); err != nil {
		t.Fatal(err)
	}
}

// Audit scenario A: an attacker mints a link token on their OWN profile and
// gets the victim to open it, binding the victim's Telegram to the attacker's
// account. The victim's Mini App must not sign them in there (they would pay
// VIP into the attacker's account); it asks for the phone instead.
func TestLegacyLinkDoesNotSignInToMiniApp(t *testing.T) {
	svc, ctx := newWebAppService(t)
	attacker, err := svc.Register(ctx, RegisterInput{Phone: "+998901120001", Password: "attacker-pass-1", Name: "A"})
	if err != nil {
		t.Fatal(err)
	}
	const victimTg = 6001
	legacyLink(t, svc, attacker.Profile.ID, victimTg)
	before := activeSessions(t, svc, attacker.Profile.ID)

	res, err := svc.TelegramWebAppLogin(ctx, signInitData(t, testBotToken, webAppFields(victimTg, time.Now())), "")
	if err != nil {
		t.Fatal(err)
	}
	if !res.NeedPhone || res.Access != "" || res.Refresh != "" {
		t.Fatalf("legacy link signed in: %+v", res)
	}
	if got := activeSessions(t, svc, attacker.Profile.ID); got != before {
		t.Fatalf("sessions %d -> %d: a legacy link issued a session", before, got)
	}
	// The row itself stays: bot digests and /status keep working off it.
	if _, ok := tgLinkOf(t, svc, attacker.Profile.ID); !ok {
		t.Fatal("need_phone must not delete the legacy link")
	}
}

// A signed phone share in the Mini App upgrades an existing legacy link of the
// same Telegram user, after which auto-login works.
func TestPhoneShareUpgradesLegacyLink(t *testing.T) {
	svc, ctx := newWebAppService(t)
	const phone, pw = "+998901120002", "upgrade-pass-1"
	reg, err := svc.Register(ctx, RegisterInput{Phone: phone, Password: pw, Name: "U"})
	if err != nil {
		t.Fatal(err)
	}
	legacyLink(t, svc, reg.Profile.ID, 6002)
	raw, contact := proof(t, 6002, phone)
	if login, err := svc.Login(ctx, LoginInput{Phone: phone, Password: pw, TgInitData: raw, TgContact: contact}); err != nil || !login.TelegramLinked {
		t.Fatalf("login: %+v %v", login, err)
	}
	if tg, verified, ok := phoneVerifiedOf(t, svc, reg.Profile.ID); !ok || tg != 6002 || !verified {
		t.Fatalf("link = %d verified=%v ok=%v", tg, verified, ok)
	}
	res, err := svc.TelegramWebAppLogin(ctx, raw, "")
	if err != nil || res.NeedPhone || res.Access == "" {
		t.Fatalf("webapp after proof: %+v %v", res, err)
	}
}

// A legacy re-point of a verified row to ANOTHER Telegram user must drop the
// proof (it was about the previous account); the same user keeps it.
func TestUnprovenUpsertKeepsProofOnlyForSameTelegramUser(t *testing.T) {
	svc, ctx := newWebAppService(t)
	const phone = "+998901120003"
	raw, contact := proof(t, 6003, phone)
	reg, err := svc.Register(ctx, RegisterInput{Phone: phone, Password: "repoint-pass-1", Name: "R", TgInitData: raw, TgContact: contact})
	if err != nil || !reg.TelegramLinked {
		t.Fatalf("register: %+v %v", reg, err)
	}
	legacyLink(t, svc, reg.Profile.ID, 6003)
	if _, verified, _ := phoneVerifiedOf(t, svc, reg.Profile.ID); !verified {
		t.Fatal("re-linking the same Telegram user dropped its phone proof")
	}
	legacyLink(t, svc, reg.Profile.ID, 6004)
	if tg, verified, _ := phoneVerifiedOf(t, svc, reg.Profile.ID); tg != 6004 || verified {
		t.Fatalf("re-pointed link tg=%d verified=%v, want 6004 unverified", tg, verified)
	}
}

// The bot reset's contact path (matching contact + «Ha, men») is Telegram's
// proof of the phone too: the link it writes is phone-verified, signs in to
// the Mini App, and survives the reset it confirmed.
func TestBotResetContactLinkIsVerifiedAndSurvivesReset(t *testing.T) {
	svc, ctx := newWebAppService(t)
	raw, nonce := contactMatchedReset(t, svc, "901120004", 6005, "+998901120004")
	if res, err := svc.AnswerTelegramPasswordResetConfirm(ctx, 6005, nonce, true); err != nil || res.Outcome != TelegramResetVerified {
		t.Fatalf("yes=%+v %v", res, err)
	}
	if err := svc.CompletePasswordReset(ctx, raw, "brandnew-pass-1", "4.4.4.4"); err != nil {
		t.Fatal(err)
	}
	profile, err := svc.Q.GetProfileByPhone(ctx, "+998901120004")
	if err != nil {
		t.Fatal(err)
	}
	if tg, verified, ok := phoneVerifiedOf(t, svc, profile.ID); !ok || tg != 6005 || !verified {
		t.Fatalf("after reset link=%d verified=%v ok=%v", tg, verified, ok)
	}
	res, err := svc.TelegramWebAppLogin(ctx, signInitData(t, testBotToken, webAppFields(6005, time.Now())), "")
	if err != nil || res.NeedPhone || res.Profile.ID != profile.ID {
		t.Fatalf("webapp after reset: %+v %v", res, err)
	}
}

// linkedReset starts a reset for profile and has tgID confirm it: through the
// linked-account shortcut when its link is phone-verified, otherwise through
// the contact step (which links tgID with the phone proven).
func linkedReset(t *testing.T, svc *Service, phone string, tgID int64) string {
	t.Helper()
	ctx := context.Background()
	// Tests start several resets for one phone back to back; the 45 s
	// per-phone cooldown is not what they are about.
	normalized, err := NormalizePhone(phone)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Lim.R.Del(ctx, "reset:cooldown:"+normalized).Err(); err != nil {
		t.Fatal(err)
	}
	start, err := svc.StartPasswordReset(ctx, phone, "5.5.5.5", "AvtoTestBot")
	if err != nil {
		t.Fatal(err)
	}
	raw := parseResetRaw(t, start.BotURL)
	begin, err := svc.BeginTelegramPasswordReset(ctx, raw, tgID)
	if err == nil && begin.Outcome == TelegramResetNeedContact {
		begin, err = svc.ConfirmTelegramPasswordResetContact(ctx, tgID, tgID, phone)
	}
	if err != nil || begin.Outcome != TelegramResetNeedConfirm {
		t.Fatalf("begin=%+v %v", begin, err)
	}
	if res, err := svc.AnswerTelegramPasswordResetConfirm(ctx, tgID, begin.ConfirmNonce, true); err != nil || res.Outcome != TelegramResetVerified {
		t.Fatalf("yes=%+v %v", res, err)
	}
	return raw
}

// Audit scenario B: someone with brief access to a session links their own
// Telegram the legacy way, after the owner confirmed the reset. A password
// reset is the owner taking the account back, so it must not leave that link
// behind for the Mini App.
func TestPasswordResetDropsUnverifiedLink(t *testing.T) {
	svc, ctx := newWebAppService(t)
	const phone = "+998901120005"
	reg, err := svc.Register(ctx, RegisterInput{Phone: phone, Password: "owner-pass-1", Name: "O"})
	if err != nil {
		t.Fatal(err)
	}
	const ownerTg, intruderTg = 6005, 6006
	raw := linkedReset(t, svc, phone, ownerTg)
	legacyLink(t, svc, reg.Profile.ID, intruderTg)
	if err := svc.CompletePasswordReset(ctx, raw, "owner-new-pass-1", "5.5.5.5"); err != nil {
		t.Fatal(err)
	}
	if tg, ok := tgLinkOf(t, svc, reg.Profile.ID); ok {
		t.Fatalf("unverified link to %d survived the reset", tg)
	}
	res, err := svc.TelegramWebAppLogin(ctx, signInitData(t, testBotToken, webAppFields(intruderTg, time.Now())), "")
	if err != nil || !res.NeedPhone || res.Access != "" {
		t.Fatalf("intruder after reset: %+v %v", res, err)
	}
}

// A verified link survives only a reset confirmed by that same Telegram user.
func TestPasswordResetKeepsOnlyTheConfirmersVerifiedLink(t *testing.T) {
	svc, ctx := newWebAppService(t)
	const phone = "+998901120006"
	raw, contact := proof(t, 6007, phone)
	reg, err := svc.Register(ctx, RegisterInput{Phone: phone, Password: "keep-pass-1", Name: "K", TgInitData: raw, TgContact: contact})
	if err != nil || !reg.TelegramLinked {
		t.Fatalf("register: %+v %v", reg, err)
	}

	// Confirmed by the linked user itself → kept.
	resetRaw := linkedReset(t, svc, phone, 6007)
	if err := svc.CompletePasswordReset(ctx, resetRaw, "keep-new-pass-1", "5.5.5.5"); err != nil {
		t.Fatal(err)
	}
	if tg, verified, ok := phoneVerifiedOf(t, svc, reg.Profile.ID); !ok || tg != 6007 || !verified {
		t.Fatalf("own reset dropped the verified link: %d %v %v", tg, verified, ok)
	}

	// A verified reset with no recorded confirmer (verified before this
	// column existed) cannot be attributed → the link goes.
	resetRaw = linkedReset(t, svc, phone, 6007)
	if _, err := svc.Pool.Exec(ctx, `UPDATE password_reset_token SET verified_tg_user_id = NULL WHERE used_at IS NULL`); err != nil {
		t.Fatal(err)
	}
	if err := svc.CompletePasswordReset(ctx, resetRaw, "keep-new-pass-2", "5.5.5.5"); err != nil {
		t.Fatal(err)
	}
	if tg, ok := tgLinkOf(t, svc, reg.Profile.ID); ok {
		t.Fatalf("unattributed reset kept link to %d", tg)
	}
}

// The link check is on the row the reset finds at completion: a verified link
// of another Telegram user, written after the reset was confirmed, goes too.
func TestPasswordResetDropsVerifiedLinkOfAnotherTelegramUser(t *testing.T) {
	svc, ctx := newWebAppService(t)
	const phone = "+998901120007"
	reg, err := svc.Register(ctx, RegisterInput{Phone: phone, Password: "other-pass-1", Name: "X"})
	if err != nil {
		t.Fatal(err)
	}
	resetRaw := linkedReset(t, svc, phone, 6008)
	raw, contact := proof(t, 6009, phone)
	if ok, err := svc.LinkTelegramWebApp(ctx, reg.Profile.ID, raw, contact); err != nil || !ok {
		t.Fatalf("link-webapp: %v %v", ok, err)
	}
	if err := svc.CompletePasswordReset(ctx, resetRaw, "other-new-pass-1", "5.5.5.5"); err != nil {
		t.Fatal(err)
	}
	if tg, ok := tgLinkOf(t, svc, reg.Profile.ID); ok {
		t.Fatalf("reset confirmed by 6008 kept 6009's link (%d)", tg)
	}
}

// The confirmer's own Telegram account survives a reset only while its link
// is still phone-verified. Here the owner confirmed, then /unlink'ed and
// re-linked the same Telegram account through a legacy token (no phone
// proof): same tg_user_id as the confirmer, but unverified, so it goes.
func TestPasswordResetDropsConfirmersLinkWhenRelinkedWithoutProof(t *testing.T) {
	svc, ctx := newWebAppService(t)
	const phone = "+998901170005"
	reg, err := svc.Register(ctx, RegisterInput{Phone: phone, Password: "yyyyyyy1", Name: "Y"})
	if err != nil {
		t.Fatal(err)
	}
	const tg = 7108
	raw := linkedReset(t, svc, phone, tg)
	if _, err := svc.Pool.Exec(ctx, `DELETE FROM telegram_account WHERE tg_user_id = $1`, tg); err != nil {
		t.Fatal(err)
	}
	legacyLink(t, svc, reg.Profile.ID, tg)
	if err := svc.CompletePasswordReset(ctx, raw, "yyyyyyy-new", "7.7.7.4"); err != nil {
		t.Fatal(err)
	}
	if got, ok := tgLinkOf(t, svc, reg.Profile.ID); ok {
		t.Fatalf("confirmer's unverified re-link %d survived the reset", got)
	}
}

// A Telegram user phone-verified-linked to profile O who resets profile P (a
// second account whose phone they also own) proves P's phone in the bot and
// taps «Ha, men». That must finish the reset — it used to answer "stale" and
// leave it pending for good — and the link follows the proof to P.
func TestPasswordResetFromTelegramLinkedToAnotherProfileCompletes(t *testing.T) {
	svc, ctx := newWebAppService(t)
	const pPhone, oPhone = "+998901170001", "+998901170002"
	p, err := svc.Register(ctx, RegisterInput{Phone: pPhone, Password: "ppppppp1", Name: "P"})
	if err != nil {
		t.Fatal(err)
	}
	o, err := svc.Register(ctx, RegisterInput{Phone: oPhone, Password: "ooooooo1", Name: "O"})
	if err != nil {
		t.Fatal(err)
	}
	const tg = 7101
	if err := svc.Q.UpsertTelegramAccount(ctx, sqlc.UpsertTelegramAccountParams{
		ProfileID: o.Profile.ID, TgUserID: tg, PhoneVerified: true,
	}); err != nil {
		t.Fatal(err)
	}
	// linkedReset goes begin → (need_contact) contact → «Ha, men» and fails
	// unless the tap verifies.
	raw := linkedReset(t, svc, pPhone, tg)
	if st := svc.PasswordResetStatus(ctx, raw).State; st != ResetStateVerified {
		t.Fatalf("status=%s want verified", st)
	}
	if err := svc.CompletePasswordReset(ctx, raw, "ppppppp-new", "7.7.7.1"); err != nil {
		t.Fatal(err)
	}
	if got, verified, ok := phoneVerifiedOf(t, svc, p.Profile.ID); !ok || got != tg || !verified {
		t.Fatalf("P's link: tg=%d verified=%v ok=%v", got, verified, ok)
	}
	if _, ok := tgLinkOf(t, svc, o.Profile.ID); ok {
		t.Fatal("O kept the link")
	}
	res, err := svc.TelegramWebAppLogin(ctx, signInitData(t, testBotToken, webAppFields(tg, time.Now())), "")
	if err != nil || res.Access == "" {
		t.Fatalf("Mini App sign-in after reset: %+v %v", res, err)
	}
}
