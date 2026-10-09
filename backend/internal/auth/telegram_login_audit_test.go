package auth

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"avtotest.uz/backend/internal/db/sqlc"
)

// Regression tests for the independent audit of «Telegram orqali kirish»
// (.superpowers/sdd/tglogin-audit.md). Each scenario below once ended with
// the attacker's browser signed in.

// A1: the attacker starts a login in their own browser and gets the victim to
// press START on the link. The victim ignores the prompt and then starts a
// genuine password reset, sharing their phone for it. That share must go to
// the reset — never approve the attacker's login.
func TestAuditA1_ResetPhoneShareDoesNotApproveALogin(t *testing.T) {
	svc, _ := resetTestService(t)
	ctx := context.Background()
	victim, err := svc.Register(ctx, RegisterInput{Phone: "901230001", Password: "secret123"})
	if err != nil {
		t.Fatal(err)
	}
	const victimTg = 9101
	attacker := startLogin(t, svc, "6.6.6.6")
	if r, err := svc.BeginTelegramLogin(ctx, attacker.Token, tgWho(victimTg)); err != nil || r.Outcome != TelegramLoginNeedContact {
		t.Fatalf("begin = %+v %v", r, err)
	}

	reset, err := svc.StartPasswordReset(ctx, "901230001", "1.1.1.1", "AvtoTestBot")
	if err != nil {
		t.Fatal(err)
	}
	if r, err := svc.BeginTelegramPasswordReset(ctx, parseResetRaw(t, reset.BotURL), victimTg); err != nil || r.Outcome != TelegramResetNeedContact {
		t.Fatalf("reset begin = %+v %v", r, err)
	}

	// The bot offers a contact to a waiting login first, then to the reset.
	if r, err := svc.ConfirmTelegramLoginContact(ctx, tgWho(victimTg), victimTg, "998901230001"); err != nil || r.Outcome != TelegramLoginNone {
		t.Fatalf("the reset's phone share reached the login: %+v %v", r, err)
	}
	if r, err := svc.ConfirmTelegramPasswordResetContact(ctx, victimTg, victimTg, "998901230001"); err != nil || r.Outcome != TelegramResetNeedConfirm {
		t.Fatalf("reset contact = %+v %v", r, err)
	}
	if got := loginState(t, svc, attacker, attacker.BrowserSecret); got != TelegramLoginStatePending {
		t.Fatalf("attacker's login is %q, want still pending", got)
	}
	if _, err := svc.CompleteTelegramLogin(ctx, attacker.Token, attacker.BrowserSecret, "6.6.6.6"); !errors.Is(err, ErrTelegramLoginNotApproved) {
		t.Fatalf("attacker complete err = %v", err)
	}
	if n := activeSessions(t, svc, victim.Profile.ID); n != 1 { // the register session only
		t.Fatalf("victim sessions = %d, want 1", n)
	}
}

