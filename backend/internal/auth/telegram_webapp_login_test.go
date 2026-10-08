package auth

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"avtotest.uz/backend/internal/testdb"
)

func webAppFields(tgID int64, now time.Time) map[string]string {
	return map[string]string{
		"auth_date": strconv.FormatInt(now.Unix(), 10),
		"user":      `{"id":` + strconv.FormatInt(tgID, 10) + `,"first_name":"Ali","username":"ali_uz"}`,
	}
}

const testWebAppURL = "https://drivergo.test/uz-Latn/tg"

// newWebAppService is a Service with the Mini App switched on (bot token AND
// TELEGRAM_WEBAPP_URL, the kill switch).
func newWebAppService(t *testing.T) (*Service, context.Context) {
	t.Helper()
	pool := testdb.New(t)
	svc, _ := newTestService(t, pool)
	svc.TelegramBotToken = testBotToken
	svc.TelegramWebAppURL = testWebAppURL
	return svc, context.Background()
}

// proof is the Telegram half of a Mini App sign-in: launch data for tgID and
// a signed contact claiming phone (digits as Telegram reports them).
func proof(t *testing.T, tgID int64, phone string) (string, string) {
	t.Helper()
	return signInitData(t, testBotToken, webAppFields(tgID, time.Now())),
		signContact(t, testBotToken, tgID, strings.TrimPrefix(phone, "+"), time.Now())
}

func tgLinkOf(t *testing.T, svc *Service, profileID uuid.UUID) (int64, bool) {
	t.Helper()
	var tgID int64
	err := svc.Pool.QueryRow(context.Background(), `SELECT tg_user_id FROM telegram_account WHERE profile_id=$1`, profileID).Scan(&tgID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false
	}
	if err != nil {
		t.Fatal(err)
	}
	return tgID, true
}

func TestTelegramWebAppLoginUnlinkedNeedsPhone(t *testing.T) {
	svc, ctx := newWebAppService(t)
	raw := signInitData(t, testBotToken, webAppFields(5001, time.Now()))

	res, err := svc.TelegramWebAppLogin(ctx, raw, "203.0.113.5")
	if err != nil {
		t.Fatal(err)
	}
	if !res.NeedPhone || res.Access != "" || res.FirstName != "Ali" {
		t.Fatalf("result = %+v", res)
	}
}

