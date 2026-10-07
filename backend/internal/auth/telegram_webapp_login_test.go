package auth

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"avtotest.uz/backend/internal/testdb"
)

func webAppFields(tgID int64, now time.Time) map[string]string {
	return map[string]string{
		"auth_date": strconv.FormatInt(now.Unix(), 10),
		"user":      `{"id":` + strconv.FormatInt(tgID, 10) + `,"first_name":"Ali","username":"ali_uz"}`,
	}
}

func linkedTgUser(t *testing.T, svc *Service, profilePhone string) (int64, string) {
	t.Helper()
	ctx := context.Background()
	var tgID int64
	if err := svc.Pool.QueryRow(ctx, `SELECT tg_user_id FROM telegram_account ta JOIN profile p ON p.id = ta.profile_id WHERE p.phone = $1`, profilePhone).Scan(&tgID); err != nil {
		t.Fatalf("linked tg user for %s: %v", profilePhone, err)
	}
	return tgID, profilePhone
}

func TestTelegramWebAppLoginUnlinkedNeedsPhone(t *testing.T) {
	pool := testdb.New(t)
	svc, _ := newTestService(t, pool)
	svc.TelegramBotToken = testBotToken
	raw := signInitData(t, testBotToken, webAppFields(5001, time.Now()))

	res, err := svc.TelegramWebAppLogin(context.Background(), raw, "203.0.113.5")
	if err != nil {
		t.Fatal(err)
	}
	if !res.NeedPhone || res.Access != "" || res.FirstName != "Ali" {
		t.Fatalf("result = %+v", res)
	}
}

func TestLoginWithInitDataLinksThenWebAppLoginIssuesSession(t *testing.T) {
	pool := testdb.New(t)
	svc, _ := newTestService(t, pool)
	svc.TelegramBotToken = testBotToken
	ctx := context.Background()
	const phone, pw = "+998901110001", "linking-password-1"
	if _, err := svc.Register(ctx, RegisterInput{Phone: phone, Password: pw, Name: "A"}); err != nil {
		t.Fatal(err)
	}
	raw := signInitData(t, testBotToken, webAppFields(5002, time.Now()))

	login, err := svc.Login(ctx, LoginInput{Phone: phone, Password: pw, TgInitData: raw})
	if err != nil {
		t.Fatal(err)
	}
	if !login.TelegramLinked {
		t.Fatal("login with valid init data must link")
	}
	res, err := svc.TelegramWebAppLogin(ctx, raw, "")
	if err != nil {
		t.Fatal(err)
	}
	if res.NeedPhone || res.Access == "" || res.Refresh == "" || res.Profile.ID != login.Profile.ID {
		t.Fatalf("webapp login = %+v", res)
	}
	var sessions int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM refresh_token WHERE profile_id=$1 AND revoked_at IS NULL`, login.Profile.ID).Scan(&sessions)
	if sessions != 3 { // register + login + webapp: each a separate device, none revoked
		t.Fatalf("active sessions = %d, want 3", sessions)
	}
}

func TestRegisterWithInitDataMovesLinkFromOtherProfile(t *testing.T) {
	pool := testdb.New(t)
	svc, _ := newTestService(t, pool)
	svc.TelegramBotToken = testBotToken
	ctx := context.Background()
	raw := signInitData(t, testBotToken, webAppFields(5003, time.Now()))
	first, err := svc.Register(ctx, RegisterInput{Phone: "+998901110002", Password: "first-password-1", Name: "A", TgInitData: raw})
	if err != nil || !first.TelegramLinked {
		t.Fatalf("first register: %+v %v", first, err)
	}
	second, err := svc.Register(ctx, RegisterInput{Phone: "+998901110003", Password: "second-password-1", Name: "B", TgInitData: raw})
	if err != nil || !second.TelegramLinked {
		t.Fatalf("second register: %+v %v", second, err)
	}
	tgID, _ := linkedTgUser(t, svc, "+998901110003")
	if tgID != 5003 {
		t.Fatalf("tg id = %d", tgID)
	}
	var left int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM telegram_account WHERE profile_id=$1`, first.Profile.ID).Scan(&left)
	if left != 0 {
		t.Fatal("link must move off the first profile")
	}
}

