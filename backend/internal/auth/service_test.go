package auth

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
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

// rotateOnce signs a fresh profile in and rotates its refresh token once, so
// the grace entry for oldRefresh exists and holds the returned pair.
func rotateOnce(t *testing.T, svc *Service, sender *captureSender, phone string) (profile sqlc.Profile, oldRefresh string, rotated Tokens) {
	t.Helper()
	ctx := context.Background()

	if _, err := svc.RequestOTP(ctx, phone, ""); err != nil {
		t.Fatalf("request: %v", err)
	}
	verifyRes, err := svc.VerifyOTP(ctx, phone, sender.last)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	rotated, err = svc.Refresh(ctx, verifyRes.Refresh)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if rotated.Access == "" || rotated.Refresh == "" {
		t.Fatalf("rotation returned empty tokens: %+v", rotated)
	}
	return verifyRes.Profile, verifyRes.Refresh, rotated
}

// TestRefreshGracePathRejectsBannedProfile pins the fix for a ban bypass: the
// grace fast path returned its cached successor pair before any DB lookup, so
// assertProfileActive never ran. A profile banned inside the 45s window still
// received a fresh 15-minute access token — and since clients refresh exactly
// when their access token is about to expire, a just-banned user was very
// likely to land on this path.
func TestRefreshGracePathRejectsBannedProfile(t *testing.T) {
	pool := testdb.New(t)
	svc, sender := newTestService(t, pool)
	ctx := context.Background()

	profile, oldRefresh, rotated := rotateOnce(t, svc, sender, testPhone)

	// While the account is active the grace entry serves the same pair; that is
	// what must keep working, and it proves the entry is live for the ban below.
	again, err := svc.Refresh(ctx, oldRefresh)
	if err != nil {
		t.Fatalf("grace reuse before ban: %v", err)
	}
	if again != rotated {
		t.Fatalf("grace reuse tokens=%+v want %+v", again, rotated)
	}

	if _, err := pool.Exec(ctx, `UPDATE profile SET status = 'banned' WHERE id = $1`, profile.ID); err != nil {
		t.Fatalf("ban: %v", err)
	}

	got, err := svc.Refresh(ctx, oldRefresh)
	if !errors.Is(err, ErrAccountBlocked) {
		t.Fatalf("grace refresh after ban: err=%v want ErrAccountBlocked", err)
	}
	if got.Access != "" || got.Refresh != "" {
		t.Fatalf("grace path handed tokens to a banned profile: %+v", got)
	}

	// The successor token is refused too, so the ban is total, not just on the
	// stale cookie.
	if _, err := svc.Refresh(ctx, rotated.Refresh); !errors.Is(err, ErrAccountBlocked) {
		t.Fatalf("rotated token after ban: err=%v want ErrAccountBlocked", err)
	}

	// Un-banning restores service from the very same grace entry: the check is a
	// live status read, not a one-way poisoning of the cached pair.
	if _, err := pool.Exec(ctx, `UPDATE profile SET status = 'active' WHERE id = $1`, profile.ID); err != nil {
		t.Fatalf("unban: %v", err)
	}
	restored, err := svc.Refresh(ctx, oldRefresh)
	if err != nil {
		t.Fatalf("grace refresh after unban: %v", err)
	}
	if restored != rotated {
		t.Fatalf("after unban tokens=%+v want %+v", restored, rotated)
	}
}

// TestRefreshGraceValueIsUselessInARedisDump pins the fix for plaintext token
// storage: the grace entry used to be JSON holding the live access and refresh
// tokens, so `redis-cli --scan`, a leaked RDB snapshot or a compromised
// read-replica handed over working sessions for every account that had
// refreshed in the last 45 seconds.
func TestRefreshGraceValueIsUselessInARedisDump(t *testing.T) {
	pool := testdb.New(t)
	svc, sender := newTestService(t, pool)
	ctx := context.Background()

	profile, oldRefresh, rotated := rotateOnce(t, svc, sender, testPhone)

	stored, err := svc.Lim.R.Get(ctx, refreshGraceKey(oldRefresh)).Result()
	if err != nil {
		t.Fatalf("grace entry missing, nothing to assert about: %v", err)
	}

	// Whole-keyspace dump: no value anywhere may carry a usable token.
	keys, err := svc.Lim.R.Keys(ctx, "*").Result()
	if err != nil {
		t.Fatal(err)
	}
	secrets := map[string]string{
		"successor access token":  rotated.Access,
		"successor refresh token": rotated.Refresh,
		"presented refresh token": oldRefresh,
	}
	for _, k := range keys {
		val, err := svc.Lim.R.Get(ctx, k).Result()
		if err != nil {
			continue // non-string key: no plaintext to leak
		}
		for name, secret := range secrets {
			if strings.Contains(val, secret) {
				t.Fatalf("redis key %q leaks the %s in plaintext", k, name)
			}
		}
	}

	blob, err := base64.RawURLEncoding.DecodeString(stored)
	if err != nil {
		t.Fatalf("grace value is not base64url: %q", stored)
	}
	for name, secret := range secrets {
		if bytes.Contains(blob, []byte(secret)) {
			t.Fatalf("decoded grace value still contains the %s", name)
		}
	}
	var probe storedRefreshPair
	if err := json.Unmarshal([]byte(stored), &probe); err == nil {
		t.Fatalf("grace value is still readable JSON: %+v", probe)
	}

	// An attacker holding the dump — and even the service secret, which svc has —
	// cannot open the entry without the pre-rotation token itself. The Redis key
	// only exposes sha256(token), so there is nothing in the dump to derive it
	// from.
	otherRaw, err := NewRefreshToken()
	if err != nil {
		t.Fatal(err)
	}
	for name, wrong := range map[string]string{
		"successor token": rotated.Refresh,
		"empty token":     "",
		"unrelated token": otherRaw,
		"key digest":      HashToken(oldRefresh),
	} {
		if _, ok := svc.openRefreshGrace(wrong, stored); ok {
			t.Fatalf("grace entry opened with the %s", name)
		}
	}

	// Copying the blob under another entry does not help either: the key is
	// bound to the token that entry belongs to.
	if err := svc.Lim.R.Set(ctx, refreshGraceKey(otherRaw), stored, refreshGraceTTL).Err(); err != nil {
		t.Fatal(err)
	}
	if _, ok := svc.readRefreshGrace(ctx, otherRaw); ok {
		t.Fatal("a grace blob moved to another key still opened")
	}

	// The legitimate holder of the pre-rotation token still gets the exact pair,
	// which is the whole point of the grace window.
	pair, ok := svc.openRefreshGrace(oldRefresh, stored)
	if !ok {
		t.Fatal("the pre-rotation token could not open its own grace entry")
	}
	if pair.Access != rotated.Access || pair.Refresh != rotated.Refresh {
		t.Fatalf("opened pair = %+v, want %+v", pair, rotated)
	}
	if pair.ProfileID != profile.ID {
		t.Fatalf("grace entry profile = %s, want %s", pair.ProfileID, profile.ID)
	}
}