func TestLoginWithPhoneProofLinksThenWebAppLoginIssuesSession(t *testing.T) {
	svc, ctx := newWebAppService(t)
	const phone, pw = "+998901110001", "linking-password-1"
	if _, err := svc.Register(ctx, RegisterInput{Phone: phone, Password: pw, Name: "A"}); err != nil {
		t.Fatal(err)
	}
	raw, contact := proof(t, 5002, phone)

	login, err := svc.Login(ctx, LoginInput{Phone: phone, Password: pw, TgInitData: raw, TgContact: contact})
	if err != nil {
		t.Fatal(err)
	}
	if !login.TelegramLinked {
		t.Fatal("login with a Telegram-signed matching phone must link")
	}
	res, err := svc.TelegramWebAppLogin(ctx, raw, "")
	if err != nil {
		t.Fatal(err)
	}
	if res.NeedPhone || res.Access == "" || res.Refresh == "" || res.Profile.ID != login.Profile.ID {
		t.Fatalf("webapp login = %+v", res)
	}
	var sessions int
	if err := svc.Pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM refresh_token WHERE profile_id=$1 AND revoked_at IS NULL`, login.Profile.ID).Scan(&sessions); err != nil {
		t.Fatal(err)
	}
	if sessions != 3 { // register + login + webapp: each a separate device, none revoked
		t.Fatalf("active sessions = %d, want 3", sessions)
	}
}

func TestRegisterWithPhoneProofLinks(t *testing.T) {
	svc, ctx := newWebAppService(t)
	const phone = "+998901110013"
	raw, contact := proof(t, 5013, phone)
	reg, err := svc.Register(ctx, RegisterInput{Phone: phone, Password: "register-password-1", Name: "A", TgInitData: raw, TgContact: contact})
	if err != nil || !reg.TelegramLinked {
		t.Fatalf("register: %+v %v", reg, err)
	}
	if tg, ok := tgLinkOf(t, svc, reg.Profile.ID); !ok || tg != 5013 {
		t.Fatalf("link = %d %v", tg, ok)
	}
}

// The linking rule: launch data alone proves only "some Telegram account";
// the account must also show Telegram's own signature over the profile's
// phone. Every condition failing on its own must leave the profile unlinked
// while the phone + password sign-in still succeeds.
func TestLoginLinksOnlyWithMatchingSignedPhone(t *testing.T) {
	svc, ctx := newWebAppService(t)
	const phone, pw = "+998901110020", "rule-password-1"
	reg, err := svc.Register(ctx, RegisterInput{Phone: phone, Password: pw, Name: "A"})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	freshInit := signInitData(t, testBotToken, webAppFields(5020, now))
	staleInit := signInitData(t, testBotToken, webAppFields(5020, now.Add(-2*time.Hour)))
	goodContact := signContact(t, testBotToken, 5020, "998901110020", now)
	cases := []struct {
		name, init, contact string
	}{
		{"no contact (init data alone)", freshInit, ""},
		{"stale init data", staleInit, goodContact},
		{"forged contact", freshInit, signContact(t, "999:other", 5020, "998901110020", now)},
		{"stale contact", freshInit, signContact(t, testBotToken, 5020, "998901110020", now.Add(-2*time.Hour))},
		{"contact of another Telegram user", freshInit, signContact(t, testBotToken, 5021, "998901110020", now)},
		{"contact phone is not the profile phone", freshInit, signContact(t, testBotToken, 5020, "998901110099", now)},
		{"9-digit foreign contact phone", freshInit, signContact(t, testBotToken, 5020, "901110020", now)},
		{"contact without init data", "", goodContact},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := svc.Login(ctx, LoginInput{Phone: phone, Password: pw, TgInitData: tc.init, TgContact: tc.contact})
			if err != nil || res.Access == "" {
				t.Fatalf("sign-in must still succeed: res=%+v err=%v", res, err)
			}
			if res.TelegramLinked {
				t.Fatal("reported linked")
			}
			if _, ok := tgLinkOf(t, svc, reg.Profile.ID); ok {
				t.Fatal("link written")
			}
		})
	}
	res, err := svc.Login(ctx, LoginInput{Phone: phone, Password: pw, TgInitData: freshInit, TgContact: goodContact})
	if err != nil || !res.TelegramLinked {
		t.Fatalf("all conditions met: res=%+v err=%v", res, err)
	}
}

// C1: an attacker plants their own launch data (and even their own signed
// contact) in a link the victim opens; the victim signs in with their own
// phone and password. The attacker's Telegram must not end up on the
// victim's profile.
func TestPhishedLaunchDataNeverLinksAttackerToVictim(t *testing.T) {
	svc, ctx := newWebAppService(t)
	const victimPhone, victimPw = "+998901110030", "victim-password-1"
	if _, err := svc.Register(ctx, RegisterInput{Phone: victimPhone, Password: victimPw, Name: "V"}); err != nil {
		t.Fatal(err)
	}
	attackerInit, attackerContact := proof(t, 7001, "+998931110031")

	res, err := svc.Login(ctx, LoginInput{Phone: victimPhone, Password: victimPw, TgInitData: attackerInit, TgContact: attackerContact})
	if err != nil || res.Access == "" {
		t.Fatalf("victim sign-in: res=%+v err=%v", res, err)
	}
	if res.TelegramLinked {
		t.Fatal("attacker Telegram linked to the victim")
	}
	att, err := svc.TelegramWebAppLogin(ctx, attackerInit, "")
	if err != nil {
		t.Fatal(err)
	}
	if !att.NeedPhone || att.Access != "" {
		t.Fatalf("attacker webapp login = %+v, want need_phone", att)
	}
}

// With the phone proven, the Telegram account may leave another profile it
// was linked to (e.g. an old account) and replace the profile's previous
// Telegram link: both are the owner of this phone re-pointing their own link.
func TestPhoneProofMovesAndReplacesLinks(t *testing.T) {
	svc, ctx := newWebAppService(t)
	old, err := svc.Register(ctx, RegisterInput{Phone: "+998901110040", Password: "old-password-11", Name: "Old"})
	if err != nil {
		t.Fatal(err)
	}
	const phone, pw = "+998901110041", "new-password-11"
	cur, err := svc.Register(ctx, RegisterInput{Phone: phone, Password: pw, Name: "New"})
	if err != nil {
		t.Fatal(err)
	}
	// 5040 sits on the old profile; the current profile is linked to 5041.
	if _, err := svc.Pool.Exec(ctx, `INSERT INTO telegram_account (profile_id, tg_user_id, username) VALUES ($1, 5040, ''), ($2, 5041, '')`, old.Profile.ID, cur.Profile.ID); err != nil {
		t.Fatal(err)
	}
	raw, contact := proof(t, 5040, phone)
	res, err := svc.Login(ctx, LoginInput{Phone: phone, Password: pw, TgInitData: raw, TgContact: contact})
	if err != nil || !res.TelegramLinked {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if tg, ok := tgLinkOf(t, svc, cur.Profile.ID); !ok || tg != 5040 {
		t.Fatalf("current profile link = %d %v, want 5040", tg, ok)
	}
	if _, ok := tgLinkOf(t, svc, old.Profile.ID); ok {
		t.Fatal("link must move off the old profile")
	}
	var n int
	if err := svc.Pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM telegram_account WHERE tg_user_id=5041`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatal("the replaced link must be gone")
	}
}