// A2: same link, but the stray contact is the Mini App's phone sheet, whose
// contact message reaches the bot before the Mini App's own request.
func TestAuditA2_MiniAppPhoneShareDoesNotApproveALogin(t *testing.T) {
	svc, ctx := newWebAppService(t)
	const victimTg = 9102
	attacker := startLogin(t, svc, "6.6.6.6")
	if r, err := svc.BeginTelegramLogin(ctx, attacker.Token, tgWho(victimTg)); err != nil || r.Outcome != TelegramLoginNeedContact {
		t.Fatalf("begin = %+v %v", r, err)
	}
	// The contact echo wins the race: at most it earns the «✅ Kirish» question.
	asked, err := svc.ConfirmTelegramLoginContact(ctx, tgWho(victimTg), victimTg, "998901230002")
	if err != nil {
		t.Fatal(err)
	}
	if asked.Outcome == TelegramLoginApproved {
		t.Fatal("a phone share alone approved the login")
	}
	if got := loginState(t, svc, attacker, attacker.BrowserSecret); got != TelegramLoginStatePending {
		t.Fatalf("attacker's login is %q, want pending", got)
	}
	if n := profilesWithPhone(t, svc, "+998901230002"); n != 0 {
		t.Fatal("a phone share alone created a profile")
	}
	// The Mini App request then lands and disarms the question.
	initData, contact := webAppPhoneProof(t, victimTg, "998901230002", "", time.Now())
	if _, err := svc.TelegramWebAppPhoneSignIn(ctx, initData, contact, "3.3.3.3"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CompleteTelegramLogin(ctx, attacker.Token, attacker.BrowserSecret, "6.6.6.6"); !errors.Is(err, ErrTelegramLoginNotApproved) {
		t.Fatalf("attacker complete err = %v", err)
	}
}

// The other ordering of A1: a reset left waiting, then /start login_. The
// login now owns the chat's next share; the reset's stops being answerable.
func TestAuditA1Reverse_LoginStartDisarmsPendingReset(t *testing.T) {
	svc, q := resetTestService(t)
	ctx := context.Background()
	reg, err := svc.Register(ctx, RegisterInput{Phone: "901230003", Password: "secret123"})
	if err != nil {
		t.Fatal(err)
	}
	const tg = 9103
	if err := q.UpsertTelegramAccount(ctx, sqlc.UpsertTelegramAccountParams{ProfileID: reg.Profile.ID, TgUserID: tg, PhoneVerified: true}); err != nil {
		t.Fatal(err)
	}
	reset, err := svc.StartPasswordReset(ctx, "901230003", "1.1.1.1", "AvtoTestBot")
	if err != nil {
		t.Fatal(err)
	}
	asked, err := svc.BeginTelegramPasswordReset(ctx, parseResetRaw(t, reset.BotURL), tg)
	if err != nil || asked.Outcome != TelegramResetNeedConfirm {
		t.Fatalf("reset begin = %+v %v", asked, err)
	}
	st := startLogin(t, svc, "1.1.1.1")
	if r, err := svc.BeginTelegramLogin(ctx, st.Token, tgWho(tg)); err != nil || r.Outcome != TelegramLoginNeedConfirm {
		t.Fatalf("login begin = %+v %v", r, err)
	}
	// The «Ha, men» sent before the login link was opened no longer works.
	if r, err := svc.AnswerTelegramPasswordResetConfirm(ctx, tg, asked.ConfirmNonce, true); err != nil || r.Outcome != TelegramResetStale {
		t.Fatalf("stale reset tap = %+v %v", r, err)
	}
	if got := svc.PasswordResetStatus(ctx, parseResetRaw(t, reset.BotURL)); got.State != ResetStatePending {
		t.Fatalf("reset state = %q, want still pending", got.State)
	}
}

// Legitimate flows, end to end, with the tap counts the owner accepted:
// linked account = 1 tap; first sign-in = share + tap.
func TestLegitimateFlowsStillComplete(t *testing.T) {
	svc, _ := resetTestService(t)
	ctx := context.Background()

	// New learner: share + tap creates the account.
	st := startLogin(t, svc, "1.1.1.1")
	if r, _ := svc.BeginTelegramLogin(ctx, st.Token, tgWho(9201)); r.Outcome != TelegramLoginNeedContact {
		t.Fatalf("begin = %+v", r)
	}
	if r := shareAndApprove(t, svc, tgWho(9201), "998901240001"); r.Outcome != TelegramLoginApproved || !r.Created {
		t.Fatalf("new learner = %+v", r)
	}
	created, err := svc.CompleteTelegramLogin(ctx, st.Token, st.BrowserSecret, "1.1.1.1")
	if err != nil || !created.Created || created.Profile.Phone != "+998901240001" {
		t.Fatalf("complete = %+v %v", created.Profile, err)
	}

	// Existing password learner, no link yet: share + tap signs in to it.
	reg, err := svc.Register(ctx, RegisterInput{Phone: "901240002", Password: "secret123"})
	if err != nil {
		t.Fatal(err)
	}
	st = startLogin(t, svc, "1.1.1.1")
	if r, _ := svc.BeginTelegramLogin(ctx, st.Token, tgWho(9202)); r.Outcome != TelegramLoginNeedContact {
		t.Fatalf("begin = %+v", r)
	}
	if r := shareAndApprove(t, svc, tgWho(9202), "998901240002"); r.Outcome != TelegramLoginApproved || r.Created {
		t.Fatalf("existing learner = %+v", r)
	}
	done, err := svc.CompleteTelegramLogin(ctx, st.Token, st.BrowserSecret, "1.1.1.1")
	if err != nil || done.Profile.ID != reg.Profile.ID {
		t.Fatalf("complete = %v %v", done.Profile.ID, err)
	}

	// The same learner again: now linked, one tap, no phone step.
	st = startLogin(t, svc, "1.1.1.1")
	begin, err := svc.BeginTelegramLogin(ctx, st.Token, tgWho(9202))
	if err != nil || begin.Outcome != TelegramLoginNeedConfirm || begin.ViaContact || begin.MaskedPhone != "+998 90 ••• •• 02" {
		t.Fatalf("linked begin = %+v %v", begin, err)
	}
	if r, _ := svc.AnswerTelegramLoginConfirm(ctx, tgWho(9202), begin.ConfirmNonce, true); r.Outcome != TelegramLoginApproved {
		t.Fatalf("one tap = %+v", r)
	}
	if done, err = svc.CompleteTelegramLogin(ctx, st.Token, st.BrowserSecret, "1.1.1.1"); err != nil || done.Profile.ID != reg.Profile.ID {
		t.Fatalf("complete = %v %v", done.Profile.ID, err)
	}
}

// Saying no after the phone share, and a number banned between the share
// and the tap.
func TestContactPathCancelAndLateBan(t *testing.T) {
	svc, _ := resetTestService(t)
	ctx := context.Background()
	st := startLogin(t, svc, "1.1.1.1")
	_, _ = svc.BeginTelegramLogin(ctx, st.Token, tgWho(9210))
	asked, _ := svc.ConfirmTelegramLoginContact(ctx, tgWho(9210), 9210, "998901250001")
	if r, err := svc.AnswerTelegramLoginConfirm(ctx, tgWho(9210), asked.ConfirmNonce, false); err != nil || r.Outcome != TelegramLoginCancelled || !r.ViaContact {
		t.Fatalf("cancel = %+v %v", r, err)
	}
	if loginState(t, svc, st, st.BrowserSecret) != TelegramLoginStateCancelled || profilesWithPhone(t, svc, "+998901250001") != 0 {
		t.Fatal("cancel must end the request and create nothing")
	}

	reg, err := svc.Register(ctx, RegisterInput{Phone: "901250002", Password: "secret123"})
	if err != nil {
		t.Fatal(err)
	}
	st = startLogin(t, svc, "1.1.1.1")
	_, _ = svc.BeginTelegramLogin(ctx, st.Token, tgWho(9211))
	asked, _ = svc.ConfirmTelegramLoginContact(ctx, tgWho(9211), 9211, "998901250002")
	if asked.Outcome != TelegramLoginNeedConfirm {
		t.Fatalf("asked = %+v", asked)
	}
	if _, err := svc.Pool.Exec(ctx, `UPDATE profile SET status='banned' WHERE id=$1`, reg.Profile.ID); err != nil {
		t.Fatal(err)
	}
	if r, err := svc.AnswerTelegramLoginConfirm(ctx, tgWho(9211), asked.ConfirmNonce, true); err != nil || r.Outcome != TelegramLoginBlocked {
		t.Fatalf("late ban = %+v %v", r, err)
	}
	if loginState(t, svc, st, st.BrowserSecret) != TelegramLoginStateBlocked {
		t.Fatal("status must say blocked")
	}
}

// Opening the link again starts the bot step over: the number shared before
// and the question sent for it are dropped.
func TestReopenDropsTheSharedNumber(t *testing.T) {
	svc, _ := resetTestService(t)
	ctx := context.Background()
	st := startLogin(t, svc, "1.1.1.1")
	_, _ = svc.BeginTelegramLogin(ctx, st.Token, tgWho(9220))
	asked, _ := svc.ConfirmTelegramLoginContact(ctx, tgWho(9220), 9220, "998901260001")
	if r, _ := svc.BeginTelegramLogin(ctx, st.Token, tgWho(9220)); r.Outcome != TelegramLoginNeedContact {
		t.Fatalf("reopen = %+v", r)
	}
	if r, _ := svc.AnswerTelegramLoginConfirm(ctx, tgWho(9220), asked.ConfirmNonce, true); r.Outcome != TelegramLoginStale {
		t.Fatalf("old question = %+v", r)
	}
	if profilesWithPhone(t, svc, "+998901260001") != 0 {
		t.Fatal("a dropped share created a profile")
	}
}

// Audit F2 (A3, A4): the first Telegram user to open a link owns it.
func TestAuditF2_FirstOpenerWins(t *testing.T) {
	svc, _ := resetTestService(t)
	ctx := context.Background()
	victim := startLogin(t, svc, "1.1.1.1") // the screen everyone can see
	const first, second = 9301, 9302
	if r, _ := svc.BeginTelegramLogin(ctx, victim.Token, tgWho(first)); r.Outcome != TelegramLoginNeedContact {
		t.Fatalf("first opener = %+v", r)
	}
	// The second opener has a login of their own waiting; opening the
	// victim's link must not touch that one either.
	own := startLogin(t, svc, "2.2.2.2")
	if r, _ := svc.BeginTelegramLogin(ctx, own.Token, tgWho(second)); r.Outcome != TelegramLoginNeedContact {
		t.Fatalf("own begin = %+v", r)
	}
	r, err := svc.BeginTelegramLogin(ctx, victim.Token, tgWho(second))
	if err != nil || r.Outcome != TelegramLoginTaken || r.Device != "" || r.ConfirmNonce != "" {
		t.Fatalf("second opener = %+v %v", r, err)
	}
	// Their share goes to their own request, not the victim's.
	if r := shareAndApprove(t, svc, tgWho(second), "998901270002"); r.Outcome != TelegramLoginApproved {
		t.Fatalf("second's own login = %+v", r)
	}
	if got := loginState(t, svc, victim, victim.BrowserSecret); got != TelegramLoginStatePending {
		t.Fatalf("victim's request is %q", got)
	}
	// The first opener's share is not swallowed.
	if r := shareAndApprove(t, svc, tgWho(first), "998901270001"); r.Outcome != TelegramLoginApproved {
		t.Fatalf("first opener = %+v", r)
	}
	done, err := svc.CompleteTelegramLogin(ctx, victim.Token, victim.BrowserSecret, "1.1.1.1")
	if err != nil || done.Profile.Phone != "+998901270001" {
		t.Fatalf("victim's browser signed in to %q %v", done.Profile.Phone, err)
	}
}

// The claim survives a disarm, and an ended request tells strangers nothing.
func TestFirstOpenerClaimSurvivesDisarmAndApproval(t *testing.T) {
	svc, ctx := newWebAppService(t)
	st := startLogin(t, svc, "1.1.1.1")
	const owner, stranger = 9311, 9312
	_, _ = svc.BeginTelegramLogin(ctx, st.Token, tgWho(owner))
	// The owner wanders into the Mini App: their login is disarmed.
	initData, contact := webAppPhoneProof(t, owner, "998901280001", "", time.Now())
	if _, err := svc.TelegramWebAppPhoneSignIn(ctx, initData, contact, "3.3.3.3"); err != nil {
		t.Fatal(err)
	}
	if r, _ := svc.BeginTelegramLogin(ctx, st.Token, tgWho(stranger)); r.Outcome != TelegramLoginTaken {
		t.Fatalf("stranger after disarm = %+v", r)
	}
	// The owner opens it again (now linked: one tap) and approves.
	begin, err := svc.BeginTelegramLogin(ctx, st.Token, tgWho(owner))
	if err != nil || begin.Outcome != TelegramLoginNeedConfirm {
		t.Fatalf("owner reopen = %+v %v", begin, err)
	}
	if r, _ := svc.AnswerTelegramLoginConfirm(ctx, tgWho(owner), begin.ConfirmNonce, true); r.Outcome != TelegramLoginApproved {
		t.Fatalf("approve = %+v", r)
	}
	// «✅ Kirildi» is for the approver only.
	if r, _ := svc.BeginTelegramLogin(ctx, st.Token, tgWho(stranger)); r.Outcome != TelegramLoginInvalid {
		t.Fatalf("stranger on an approved request = %+v", r)
	}
	if r, _ := svc.BeginTelegramLogin(ctx, st.Token, tgWho(owner)); r.Outcome != TelegramLoginApproved {
		t.Fatalf("approver on an approved request = %+v", r)
	}
}

func TestLimiterIPFoldsIPv6ToSlash64(t *testing.T) {
	cases := map[string]string{
		"203.0.113.7":            "203.0.113.7",
		"::ffff:203.0.113.7":     "203.0.113.7",
		"2001:db8:1:2::1":        "2001:db8:1:2::/64",
		"2001:db8:1:2:ffff::9":   "2001:db8:1:2::/64",
		"2001:db8:1:3::1":        "2001:db8:1:3::/64",
		"fe80::1%eth0":           "fe80::/64",
		" 2001:db8:1:2::1 ":      "2001:db8:1:2::/64",
		"":                       "",
		"not-an-ip":              "not-an-ip",
		"2001:DB8:1:2:0:0:0:abc": "2001:db8:1:2::/64",
	}
	for in, want := range cases {
		if got := limiterIP(in); got != want {
			t.Errorf("limiterIP(%q) = %q, want %q", in, got, want)
		}
	}
}

// Audit F3 (B6): 42 addresses of one IPv6 /64 are one client, and no amount
// of starts from anywhere locks other visitors out.
func TestAuditF3_OneSlash64IsOneBucketAndNoGlobalLockout(t *testing.T) {
	svc, _ := resetTestService(t)
	core, logs := observer.New(zap.WarnLevel)
	svc.Log = zap.New(core)
	ctx := context.Background()
	for i := 0; i < telegramLoginStartPerIP; i++ {
		startLogin(t, svc, "2001:db8:1:2::"+strconv.FormatInt(int64(i%42+1), 16))
	}
	if _, err := svc.StartTelegramLogin(ctx, "2001:db8:1:2::beef", "", testLoginBot); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("43rd address of the same /64: err = %v, want rate limited", err)
	}
	startLogin(t, svc, "2001:db8:1:3::1") // the neighbouring /64

	// The hour's total far past the alert threshold: still served, and the
	// operator is told once at the crossing.
	if err := svc.Lim.R.Set(ctx, telegramLoginStartAllKey, telegramLoginStartAlert, time.Hour).Err(); err != nil {
		t.Fatal(err)
	}
	if logs.FilterMessage("auth.telegram_login_start_flood").Len() != 0 {
		t.Fatal("warned below the threshold")
	}
	startLogin(t, svc, "198.51.100.1")
	startLogin(t, svc, "198.51.100.2")
	if n := logs.FilterMessage("auth.telegram_login_start_flood").Len(); n != 1 {
		t.Fatalf("flood warnings = %d, want exactly 1 at the crossing", n)
	}
}

