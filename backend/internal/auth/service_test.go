package auth

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/redisx"
	"avtotest.uz/backend/internal/testdb"
)

type captureSender struct{ last string }

func (c *captureSender) Send(_ context.Context, _, code string) error {
	c.last = code
	return nil
}

func (*captureSender) Channel() string { return "sandbox" }

func newTestService(t *testing.T, pool *pgxpool.Pool) (*Service, *captureSender) {
	t.Helper()
	c := redisx.NewTest(t)
	sender := &captureSender{}
	return NewService(sqlc.New(pool), pool, Limiter{R: c}, sender, []byte("test-secret"), "dev"), sender
}

const testPhone = "+998901234567"

func TestRequestVerifyOTPHappyPath(t *testing.T) {
	pool := testdb.New(t)
	svc, sender := newTestService(t, pool)
	ctx := context.Background()

	if _, err := svc.RequestOTP(ctx, "901234567", "1.2.3.4"); err != nil {
		t.Fatalf("RequestOTP: %v", err)
	}
	res, err := svc.VerifyOTP(ctx, "901234567", sender.last)
	if err != nil {
		t.Fatalf("VerifyOTP: %v", err)
	}
	if res.Access == "" || res.Refresh == "" {
		t.Fatal("expected non-empty tokens")
	}
	if !res.Created {
		t.Fatal("expected Created=true on first sign-in")
	}
	if !res.Profile.ReferralCode.Valid || len(res.Profile.ReferralCode.String) != 8 {
		t.Fatalf("referral code = %+v", res.Profile.ReferralCode)
	}
}