func TestLoginWithInvalidInitDataStillSignsIn(t *testing.T) {
	svc, ctx := newWebAppService(t)
	const phone, pw = "+998901110004", "invalid-init-password"
	if _, err := svc.Register(ctx, RegisterInput{Phone: phone, Password: pw, Name: "A"}); err != nil {
		t.Fatal(err)
	}
	res, err := svc.Login(ctx, LoginInput{Phone: phone, Password: pw, TgInitData: "user=%7B%7D&hash=00", TgContact: "contact=%7B%7D&hash=00"})
	if err != nil {
		t.Fatalf("login must not fail on bad init data: %v", err)
	}
	if res.TelegramLinked || res.Access == "" {
		t.Fatalf("res = %+v", res)
	}
}

func TestLoginWithoutInitDataDoesNotTouchTelegramAccount(t *testing.T) {
	svc, ctx := newWebAppService(t)
	raw, contact := proof(t, 5005, "+998901110005")
	reg, err := svc.Register(ctx, RegisterInput{Phone: "+998901110005", Password: "plain-password-11", Name: "A", TgInitData: raw, TgContact: contact})
	if err != nil || !reg.TelegramLinked {
		t.Fatalf("register: %+v %v", reg, err)
	}
	if _, err := svc.Login(ctx, LoginInput{Phone: "+998901110005", Password: "plain-password-11"}); err != nil {
		t.Fatal(err)
	}
	if tg, ok := tgLinkOf(t, svc, reg.Profile.ID); !ok || tg != 5005 {
		t.Fatalf("plain login changed the link: %d %v", tg, ok)
	}
}