func seedLimiter(t *testing.T, svc *Service, key string, n int) {
	t.Helper()
	if err := svc.Lim.R.Set(context.Background(), key, n, time.Hour).Err(); err != nil {
		t.Fatal(err)
	}
}

// Status: bounded per request for its own browser and per client address;
// polls that do not prove the browser secret never spend the request's budget.
func TestTelegramLoginStatusLimits(t *testing.T) {
	svc, _ := resetTestService(t)
	ctx := context.Background()
	st := startLogin(t, svc, "1.1.1.1")
	perToken := "tglogin:status:" + HashToken(st.Token)

	if s, err := svc.TelegramLoginStatus(ctx, st.Token, "guess", "6.6.6.6"); err != nil || s != TelegramLoginStateInvalid {
		t.Fatalf("stranger poll = %q %v", s, err)
	}
	if n, _ := svc.Lim.Count(ctx, perToken); n != 0 {
		t.Fatalf("a stranger's poll spent %d of the request's budget", n)
	}
	seedLimiter(t, svc, perToken, telegramLoginStatusPerToken)
	if _, err := svc.TelegramLoginStatus(ctx, st.Token, st.BrowserSecret, "1.1.1.1"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("per-request: err = %v, want rate limited", err)
	}

	other := startLogin(t, svc, "1.1.1.1")
	seedLimiter(t, svc, "tglogin:status:ip:2001:db8:9:9::/64", telegramLoginStatusPerIP)
	if _, err := svc.TelegramLoginStatus(ctx, other.Token, other.BrowserSecret, "2001:db8:9:9::77"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("per-IP: err = %v, want rate limited", err)
	}
	if s, err := svc.TelegramLoginStatus(ctx, other.Token, other.BrowserSecret, "9.9.9.9"); err != nil || s != TelegramLoginStatePending {
		t.Fatalf("another address = %q %v", s, err)
	}
}