// TestRefreshGraceEntryFromAnotherSecretDoesNotRevokeEverything guards the
// rollout/rotation edge of sealing the grace entry: a value the current secret
// cannot open must degrade to a plain grace miss (the DB decides), exactly like
// the corrupt-JSON case did before.
func TestRefreshGraceEntryFromAnotherSecretDoesNotRevokeEverything(t *testing.T) {
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

	stale := *svc
	stale.Secret = []byte("previous-secret")
	sealed, err := stale.sealRefreshGrace(verifyRes.Refresh, storedRefreshPair{
		Access: "stale-access", Refresh: "stale-refresh", ProfileID: verifyRes.Profile.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Lim.R.Set(ctx, refreshGraceKey(verifyRes.Refresh), sealed, refreshGraceTTL).Err(); err != nil {
		t.Fatal(err)
	}

	tokens, err := svc.Refresh(ctx, verifyRes.Refresh)
	if err != nil {
		t.Fatalf("refresh over an unreadable grace entry: %v", err)
	}
	if tokens.Refresh == "stale-refresh" || tokens.Access == "stale-access" {
		t.Fatal("an entry sealed under a different secret was served to the caller")
	}
	if tokens.Refresh == verifyRes.Refresh {
		t.Fatal("expected a real rotation")
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

func TestPasswordlessOTPAccountCannotClaimPasswordWithoutAnotherProof(t *testing.T) {
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

	var passwordHash *string
	if err := pool.QueryRow(ctx, `SELECT password_hash FROM profile WHERE phone = $1`, testPhone).Scan(&passwordHash); err != nil {
		t.Fatal(err)
	}
	if passwordHash != nil {
		t.Fatal("OTP-only account unexpectedly acquired a password")
	}
}

func TestRegisterWeakPassword(t *testing.T) {
	pool := testdb.New(t)
	svc, _ := newTestService(t, pool)
	if _, err := svc.Register(context.Background(), RegisterInput{Phone: "901234567", Password: "short"}); !errors.Is(err, ErrWeakPassword) {
		t.Fatalf("err=%v want ErrWeakPassword", err)
	}
}

func TestRegisterRejectsDuplicatePhone(t *testing.T) {
	pool := testdb.New(t)
	svc, _ := newTestService(t, pool)
	ctx := context.Background()

	if _, err := svc.Register(ctx, RegisterInput{
		Phone: "901234567", Password: "secret123", IP: "1.1.1.1",
	}); err != nil {
		t.Fatalf("first register: %v", err)
	}
	if _, err := svc.Register(ctx, RegisterInput{
		Phone: "901234567", Password: "secret123", IP: "1.1.1.1",
	}); !errors.Is(err, ErrPhoneTaken) {
		t.Fatalf("err=%v want ErrPhoneTaken", err)
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

type disabledChannelSender struct{ called bool }

func (d *disabledChannelSender) Send(context.Context, string, string) error {
	d.called = true
	return nil
}

func (*disabledChannelSender) Channel() string { return "off" }

// TestOTPRefusedWhenChannelOff covers the deployment this product actually
// runs: sign-in is phone+password and no SMS/Telegram gateway exists, so
// OTP_CHANNEL=off keeps the endpoints mounted but inert. Both halves must
// refuse — request so no code is ever minted, and verify so a challenge
// created before the switch cannot still be redeemed into a session.
func TestOTPRefusedWhenChannelOff(t *testing.T) {
	pool := testdb.New(t)
	c := redisx.NewTest(t)
	sender := &disabledChannelSender{}
	svc := NewService(sqlc.New(pool), pool, Limiter{R: c}, sender, []byte("test-secret"), "prod")
	ctx := context.Background()

	if _, err := svc.RequestOTP(ctx, "901234567", "1.2.3.4"); !errors.Is(err, ErrOTPDisabled) {
		t.Fatalf("RequestOTP err = %v, want ErrOTPDisabled", err)
	}
	if sender.called {
		t.Error("sender was invoked: no code should be generated when the channel is off")
	}
	var challenges int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM otp_challenge").Scan(&challenges); err != nil {
		t.Fatalf("count otp_challenge: %v", err)
	}
	if challenges != 0 {
		t.Errorf("otp_challenge rows = %d, want 0", challenges)
	}

	if _, err := svc.VerifyOTP(ctx, "901234567", "123456"); !errors.Is(err, ErrOTPDisabled) {
		t.Fatalf("VerifyOTP err = %v, want ErrOTPDisabled", err)
	}
}