func TestTelegramWebAppLoginRejectsBannedAndBadData(t *testing.T) {
	svc, ctx := newWebAppService(t)
	raw, contact := proof(t, 5006, "+998901110006")
	reg, err := svc.Register(ctx, RegisterInput{Phone: "+998901110006", Password: "banned-password-1", Name: "A", TgInitData: raw, TgContact: contact})
	if err != nil || !reg.TelegramLinked {
		t.Fatalf("register: %+v %v", reg, err)
	}
	if _, err := svc.Pool.Exec(ctx, `UPDATE profile SET status='banned' WHERE id=$1`, reg.Profile.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.TelegramWebAppLogin(ctx, raw, ""); !errors.Is(err, ErrAccountBlocked) {
		t.Fatalf("banned: err = %v", err)
	}
	if _, err := svc.TelegramWebAppLogin(ctx, "hash=00&user=%7B%7D", ""); !errors.Is(err, ErrInitDataInvalid) {
		t.Fatalf("bad data: err = %v", err)
	}
	svc.TelegramBotToken = ""
	if _, err := svc.TelegramWebAppLogin(ctx, raw, ""); !errors.Is(err, ErrTelegramBotUnconfigured) {
		t.Fatalf("no token: err = %v", err)
	}
}

// TELEGRAM_WEBAPP_URL is the kill switch: cleared, the Mini App neither signs
// anyone in nor links, even with a bot token and a perfect phone proof.
func TestEmptyWebAppURLSwitchesMiniAppOff(t *testing.T) {
	svc, ctx := newWebAppService(t)
	const phone, pw = "+998901110050", "killswitch-password-1"
	raw, contact := proof(t, 5050, phone)
	reg, err := svc.Register(ctx, RegisterInput{Phone: phone, Password: pw, Name: "A", TgInitData: raw, TgContact: contact})
	if err != nil || !reg.TelegramLinked {
		t.Fatalf("register: %+v %v", reg, err)
	}
	svc.TelegramWebAppURL = ""
	if _, err := svc.TelegramWebAppLogin(ctx, raw, ""); !errors.Is(err, ErrTelegramBotUnconfigured) {
		t.Fatalf("webapp login with the switch off: err = %v", err)
	}
	if _, err := svc.Pool.Exec(ctx, `DELETE FROM telegram_account WHERE profile_id=$1`, reg.Profile.ID); err != nil {
		t.Fatal(err)
	}
	res, err := svc.Login(ctx, LoginInput{Phone: phone, Password: pw, TgInitData: raw, TgContact: contact})
	if err != nil || res.Access == "" || res.TelegramLinked {
		t.Fatalf("login with the switch off: res=%+v err=%v", res, err)
	}
	if linked, err := svc.LinkTelegramWebApp(ctx, reg.Profile.ID, raw, contact); !errors.Is(err, ErrTelegramBotUnconfigured) || linked {
		t.Fatalf("link-webapp with the switch off: linked=%v err=%v", linked, err)
	}
	if _, ok := tgLinkOf(t, svc, reg.Profile.ID); ok {
		t.Fatal("linked while switched off")
	}
}

// POST /me/telegram/link-webapp: a signed-in learner who typed their phone
// links afterwards by sharing it from Telegram. Same rule as login.
func TestLinkTelegramWebAppAppliesTheSameRule(t *testing.T) {
	svc, ctx := newWebAppService(t)
	const phone = "+998901110060"
	reg, err := svc.Register(ctx, RegisterInput{Phone: phone, Password: "linkwebapp-password-1", Name: "A"})
	if err != nil {
		t.Fatal(err)
	}
	raw := signInitData(t, testBotToken, webAppFields(5060, time.Now()))
	wrongPhone := signContact(t, testBotToken, 5060, "998901110061", time.Now())
	if linked, err := svc.LinkTelegramWebApp(ctx, reg.Profile.ID, raw, wrongPhone); err != nil || linked {
		t.Fatalf("wrong phone: linked=%v err=%v", linked, err)
	}
	otherUser := signContact(t, testBotToken, 5061, "998901110060", time.Now())
	if linked, err := svc.LinkTelegramWebApp(ctx, reg.Profile.ID, raw, otherUser); err != nil || linked {
		t.Fatalf("other user's contact: linked=%v err=%v", linked, err)
	}
	if _, ok := tgLinkOf(t, svc, reg.Profile.ID); ok {
		t.Fatal("linked on a failed proof")
	}
	_, contact := proof(t, 5060, phone)
	if linked, err := svc.LinkTelegramWebApp(ctx, reg.Profile.ID, raw, contact); err != nil || !linked {
		t.Fatalf("matching proof: linked=%v err=%v", linked, err)
	}
	if tg, ok := tgLinkOf(t, svc, reg.Profile.ID); !ok || tg != 5060 {
		t.Fatalf("link = %d %v", tg, ok)
	}
	if _, err := svc.Pool.Exec(ctx, `UPDATE profile SET status='banned' WHERE id=$1`, reg.Profile.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.LinkTelegramWebApp(ctx, reg.Profile.ID, raw, contact); !errors.Is(err, ErrAccountBlocked) {
		t.Fatalf("banned: err = %v", err)
	}
}

func TestLinkTelegramWebAppIsRateLimitedPerProfile(t *testing.T) {
	svc, ctx := newWebAppService(t)
	reg, err := svc.Register(ctx, RegisterInput{Phone: "+998901110070", Password: "ratelimit-password-1", Name: "A"})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < linkWebAppPerProfileLimit; i++ {
		if _, err := svc.LinkTelegramWebApp(ctx, reg.Profile.ID, "hash=00", ""); err != nil {
			t.Fatalf("call %d: err = %v", i, err)
		}
	}
	if _, err := svc.LinkTelegramWebApp(ctx, reg.Profile.ID, "hash=00", ""); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("over the limit: err = %v", err)
	}
}

func TestTelegramWebAppLoginRateLimitsGarbageByIP(t *testing.T) {
	svc, ctx := newWebAppService(t)
	for i := 0; i < 300; i++ {
		if _, err := svc.TelegramWebAppLogin(ctx, "hash=00", "203.0.113.9"); !errors.Is(err, ErrInitDataInvalid) {
			t.Fatalf("call %d: err = %v", i, err)
		}
	}
	if _, err := svc.TelegramWebAppLogin(ctx, "hash=00", "203.0.113.9"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("301st garbage call must be throttled before validation, err = %v", err)
	}
}

func TestLoginIgnoresOversizedInitData(t *testing.T) {
	svc, ctx := newWebAppService(t)
	if _, err := svc.Register(ctx, RegisterInput{Phone: "+998901110007", Password: "oversize-password-1", Name: "A"}); err != nil {
		t.Fatal(err)
	}
	_, contact := proof(t, 5007, "+998901110007")
	res, err := svc.Login(ctx, LoginInput{Phone: "+998901110007", Password: "oversize-password-1", TgInitData: strings.Repeat("a", InitDataMaxBytes+1), TgContact: contact})
	if err != nil || res.TelegramLinked {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

// The same person signs in from several Mini App tabs at once with the same
// proof. Every sign-in must succeed and exactly one link row may remain.
func TestConcurrentMiniAppLoginsLinkSameTelegramAccountOnce(t *testing.T) {
	svc, ctx := newWebAppService(t)
	const phone, pw = "+998901110008", "concurrent-password-1"
	reg, err := svc.Register(ctx, RegisterInput{Phone: phone, Password: pw, Name: "A"})
	if err != nil {
		t.Fatal(err)
	}
	raw, contact := proof(t, 5008, phone)
	const n = 4
	var wg sync.WaitGroup
	errs := make([]error, n)
	res := make([]VerifyResult, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res[i], errs[i] = svc.Login(ctx, LoginInput{Phone: phone, Password: pw, TgInitData: raw, TgContact: contact})
		}()
	}
	wg.Wait()
	linked := 0
	for i := 0; i < n; i++ {
		if errs[i] != nil || res[i].Access == "" {
			t.Fatalf("login %d must succeed: %v", i, errs[i])
		}
		if res[i].TelegramLinked {
			linked++
		}
	}
	var rows int
	if err := svc.Pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM telegram_account WHERE tg_user_id=5008`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 1 || linked == 0 {
		t.Fatalf("rows=%d linked=%d", rows, linked)
	}
	if tg, ok := tgLinkOf(t, svc, reg.Profile.ID); !ok || tg != 5008 {
		t.Fatalf("link = %d %v", tg, ok)
	}
}

// Init data is a 24h bearer token for sign-in, but creating or moving a link
// demands fresher data: a stale payload still signs the person in, unlinked.
func TestLinkRequiresFresherInitDataThanSignIn(t *testing.T) {
	svc, ctx := newWebAppService(t)
	const phone, pw = "+998901110012", "freshness-password-1"
	reg, err := svc.Register(ctx, RegisterInput{Phone: phone, Password: pw, Name: "A"})
	if err != nil {
		t.Fatal(err)
	}
	contact := signContact(t, testBotToken, 5012, "998901110012", time.Now())
	stale := signInitData(t, testBotToken, webAppFields(5012, time.Now().Add(-2*time.Hour)))
	res, err := svc.Login(ctx, LoginInput{Phone: phone, Password: pw, TgInitData: stale, TgContact: contact})
	if err != nil || res.Access == "" || res.TelegramLinked {
		t.Fatalf("2h-old init data: res=%+v err=%v", res, err)
	}
	if _, ok := tgLinkOf(t, svc, reg.Profile.ID); ok {
		t.Fatal("2h-old init data must not write a link")
	}
	fresh := signInitData(t, testBotToken, webAppFields(5012, time.Now().Add(-30*time.Minute)))
	res, err = svc.Login(ctx, LoginInput{Phone: phone, Password: pw, TgInitData: fresh, TgContact: contact})
	if err != nil || !res.TelegramLinked {
		t.Fatalf("30min-old init data: res=%+v err=%v", res, err)
	}
}

// A bot password reset waits for "the contact of Telegram user N". If N then
// links through the Mini App, the reset must not stay armed: a Mini App share
// would otherwise complete a reset that someone else started.
func TestMiniAppLinkClearsPendingBotPasswordReset(t *testing.T) {
	svc, ctx := newWebAppService(t)
	victim, err := svc.Register(ctx, RegisterInput{Phone: "+998901110071", Password: "victim-password-1", Name: "V"})
	if err != nil {
		t.Fatal(err)
	}
	const tgID = 5071
	pending := func() bool {
		var n int
		if err := svc.Pool.QueryRow(ctx, `SELECT count(*) FROM password_reset_token WHERE pending_tg_user_id=$1`, tgID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n > 0
	}
	arm := func() {
		if _, err := svc.Pool.Exec(ctx, `DELETE FROM password_reset_token`); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.Pool.Exec(ctx,
			`INSERT INTO password_reset_token (profile_id, token_hash, expires_at, pending_tg_user_id) VALUES ($1, $2, now() + interval '1 hour', $3)`,
			victim.Profile.ID, uuid.NewString(), tgID); err != nil {
			t.Fatal(err)
		}
		if !pending() {
			t.Fatal("reset not armed")
		}
	}

	// A failed proof must not disturb the reset.
	arm()
	raw, wrong := proof(t, tgID, "+998901110099")
	attacker, err := svc.Register(ctx, RegisterInput{Phone: "+998901110072", Password: "attacker-password-1", Name: "A", TgInitData: raw, TgContact: wrong})
	if err != nil || attacker.TelegramLinked {
		t.Fatalf("register: %+v %v", attacker, err)
	}
	if !pending() {
		t.Fatal("a failed proof cleared the pending reset")
	}

	// A proven link clears it, via login...
	raw, good := proof(t, tgID, "+998901110072")
	if _, err := svc.Login(ctx, LoginInput{Phone: "+998901110072", Password: "attacker-password-1", TgInitData: raw, TgContact: good}); err != nil {
		t.Fatal(err)
	}
	if pending() {
		t.Fatal("login link left the pending reset armed")
	}

	// ...and via link-webapp.
	arm()
	if _, err := svc.Pool.Exec(ctx, `DELETE FROM telegram_account`); err != nil {
		t.Fatal(err)
	}
	if linked, err := svc.LinkTelegramWebApp(ctx, attacker.Profile.ID, raw, good); err != nil || !linked {
		t.Fatalf("link: %v %v", linked, err)
	}
	if pending() {
		t.Fatal("link-webapp left the pending reset armed")
	}
}

// LinkTelegramWebApp must refuse a banned profile even when nothing is linked
// yet and the proof is perfect (RejectBanned middleware is not the only gate).
func TestLinkTelegramWebAppRefusesBannedProfile(t *testing.T) {
	svc, ctx := newWebAppService(t)
	const phone = "+998901160005"
	reg, err := svc.Register(ctx, RegisterInput{Phone: phone, Password: "bannedlink-pass-1", Name: "B"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Pool.Exec(ctx, `UPDATE profile SET status='banned' WHERE id=$1`, reg.Profile.ID); err != nil {
		t.Fatal(err)
	}
	raw := signInitData(t, testBotToken, webAppFields(5160, time.Now()))
	contact := signContact(t, testBotToken, 5160, "998901160005", time.Now())
	if linked, err := svc.LinkTelegramWebApp(ctx, reg.Profile.ID, raw, contact); err == nil || linked {
		t.Fatalf("banned profile: linked=%v err=%v", linked, err)
	}
	if _, ok := tgLinkOf(t, svc, reg.Profile.ID); ok {
		t.Fatal("a banned profile got a Telegram link")
	}
}