// Every bot step is bounded per Telegram user: opening a link, sharing a
// phone, tapping «✅ Kirish».
func TestTelegramLoginBotStepsAreLimitedPerTelegramUser(t *testing.T) {
	svc, _ := resetTestService(t)
	ctx := context.Background()
	const tg = 9401
	st := startLogin(t, svc, "1.1.1.1")

	seedLimiter(t, svc, telegramLoginBotKey(tg), telegramLoginBotPerUser)
	if r, err := svc.BeginTelegramLogin(ctx, st.Token, tgWho(tg)); err != nil || r.Outcome != TelegramLoginRateLimited {
		t.Fatalf("begin = %+v %v", r, err)
	}
	seedLimiter(t, svc, telegramLoginBotKey(tg), 0)
	if r, _ := svc.BeginTelegramLogin(ctx, st.Token, tgWho(tg)); r.Outcome != TelegramLoginNeedContact {
		t.Fatalf("begin = %+v", r)
	}

	seedLimiter(t, svc, telegramLoginBotKey(tg), telegramLoginBotPerUser)
	if r, err := svc.ConfirmTelegramLoginContact(ctx, tgWho(tg), tg, "998901290001"); err != nil || r.Outcome != TelegramLoginRateLimited {
		t.Fatalf("contact = %+v %v", r, err)
	}
	seedLimiter(t, svc, telegramLoginBotKey(tg), 0)
	asked, _ := svc.ConfirmTelegramLoginContact(ctx, tgWho(tg), tg, "998901290001")
	if asked.Outcome != TelegramLoginNeedConfirm {
		t.Fatalf("contact = %+v", asked)
	}

	seedLimiter(t, svc, telegramLoginBotKey(tg), telegramLoginBotPerUser)
	if r, err := svc.AnswerTelegramLoginConfirm(ctx, tgWho(tg), asked.ConfirmNonce, true); err != nil || r.Outcome != TelegramLoginRateLimited {
		t.Fatalf("tap = %+v %v", r, err)
	}
	if loginState(t, svc, st, st.BrowserSecret) != TelegramLoginStatePending {
		t.Fatal("a limited tap must leave the request waiting")
	}
	// Another Telegram user is not affected.
	other := startLogin(t, svc, "1.1.1.1")
	if r, _ := svc.BeginTelegramLogin(ctx, other.Token, tgWho(tg+1)); r.Outcome != TelegramLoginNeedContact {
		t.Fatalf("other user = %+v", r)
	}
}