func TestLoginWithInvalidInitDataStillSignsIn(t *testing.T) {
	pool := testdb.New(t)
	svc, _ := newTestService(t, pool)
	svc.TelegramBotToken = testBotToken
	ctx := context.Background()
	const phone, pw = "+998901110004", "invalid-init-password"
	if _, err := svc.Register(ctx, RegisterInput{Phone: phone, Password: pw, Name: "A"}); err != nil {
		t.Fatal(err)
	}
	res, err := svc.Login(ctx, LoginInput{Phone: phone, Password: pw, TgInitData: "user=%7B%7D&hash=00"})
	if err != nil {
		t.Fatalf("login must not fail on bad init data: %v", err)
	}
	if res.TelegramLinked || res.Access == "" {
		t.Fatalf("res = %+v", res)
	}
}

func TestLoginWithoutInitDataDoesNotTouchTelegramAccount(t *testing.T) {
	pool := testdb.New(t)
	svc, _ := newTestService(t, pool)
	svc.TelegramBotToken = testBotToken
	ctx := context.Background()
	raw := signInitData(t, testBotToken, webAppFields(5005, time.Now()))
	reg, err := svc.Register(ctx, RegisterInput{Phone: "+998901110005", Password: "plain-password-11", Name: "A", TgInitData: raw})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Login(ctx, LoginInput{Phone: "+998901110005", Password: "plain-password-11"}); err != nil {
		t.Fatal(err)
	}
	var n int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM telegram_account WHERE profile_id=$1`, reg.Profile.ID).Scan(&n)
	if n != 1 {
		t.Fatalf("plain login changed telegram_account rows: %d", n)
	}
}

func TestTelegramWebAppLoginRejectsBannedAndBadData(t *testing.T) {
	pool := testdb.New(t)
	svc, _ := newTestService(t, pool)
	svc.TelegramBotToken = testBotToken
	ctx := context.Background()
	raw := signInitData(t, testBotToken, webAppFields(5006, time.Now()))
	reg, err := svc.Register(ctx, RegisterInput{Phone: "+998901110006", Password: "banned-password-1", Name: "A", TgInitData: raw})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE profile SET status='banned' WHERE id=$1`, reg.Profile.ID); err != nil {
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

func TestTelegramWebAppLoginRateLimitsGarbageByIP(t *testing.T) {
	pool := testdb.New(t)
	svc, _ := newTestService(t, pool)
	svc.TelegramBotToken = testBotToken
	ctx := context.Background()
	for i := 0; i < 60; i++ {
		if _, err := svc.TelegramWebAppLogin(ctx, "hash=00", "203.0.113.9"); !errors.Is(err, ErrInitDataInvalid) {
			t.Fatalf("call %d: err = %v", i, err)
		}
	}
	if _, err := svc.TelegramWebAppLogin(ctx, "hash=00", "203.0.113.9"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("61st garbage call must be throttled before validation, err = %v", err)
	}
}

func TestLoginIgnoresOversizedInitData(t *testing.T) {
	pool := testdb.New(t)
	svc, _ := newTestService(t, pool)
	svc.TelegramBotToken = testBotToken
	ctx := context.Background()
	if _, err := svc.Register(ctx, RegisterInput{Phone: "+998901110007", Password: "oversize-password-1", Name: "A"}); err != nil {
		t.Fatal(err)
	}
	res, err := svc.Login(ctx, LoginInput{Phone: "+998901110007", Password: "oversize-password-1", TgInitData: strings.Repeat("a", InitDataMaxBytes+1)})
	if err != nil || res.TelegramLinked {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

// Two people complete a Mini App sign-in with the same Telegram account at
// once. The unique constraint admits one link; the loser must still be signed
// in (link skipped) and exactly one row may remain.
func TestConcurrentMiniAppLoginsLinkSameTelegramAccountOnce(t *testing.T) {
	pool := testdb.New(t)
	svc, _ := newTestService(t, pool)
	svc.TelegramBotToken = testBotToken
	ctx := context.Background()
	raw := signInitData(t, testBotToken, webAppFields(5008, time.Now()))
	phones := []string{"+998901110008", "+998901110009", "+998901110010", "+998901110011"}
	for _, p := range phones {
		if _, err := svc.Register(ctx, RegisterInput{Phone: p, Password: "concurrent-password-1", Name: "A"}); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	errs := make([]error, len(phones))
	res := make([]VerifyResult, len(phones))
	for i, p := range phones {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res[i], errs[i] = svc.Login(ctx, LoginInput{Phone: p, Password: "concurrent-password-1", TgInitData: raw})
		}()
	}
	wg.Wait()
	for i := range phones {
		if errs[i] != nil || res[i].Access == "" {
			t.Fatalf("login %d must succeed: %v", i, errs[i])
		}
	}
	var n int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM telegram_account WHERE tg_user_id=5008`).Scan(&n)
	if n != 1 {
		t.Fatalf("telegram_account rows for tg 5008 = %d, want 1", n)
	}
}