// TestSignupGrantsTrialEntitlement pins the welcome trial: a first sign-in
// must land with full access already active, and the grant must be recorded
// as a trial so it never counts as revenue. A repeat sign-in must not extend
// it — otherwise logging out and back in would renew VIP forever.
func TestSignupGrantsTrialEntitlement(t *testing.T) {
	pool := testdb.New(t)
	svc, sender := newTestService(t, pool)
	ctx := context.Background()

	if _, err := svc.RequestOTP(ctx, testPhone, ""); err != nil {
		t.Fatalf("RequestOTP: %v", err)
	}
	res, err := svc.VerifyOTP(ctx, testPhone, sender.last)
	if err != nil {
		t.Fatalf("VerifyOTP: %v", err)
	}

	var source string
	var startsAt, endsAt time.Time
	row := pool.QueryRow(ctx,
		`SELECT source, starts_at, ends_at FROM entitlement WHERE profile_id = $1`, res.Profile.ID)
	if err := row.Scan(&source, &startsAt, &endsAt); err != nil {
		t.Fatalf("expected a trial entitlement for a new profile: %v", err)
	}
	if source != "trial" {
		t.Fatalf("source = %q, want trial", source)
	}
	if got := endsAt.Sub(startsAt); got != SignupTrialDuration {
		t.Fatalf("trial length = %v, want %v", got, SignupTrialDuration)
	}

	// clear the resend cooldown so the second request isn't rate limited
	svc.Lim.R.Del(ctx, "otp:cooldown:"+testPhone)
	if _, err := svc.RequestOTP(ctx, testPhone, ""); err != nil {
		t.Fatalf("second RequestOTP: %v", err)
	}
	if _, err := svc.VerifyOTP(ctx, testPhone, sender.last); err != nil {
		t.Fatalf("second VerifyOTP: %v", err)
	}
	var count int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM entitlement WHERE profile_id = $1`, res.Profile.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("entitlement rows after re-login = %d, want 1", count)
	}
}

func TestVerifyOTPSecondSignInNotCreated(t *testing.T) {
	pool := testdb.New(t)
	svc, sender := newTestService(t, pool)
	ctx := context.Background()

	if _, err := svc.RequestOTP(ctx, testPhone, ""); err != nil {
		t.Fatalf("first request: %v", err)
	}
	if _, err := svc.VerifyOTP(ctx, testPhone, sender.last); err != nil {
		t.Fatalf("first verify: %v", err)
	}

	// clear the resend cooldown so the second request isn't rate limited
	svc.Lim.R.Del(ctx, "otp:cooldown:"+testPhone)
	if _, err := svc.RequestOTP(ctx, testPhone, ""); err != nil {
		t.Fatalf("second request: %v", err)
	}
	res, err := svc.VerifyOTP(ctx, testPhone, sender.last)
	if err != nil {
		t.Fatalf("second verify: %v", err)
	}
	if res.Created {
		t.Fatal("expected Created=false on repeat sign-in")
	}
}

func TestVerifyOTPTooManyAttempts(t *testing.T) {
	pool := testdb.New(t)
	svc, sender := newTestService(t, pool)
	ctx := context.Background()

	if _, err := svc.RequestOTP(ctx, testPhone, ""); err != nil {
		t.Fatalf("request: %v", err)
	}
	right := sender.last

	for i := 0; i < 5; i++ {
		if _, err := svc.VerifyOTP(ctx, testPhone, "000000"); !errors.Is(err, ErrInvalidCode) {
			t.Fatalf("attempt %d: err=%v want ErrInvalidCode", i+1, err)
		}
	}
	if _, err := svc.VerifyOTP(ctx, testPhone, right); !errors.Is(err, ErrTooManyAttempts) {
		t.Fatalf("6th attempt: err=%v want ErrTooManyAttempts", err)
	}
}

func TestVerifyOTPExpiredCode(t *testing.T) {
	pool := testdb.New(t)
	svc, sender := newTestService(t, pool)
	svc.CodeTTL = -time.Second
	ctx := context.Background()

	if _, err := svc.RequestOTP(ctx, testPhone, ""); err != nil {
		t.Fatalf("request: %v", err)
	}
	if _, err := svc.VerifyOTP(ctx, testPhone, sender.last); !errors.Is(err, ErrExpiredCode) {
		t.Fatalf("err=%v want ErrExpiredCode", err)
	}
}

func TestRefreshRotationAndReuseDetection(t *testing.T) {
	pool := testdb.New(t)
	svc, sender := newTestService(t, pool)
	ctx := context.Background()

	if _, err := svc.RequestOTP(ctx, testPhone, ""); err != nil {
		t.Fatalf("request: %v", err)
	}
	verifyRes, err := svc.VerifyOTP(ctx, testPhone, sender.last)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	oldRefresh := verifyRes.Refresh

	rotated, err := svc.Refresh(ctx, oldRefresh)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if rotated.Access == "" || rotated.Refresh == "" || rotated.Refresh == oldRefresh {
		t.Fatalf("expected new distinct tokens: %+v", rotated)
	}

	// Within the grace window, presenting the old token must return the same
	// successor pair — not revoke-all (parallel BFF / multi-tab).
	again, err := svc.Refresh(ctx, oldRefresh)
	if err != nil {
		t.Fatalf("grace reuse: %v", err)
	}
	if again.Access != rotated.Access || again.Refresh != rotated.Refresh {
		t.Fatalf("grace reuse tokens=%+v want %+v", again, rotated)
	}

	// Past grace: reuse is theft → revoke-all, including the rotated token.
	if err := svc.Lim.R.Del(ctx, refreshGraceKey(oldRefresh), refreshClaimKey(oldRefresh)).Err(); err != nil {
		t.Fatalf("clear grace: %v", err)
	}
	if _, err := svc.Refresh(ctx, oldRefresh); !errors.Is(err, ErrReusedRefresh) {
		t.Fatalf("reuse of old token after grace: err=%v want ErrReusedRefresh", err)
	}

	if _, err := svc.Refresh(ctx, rotated.Refresh); !errors.Is(err, ErrReusedRefresh) {
		t.Fatalf("rotated token should have been revoked by revoke-all: err=%v want ErrReusedRefresh", err)
	}
}

func TestRefreshConcurrentCallersShareGracePair(t *testing.T) {
	pool := testdb.New(t)
	svc, sender := newTestService(t, pool)
	ctx := context.Background()

	if _, err := svc.RequestOTP(ctx, testPhone, ""); err != nil {
		t.Fatalf("request: %v", err)
	}
	verifyRes, err := svc.VerifyOTP(ctx, testPhone, sender.last)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	oldRefresh := verifyRes.Refresh

	const n = 8
	results := make([]Tokens, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = svc.Refresh(ctx, oldRefresh)
		}(i)
	}
	wg.Wait()

	var first Tokens
	for i := 0; i < n; i++ {
		if errs[i] != nil {
			t.Fatalf("caller %d: %v", i, errs[i])
		}
		if results[i].Access == "" || results[i].Refresh == "" {
			t.Fatalf("caller %d empty tokens", i)
		}
		if first.Refresh == "" {
			first = results[i]
			continue
		}
		if results[i].Refresh != first.Refresh || results[i].Access != first.Access {
			t.Fatalf("caller %d got different pair %+v want %+v", i, results[i], first)
		}
	}
}

func TestLogoutThenRefreshFails(t *testing.T) {
	pool := testdb.New(t)
	svc, sender := newTestService(t, pool)
	ctx := context.Background()

	if _, err := svc.RequestOTP(ctx, testPhone, ""); err != nil {
		t.Fatalf("request: %v", err)
	}
	verifyRes, err := svc.VerifyOTP(ctx, testPhone, sender.last)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}

	if err := svc.Logout(ctx, verifyRes.Refresh); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if _, err := svc.Refresh(ctx, verifyRes.Refresh); !errors.Is(err, ErrInvalidRefresh) {
		t.Fatalf("err=%v want ErrInvalidRefresh", err)
	}
	if err := svc.Logout(ctx, verifyRes.Refresh); err != nil {
		t.Fatalf("logout on missing token should be a no-op: %v", err)
	}
}

func TestRequestOTPCooldown(t *testing.T) {
	pool := testdb.New(t)
	svc, _ := newTestService(t, pool)
	ctx := context.Background()

	if _, err := svc.RequestOTP(ctx, testPhone, ""); err != nil {
		t.Fatalf("first request: %v", err)
	}
	if _, err := svc.RequestOTP(ctx, testPhone, ""); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("err=%v want ErrRateLimited", err)
	}
}

func TestRegisterLoginHappyPath(t *testing.T) {
	pool := testdb.New(t)
	svc, _ := newTestService(t, pool)
	ctx := context.Background()

	reg, err := svc.Register(ctx, RegisterInput{
		Phone:    "901234567",
		Password: "secret123",
		Name:     "Ali",
		IP:       "1.1.1.1",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if !reg.Created || reg.Access == "" || reg.Refresh == "" {
		t.Fatal("expected created profile with tokens")
	}
	if reg.Profile.Name != "Ali" {
		t.Fatalf("name=%q", reg.Profile.Name)
	}
	if !reg.Profile.PasswordHash.Valid || reg.Profile.PasswordHash.String == "secret123" {
		t.Fatal("password must be stored as bcrypt hash")
	}

	login, err := svc.Login(ctx, LoginInput{Phone: "901234567", Password: "secret123", IP: "1.1.1.1"})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if login.Created || login.Access == "" {
		t.Fatal("expected session on login")
	}

	if _, err := svc.Login(ctx, LoginInput{Phone: "901234567", Password: "wrongpass", IP: "1.1.1.1"}); !errors.Is(err, ErrInvalidCreds) {
		t.Fatalf("err=%v want ErrInvalidCreds", err)
	}
	if _, err := svc.Register(ctx, RegisterInput{Phone: "901234567", Password: "secret123", IP: "1.1.1.1"}); !errors.Is(err, ErrPhoneTaken) {
		t.Fatalf("err=%v want ErrPhoneTaken", err)
	}
}

func TestLoginAndRefreshRejectBanned(t *testing.T) {
	pool := testdb.New(t)
	svc, _ := newTestService(t, pool)
	ctx := context.Background()

	reg, err := svc.Register(ctx, RegisterInput{
		Phone:    "901112233",
		Password: "secret123",
		IP:       "1.1.1.1",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	if _, err := pool.Exec(ctx, `UPDATE profile SET status = 'banned' WHERE id = $1`, reg.Profile.ID); err != nil {
		t.Fatalf("ban: %v", err)
	}

	if _, err := svc.Login(ctx, LoginInput{Phone: "901112233", Password: "secret123", IP: "1.1.1.1"}); !errors.Is(err, ErrAccountBlocked) {
		t.Fatalf("login err=%v want ErrAccountBlocked", err)
	}

	if _, err := svc.Refresh(ctx, reg.Refresh); !errors.Is(err, ErrAccountBlocked) {
		t.Fatalf("refresh err=%v want ErrAccountBlocked", err)
	}
}

func TestLoginPasswordNotSetThenSetPassword(t *testing.T) {
	pool := testdb.New(t)
	svc, sender := newTestService(t, pool)
	ctx := context.Background()

	if _, err := svc.RequestOTP(ctx, testPhone, ""); err != nil {
		t.Fatalf("RequestOTP: %v", err)
	}
	if _, err := svc.VerifyOTP(ctx, testPhone, sender.last); err != nil {
		t.Fatalf("VerifyOTP: %v", err)
	}

	if _, err := svc.Login(ctx, LoginInput{Phone: testPhone, Password: "secret123", IP: ""}); !errors.Is(err, ErrPasswordNotSet) {
		t.Fatalf("err=%v want ErrPasswordNotSet", err)
	}

	set, err := svc.SetPassword(ctx, SetPasswordInput{Phone: testPhone, Password: "secret123", IP: ""})
	if err != nil {
		t.Fatalf("SetPassword: %v", err)
	}
	if set.Access == "" {
		t.Fatal("expected tokens after set-password")
	}

	if _, err := svc.SetPassword(ctx, SetPasswordInput{Phone: testPhone, Password: "another12", IP: ""}); !errors.Is(err, ErrPasswordSet) {
		t.Fatalf("err=%v want ErrPasswordSet", err)
	}

	if _, err := svc.Login(ctx, LoginInput{Phone: testPhone, Password: "secret123", IP: ""}); err != nil {
		t.Fatalf("Login after set-password: %v", err)
	}
}

func TestRegisterWeakPassword(t *testing.T) {
	pool := testdb.New(t)
	svc, _ := newTestService(t, pool)
	if _, err := svc.Register(context.Background(), RegisterInput{Phone: "901234567", Password: "short"}); !errors.Is(err, ErrWeakPassword) {
		t.Fatalf("err=%v want ErrWeakPassword", err)
	}
}

// TestRequestOTPNeverEchoesCodeByDefault pins the fix for a live account
// takeover: the OTP used to be echoed in the HTTP response whenever
// Env=="dev" and the sandbox sender was active. Production ran with
// ENV=dev + OTP_CHANNEL=sandbox, so POST /auth/otp/request returned a
// working code for ANY phone number. The echo is now an explicit opt-in
// (DebugEcho) that defaults to off, so an Env misconfiguration alone can
// never re-open it. newTestService deliberately builds the exact old
// trigger conditions — Env "dev" plus a sandbox-channel sender.
func TestRequestOTPNeverEchoesCodeByDefault(t *testing.T) {
	pool := testdb.New(t)
	svc, sender := newTestService(t, pool)

	res, err := svc.RequestOTP(context.Background(), "901234567", "1.2.3.4")
	if err != nil {
		t.Fatalf("RequestOTP: %v", err)
	}
	if res.DebugCode != "" {
		t.Errorf("DebugCode = %q, want empty: the OTP must never reach the response without an explicit opt-in", res.DebugCode)
	}
	if sender.last == "" {
		t.Fatal("sender received no code: the OTP should still be generated and delivered")
	}
}

// TestRequestOTPEchoesCodeWhenExplicitlyEnabled keeps the local-dev
// convenience working, so the fix above cannot be "reverted" later on the
// grounds that it broke developer flow.
func TestRequestOTPEchoesCodeWhenExplicitlyEnabled(t *testing.T) {
	pool := testdb.New(t)
	svc, sender := newTestService(t, pool)
	svc.DebugEcho = true

	res, err := svc.RequestOTP(context.Background(), "901234567", "1.2.3.4")
	if err != nil {
		t.Fatalf("RequestOTP: %v", err)
	}
	if res.DebugCode != sender.last {
		t.Errorf("DebugCode = %q, want the delivered code %q", res.DebugCode, sender.last)
	}
}