// Mutant M10: a tapper who is themselves phone-verified-linked (so the
// "tapper must be linked" re-check passes) still cannot answer a question
// that was asked of someone else.
func TestForeignLinkedTapperCannotAnswer(t *testing.T) {
	svc, q := resetTestService(t)
	ctx := context.Background()
	victim, _ := svc.Register(ctx, RegisterInput{Phone: "901300001", Password: "secret123"})
	intruder, _ := svc.Register(ctx, RegisterInput{Phone: "901300002", Password: "secret123"})
	_ = q.UpsertTelegramAccount(ctx, sqlc.UpsertTelegramAccountParams{ProfileID: victim.Profile.ID, TgUserID: 9501, PhoneVerified: true})
	_ = q.UpsertTelegramAccount(ctx, sqlc.UpsertTelegramAccountParams{ProfileID: intruder.Profile.ID, TgUserID: 9502, PhoneVerified: true})
	st := startLogin(t, svc, "1.1.1.1")
	begin, err := svc.BeginTelegramLogin(ctx, st.Token, tgWho(9501))
	if err != nil || begin.Outcome != TelegramLoginNeedConfirm {
		t.Fatalf("begin = %+v %v", begin, err)
	}
	for _, accept := range []bool{true, false} {
		if r, err := svc.AnswerTelegramLoginConfirm(ctx, tgWho(9502), begin.ConfirmNonce, accept); err != nil || r.Outcome != TelegramLoginStale {
			t.Fatalf("foreign linked tap (accept=%v) = %+v %v", accept, r, err)
		}
	}
	if loginState(t, svc, st, st.BrowserSecret) != TelegramLoginStatePending {
		t.Fatal("a foreign tap changed the request")
	}
	if activeSessions(t, svc, intruder.Profile.ID) != 1 {
		t.Fatal("the intruder's account got a session")
	}
}

// Mutant M14: while a «✅ Kirish» question is open only the tap answers it.
func TestContactWhileQuestionOpenIsNotAnAnswer(t *testing.T) {
	svc, q := resetTestService(t)
	ctx := context.Background()
	reg, _ := svc.Register(ctx, RegisterInput{Phone: "901310001", Password: "secret123"})
	_ = q.UpsertTelegramAccount(ctx, sqlc.UpsertTelegramAccountParams{ProfileID: reg.Profile.ID, TgUserID: 9511, PhoneVerified: true})
	st := startLogin(t, svc, "1.1.1.1")
	begin, _ := svc.BeginTelegramLogin(ctx, st.Token, tgWho(9511))
	if begin.Outcome != TelegramLoginNeedConfirm {
		t.Fatalf("begin = %+v", begin)
	}
	// Even their own, correct number.
	if r, err := svc.ConfirmTelegramLoginContact(ctx, tgWho(9511), 9511, "998901310001"); err != nil || r.Outcome != TelegramLoginNone {
		t.Fatalf("contact with a question open = %+v %v", r, err)
	}
	if loginState(t, svc, st, st.BrowserSecret) != TelegramLoginStatePending {
		t.Fatal("a contact answered the question")
	}
	// The question is unchanged: its tap still works.
	if r, _ := svc.AnswerTelegramLoginConfirm(ctx, tgWho(9511), begin.ConfirmNonce, true); r.Outcome != TelegramLoginApproved {
		t.Fatalf("tap = %+v", r)
	}

	// Same after a phone share: a second share neither re-asks nor approves.
	st2 := startLogin(t, svc, "1.1.1.1")
	_, _ = svc.BeginTelegramLogin(ctx, st2.Token, tgWho(9512))
	asked, _ := svc.ConfirmTelegramLoginContact(ctx, tgWho(9512), 9512, "998901310002")
	if r, _ := svc.ConfirmTelegramLoginContact(ctx, tgWho(9512), 9512, "998901310002"); r.Outcome != TelegramLoginNone {
		t.Fatalf("second share = %+v", r)
	}
	if r, _ := svc.AnswerTelegramLoginConfirm(ctx, tgWho(9512), asked.ConfirmNonce, true); r.Outcome != TelegramLoginApproved {
		t.Fatalf("first question must still stand: %+v", r)
	}
}

