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
	"github.com/jackc/pgx/v5/pgtype"

	"avtotest.uz/backend/internal/db/sqlc"
)

const testLoginBot = "DriverGoTestBot"

const chromeAndroidUA = "Mozilla/5.0 (Linux; Android 14; SM-A546E) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Mobile Safari/537.36"

func startLogin(t *testing.T, svc *Service, ip string) TelegramLoginStart {
	t.Helper()
	st, err := svc.StartTelegramLogin(context.Background(), ip, chromeAndroidUA, testLoginBot)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func loginState(t *testing.T, svc *Service, st TelegramLoginStart, secret string) string {
	t.Helper()
	state, err := svc.TelegramLoginStatus(context.Background(), st.Token, secret, "9.9.9.9")
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func profilesWithPhone(t *testing.T, svc *Service, phone string) int {
	t.Helper()
	var n int
	if err := svc.Pool.QueryRow(context.Background(), `SELECT count(*) FROM profile WHERE phone=$1`, phone).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// shareAndApprove is the first-time bot flow: share your own number, get the
// «✅ Kirish» question for it, tap it. It returns the tap's result.
func shareAndApprove(t *testing.T, svc *Service, who TelegramLoginUser, phone string) TelegramLoginBegin {
	t.Helper()
	ctx := context.Background()
	asked, err := svc.ConfirmTelegramLoginContact(ctx, who, who.ID, phone)
	if err != nil {
		t.Fatal(err)
	}
	if asked.Outcome != TelegramLoginNeedConfirm || asked.ConfirmNonce == "" || !asked.ViaContact {
		t.Fatalf("a phone share must only earn the question, got %+v", asked)
	}
	res, err := svc.AnswerTelegramLoginConfirm(ctx, who, asked.ConfirmNonce, true)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func tgWho(id int64) TelegramLoginUser {
	return TelegramLoginUser{ID: id, FirstName: "Ali", LastName: "Valiyev", Username: "ali_uz"}
}

func TestDescribeDevice(t *testing.T) {
	cases := map[string]string{
		chromeAndroidUA: "Chrome · Android",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1": "Safari · iPhone",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36 Edg/128.0.0.0":           "Edge · Windows",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:130.0) Gecko/20100101 Firefox/130.0":                                                        "Firefox · Windows",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_5) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Safari/605.1.15":                      "Safari · macOS",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 YaBrowser/24.7.0 Safari/537.36":                  "Yandex · Linux",
		"curl/8.0": "",
		"":         "",
		// Free text never reaches the bot message: only fixed words come out.
		"<b>evil</b> Firefox/1.0": "Firefox",
	}
	for ua, want := range cases {
		if got := DescribeDevice(ua); got != want {
			t.Errorf("DescribeDevice(%q) = %q, want %q", ua, got, want)
		}
	}
}

func TestParseReferralStartParam(t *testing.T) {
	cases := map[string]string{
		"ref_REF-AB23CD":                 "REF-AB23CD",
		"ref_abc_9":                      "abc_9",
		"ref_":                           "",
		"REF-AB23CD":                     "",
		"ref_a b":                        "",
		"ref_<x>":                        "",
		"ref_" + strings.Repeat("A", 61): "",
	}
	for in, want := range cases {
		got, ok := ParseReferralStartParam(in)
		if got != want || ok != (want != "") {
			t.Errorf("ParseReferralStartParam(%q) = %q,%v want %q", in, got, ok, want)
		}
	}
}

func TestTelegramLoginStartLink(t *testing.T) {
	svc, _ := resetTestService(t)
	st := startLogin(t, svc, "1.1.1.1")
	if st.BotURL != "https://t.me/"+testLoginBot+"?start=login_"+st.Token {
		t.Fatalf("bot url %q", st.BotURL)
	}
	if len("login_"+st.Token) > 64 {
		t.Fatal("start payload over Telegram's 64-byte limit")
	}
	if st.BrowserSecret == "" || st.BrowserSecret == st.Token || st.ExpiresInSec != 300 {
		t.Fatalf("start = %+v", st)
	}
	// Only digests at rest.
	var tokenHash, secretHash, device string
	if err := svc.Pool.QueryRow(context.Background(),
		`SELECT token_hash, browser_secret_hash, device FROM telegram_login_request`).Scan(&tokenHash, &secretHash, &device); err != nil {
		t.Fatal(err)
	}
	if tokenHash != HashToken(st.Token) || secretHash != HashToken(st.BrowserSecret) || device != "Chrome · Android" {
		t.Fatalf("stored %q %q %q", tokenHash, secretHash, device)
	}
	if _, err := svc.StartTelegramLogin(context.Background(), "1.1.1.1", "", ""); !errors.Is(err, ErrTelegramBotUnconfigured) {
		t.Fatalf("no bot: err = %v", err)
	}
	if loginState(t, svc, st, st.BrowserSecret) != TelegramLoginStatePending {
		t.Fatal("fresh request must be pending")
	}
}

func TestTelegramLoginStartRateLimitedPerIP(t *testing.T) {
	svc, _ := resetTestService(t)
	for i := 0; i < telegramLoginStartPerIP; i++ {
		startLogin(t, svc, "2.2.2.2")
	}
	if _, err := svc.StartTelegramLogin(context.Background(), "2.2.2.2", "", testLoginBot); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("err = %v, want rate limited", err)
	}
	startLogin(t, svc, "2.2.2.3")
}

func TestTelegramLoginNewUserSharesPhoneThenCompletes(t *testing.T) {
	svc, _ := resetTestService(t)
	ctx := context.Background()
	st := startLogin(t, svc, "1.1.1.1")

	begin, err := svc.BeginTelegramLogin(ctx, st.Token, tgWho(7001))
	if err != nil {
		t.Fatal(err)
	}
	if begin.Outcome != TelegramLoginNeedContact || begin.Device != "Chrome · Android" {
		t.Fatalf("begin = %+v", begin)
	}
	if loginState(t, svc, st, st.BrowserSecret) != TelegramLoginStatePending {
		t.Fatal("must stay pending until the phone is shared")
	}
	if _, err := svc.CompleteTelegramLogin(ctx, st.Token, st.BrowserSecret, "1.1.1.1"); !errors.Is(err, ErrTelegramLoginNotApproved) {
		t.Fatalf("early complete err = %v", err)
	}

	// Sharing the number is not consent: it earns the question, naming the
	// device and the (masked) account, and changes nothing else.
	asked, err := svc.ConfirmTelegramLoginContact(ctx, tgWho(7001), 7001, "998901112233")
	if err != nil {
		t.Fatal(err)
	}
	if asked.Outcome != TelegramLoginNeedConfirm || asked.ConfirmNonce == "" ||
		asked.MaskedPhone != "+998 90 ••• •• 33" || asked.Device != "Chrome · Android" {
		t.Fatalf("contact = %+v", asked)
	}
	if loginState(t, svc, st, st.BrowserSecret) != TelegramLoginStatePending || profilesWithPhone(t, svc, "+998901112233") != 0 {
		t.Fatal("nothing may be approved or created before the tap")
	}
	res, err := svc.AnswerTelegramLoginConfirm(ctx, tgWho(7001), asked.ConfirmNonce, true)
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != TelegramLoginApproved || !res.Created || !res.ViaContact {
		t.Fatalf("tap = %+v", res)
	}
	var kept *string
	if err := svc.Pool.QueryRow(ctx, `SELECT contact_phone FROM telegram_login_request`).Scan(&kept); err != nil || kept != nil {
		t.Fatalf("the shared number must not outlive the approval: %v %v", kept, err)
	}
	if loginState(t, svc, st, st.BrowserSecret) != TelegramLoginStateApproved {
		t.Fatal("status must say approved")
	}
	done, err := svc.CompleteTelegramLogin(ctx, st.Token, st.BrowserSecret, "1.1.1.1")
	if err != nil {
		t.Fatal(err)
	}
	if done.Access == "" || done.Refresh == "" || !done.Created {
		t.Fatalf("complete = %+v", done)
	}
	p := done.Profile
	if p.Phone != "+998901112233" || p.Name != "Ali Valiyev" || p.Kind != "user" || p.PasswordHash.Valid {
		t.Fatalf("profile = %+v", p)
	}
	if tg, verified, ok := phoneVerifiedOf(t, svc, p.ID); !ok || tg != 7001 || !verified {
		t.Fatalf("link tg=%d verified=%v ok=%v", tg, verified, ok)
	}
	var trials int
	if err := svc.Pool.QueryRow(ctx, `SELECT count(*) FROM entitlement WHERE profile_id=$1 AND source='trial'`, p.ID).Scan(&trials); err != nil || trials != 1 {
		t.Fatalf("signup trial: %v %d", err, trials)
	}
	// One-time.
	if _, err := svc.CompleteTelegramLogin(ctx, st.Token, st.BrowserSecret, "1.1.1.1"); !errors.Is(err, ErrTelegramLoginInvalid) {
		t.Fatalf("replay err = %v", err)
	}
	if loginState(t, svc, st, st.BrowserSecret) != TelegramLoginStateInvalid {
		t.Fatal("consumed request must read invalid")
	}
	// A password login for this account now explains itself.
	if _, err := svc.Login(ctx, LoginInput{Phone: "901112233", Password: "whatever1"}); !errors.Is(err, ErrPasswordNotSet) {
		t.Fatalf("password login err = %v", err)
	}
}

func TestTelegramLoginExistingAccountMatchedByPhone(t *testing.T) {
	svc, _ := resetTestService(t)
	ctx := context.Background()
	reg, err := svc.Register(ctx, RegisterInput{Phone: "901112244", Password: "secret123", Name: "Old"})
	if err != nil {
		t.Fatal(err)
	}
	st := startLogin(t, svc, "1.1.1.1")
	if _, err := svc.BeginTelegramLogin(ctx, st.Token, tgWho(7002)); err != nil {
		t.Fatal(err)
	}
	// Address-book formatting is normalised to the stored +998 form.
	res := shareAndApprove(t, svc, tgWho(7002), "+998 (90) 111-22-44")
	if res.Outcome != TelegramLoginApproved || res.Created {
		t.Fatalf("tap = %+v", res)
	}
	done, err := svc.CompleteTelegramLogin(ctx, st.Token, st.BrowserSecret, "1.1.1.1")
	if err != nil {
		t.Fatal(err)
	}
	if done.Profile.ID != reg.Profile.ID || done.Created || done.Profile.Name != "Old" {
		t.Fatalf("signed in to %+v, want existing %s", done.Profile, reg.Profile.ID)
	}
	if n := profilesWithPhone(t, svc, "+998901112244"); n != 1 {
		t.Fatalf("profiles with phone = %d", n)
	}
	// The password still works: Telegram login added a way in, took none away.
	if _, err := svc.Login(ctx, LoginInput{Phone: "901112244", Password: "secret123"}); err != nil {
		t.Fatal(err)
	}
}

func TestTelegramLoginLinkedUserConfirmsWithOneTap(t *testing.T) {
	svc, q := resetTestService(t)
	ctx := context.Background()
	reg, err := svc.Register(ctx, RegisterInput{Phone: "901112255", Password: "secret123", Name: "L"})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.UpsertTelegramAccount(ctx, sqlc.UpsertTelegramAccountParams{ProfileID: reg.Profile.ID, TgUserID: 7003, PhoneVerified: true}); err != nil {
		t.Fatal(err)
	}
	st := startLogin(t, svc, "1.1.1.1")
	begin, err := svc.BeginTelegramLogin(ctx, st.Token, tgWho(7003))
	if err != nil {
		t.Fatal(err)
	}
	if begin.Outcome != TelegramLoginNeedConfirm || begin.ConfirmNonce == "" || begin.MaskedPhone != "+998 90 ••• •• 55" {
		t.Fatalf("begin = %+v", begin)
	}
	if strings.Contains(begin.ConfirmNonce, st.Token) || len("tgl:y:"+begin.ConfirmNonce) > 64 {
		t.Fatal("nonce must be independent of the token and fit callback_data")
	}
	// Someone else tapping a forwarded question changes nothing.
	if r, err := svc.AnswerTelegramLoginConfirm(ctx, tgWho(9999), begin.ConfirmNonce, true); err != nil || r.Outcome != TelegramLoginStale {
		t.Fatalf("other user = %+v %v", r, err)
	}
	if r, err := svc.AnswerTelegramLoginConfirm(ctx, tgWho(7003), begin.ConfirmNonce, true); err != nil || r.Outcome != TelegramLoginApproved {
		t.Fatalf("accept = %+v %v", r, err)
	}
	if r, _ := svc.AnswerTelegramLoginConfirm(ctx, tgWho(7003), begin.ConfirmNonce, true); r.Outcome != TelegramLoginStale {
		t.Fatalf("second tap = %+v", r)
	}
	done, err := svc.CompleteTelegramLogin(ctx, st.Token, st.BrowserSecret, "1.1.1.1")
	if err != nil || done.Profile.ID != reg.Profile.ID {
		t.Fatalf("complete %+v %v", done.Profile.ID, err)
	}
}

func TestTelegramLoginLegacyUnverifiedLinkMustShareThePhone(t *testing.T) {
	svc, _ := resetTestService(t)
	ctx := context.Background()
	reg, err := svc.Register(ctx, RegisterInput{Phone: "901112266", Password: "secret123"})
	if err != nil {
		t.Fatal(err)
	}
	legacyLink(t, svc, reg.Profile.ID, 7004)
	st := startLogin(t, svc, "1.1.1.1")
	begin, err := svc.BeginTelegramLogin(ctx, st.Token, tgWho(7004))
	if err != nil || begin.Outcome != TelegramLoginNeedContact {
		t.Fatalf("legacy link must not be identity: %+v %v", begin, err)
	}
}

func TestTelegramLoginCancelAndWrongCookie(t *testing.T) {
	svc, q := resetTestService(t)
	ctx := context.Background()
	reg, err := svc.Register(ctx, RegisterInput{Phone: "901112277", Password: "secret123"})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.UpsertTelegramAccount(ctx, sqlc.UpsertTelegramAccountParams{ProfileID: reg.Profile.ID, TgUserID: 7005, PhoneVerified: true}); err != nil {
		t.Fatal(err)
	}
	st := startLogin(t, svc, "1.1.1.1")
	other := startLogin(t, svc, "1.1.1.1")
	// Status never answers for a browser that did not start the request.
	if loginState(t, svc, st, other.BrowserSecret) != TelegramLoginStateInvalid {
		t.Fatal("wrong cookie must read invalid")
	}
	begin, _ := svc.BeginTelegramLogin(ctx, st.Token, tgWho(7005))
	if r, err := svc.AnswerTelegramLoginConfirm(ctx, tgWho(7005), begin.ConfirmNonce, false); err != nil || r.Outcome != TelegramLoginCancelled {
		t.Fatalf("cancel = %+v %v", r, err)
	}
	if loginState(t, svc, st, st.BrowserSecret) != TelegramLoginStateCancelled {
		t.Fatal("status must say cancelled")
	}
	if _, err := svc.CompleteTelegramLogin(ctx, st.Token, st.BrowserSecret, "1.1.1.1"); !errors.Is(err, ErrTelegramLoginInvalid) {
		t.Fatalf("complete after cancel err = %v", err)
	}

	// An approved request still needs the browser that started it.
	st2 := startLogin(t, svc, "1.1.1.1")
	b2, _ := svc.BeginTelegramLogin(ctx, st2.Token, tgWho(7005))
	if r, _ := svc.AnswerTelegramLoginConfirm(ctx, tgWho(7005), b2.ConfirmNonce, true); r.Outcome != TelegramLoginApproved {
		t.Fatal("approve failed")
	}
	if _, err := svc.CompleteTelegramLogin(ctx, st2.Token, other.BrowserSecret, "1.1.1.1"); !errors.Is(err, ErrTelegramLoginInvalid) {
		t.Fatalf("wrong-cookie complete err = %v", err)
	}
	if _, err := svc.CompleteTelegramLogin(ctx, st2.Token, st2.BrowserSecret, "1.1.1.1"); err != nil {
		t.Fatalf("right cookie still completes: %v", err)
	}
}

func TestTelegramLoginExpired(t *testing.T) {
	svc, _ := resetTestService(t)
	ctx := context.Background()
	st := startLogin(t, svc, "1.1.1.1")
	if _, err := svc.Pool.Exec(ctx, `UPDATE telegram_login_request SET expires_at = now() - interval '1 second'`); err != nil {
		t.Fatal(err)
	}
	if b, err := svc.BeginTelegramLogin(ctx, st.Token, tgWho(7006)); err != nil || b.Outcome != TelegramLoginInvalid {
		t.Fatalf("begin = %+v %v", b, err)
	}
	if loginState(t, svc, st, st.BrowserSecret) != TelegramLoginStateInvalid {
		t.Fatal("expired must read invalid")
	}
	if b, _ := svc.BeginTelegramLogin(ctx, "never-issued", tgWho(7006)); b.Outcome != TelegramLoginInvalid {
		t.Fatal("unknown token")
	}
}

func TestTelegramLoginContactChecks(t *testing.T) {
	svc, _ := resetTestService(t)
	ctx := context.Background()
	// No request waiting: not ours (the Mini App share, or a reset reply).
	if r, err := svc.ConfirmTelegramLoginContact(ctx, tgWho(7007), 7007, "998901112288"); err != nil || r.Outcome != TelegramLoginNone {
		t.Fatalf("no request = %+v %v", r, err)
	}
	st := startLogin(t, svc, "1.1.1.1")
	if _, err := svc.BeginTelegramLogin(ctx, st.Token, tgWho(7007)); err != nil {
		t.Fatal(err)
	}
	if r, _ := svc.ConfirmTelegramLoginContact(ctx, tgWho(7007), 1234, "998901112288"); r.Outcome != TelegramLoginNotOwnContact {
		t.Fatalf("forwarded contact = %+v", r)
	}
	if r, _ := svc.ConfirmTelegramLoginContact(ctx, tgWho(7007), 7007, "79161234567"); r.Outcome != TelegramLoginForeignPhone {
		t.Fatalf("foreign = %+v", r)
	}
	if loginState(t, svc, st, st.BrowserSecret) != TelegramLoginStatePending {
		t.Fatal("a bad contact leaves the request waiting")
	}
	if n := profilesWithPhone(t, svc, "+998901112288"); n != 0 {
		t.Fatal("nothing created")
	}
}

func TestTelegramLoginRefusesBannedAccount(t *testing.T) {
	svc, _ := resetTestService(t)
	ctx := context.Background()
	reg, err := svc.Register(ctx, RegisterInput{Phone: "901112299", Password: "secret123"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Pool.Exec(ctx, `UPDATE profile SET status='banned' WHERE id=$1`, reg.Profile.ID); err != nil {
		t.Fatal(err)
	}
	st := startLogin(t, svc, "1.1.1.1")
	if _, err := svc.BeginTelegramLogin(ctx, st.Token, tgWho(7008)); err != nil {
		t.Fatal(err)
	}
	r, err := svc.ConfirmTelegramLoginContact(ctx, tgWho(7008), 7008, "998901112299")
	if err != nil || r.Outcome != TelegramLoginBlocked {
		t.Fatalf("banned = %+v %v", r, err)
	}
	if loginState(t, svc, st, st.BrowserSecret) != TelegramLoginStateBlocked {
		t.Fatal("status must say blocked")
	}
	if _, err := svc.CompleteTelegramLogin(ctx, st.Token, st.BrowserSecret, "1.1.1.1"); !errors.Is(err, ErrTelegramLoginInvalid) {
		t.Fatalf("complete err = %v", err)
	}
	if activeSessions(t, svc, reg.Profile.ID) != 1 { // the register session only
		t.Fatal("no new session for a banned account")
	}
}

func TestTelegramLoginNeverTouchesStationProfile(t *testing.T) {
	svc, _ := resetTestService(t)
	ctx := context.Background()
	// Pathological on purpose: a station row whose phone column holds a real
	// +998 number. Real stations hold st:<uuid>, but kind must decide anyway.
	var stationID uuid.UUID
	if err := svc.Pool.QueryRow(ctx,
		`INSERT INTO profile (phone, kind) VALUES ('+998901113300', 'station') RETURNING id`).Scan(&stationID); err != nil {
		t.Fatal(err)
	}
	st := startLogin(t, svc, "1.1.1.1")
	if _, err := svc.BeginTelegramLogin(ctx, st.Token, tgWho(7009)); err != nil {
		t.Fatal(err)
	}
	r, err := svc.ConfirmTelegramLoginContact(ctx, tgWho(7009), 7009, "998901113300")
	if err != nil {
		t.Fatal(err)
	}
	if r.Outcome == TelegramLoginApproved {
		t.Fatal("a station profile was signed in to")
	}
	if activeSessions(t, svc, stationID) != 0 {
		t.Fatal("station got a session")
	}
	if _, _, ok := phoneVerifiedOf(t, svc, stationID); ok {
		t.Fatal("station got a Telegram link")
	}
	if _, err := svc.CompleteTelegramLogin(ctx, st.Token, st.BrowserSecret, "1.1.1.1"); err == nil {
		t.Fatal("complete must fail")
	}
}

// Two approvals racing to create the same new phone end on one profile.
func TestTelegramLoginConcurrentCreationYieldsOneProfile(t *testing.T) {
	svc, _ := resetTestService(t)
	ctx := context.Background()
	const n = 4
	starts := make([]TelegramLoginStart, n)
	for i := 0; i < n; i++ {
		starts[i] = startLogin(t, svc, "1.1.1."+strconv.Itoa(i))
		if _, err := svc.BeginTelegramLogin(ctx, starts[i].Token, tgWho(int64(7100+i))); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	outcomes := make([]string, n)
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			who := tgWho(int64(7100 + i))
			asked, err := svc.ConfirmTelegramLoginContact(ctx, who, who.ID, "998901114400")
			if err != nil {
				errs[i] = err
				return
			}
			r, err := svc.AnswerTelegramLoginConfirm(ctx, who, asked.ConfirmNonce, true)
			outcomes[i], errs[i] = r.Outcome, err
		}(i)
	}
	wg.Wait()
	for i := 0; i < n; i++ {
		if errs[i] != nil || outcomes[i] != TelegramLoginApproved {
			t.Fatalf("racer %d: %v %v", i, outcomes[i], errs[i])
		}
	}
	if c := profilesWithPhone(t, svc, "+998901114400"); c != 1 {
		t.Fatalf("profiles = %d, want 1", c)
	}
	ids := map[uuid.UUID]bool{}
	for i := 0; i < n; i++ {
		done, err := svc.CompleteTelegramLogin(ctx, starts[i].Token, starts[i].BrowserSecret, "1.1.1.1")
		if err != nil {
			t.Fatal(err)
		}
		ids[done.Profile.ID] = true
	}
	if len(ids) != 1 {
		t.Fatalf("signed in to %d profiles", len(ids))
	}
}

// Many simultaneous «✅ Kirish» taps approve the request exactly once.
func TestTelegramLoginDoubleApprovalOnce(t *testing.T) {
	svc, q := resetTestService(t)
	ctx := context.Background()
	reg, err := svc.Register(ctx, RegisterInput{Phone: "901114455", Password: "secret123"})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.UpsertTelegramAccount(ctx, sqlc.UpsertTelegramAccountParams{ProfileID: reg.Profile.ID, TgUserID: 7200, PhoneVerified: true}); err != nil {
		t.Fatal(err)
	}
	st := startLogin(t, svc, "1.1.1.1")
	begin, _ := svc.BeginTelegramLogin(ctx, st.Token, tgWho(7200))
	const n = 6
	var wg sync.WaitGroup
	var mu sync.Mutex
	approved := 0
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := svc.AnswerTelegramLoginConfirm(ctx, tgWho(7200), begin.ConfirmNonce, true)
			if err != nil {
				t.Error(err)
				return
			}
			if r.Outcome == TelegramLoginApproved {
				mu.Lock()
				approved++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if approved != 1 {
		t.Fatalf("approved %d times", approved)
	}
	// And completing it twice at once hands out one session.
	var ok int
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := svc.CompleteTelegramLogin(ctx, st.Token, st.BrowserSecret, "1.1.1.1"); err == nil {
				mu.Lock()
				ok++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if ok != 1 {
		t.Fatalf("completed %d times", ok)
	}
}

func referralOwner(t *testing.T, svc *Service, phone, code string) uuid.UUID {
	t.Helper()
	reg, err := svc.Register(context.Background(), RegisterInput{Phone: phone, Password: "secret123"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Pool.Exec(context.Background(), `INSERT INTO user_referral_code (user_id, code) VALUES ($1, $2)`, reg.Profile.ID, code); err != nil {
		t.Fatal(err)
	}
	return reg.Profile.ID
}

func referrerOf(t *testing.T, svc *Service, referee uuid.UUID) (uuid.UUID, bool) {
	t.Helper()
	var id uuid.UUID
	err := svc.Pool.QueryRow(context.Background(), `SELECT referrer_id FROM referral WHERE referee_id=$1`, referee).Scan(&id)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

func TestTelegramLoginAppliesBotReferralToNewProfileOnly(t *testing.T) {
	svc, q := resetTestService(t)
	ctx := context.Background()
	owner := referralOwner(t, svc, "901115500", "REF-AB23CD")
	if err := q.SetTelegramBotUserPendingReferral(ctx, sqlc.SetTelegramBotUserPendingReferralParams{
		TgUserID: 7300, PendingReferralCode: textOf("REF-AB23CD"),
	}); err != nil {
		t.Fatal(err)
	}
	st := startLogin(t, svc, "1.1.1.1")
	if _, err := svc.BeginTelegramLogin(ctx, st.Token, tgWho(7300)); err != nil {
		t.Fatal(err)
	}
	if r := shareAndApprove(t, svc, tgWho(7300), "998901115511"); r.Outcome != TelegramLoginApproved {
		t.Fatal("approve")
	}
	done, err := svc.CompleteTelegramLogin(ctx, st.Token, st.BrowserSecret, "1.1.1.1")
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := referrerOf(t, svc, done.Profile.ID); !ok || got != owner {
		t.Fatalf("referrer = %v %v", got, ok)
	}
	if _, err := q.GetTelegramBotUserPendingReferral(ctx, 7300); err == nil {
		t.Fatal("pending referral must be spent")
	}

	// An existing account never picks a referral up this way.
	existing, err := svc.Register(ctx, RegisterInput{Phone: "901115522", Password: "secret123"})
	if err != nil {
		t.Fatal(err)
	}
	_ = q.SetTelegramBotUserPendingReferral(ctx, sqlc.SetTelegramBotUserPendingReferralParams{TgUserID: 7301, PendingReferralCode: textOf("REF-AB23CD")})
	st2 := startLogin(t, svc, "1.1.1.1")
	_, _ = svc.BeginTelegramLogin(ctx, st2.Token, tgWho(7301))
	if r := shareAndApprove(t, svc, tgWho(7301), "998901115522"); r.Outcome != TelegramLoginApproved {
		t.Fatal("approve existing")
	}
	if _, ok := referrerOf(t, svc, existing.Profile.ID); ok {
		t.Fatal("existing account got a referral")
	}
}

func webAppPhoneProof(t *testing.T, tgID int64, phone, startParam string, at time.Time) (string, string) {
	t.Helper()
	fields := map[string]string{
		"auth_date": strconv.FormatInt(at.Unix(), 10),
		"user":      `{"id":` + strconv.FormatInt(tgID, 10) + `,"first_name":"Zarina","last_name":"Karimova","username":"zk"}`,
	}
	if startParam != "" {
		fields["start_param"] = startParam
	}
	return signInitData(t, testBotToken, fields), signContact(t, testBotToken, tgID, phone, at)
}

func TestWebAppPhoneSignInCreatesLinksAndAppliesStartParamReferral(t *testing.T) {
	svc, ctx := newWebAppService(t)
	owner := referralOwner(t, svc, "901116600", "REF-ZX98YW")
	initData, contact := webAppPhoneProof(t, 7400, "998901116611", "ref_REF-ZX98YW", time.Now())
	res, err := svc.TelegramWebAppPhoneSignIn(ctx, initData, contact, "3.3.3.3")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Created || res.Access == "" || res.Profile.Phone != "+998901116611" || res.Profile.Name != "Zarina Karimova" {
		t.Fatalf("res = %+v", res)
	}
	if got, ok := referrerOf(t, svc, res.Profile.ID); !ok || got != owner {
		t.Fatal("start_param referral not applied")
	}
	if tg, verified, ok := phoneVerifiedOf(t, svc, res.Profile.ID); !ok || tg != 7400 || !verified {
		t.Fatal("not linked phone-verified")
	}
	// Next launch signs in silently.
	login, err := svc.TelegramWebAppLogin(ctx, initData, "3.3.3.3")
	if err != nil || login.NeedPhone || login.Profile.ID != res.Profile.ID {
		t.Fatalf("relaunch = %+v %v", login, err)
	}
	// Second share: same profile, nothing created, no second referral.
	again, err := svc.TelegramWebAppPhoneSignIn(ctx, initData, contact, "3.3.3.3")
	if err != nil || again.Created || again.Profile.ID != res.Profile.ID {
		t.Fatalf("again = %+v %v", again, err)
	}
}

func TestWebAppPhoneSignInExistingAccountAndRefusals(t *testing.T) {
	svc, ctx := newWebAppService(t)
	reg, err := svc.Register(ctx, RegisterInput{Phone: "901116622", Password: "secret123", Name: "Bor"})
	if err != nil {
		t.Fatal(err)
	}
	referralOwner(t, svc, "901116633", "REF-QQ22WW")
	initData, contact := webAppPhoneProof(t, 7401, "998901116622", "ref_REF-QQ22WW", time.Now())
	res, err := svc.TelegramWebAppPhoneSignIn(ctx, initData, contact, "3.3.3.3")
	if err != nil || res.Created || res.Profile.ID != reg.Profile.ID {
		t.Fatalf("existing = %+v %v", res, err)
	}
	if _, ok := referrerOf(t, svc, reg.Profile.ID); ok {
		t.Fatal("existing account got the start_param referral")
	}

	// Contact of another Telegram user.
	init2, _ := webAppPhoneProof(t, 7402, "998901116644", "", time.Now())
	_, foreignContact := webAppPhoneProof(t, 7403, "998901116644", "", time.Now())
	if _, err := svc.TelegramWebAppPhoneSignIn(ctx, init2, foreignContact, "3.3.3.3"); !errors.Is(err, ErrInitDataInvalid) {
		t.Fatalf("other user's contact err = %v", err)
	}
	// Non-UZ number.
	init3, c3 := webAppPhoneProof(t, 7404, "79161234567", "", time.Now())
	if _, err := svc.TelegramWebAppPhoneSignIn(ctx, init3, c3, "3.3.3.3"); !errors.Is(err, ErrInvalidPhone) {
		t.Fatalf("foreign err = %v", err)
	}
	// Stale proof (over the link age).
	init4, c4 := webAppPhoneProof(t, 7405, "998901116655", "", time.Now().Add(-2*time.Hour))
	if _, err := svc.TelegramWebAppPhoneSignIn(ctx, init4, c4, "3.3.3.3"); !errors.Is(err, ErrInitDataExpired) {
		t.Fatalf("stale err = %v", err)
	}
	if n := profilesWithPhone(t, svc, "+998901116644") + profilesWithPhone(t, svc, "+998901116655"); n != 0 {
		t.Fatal("refusals created profiles")
	}
	// Banned.
	if _, err := svc.Pool.Exec(ctx, `UPDATE profile SET status='banned' WHERE id=$1`, reg.Profile.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.TelegramWebAppPhoneSignIn(ctx, initData, contact, "3.3.3.3"); !errors.Is(err, ErrAccountBlocked) {
		t.Fatalf("banned err = %v", err)
	}
	// Kill switch.
	svc.TelegramWebAppURL = ""
	if _, err := svc.TelegramWebAppPhoneSignIn(ctx, initData, contact, "3.3.3.3"); !errors.Is(err, ErrTelegramBotUnconfigured) {
		t.Fatalf("kill switch err = %v", err)
	}
}

func TestWebAppPhoneSignInConcurrentSharesOneProfile(t *testing.T) {
	svc, ctx := newWebAppService(t)
	initData, contact := webAppPhoneProof(t, 7500, "998901117700", "", time.Now())
	var wg sync.WaitGroup
	errs := make([]error, 4)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = svc.TelegramWebAppPhoneSignIn(ctx, initData, contact, "3.3.3."+strconv.Itoa(i))
		}(i)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if n := profilesWithPhone(t, svc, "+998901117700"); n != 1 {
		t.Fatalf("profiles = %d", n)
	}
}

// A Mini App phone share used for the Mini App is not consent to a website
// login the same Telegram user opened in the bot.
func TestMiniAppPhoneShareDisarmsPendingWebsiteLogin(t *testing.T) {
	svc, ctx := newWebAppService(t)
	st := startLogin(t, svc, "1.1.1.1")
	if _, err := svc.BeginTelegramLogin(ctx, st.Token, tgWho(7600)); err != nil {
		t.Fatal(err)
	}
	initData, contact := webAppPhoneProof(t, 7600, "998901118800", "", time.Now())
	if _, err := svc.TelegramWebAppPhoneSignIn(ctx, initData, contact, "3.3.3.3"); err != nil {
		t.Fatal(err)
	}
	// The same contact now reaches the bot.
	if r, err := svc.ConfirmTelegramLoginContact(ctx, tgWho(7600), 7600, "998901118800"); err != nil || r.Outcome != TelegramLoginNone {
		t.Fatalf("contact after Mini App = %+v %v", r, err)
	}
	if loginState(t, svc, st, st.BrowserSecret) != TelegramLoginStatePending {
		t.Fatal("website login must still wait for an explicit approval")
	}
}

func TestTelegramLoginNameIsTrimmedAndCapped(t *testing.T) {
	who := TelegramLoginUser{FirstName: "  " + strings.Repeat("Я", 80) + "\x07", LastName: ""}
	name := telegramProfileName(who)
	if len([]rune(name)) != telegramNameMaxRunes || strings.ContainsRune(name, '\x07') || strings.HasPrefix(name, " ") {
		t.Fatalf("name = %q", name)
	}
	if got := telegramProfileName(TelegramLoginUser{FirstName: " Ali ", LastName: " Vali "}); got != "Ali Vali" {
		t.Fatalf("name = %q", got)
	}
}

func textOf(s string) pgtype.Text { return pgtype.Text{String: s, Valid: true} }

// Audit F4 (B7): whoever merely holds the link must not be able to spend the
// rightful browser's completion budget.
func TestTelegramLoginCompleteBudgetIsTheOwningBrowsers(t *testing.T) {
	svc, _ := resetTestService(t)
	ctx := context.Background()
	st := startLogin(t, svc, "1.1.1.1")
	if _, err := svc.BeginTelegramLogin(ctx, st.Token, tgWho(7700)); err != nil {
		t.Fatal(err)
	}
	if r := shareAndApprove(t, svc, tgWho(7700), "998901117711"); r.Outcome != TelegramLoginApproved {
		t.Fatalf("approve = %+v", r)
	}
	for i := 0; i < 3*telegramLoginCompletePerToken; i++ {
		if _, err := svc.CompleteTelegramLogin(ctx, st.Token, "guess", "6.6.6.6"); !errors.Is(err, ErrTelegramLoginInvalid) {
			t.Fatalf("cookie-less attempt %d err = %v", i, err)
		}
	}
	if _, err := svc.CompleteTelegramLogin(ctx, st.Token, st.BrowserSecret, "1.1.1.1"); err != nil {
		t.Fatalf("the rightful browser was locked out: %v", err)
	}
}

// The owning browser itself is still bounded per request, and one request's
// budget is not another's (a classroom shares an IP).
func TestTelegramLoginCompleteIsLimitedPerRequest(t *testing.T) {
	svc, _ := resetTestService(t)
	ctx := context.Background()
	st := startLogin(t, svc, "1.1.1.1")
	for i := 0; i < telegramLoginCompletePerToken; i++ {
		if _, err := svc.CompleteTelegramLogin(ctx, st.Token, st.BrowserSecret, "1.1.1.1"); !errors.Is(err, ErrTelegramLoginNotApproved) {
			t.Fatalf("attempt %d err = %v", i, err)
		}
	}
	if _, err := svc.CompleteTelegramLogin(ctx, st.Token, st.BrowserSecret, "1.1.1.1"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("err = %v, want rate limited", err)
	}
	other := startLogin(t, svc, "1.1.1.1")
	if _, err := svc.CompleteTelegramLogin(ctx, other.Token, other.BrowserSecret, "1.1.1.1"); !errors.Is(err, ErrTelegramLoginNotApproved) {
		t.Fatalf("other request err = %v", err)
	}
}

// Completes that never prove the browser secret are bounded per client
// address instead (IPv6 per /64).
func TestTelegramLoginCompleteMissesAreLimitedPerIP(t *testing.T) {
	svc, _ := resetTestService(t)
	ctx := context.Background()
	st := startLogin(t, svc, "1.1.1.1")
	if err := svc.Lim.R.Set(ctx, "tglogin:complete:miss:ip:2001:db8:1:2::/64", telegramLoginCompleteMissPerIP, time.Hour).Err(); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CompleteTelegramLogin(ctx, st.Token, "guess", "2001:db8:1:2::99"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("err = %v, want rate limited", err)
	}
	if _, err := svc.CompleteTelegramLogin(ctx, st.Token, st.BrowserSecret, "2001:db8:1:2::99"); !errors.Is(err, ErrTelegramLoginNotApproved) {
		t.Fatalf("the owning browser behind the same prefix: %v", err)
	}
}