// Mutant M19: Telegram reports full international numbers; a bare 9-digit
// national number (what the website form accepts) is not one.
func TestContactPhoneMustBeFullPlus998(t *testing.T) {
	svc, _ := resetTestService(t)
	ctx := context.Background()
	st := startLogin(t, svc, "1.1.1.1")
	_, _ = svc.BeginTelegramLogin(ctx, st.Token, tgWho(9521))
	for _, phone := range []string{"901320001", "8901320001", "00998901320001", "+7 998 901 32 00"} {
		if r, err := svc.ConfirmTelegramLoginContact(ctx, tgWho(9521), 9521, phone); err != nil || r.Outcome != TelegramLoginForeignPhone {
			t.Fatalf("contact %q = %+v %v", phone, r, err)
		}
	}
	if loginState(t, svc, st, st.BrowserSecret) != TelegramLoginStatePending {
		t.Fatal("must still wait")
	}
}

// Mutant M21: a /start ref_ older than 30 days is not applied.
func TestBotReferralExpiresAfter30Days(t *testing.T) {
	svc, q := resetTestService(t)
	ctx := context.Background()
	owner := referralOwner(t, svc, "901330000", "REF-OLD111")
	for _, tc := range []struct {
		tg      int64
		phone   string
		ageDays int
		want    bool
	}{{9531, "998901330001", 31, false}, {9532, "998901330002", 29, true}} {
		if err := q.SetTelegramBotUserPendingReferral(ctx, sqlc.SetTelegramBotUserPendingReferralParams{
			TgUserID: tc.tg, PendingReferralCode: pgtype.Text{String: "REF-OLD111", Valid: true},
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.Pool.Exec(ctx,
			`UPDATE telegram_bot_user SET pending_referral_at = now() - make_interval(days => $2) WHERE tg_user_id = $1`, tc.tg, tc.ageDays); err != nil {
			t.Fatal(err)
		}
		st := startLogin(t, svc, "1.1.1.1")
		_, _ = svc.BeginTelegramLogin(ctx, st.Token, tgWho(tc.tg))
		if r := shareAndApprove(t, svc, tgWho(tc.tg), tc.phone); r.Outcome != TelegramLoginApproved || !r.Created {
			t.Fatalf("approve = %+v", r)
		}
		done, err := svc.CompleteTelegramLogin(ctx, st.Token, st.BrowserSecret, "1.1.1.1")
		if err != nil {
			t.Fatal(err)
		}
		got, ok := referrerOf(t, svc, done.Profile.ID)
		if ok != tc.want || (ok && got != owner) {
			t.Fatalf("referral %d days old: applied=%v, want %v", tc.ageDays, ok, tc.want)
		}
	}
}

func stationWithPhone(t *testing.T, svc *Service, phone string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := svc.Pool.QueryRow(context.Background(),
		`INSERT INTO profile (phone, kind) VALUES ($1, 'station') RETURNING id`, phone).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

// Mutant M24: a phone-verified link to a non-learner row is not identity.
func TestVerifiedLinkToStationIsNotIdentity(t *testing.T) {
	svc, q := resetTestService(t)
	ctx := context.Background()
	station := stationWithPhone(t, svc, "st:"+uuid.NewString())
	if err := q.UpsertTelegramAccount(ctx, sqlc.UpsertTelegramAccountParams{ProfileID: station, TgUserID: 9541, PhoneVerified: true}); err != nil {
		t.Fatal(err)
	}
	st := startLogin(t, svc, "1.1.1.1")
	r, err := svc.BeginTelegramLogin(ctx, st.Token, tgWho(9541))
	if err != nil || r.Outcome != TelegramLoginNeedContact || r.ConfirmNonce != "" {
		t.Fatalf("a station link was offered «✅ Kirish»: %+v %v", r, err)
	}
}

// Mutant M03: completion re-checks the kind of the approved profile.
func TestCompleteRefusesANonLearnerProfile(t *testing.T) {
	svc, _ := resetTestService(t)
	ctx := context.Background()
	st := startLogin(t, svc, "1.1.1.1")
	_, _ = svc.BeginTelegramLogin(ctx, st.Token, tgWho(9551))
	if r := shareAndApprove(t, svc, tgWho(9551), "998901350001"); r.Outcome != TelegramLoginApproved {
		t.Fatalf("approve = %+v", r)
	}
	var id uuid.UUID
	if err := svc.Pool.QueryRow(ctx, `UPDATE profile SET kind='station' WHERE phone='+998901350001' RETURNING id`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CompleteTelegramLogin(ctx, st.Token, st.BrowserSecret, "1.1.1.1"); !errors.Is(err, ErrTelegramLoginInvalid) {
		t.Fatalf("complete err = %v", err)
	}
	if activeSessions(t, svc, id) != 0 {
		t.Fatal("a non-learner profile got a session")
	}
}

// Mutant M06a: the approval UPDATE itself only ever moves a pending request.
func TestApproveQueryOnlyMovesPendingRequests(t *testing.T) {
	svc, q := resetTestService(t)
	ctx := context.Background()
	a, _ := svc.Register(ctx, RegisterInput{Phone: "901360001", Password: "secret123"})
	b, _ := svc.Register(ctx, RegisterInput{Phone: "901360002", Password: "secret123"})
	_ = q.UpsertTelegramAccount(ctx, sqlc.UpsertTelegramAccountParams{ProfileID: a.Profile.ID, TgUserID: 9561, PhoneVerified: true})
	approveAs := func(id uuid.UUID, profile uuid.UUID) int64 {
		n, err := q.ApproveTelegramLoginRequest(ctx, sqlc.ApproveTelegramLoginRequestParams{
			ID: id, ProfileID: uuid.NullUUID{UUID: profile, Valid: true}, ApprovedTgUserID: pgtype.Int8{Int64: 9562, Valid: true},
		})
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	rowOf := func(st TelegramLoginStart) sqlc.TelegramLoginRequest {
		row, err := q.GetTelegramLoginRequestByTokenHash(ctx, HashToken(st.Token))
		if err != nil {
			t.Fatal(err)
		}
		return row
	}

	cancelled := startLogin(t, svc, "1.1.1.1")
	begin, _ := svc.BeginTelegramLogin(ctx, cancelled.Token, tgWho(9561))
	_, _ = svc.AnswerTelegramLoginConfirm(ctx, tgWho(9561), begin.ConfirmNonce, false)
	if n := approveAs(rowOf(cancelled).ID, b.Profile.ID); n != 0 || rowOf(cancelled).Status != TelegramLoginStateCancelled {
		t.Fatalf("a cancelled request was approved (rows=%d)", n)
	}

	// An approved request cannot be re-pointed at another profile, nor a
	// consumed one revived.
	done := startLogin(t, svc, "1.1.1.1")
	begin, _ = svc.BeginTelegramLogin(ctx, done.Token, tgWho(9561))
	_, _ = svc.AnswerTelegramLoginConfirm(ctx, tgWho(9561), begin.ConfirmNonce, true)
	if n := approveAs(rowOf(done).ID, b.Profile.ID); n != 0 || rowOf(done).ProfileID.UUID != a.Profile.ID {
		t.Fatalf("an approved request was re-pointed (rows=%d)", n)
	}
	if _, err := svc.CompleteTelegramLogin(ctx, done.Token, done.BrowserSecret, "1.1.1.1"); err != nil {
		t.Fatal(err)
	}
	if n := approveAs(rowOf(done).ID, b.Profile.ID); n != 0 || rowOf(done).Status != "consumed" {
		t.Fatalf("a consumed request was revived (rows=%d)", n)
	}
}

// Mutant M06b: a tap on an expired question approves nothing.
func TestTapAfterExpiryIsStale(t *testing.T) {
	svc, q := resetTestService(t)
	ctx := context.Background()
	reg, _ := svc.Register(ctx, RegisterInput{Phone: "901370001", Password: "secret123"})
	_ = q.UpsertTelegramAccount(ctx, sqlc.UpsertTelegramAccountParams{ProfileID: reg.Profile.ID, TgUserID: 9571, PhoneVerified: true})
	st := startLogin(t, svc, "1.1.1.1")
	begin, _ := svc.BeginTelegramLogin(ctx, st.Token, tgWho(9571))
	if _, err := svc.Pool.Exec(ctx, `UPDATE telegram_login_request SET expires_at = now() - interval '1 second'`); err != nil {
		t.Fatal(err)
	}
	if r, err := svc.AnswerTelegramLoginConfirm(ctx, tgWho(9571), begin.ConfirmNonce, true); err != nil || r.Outcome != TelegramLoginStale {
		t.Fatalf("late tap = %+v %v", r, err)
	}
	if got := loginState(t, svc, st, st.BrowserSecret); got != TelegramLoginStateInvalid {
		t.Fatalf("state = %q", got)
	}
	if activeSessions(t, svc, reg.Profile.ID) != 1 {
		t.Fatal("session issued")
	}
}

func setTelegramLoginFlag(t *testing.T, svc *Service, on bool) {
	t.Helper()
	set := func(v bool) {
		if _, err := svc.Pool.Exec(context.Background(),
			`UPDATE feature_flag SET value_json = to_jsonb($1::boolean) WHERE key = 'telegram_login'`, v); err != nil {
			t.Fatal(err)
		}
	}
	set(on)
	t.Cleanup(func() { set(true) })
}

// Kill switch: feature flag telegram_login (seeded ON by migration 0081).
func TestTelegramLoginKillSwitch(t *testing.T) {
	svc, ctx := newWebAppService(t)
	var seeded bool
	if err := svc.Pool.QueryRow(ctx,
		`SELECT value_json::text = 'true' FROM feature_flag WHERE key = 'telegram_login' AND type = 'boolean'`).Scan(&seeded); err != nil || !seeded {
		t.Fatalf("flag must be seeded ON: %v %v", seeded, err)
	}
	// Two requests caught mid-flight by the switch: one waiting for a phone,
	// one already approved.
	waiting := startLogin(t, svc, "1.1.1.1")
	_, _ = svc.BeginTelegramLogin(ctx, waiting.Token, tgWho(9601))
	asked, _ := svc.ConfirmTelegramLoginContact(ctx, tgWho(9601), 9601, "998901380001")
	approved := startLogin(t, svc, "1.1.1.1")
	_, _ = svc.BeginTelegramLogin(ctx, approved.Token, tgWho(9602))
	if r := shareAndApprove(t, svc, tgWho(9602), "998901380002"); r.Outcome != TelegramLoginApproved {
		t.Fatalf("approve = %+v", r)
	}
	fresh := startLogin(t, svc, "1.1.1.1")
	initData, contact := webAppPhoneProof(t, 9603, "998901380003", "", time.Now())

	setTelegramLoginFlag(t, svc, false)

	if _, err := svc.StartTelegramLogin(ctx, "1.1.1.1", chromeAndroidUA, testLoginBot); !errors.Is(err, ErrTelegramLoginDisabled) {
		t.Fatalf("start err = %v", err)
	}
	if r, err := svc.BeginTelegramLogin(ctx, fresh.Token, tgWho(9604)); err != nil || r.Outcome != TelegramLoginDisabled {
		t.Fatalf("begin = %+v %v", r, err)
	}
	if r, err := svc.AnswerTelegramLoginConfirm(ctx, tgWho(9601), asked.ConfirmNonce, true); err != nil || r.Outcome != TelegramLoginDisabled {
		t.Fatalf("tap = %+v %v", r, err)
	}
	if _, err := svc.CompleteTelegramLogin(ctx, approved.Token, approved.BrowserSecret, "1.1.1.1"); !errors.Is(err, ErrTelegramLoginDisabled) {
		t.Fatalf("complete err = %v", err)
	}
	if _, err := svc.TelegramWebAppPhoneSignIn(ctx, initData, contact, "3.3.3.3"); !errors.Is(err, ErrTelegramLoginDisabled) {
		t.Fatalf("mini app phone err = %v", err)
	}
	if profilesWithPhone(t, svc, "+998901380001")+profilesWithPhone(t, svc, "+998901380003") != 0 {
		t.Fatal("the switched-off feature created a profile")
	}
	// A contact is passed on (to the password reset), which keeps working.
	requireLoginRequests(t, svc, 3) // no request was written with the flag off
	if r, err := svc.ConfirmTelegramLoginContact(ctx, tgWho(9601), 9601, "998901380001"); err != nil || r.Outcome != TelegramLoginNone {
		t.Fatalf("contact = %+v %v", r, err)
	}
	// Saying no still works.
	if r, err := svc.AnswerTelegramLoginConfirm(ctx, tgWho(9601), asked.ConfirmNonce, false); err != nil || r.Outcome != TelegramLoginCancelled {
		t.Fatalf("cancel = %+v %v", r, err)
	}

	setTelegramLoginFlag(t, svc, true)
	if _, err := svc.CompleteTelegramLogin(ctx, approved.Token, approved.BrowserSecret, "1.1.1.1"); err != nil {
		t.Fatalf("back on: complete err = %v", err)
	}
}

func requireLoginRequests(t *testing.T, svc *Service, want int) {
	t.Helper()
	var n int
	if err := svc.Pool.QueryRow(context.Background(), `SELECT count(*) FROM telegram_login_request`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != want {
		t.Fatalf("login requests = %d, want %d", n, want)
	}
}

func TestTelegramLoginKillSwitchOverHTTP(t *testing.T) {
	ts, svc := telegramLoginServer(t)
	setTelegramLoginFlag(t, svc, false)
	status, env := postJSON(t, ts, "/auth/telegram-login/start", map[string]string{"user_agent": chromeAndroidUA})
	if status != http.StatusServiceUnavailable || env.Error == nil || env.Error.Code != "telegram_login_disabled" {
		t.Fatalf("start = %d %+v", status, env.Error)
	}
	initData, contact := webAppPhoneProof(t, 9610, "998901390001", "", time.Now())
	status, env = postJSON(t, ts, "/auth/telegram/webapp/phone", map[string]string{"init_data": initData, "contact": contact})
	if status != http.StatusServiceUnavailable || env.Error == nil || env.Error.Code != "telegram_login_disabled" {
		t.Fatalf("mini app phone = %d %+v", status, env.Error)
	}
}

// Telegram names are free text: nothing invisible or direction-changing
// reaches a profile name.
func TestTelegramProfileNameStripsInvisibleAndBidiCharacters(t *testing.T) {
	who := TelegramLoginUser{
		FirstName: "\u202eAli\u202c\u200b\ufeff",
		LastName:  "\u2066Vali\u2069\u200f\u200d yev\x00",
	}
	if got := telegramProfileName(who); got != "Ali Vali yev" {
		t.Fatalf("name = %q", got)
	}
	// A name made only of such characters is empty, not a blank-looking one.
	if got := telegramProfileName(TelegramLoginUser{FirstName: "\u200b\u200e", LastName: "\u202e"}); got != "" {
		t.Fatalf("name = %q", got)
	}
	for _, r := range telegramProfileName(TelegramLoginUser{FirstName: strings.Repeat("\u202eЯ", 90)}) {
		if r != 'Я' {
			t.Fatalf("unexpected rune %U", r)
		}
	}
}

// Audit F5 (accepted by design, REQ-016): password_not_set tells a prober
// that a phone has a Telegram-created account. It stays behind the login
// limits: 10 tries per phone per hour.
func TestPasswordNotSetOracleIsRateLimited(t *testing.T) {
	svc, _ := resetTestService(t)
	ctx := context.Background()
	st := startLogin(t, svc, "1.1.1.1")
	_, _ = svc.BeginTelegramLogin(ctx, st.Token, tgWho(9701))
	if r := shareAndApprove(t, svc, tgWho(9701), "998901400001"); r.Outcome != TelegramLoginApproved {
		t.Fatalf("approve = %+v", r)
	}
	for i := 0; i < 10; i++ {
		if _, err := svc.Login(ctx, LoginInput{Phone: "901400001", Password: "whatever" + strconv.Itoa(i), IP: "6.6.6." + strconv.Itoa(i)}); !errors.Is(err, ErrPasswordNotSet) {
			t.Fatalf("probe %d err = %v", i, err)
		}
	}
	if _, err := svc.Login(ctx, LoginInput{Phone: "901400001", Password: "whatever", IP: "6.6.7.1"}); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("11th probe err = %v, want rate limited", err)
	}
	// And per address (IPv6 by /64) across phones: 30 per hour.
	seedLimiter(t, svc, "login:ip:2001:db8:5:5::/64", 30)
	if _, err := svc.Login(ctx, LoginInput{Phone: "901400002", Password: "whatever", IP: "2001:db8:5:5::1234"}); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("per-IP probe err = %v, want rate limited", err)
	}
}
