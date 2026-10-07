# Telegram Mini App Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Open the DriverGo learner app as a Telegram Mini App from the existing bot, with instant sign-in for linked Telegram users and the unchanged phone + password sign-up/sign-in for everyone else.

**Architecture:** Backend validates Telegram `initData` (HMAC with the bot token) and either issues a normal session for a linked `telegram_account` or answers `need_phone`; phone login/register accept an optional `tg_init_data` that links the account in the same transaction. The Next BFF issues the same `at`/`rt` cookies in a `SameSite=None; Partitioned` "telegram mode" (marked by a `tgp` cookie) and gains an Origin guard. The frontend loads Telegram's SDK only inside Telegram and adapts chrome (theme, BackButton, safe area, haptics, closing confirmation).

**Spec:** `docs/superpowers/specs/2026-10-07-telegram-mini-app-design.md` — read it first.

**Tech Stack:** Go 1.x + chi + pgx/sqlc (backend), Next 16.3 + React 19 + next-intl 4 + next-themes + vitest + Playwright (frontend).

## Global Constraints

- Work directly on `main` (solo project convention). Commit after every task. Never `--no-verify`.
- Go toolchain: `export PATH=/home/sher/.local/go/bin:$PATH`. Backend tests: `cd backend && TEST_DATABASE_URL=postgres://avtotest:avtotest@localhost:5432/avtotest_test?sslmode=disable go test -p 1 ./internal/auth/... -count=1` (start DB with `make up` from repo root if needed).
- Backend lint gate before finishing any backend task: `cd backend && golangci-lint run ./...` (`go vet` is not enough — CI uses golangci-lint).
- sqlc: regenerate with `cd backend && sqlc generate` (v1.31.1). Never hand-edit `internal/db/sqlc/*.go`.
- Frontend gates: `cd frontend && rm -rf .next && npx tsc --noEmit && npm run lint && npx vitest run`; e2e: `CI=true PORT=3112 npx playwright test` (no backend; stub APIs with `page.route`).
- Absent `tg_init_data` ⇒ login/register behaviour byte-for-byte unchanged. Website cookies stay `SameSite=lax`. Nothing Telegram-specific may execute outside Telegram.
- No new npm or Go dependencies.
- Learner-facing copy exists in all three locale files: `frontend/messages/uz-Latn.json`, `uz-Cyrl.json`, `ru.json`.
- Comments explain *why*, matching the surrounding density.

---

### Task 1: `initData` validator (Go, pure)

**Files:**
- Create: `backend/internal/auth/telegram_webapp.go`
- Test: `backend/internal/auth/telegram_webapp_test.go`

**Interfaces:**
- Produces:
  - `type WebAppUser struct { ID int64; FirstName, Username, LanguageCode string }`
  - `func ValidateInitData(raw, botToken string, now time.Time, maxAge time.Duration) (WebAppUser, error)`
  - `const InitDataMaxAge = 24 * time.Hour`
  - errors `ErrInitDataInvalid`, `ErrInitDataExpired` (reuse existing `ErrTelegramBotUnconfigured` for empty token)
  - test helper (in `_test.go`, package `auth`): `func signInitData(t *testing.T, botToken string, fields map[string]string) string`

- [ ] **Step 1: Write the failing tests** — `telegram_webapp_test.go`:

```go
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

const testBotToken = "123456:TEST-bot-token"

// signInitData builds an initData string exactly as Telegram does, so tests
// exercise the real algorithm instead of a recorded fixture.
func signInitData(t *testing.T, botToken string, fields map[string]string) string {
	t.Helper()
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		lines = append(lines, k+"="+fields[k])
	}
	secret := hmac.New(sha256.New, []byte("WebAppData"))
	secret.Write([]byte(botToken))
	mac := hmac.New(sha256.New, secret.Sum(nil))
	mac.Write([]byte(strings.Join(lines, "\n")))
	v := url.Values{}
	for k, val := range fields {
		v.Set(k, val)
	}
	v.Set("hash", hex.EncodeToString(mac.Sum(nil)))
	return v.Encode()
}

func baseFields(now time.Time) map[string]string {
	return map[string]string{
		"auth_date": strconv.FormatInt(now.Unix(), 10),
		"query_id":  "AAHdF6IQAAAAAN0XohDhrOrc",
		"user":      `{"id":279058397,"first_name":"Ali","username":"ali_uz","language_code":"uz"}`,
		"signature": "6fbdaba2c1b6f1d1f2d0a0c4a8c3b0f0",
	}
}

func TestValidateInitDataAcceptsGenuineData(t *testing.T) {
	now := time.Unix(1_760_000_000, 0)
	raw := signInitData(t, testBotToken, baseFields(now))
	u, err := ValidateInitData(raw, testBotToken, now.Add(time.Minute), InitDataMaxAge)
	if err != nil {
		t.Fatalf("valid init data rejected: %v", err)
	}
	if u.ID != 279058397 || u.FirstName != "Ali" || u.Username != "ali_uz" || u.LanguageCode != "uz" {
		t.Fatalf("user = %+v", u)
	}
}

func TestValidateInitDataRejects(t *testing.T) {
	now := time.Unix(1_760_000_000, 0)
	genuine := signInitData(t, testBotToken, baseFields(now))

	tampered := strings.Replace(genuine, "279058397", "279058398", 1)
	noHash := func() string { v, _ := url.ParseQuery(genuine); v.Del("hash"); return v.Encode() }()
	dupKey := genuine + "&auth_date=1"
	expiredFields := baseFields(now.Add(-25 * time.Hour))
	futureFields := baseFields(now.Add(5 * time.Minute))
	badUser := baseFields(now)
	badUser["user"] = `{"id":0}`
	noSigInCheck := func() string {
		// A hash computed WITHOUT the signature field must not validate:
		// Telegram includes every field but hash in data_check_string.
		f := baseFields(now)
		sig := f["signature"]
		delete(f, "signature")
		v, _ := url.ParseQuery(signInitData(t, testBotToken, f))
		v.Set("signature", sig)
		return v.Encode()
	}()

	cases := []struct {
		name string
		raw  string
		tok  string
		want error
	}{
		{"tampered user", tampered, testBotToken, ErrInitDataInvalid},
		{"wrong token", genuine, "999:other", ErrInitDataInvalid},
		{"missing hash", noHash, testBotToken, ErrInitDataInvalid},
		{"duplicate key", dupKey, testBotToken, ErrInitDataInvalid},
		{"empty", "", testBotToken, ErrInitDataInvalid},
		{"expired", signInitData(t, testBotToken, expiredFields), testBotToken, ErrInitDataExpired},
		{"future dated", signInitData(t, testBotToken, futureFields), testBotToken, ErrInitDataExpired},
		{"bad user", signInitData(t, testBotToken, badUser), testBotToken, ErrInitDataInvalid},
		{"signature excluded from check", noSigInCheck, testBotToken, ErrInitDataInvalid},
		{"no bot token", genuine, "", ErrTelegramBotUnconfigured},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ValidateInitData(tc.raw, tc.tok, now, InitDataMaxAge)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run, verify FAIL** — `cd backend && go test ./internal/auth/ -run ValidateInitData -count=1` → compile error `undefined: ValidateInitData`.

- [ ] **Step 3: Implement** — `telegram_webapp.go`:

```go
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	ErrInitDataInvalid = errors.New("telegram init data invalid")
	ErrInitDataExpired = errors.New("telegram init data expired")
)

// InitDataMaxAge bounds how long a Mini App launch payload can be exchanged
// for a session. Requests after sign-in ride the cookies, so this only limits
// how stale a re-auth (session expired while the Mini App stayed open) may be.
const InitDataMaxAge = 24 * time.Hour

// initDataClockSkew tolerates a phone clock slightly ahead of ours.
const initDataClockSkew = time.Minute

type WebAppUser struct {
	ID           int64
	FirstName    string
	Username     string
	LanguageCode string
}

// ValidateInitData verifies a Telegram Mini App initData string with the
// algorithm from core.telegram.org/bots/webapps#validating-data-received-via-the-mini-app
// and returns the signed user. Nothing outside the signed string is trusted.
func ValidateInitData(raw, botToken string, now time.Time, maxAge time.Duration) (WebAppUser, error) {
	if strings.TrimSpace(botToken) == "" {
		return WebAppUser{}, ErrTelegramBotUnconfigured
	}
	values, err := url.ParseQuery(raw)
	if err != nil || len(values) == 0 {
		return WebAppUser{}, ErrInitDataInvalid
	}
	pairs := make([]string, 0, len(values))
	var gotHash string
	for k, vs := range values {
		// A repeated key makes "which value was signed" ambiguous.
		if len(vs) != 1 {
			return WebAppUser{}, ErrInitDataInvalid
		}
		if k == "hash" {
			gotHash = vs[0]
			continue
		}
		pairs = append(pairs, k+"="+vs[0])
	}
	if gotHash == "" {
		return WebAppUser{}, ErrInitDataInvalid
	}
	sort.Strings(pairs)

	secret := hmac.New(sha256.New, []byte("WebAppData"))
	secret.Write([]byte(botToken))
	mac := hmac.New(sha256.New, secret.Sum(nil))
	mac.Write([]byte(strings.Join(pairs, "\n")))
	want := mac.Sum(nil)
	got, err := hex.DecodeString(gotHash)
	if err != nil || !hmac.Equal(got, want) {
		return WebAppUser{}, ErrInitDataInvalid
	}

	authUnix, err := strconv.ParseInt(values.Get("auth_date"), 10, 64)
	if err != nil {
		return WebAppUser{}, ErrInitDataInvalid
	}
	authAt := time.Unix(authUnix, 0)
	if authAt.After(now.Add(initDataClockSkew)) || now.Sub(authAt) > maxAge {
		return WebAppUser{}, ErrInitDataExpired
	}

	var u struct {
		ID           int64  `json:"id"`
		FirstName    string `json:"first_name"`
		Username     string `json:"username"`
		LanguageCode string `json:"language_code"`
	}
	if err := json.Unmarshal([]byte(values.Get("user")), &u); err != nil || u.ID <= 0 {
		return WebAppUser{}, ErrInitDataInvalid
	}
	return WebAppUser{ID: u.ID, FirstName: u.FirstName, Username: u.Username, LanguageCode: u.LanguageCode}, nil
}
```

- [ ] **Step 4: Run, verify PASS** — same command → `ok`.
- [ ] **Step 5: Commit** — `git add backend/internal/auth/telegram_webapp*.go && git commit -m "feat(auth): validate Telegram Mini App initData"`.

---

### Task 2: Mini App sign-in and phone-auth linking (Go service + HTTP)

**Files:**
- Modify: `backend/internal/db/queries/telegram.sql` (append query) → regenerate `backend/internal/db/sqlc/telegram.sql.go`
- Modify: `backend/internal/auth/service.go` (Service field, `RegisterInput`/`LoginInput`, `Register`, `Login`, `VerifyResult`)
- Create: `backend/internal/auth/telegram_webapp_login.go`
- Modify: `backend/internal/auth/handlers.go` (route, bodies, responses, error map)
- Modify: `backend/internal/server/server.go:246-249` (set `svc.TelegramBotToken = cfg.TelegramBotToken`)
- Test: `backend/internal/auth/telegram_webapp_login_test.go`

**Interfaces:**
- Consumes: Task 1 `ValidateInitData`, `signInitData`, `testBotToken`, `InitDataMaxAge`.
- Produces:
  - `Service.TelegramBotToken string`; `Service.now func() time.Time` (nil ⇒ `time.Now`)
  - `RegisterInput.TgInitData string`, `LoginInput.TgInitData string`
  - `VerifyResult.TelegramLinked bool`
  - `type WebAppLoginResult struct { Tokens; Profile sqlc.Profile; NeedPhone bool; FirstName string }`
  - `func (s *Service) TelegramWebAppLogin(ctx context.Context, initData, ip string) (WebAppLoginResult, error)`
  - HTTP `POST /auth/telegram/webapp {init_data}` → `200 {access_token, refresh_token, must_change_password}` or `200 {need_phone:true, first_name}`; errors `401 invalid_init_data`, `503 telegram_bot_unconfigured`, `429 rate_limited`, `403 account_blocked`.
  - `/auth/login` and `/auth/register` responses gain `telegram_linked: bool`.

- [ ] **Step 1: Add the query** — append to `backend/internal/db/queries/telegram.sql`:

```sql
-- name: DeleteTelegramAccountForOtherProfiles :exec
-- Mini App phone sign-in moves a Telegram account to the profile the person
-- just proved they own (spec D7). Runs in the same tx as the upsert, so the
-- tg_user_id unique constraint is never transiently violated by us.
DELETE FROM telegram_account WHERE tg_user_id = $1 AND profile_id <> $2;
```

Run `cd backend && sqlc generate && git diff --stat internal/db/sqlc` → only `telegram.sql.go` changes.

- [ ] **Step 2: Write failing tests** — `telegram_webapp_login_test.go`:

```go
package auth

import (
	"context"
	"errors"
	"strconv"
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
```

Also extend `handlers_test.go` with one HTTP round trip (uses `setupHandlerServer`, which must now set `svc.TelegramBotToken = testBotToken`):

```go
func TestTelegramWebAppHandlerNeedPhoneAndInvalid(t *testing.T) {
	ts := setupHandlerServer(t)
	raw := signInitData(t, testBotToken, webAppFields(6001, time.Now()))
	status, env := postJSON(t, ts, "/auth/telegram/webapp", map[string]string{"init_data": raw})
	if status != http.StatusOK || !strings.Contains(string(env.Data), `"need_phone":true`) {
		t.Fatalf("status=%d data=%s", status, env.Data)
	}
	status, env = postJSON(t, ts, "/auth/telegram/webapp", map[string]string{"init_data": "hash=00"})
	if status != http.StatusUnauthorized || env.Error == nil || env.Error.Code != "invalid_init_data" {
		t.Fatalf("status=%d env=%+v", status, env)
	}
}
```

(add `"time"` to that file's imports.)

- [ ] **Step 3: Run, verify FAIL** — `go test ./internal/auth/ -run 'TelegramWebApp|InitData|MovesLink' -count=1` → compile errors (`TgInitData`, `TelegramWebAppLogin` undefined).

- [ ] **Step 4: Implement service changes** in `service.go`:
  - Add to `Service` (after `Log`):
    ```go
    // TelegramBotToken verifies Mini App initData (the same bot hosts the
    // Mini App, spec D2). Empty disables Telegram sign-in only.
    TelegramBotToken string
    // now is injectable for initData age checks; nil means time.Now.
    now func() time.Time
    ```
    plus `func (s *Service) clock() time.Time { if s.now != nil { return s.now() }; return time.Now() }`.
  - `RegisterInput` and `LoginInput` gain `TgInitData string`; `VerifyResult` gains `TelegramLinked bool` (find its declaration with `grep -n "type VerifyResult" backend/internal/auth/*.go`).
  - In `Register` and `Login`, immediately after `issueSession` succeeds and **before** `tx.Commit`, add:
    ```go
    linked := s.linkTelegramInTx(ctx, q, profile.ID, in.TgInitData)
    ```
    and return `VerifyResult{..., TelegramLinked: linked}`.

  Create `telegram_webapp_login.go`:

```go
package auth

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"avtotest.uz/backend/internal/db/sqlc"
)

type WebAppLoginResult struct {
	Tokens
	Profile   sqlc.Profile
	NeedPhone bool
	FirstName string
}

// TelegramWebAppLogin exchanges a Mini App launch payload for a session when
// the Telegram account is linked to a profile. Unlinked users get NeedPhone
// and go through the ordinary phone sign-in/registration (spec §1.2) — a
// Telegram identity never creates a profile on its own.
func (s *Service) TelegramWebAppLogin(ctx context.Context, initData, ip string) (WebAppLoginResult, error) {
	u, err := ValidateInitData(initData, s.TelegramBotToken, s.clock(), InitDataMaxAge)
	if err != nil {
		return WebAppLoginResult{}, err
	}
	if err := s.rateLimitTelegram(ctx, u.ID, ip); err != nil {
		return WebAppLoginResult{}, err
	}
	account, err := s.Q.GetTelegramAccountByTgUserID(ctx, u.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return WebAppLoginResult{NeedPhone: true, FirstName: u.FirstName}, nil
	}
	if err != nil {
		return WebAppLoginResult{}, err
	}
	profile, err := s.Q.GetProfileByID(ctx, account.ProfileID)
	if err != nil {
		return WebAppLoginResult{}, err
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return WebAppLoginResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// issueSession adds one refresh-token row, exactly like a new device; it
	// never revokes other sessions.
	toks, err := s.issueSession(ctx, sqlc.New(tx), profile)
	if err != nil {
		return WebAppLoginResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return WebAppLoginResult{}, err
	}
	return WebAppLoginResult{Tokens: toks, Profile: profile, FirstName: u.FirstName}, nil
}

func (s *Service) rateLimitTelegram(ctx context.Context, tgUserID int64, ip string) error {
	if ok, err := s.Lim.Allow(ctx, "tgwebapp:tg:"+strconv.FormatInt(tgUserID, 10), 30, time.Hour); err != nil {
		return err
	} else if !ok {
		return ErrRateLimited
	}
	if ip != "" {
		if ok, err := s.Lim.Allow(ctx, "tgwebapp:ip:"+ip, 60, time.Hour); err != nil {
			return err
		} else if !ok {
			return ErrRateLimited
		}
	}
	return nil
}
```

  Append `linkTelegramInTx` to the same file. It takes the sign-in `pgx.Tx` and opens a SAVEPOINT (nested `tx.Begin`):

```go
// linkTelegramInTx links the Mini App's Telegram account to profileID inside
// the caller's sign-in transaction. It never fails the sign-in: a person who
// typed the right phone and password is signed in even if the Telegram half
// is unusable, they just are not linked (logged for diagnosis).
func (s *Service) linkTelegramInTx(ctx context.Context, tx pgx.Tx, profileID uuid.UUID, initData string) bool {
	if initData == "" {
		return false
	}
	u, err := ValidateInitData(initData, s.TelegramBotToken, s.clock(), InitDataMaxAge)
	if err != nil {
		s.logger().Warn("auth.telegram_link_skipped", zap.String("profile_id", profileID.String()), zap.Error(err))
		return false
	}
	// A nested tx (SAVEPOINT) keeps a failed link — e.g. a concurrent link of
	// the same Telegram account hitting the unique constraint — from aborting
	// the sign-in transaction around it.
	sp, err := tx.Begin(ctx)
	if err != nil {
		s.logger().Warn("auth.telegram_link_skipped", zap.String("profile_id", profileID.String()), zap.Error(err))
		return false
	}
	defer func() { _ = sp.Rollback(ctx) }()
	q := sqlc.New(sp)
	moved, err := q.GetTelegramAccountByTgUserID(ctx, u.ID)
	if err == nil && moved.ProfileID != profileID {
		s.logger().Info("auth.telegram_link_moved",
			zap.String("from_profile_id", moved.ProfileID.String()),
			zap.String("to_profile_id", profileID.String()))
	}
	if err := q.DeleteTelegramAccountForOtherProfiles(ctx, sqlc.DeleteTelegramAccountForOtherProfilesParams{TgUserID: u.ID, ProfileID: profileID}); err == nil {
		err = q.UpsertTelegramAccount(ctx, sqlc.UpsertTelegramAccountParams{ProfileID: profileID, TgUserID: u.ID, Username: u.Username})
		if err == nil {
			err = sp.Commit(ctx)
		}
		if err == nil {
			return true
		}
		s.logger().Warn("auth.telegram_link_skipped", zap.String("profile_id", profileID.String()), zap.Error(err))
		return false
	} else {
		s.logger().Warn("auth.telegram_link_skipped", zap.String("profile_id", profileID.String()), zap.Error(err))
		return false
	}
}
```

  Call it in `Register`/`Login` as `linked := s.linkTelegramInTx(ctx, tx, profile.ID, in.TgInitData)`. Match exact sqlc param struct/field names and the `Username` type to the generated code (`grep -n "UpsertTelegramAccountParams" -A5 internal/db/sqlc/telegram.sql.go`; if `Username` is `pgtype.Text` or `string`, follow it). `GetProfileByID` — confirm the name with `grep -n "func (q \*Queries) GetProfileBy" internal/db/sqlc/*.go`. Add the `time` import to `telegram_webapp_login.go`. Rewrite the `if/else` above into early returns so golangci-lint (`revive`/`gocritic`) is happy.

- [ ] **Step 5: HTTP layer** in `handlers.go`:
  - route: `r.Post("/auth/telegram/webapp", h.telegramWebApp)`
  - `registerBody`/`loginBody` gain `TgInitData string \`json:"tg_init_data"\``, passed into the inputs.
  - `tokensResponse` gains `TelegramLinked bool \`json:"telegram_linked"\``; set from `res.TelegramLinked` in register/login only.
  - handler:
    ```go
    type telegramWebAppBody struct {
    	InitData string `json:"init_data"`
    }

    type telegramNeedPhoneResponse struct {
    	NeedPhone bool   `json:"need_phone"`
    	FirstName string `json:"first_name"`
    }

    func (h *Handler) telegramWebApp(w http.ResponseWriter, r *http.Request) {
    	var body telegramWebAppBody
    	if !decodeBody(w, r, &body) {
    		return
    	}
    	res, err := h.Svc.TelegramWebAppLogin(r.Context(), body.InitData, h.ClientIPs.Resolve(r))
    	if err != nil {
    		writeAuthError(w, err)
    		return
    	}
    	if res.NeedPhone {
    		httpx.Data(w, http.StatusOK, telegramNeedPhoneResponse{NeedPhone: true, FirstName: res.FirstName})
    		return
    	}
    	httpx.Data(w, http.StatusOK, tokensResponse{
    		AccessToken:        res.Access,
    		RefreshToken:       res.Refresh,
    		MustChangePassword: res.Profile.MustChangePassword,
    		TelegramLinked:     true,
    	})
    }
    ```
  - `writeAuthError`: add before `default`:
    ```go
    case errors.Is(err, ErrInitDataInvalid), errors.Is(err, ErrInitDataExpired):
    	httpx.Error(w, http.StatusUnauthorized, "invalid_init_data", "telegram launch data is invalid or expired")
    ```
  - `setupHandlerServer` in `handlers_test.go`: add `svc.TelegramBotToken = testBotToken`.
  - `server.go`: after `svc.Log = log` add `svc.TelegramBotToken = cfg.TelegramBotToken`.

- [ ] **Step 6: Run, verify PASS** — `go test -p 1 ./internal/auth/... -count=1` (whole package, catches regressions) → `ok`. Then `go build ./... && golangci-lint run ./...` → clean.
- [ ] **Step 7: Commit** — `git add backend && git commit -m "feat(auth): Telegram Mini App sign-in and phone-auth linking"`.

---

### Task 3: Bot menu button and `/start` Mini App button

**Files:**
- Modify: `backend/internal/config/config.go` (field `TelegramWebAppURL string`, env `TELEGRAM_WEBAPP_URL`, validate https when set)
- Modify: `backend/internal/bot/types.go` (`WebAppInfo`, `InlineKeyboardButton.WebApp`)
- Modify: `backend/internal/bot/client.go` (`SetChatMenuButton`)
- Create: `backend/internal/bot/menu_button.go` (`SyncMenuButton`)
- Modify: `backend/internal/bot/dispatcher.go` (`Bot.WebAppURL`, private `/start` reply carries the button)
- Modify: `backend/cmd/api/main.go` (call `SyncMenuButton` at startup when bot mode ≠ off; set `WebAppURL` on both `bot.Bot` constructions — main.go long-poll and server.go webhook)
- Test: `backend/internal/bot/menu_button_test.go`, extend `dispatcher_test.go`

**Interfaces:**
- Produces: `func (c *Client) SetChatMenuButton(ctx context.Context, button any) error`; `func SyncMenuButton(ctx context.Context, c *Client, webAppURL string) error`; `Bot.WebAppURL string`.

- [ ] **Step 1: Failing tests.** Look at `client_test.go` for the existing `httptest` fake-Telegram pattern and reuse it. `menu_button_test.go`:

```go
package bot

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSyncMenuButtonSendsWebAppOrDefault(t *testing.T) {
	var got []map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = json.Unmarshal(b, &payload)
		got = append(got, payload)
		_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
	}))
	defer ts.Close()
	c := NewClient(ts.URL, "tok", ts.Client())

	if err := SyncMenuButton(context.Background(), c, "https://drivergo.uz/uz-Latn/tg"); err != nil {
		t.Fatal(err)
	}
	if err := SyncMenuButton(context.Background(), c, ""); err != nil {
		t.Fatal(err)
	}
	first := got[0]["menu_button"].(map[string]any)
	if first["type"] != "web_app" || first["web_app"].(map[string]any)["url"] != "https://drivergo.uz/uz-Latn/tg" {
		t.Fatalf("first = %v", first)
	}
	if got[1]["menu_button"].(map[string]any)["type"] != "default" {
		t.Fatalf("second = %v", got[1])
	}
}
```

  In `dispatcher_test.go` add a test that a private `/start` with no arg and `WebAppURL` set sends `reply_markup.inline_keyboard[0][0].web_app.url` (follow the existing `/start` test's fake client/recorder; if `/start` currently uses `SendMessage`, switch that path to `SendText` with the markup only when `WebAppURL != ""`).

- [ ] **Step 2: Run, verify FAIL** — `go test ./internal/bot/ -run 'MenuButton|Start' -count=1`.

- [ ] **Step 3: Implement.**
  - `types.go`: `type WebAppInfo struct { URL string \`json:"url"\` }`; add `WebApp *WebAppInfo \`json:"web_app,omitempty"\`` to `InlineKeyboardButton`.
  - `client.go`:
    ```go
    // SetChatMenuButton sets the bot's default menu button for every private
    // chat (no chat_id = default for all users).
    func (c *Client) SetChatMenuButton(ctx context.Context, button any) error {
    	return c.call(ctx, "setChatMenuButton", map[string]any{"menu_button": button}, nil)
    }
    ```
  - `menu_button.go`:
    ```go
    package bot

    import "context"

    // SyncMenuButton makes the bot's menu button match config on every start:
    // a Mini App launcher when TELEGRAM_WEBAPP_URL is set, Telegram's default
    // commands menu when it is cleared — so unsetting the variable and
    // restarting is the kill switch (spec §1.4).
    func SyncMenuButton(ctx context.Context, c *Client, webAppURL string) error {
    	if webAppURL == "" {
    		return c.SetChatMenuButton(ctx, map[string]any{"type": "default"})
    	}
    	return c.SetChatMenuButton(ctx, map[string]any{
    		"type":    "web_app",
    		"text":    "Ochish",
    		"web_app": WebAppInfo{URL: webAppURL},
    	})
    }
    ```
  - `dispatcher.go`: `Bot.WebAppURL string`; in the private `/start` branch where `reply` is sent and `arg == ""`, when `b.WebAppURL != ""` send `b.TG.SendText(ctx, chatID, reply, &InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{{{Text: "📱 DriverGo'ni ochish", WebApp: &WebAppInfo{URL: b.WebAppURL}}}}})`.
  - `config.go`: `TelegramWebAppURL: strings.TrimSpace(getenv("TELEGRAM_WEBAPP_URL", ""))`; in validation, when non-empty and `ENV` is not dev, require prefix `https://` (`fmt.Errorf("TELEGRAM_WEBAPP_URL must be https")`).
  - `main.go`: after config load, when `cfg.TelegramBotMode != "off"` and token set: `go func() { ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second); defer cancel(); if err := bot.SyncMenuButton(ctx, bot.NewClient(cfg.TelegramBotAPIBaseURL, cfg.TelegramBotToken, nil), cfg.TelegramWebAppURL); err != nil { logger.Warn("telegram bot: menu button sync failed", zap.Error(err)) } }()`. Set `WebAppURL: cfg.TelegramWebAppURL` in both `bot.Bot{}` literals (main.go and server.go).
  - Add `TELEGRAM_WEBAPP_URL=` with a comment to whichever env example the repo keeps (`grep -rln TELEGRAM_BOT_MODE --include='*.example' -r . deploy compose`).

- [ ] **Step 4: Run, verify PASS** — `go test -p 1 ./internal/bot/... ./internal/config/... -count=1 && golangci-lint run ./...`.
- [ ] **Step 5: Commit** — `feat(bot): Mini App menu button and /start launcher`.

---

### Task 4: BFF cookie modes, Origin guard and `/api/auth/telegram`

**Files:**
- Modify: `frontend/src/lib/auth-cookies.ts`
- Create: `frontend/src/lib/same-origin.ts`
- Create: `frontend/src/app/api/auth/telegram/route.ts`
- Modify: `frontend/src/app/api/proxy/[...path]/route.ts`, `frontend/src/app/api/auth/{login,register,refresh,logout}/route.ts`
- Test: `frontend/src/lib/auth-cookies.test.ts` (extend), `frontend/src/lib/same-origin.test.ts`, `frontend/src/app/api/auth/telegram/route.test.ts`, extend `login/route.test.ts`, `refresh/route.test.ts`

**Interfaces:**
- Produces (TS):
  - `export type CookieMode = "site" | "telegram"`
  - `export const TG_MODE_COOKIE = "tgp"`
  - `setAuthCookies(res, tokens, mode: CookieMode = "site")`, `clearAuthCookies(res, mode: CookieMode = "site")`, `cookieModeFor(request: Request): CookieMode`
  - `rejectCrossSite(request: Request): NextResponse | null`
  - `POST /api/auth/telegram {init_data}` → `200 {data:{ok:true, must_change_password}}` | `200 {data:{need_phone:true, first_name}}` | backend error passthrough.
  - login/register BFF responses gain `data.telegram_linked: boolean`.

- [ ] **Step 1: Failing tests.**

`auth-cookies.test.ts` additions (look at the existing file first and follow its style for reading `Set-Cookie`):

```ts
import { NextResponse } from "next/server";
import { cookieModeFor, setAuthCookies, clearAuthCookies, TG_MODE_COOKIE } from "./auth-cookies";

const tokens = { accessToken: "a", refreshToken: "r" };

function setCookies(res: NextResponse): string[] {
  return res.headers.getSetCookie();
}

describe("telegram cookie mode", () => {
  it("site mode stays lax and never writes the marker", () => {
    const res = NextResponse.json({});
    setAuthCookies(res, tokens);
    const all = setCookies(res).join("\n").toLowerCase();
    expect(all).toContain("samesite=lax");
    expect(all).not.toContain("partitioned");
    expect(all).not.toContain(`${TG_MODE_COOKIE}=`);
  });

  it("telegram mode writes None+Secure+Partitioned for at, rt and the marker", () => {
    const res = NextResponse.json({});
    setAuthCookies(res, tokens, "telegram");
    const cookies = setCookies(res);
    for (const name of ["at=", "rt=", `${TG_MODE_COOKIE}=`]) {
      const c = cookies.find((x) => x.startsWith(name))!.toLowerCase();
      expect(c).toContain("samesite=none");
      expect(c).toContain("secure");
      expect(c).toContain("partitioned");
    }
  });

  it("telegram clear repeats Partitioned so the partitioned cookie is really removed", () => {
    const res = NextResponse.json({});
    clearAuthCookies(res, "telegram");
    for (const c of setCookies(res)) {
      expect(c.toLowerCase()).toContain("partitioned");
      expect(c.toLowerCase()).toContain("max-age=0");
    }
  });

  it("cookieModeFor reads the marker", () => {
    expect(cookieModeFor(new Request("https://x/", { headers: { cookie: "at=1" } }))).toBe("site");
    expect(cookieModeFor(new Request("https://x/", { headers: { cookie: `at=1; ${TG_MODE_COOKIE}=1` } }))).toBe("telegram");
  });
});
```

`same-origin.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { rejectCrossSite } from "./same-origin";

function req(method: string, headers: Record<string, string>) {
  return new Request("http://internal:3000/api/proxy/x", { method, headers: { host: "drivergo.uz", ...headers } });
}

describe("rejectCrossSite", () => {
  it("lets safe methods through untouched", () => {
    expect(rejectCrossSite(req("GET", { origin: "https://evil.example" }))).toBeNull();
  });
  it("allows same-host Origin", () => {
    expect(rejectCrossSite(req("POST", { origin: "https://drivergo.uz" }))).toBeNull();
  });
  it("blocks a foreign Origin with 403 cross_site", async () => {
    const res = rejectCrossSite(req("POST", { origin: "https://evil.example" }))!;
    expect(res.status).toBe(403);
    expect((await res.json()).error.code).toBe("cross_site");
  });
  it("blocks Origin: null (sandboxed frames)", () => {
    expect(rejectCrossSite(req("DELETE", { origin: "null" }))?.status).toBe(403);
  });
  it("without Origin, blocks Sec-Fetch-Site cross-site and allows the rest", () => {
    expect(rejectCrossSite(req("POST", { "sec-fetch-site": "cross-site" }))?.status).toBe(403);
    expect(rejectCrossSite(req("POST", { "sec-fetch-site": "same-origin" }))).toBeNull();
    expect(rejectCrossSite(req("POST", {}))).toBeNull();
  });
});
```

`api/auth/telegram/route.test.ts` — mirror `login/route.test.ts`'s mocking of `@/lib/backend` (`vi.mock("@/lib/backend", ...)`). Cases: tokens → 200, `Set-Cookie` has `partitioned` + `tgp=`; `need_phone` → 200 passthrough without `Set-Cookie`; backend 401 → 401 passthrough; foreign Origin → 403 without calling backend.

`login/route.test.ts` addition: body containing `tg_init_data` → cookies partitioned and response `data.telegram_linked` mirrors backend; body without it → lax (existing tests keep passing).

`refresh/route.test.ts` addition: request carrying `tgp=1` → refreshed cookies partitioned; a 401 refresh with `tgp=1` → cleared cookies partitioned.

- [ ] **Step 2: Run, verify FAIL** — `cd frontend && npx vitest run src/lib/auth-cookies.test.ts src/lib/same-origin.test.ts src/app/api/auth`.

- [ ] **Step 3: Implement.**

`auth-cookies.ts` (replace the cookie-setting part; keep `readCookie` as is):

```ts
export const AUTH_COOKIE = "at";
export const REFRESH_COOKIE = "rt";
/**
 * Marks a cookie jar that belongs to the Telegram Mini App. Its only job is to
 * tell refresh/logout which attributes to re-issue: a partitioned cookie can
 * only be replaced or deleted by a Set-Cookie that is itself Partitioned.
 * The website never receives it, so the site's lax cookies never change.
 */
export const TG_MODE_COOKIE = "tgp";

export type CookieMode = "site" | "telegram";

const AT_MAX_AGE = 900; // 15 minutes, matches backend access-token TTL
const RT_MAX_AGE = 60 * 60 * 24 * 30; // 30 days, matches backend rotating refresh TTL

const siteOptions = {
  httpOnly: true,
  sameSite: "lax" as const,
  secure: process.env.NODE_ENV === "production",
  path: "/",
};

// web.telegram.org runs the Mini App in a cross-site iframe, where lax cookies
// are never sent. Partitioned (CHIPS) keys them to the telegram.org top level,
// so they cannot be replayed from any other site; the Origin guard in
// same-origin.ts covers other Mini Apps inside the same top level.
const telegramOptions = {
  httpOnly: true,
  sameSite: "none" as const,
  secure: true,
  partitioned: true,
  path: "/",
};

function optionsFor(mode: CookieMode) {
  return mode === "telegram" ? telegramOptions : siteOptions;
}

export function cookieModeFor(request: Request): CookieMode {
  return readCookie(request, TG_MODE_COOKIE) === "1" ? "telegram" : "site";
}

export function setAuthCookies(
  res: NextResponse,
  tokens: { accessToken: string; refreshToken: string },
  mode: CookieMode = "site"
): void {
  const options = optionsFor(mode);
  res.cookies.set(AUTH_COOKIE, tokens.accessToken, { ...options, maxAge: AT_MAX_AGE });
  res.cookies.set(REFRESH_COOKIE, tokens.refreshToken, { ...options, maxAge: RT_MAX_AGE });
  if (mode === "telegram") {
    res.cookies.set(TG_MODE_COOKIE, "1", { ...options, maxAge: RT_MAX_AGE });
  }
}

export function clearAuthCookies(res: NextResponse, mode: CookieMode = "site"): void {
  const options = optionsFor(mode);
  res.cookies.set(AUTH_COOKIE, "", { ...options, maxAge: 0 });
  res.cookies.set(REFRESH_COOKIE, "", { ...options, maxAge: 0 });
  if (mode === "telegram") {
    res.cookies.set(TG_MODE_COOKIE, "", { ...options, maxAge: 0 });
  }
}
```

`same-origin.ts`:

```ts
import { NextResponse } from "next/server";

const SAFE_METHODS = new Set(["GET", "HEAD", "OPTIONS"]);

function forbidden() {
  return NextResponse.json(
    { error: { code: "cross_site", message: "cross-site request refused" } },
    { status: 403 }
  );
}

/**
 * CSRF guard for cookie-authenticated BFF writes. The site's lax cookies were
 * the only CSRF defence; Telegram-mode cookies are SameSite=None, so another
 * Mini App framed inside web.telegram.org could otherwise POST with them.
 * nginx forwards the public Host ($host), which is what a same-origin browser
 * request's Origin carries.
 */
export function rejectCrossSite(request: Request): NextResponse | null {
  if (SAFE_METHODS.has(request.method.toUpperCase())) return null;
  const origin = request.headers.get("origin");
  if (origin !== null) {
    let originHost: string;
    try {
      originHost = new URL(origin).host;
    } catch {
      return forbidden(); // "null" from sandboxed frames, or garbage
    }
    return originHost === request.headers.get("host") ? null : forbidden();
  }
  // Browsers always send Origin on cross-site POSTs; a missing Origin plus an
  // explicit cross-site fetch-metadata header is still refused.
  return request.headers.get("sec-fetch-site") === "cross-site" ? forbidden() : null;
}
```

`api/auth/telegram/route.ts`:

```ts
import { NextResponse } from "next/server";
import { backendFetch } from "@/lib/backend";
import { extractTokenPair, readBackendJson } from "@/lib/backend-response";
import { setAuthCookies } from "@/lib/auth-cookies";
import { buildClientIPAssertionHeaders } from "@/lib/client-ip-assertion";
import { rejectCrossSite } from "@/lib/same-origin";

export const runtime = "nodejs";

function unavailableResponse() {
  return NextResponse.json(
    { error: { code: "network_error", message: "service temporarily unavailable" } },
    { status: 502 }
  );
}

// Exchanges Telegram Mini App launch data for a session. A linked Telegram
// account gets Telegram-mode cookies; an unlinked one gets need_phone and goes
// through the ordinary phone sign-in.
export async function POST(request: Request) {
  const refused = rejectCrossSite(request);
  if (refused) return refused;

  let backendRes: Response;
  let data: unknown;
  try {
    backendRes = await backendFetch("/auth/telegram/webapp", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...buildClientIPAssertionHeaders(request, "/auth/telegram/webapp"),
      },
      body: await request.text(),
    });
    data = await readBackendJson(backendRes);
  } catch {
    return unavailableResponse();
  }
  if (!backendRes.ok) {
    return NextResponse.json(data, { status: backendRes.status });
  }

  const payload = (data as { data?: { need_phone?: unknown; first_name?: unknown; must_change_password?: unknown } }).data;
  if (payload?.need_phone === true) {
    return NextResponse.json(
      { data: { need_phone: true, first_name: typeof payload.first_name === "string" ? payload.first_name : "" } },
      { status: 200 }
    );
  }

  let tokens: { accessToken: string; refreshToken: string };
  try {
    tokens = extractTokenPair(data);
  } catch {
    return unavailableResponse();
  }
  const response = NextResponse.json(
    { data: { ok: true, must_change_password: payload?.must_change_password === true } },
    { status: 200 }
  );
  setAuthCookies(response, tokens, "telegram");
  return response;
}
```

  Check `buildClientIPAssertionHeaders`'s second argument semantics in `src/lib/client-ip-assertion.ts` (it may sign the path; the backend resolver must accept `/auth/telegram/webapp`). If the backend `ClientIPResolver` has a path allow-list, add the new path there in this task.

  `login/route.ts` and `register/route.ts`: add `rejectCrossSite` at the top; parse the mode from the body before forwarding:

```ts
function modeForBody(body: string): CookieMode {
  try {
    const parsed = JSON.parse(body) as { tg_init_data?: unknown };
    return typeof parsed.tg_init_data === "string" && parsed.tg_init_data !== "" ? "telegram" : "site";
  } catch {
    return "site";
  }
}
```

  (put `modeForBody` in `auth-cookies.ts` and export it, so login and register share one copy), use `setAuthCookies(response, tokens, modeForBody(body))`, and add `telegram_linked: payload?.telegram_linked === true` to the response `data`.

  `refresh/route.ts`, `logout/route.ts`: `const mode = cookieModeFor(request);` and pass `mode` to every `setAuthCookies`/`clearAuthCookies`; add `rejectCrossSite` at the top of both.

  `proxy/[...path]/route.ts`: at the top of `handle` add `const refused = rejectCrossSite(request); if (refused) return refused;` and `const mode = cookieModeFor(request);`; pass `mode` to every `setAuthCookies`/`clearAuthCookies` (including `unavailableResponse(tokens)` — give it a `mode` parameter).

- [ ] **Step 4: Run, verify PASS** — `npx vitest run src/lib src/app/api` then the full `npx vitest run`.
- [ ] **Step 5: Commit** — `feat(bff): Telegram-mode partitioned cookies, Origin guard, /api/auth/telegram`.

---

### Task 5: Framing policy for Telegram Web

**Files:**
- Modify: `frontend/next.config.mjs`
- Test: `frontend/tests/unit/next-config.test.ts`

- [ ] **Step 1: Failing test** — add:

```ts
describe("next.config framing", () => {
  it("lets only Telegram Web frame the app", () => {
    expect(config).toContain("frame-ancestors 'self' https://web.telegram.org");
    expect(config).not.toMatch(/key: "X-Frame-Options", value: "DENY" },\n\s*\{ key: "Referrer-Policy"/);
  });
  it("loads the Telegram SDK only from telegram.org", () => {
    expect(config).toMatch(/script-src 'self' 'unsafe-inline' https:\/\/static\.cloudflareinsights\.com https:\/\/telegram\.org/);
  });
  it("keeps admin unframeable", () => {
    expect(config).toContain('source: "/:locale/admin/:path*"');
    expect(config).toContain('source: "/api/admin/:path*"');
    expect(config).toContain("frame-ancestors 'none'");
  });
});
```

- [ ] **Step 2: Run** `npx vitest run tests/unit/next-config.test.ts` → FAIL.
- [ ] **Step 3: Implement** in `next.config.mjs`:
  - CSP list: `"frame-ancestors 'self' https://web.telegram.org"` (comment: Telegram Web runs Mini Apps in an iframe; mobile/desktop clients use a webview and need nothing).
  - both `script-src` variants: append ` https://telegram.org`.
  - remove `{ key: "X-Frame-Options", value: "DENY" }` from `securityHeaders` (comment: XFO cannot express an allow-list; CSP frame-ancestors supersedes it in every browser we support).
  - add `const adminFrameHeaders = [{ key: "Content-Security-Policy", value: "frame-ancestors 'none'" }, { key: "X-Frame-Options", value: "DENY" }];` and in `headers()` add entries `{ source: "/:locale/admin/:path*", headers: adminFrameHeaders }`, `{ source: "/api/admin/:path*", headers: adminFrameHeaders }` (comment: a second CSP header is intersected with the global one, so admin stays 'none').
- [ ] **Step 4: Run, verify PASS**; run `npx vitest run` fully.
- [ ] **Step 5: Commit** — `feat(security): allow Telegram Web to frame the learner app, keep admin unframeable`.

---

### Task 6: Telegram runtime layer (detection, SDK, provider, haptics)

**Files:**
- Create: `frontend/src/lib/telegram/web-app.ts` (types + detection + safe accessors)
- Create: `frontend/src/lib/telegram/haptics.ts`
- Create: `frontend/src/components/telegram/telegram-provider.tsx`
- Modify: `frontend/src/app/providers.tsx` (mount provider; pass to SW)
- Modify: `frontend/src/components/pwa/register-sw.tsx` (skip in Mini App)
- Modify: `frontend/src/hooks/use-session-engine.ts` (haptics after a recorded answer)
- Test: `frontend/src/lib/telegram/web-app.test.ts`, `frontend/src/components/telegram/telegram-provider.test.tsx`

**Interfaces:**
- Produces:
  ```ts
  // web-app.ts
  export interface TelegramWebApp {
    initData: string;
    initDataUnsafe: { user?: { id: number; first_name?: string; language_code?: string } };
    colorScheme: "light" | "dark";
    version: string;
    platform: string;
    ready(): void;
    expand(): void;
    isVersionAtLeast(v: string): boolean;
    disableVerticalSwipes?(): void;
    enableClosingConfirmation(): void;
    disableClosingConfirmation(): void;
    setHeaderColor(color: string): void;
    setBackgroundColor(color: string): void;
    setBottomBarColor?(color: string): void;
    onEvent(event: string, cb: () => void): void;
    offEvent(event: string, cb: () => void): void;
    openLink(url: string): void;
    openTelegramLink(url: string): void;
    requestContact(cb: (shared: boolean, res?: { responseUnsafe?: { contact?: { phone_number?: string } } }) => void): void;
    BackButton: { show(): void; hide(): void; onClick(cb: () => void): void; offClick(cb: () => void): void };
    HapticFeedback: {
      impactOccurred(style: "light" | "medium" | "heavy" | "rigid" | "soft"): void;
      notificationOccurred(type: "error" | "success" | "warning"): void;
      selectionChanged(): void;
    };
    CloudStorage?: {
      getItem(key: string, cb: (err: string | null, value?: string) => void): void;
      setItem(key: string, value: string, cb?: (err: string | null, ok?: boolean) => void): void;
      removeItem(key: string, cb?: (err: string | null, ok?: boolean) => void): void;
    };
  }
  export const TELEGRAM_SDK_URL = "https://telegram.org/js/telegram-web-app.js";
  export const TG_SESSION_FLAG = "tg-webapp";
  export function markTelegramMiniApp(): void;        // sets sessionStorage flag
  export function isTelegramMiniApp(): boolean;        // flag || location.hash has tgWebAppData
  export function getWebApp(): TelegramWebApp | null;  // window.Telegram?.WebApp when initData non-empty
  export function cloudGet(key: string): Promise<string | null>;  // resolves null on any failure
  export function cloudSet(key: string, value: string): Promise<void>;
  export function cloudRemove(key: string): Promise<void>;
  export const AUTOLOGIN_OFF_KEY = "autologin_off";
  // haptics.ts
  export const haptics: { select(): void; result(correct: boolean): void; impact(): void };
  // telegram-provider.tsx
  export function TelegramProvider({ children }: { children: React.ReactNode }): JSX.Element;
  export function useTelegram(): TelegramWebApp | null;
  ```

- [ ] **Step 1: Failing tests.**

`web-app.test.ts`:

```ts
import { afterEach, describe, expect, it } from "vitest";
import { cloudGet, getWebApp, isTelegramMiniApp, markTelegramMiniApp, TG_SESSION_FLAG } from "./web-app";
import { haptics } from "./haptics";

afterEach(() => {
  sessionStorage.clear();
  delete (window as { Telegram?: unknown }).Telegram;
  window.history.replaceState(null, "", "/");
});

describe("telegram detection", () => {
  it("is false on the plain website", () => {
    expect(isTelegramMiniApp()).toBe(false);
    expect(getWebApp()).toBeNull();
  });
  it("is true after the launch hash or the session flag", () => {
    window.history.replaceState(null, "", "/uz-Latn/tg#tgWebAppData=x&tgWebAppVersion=8.0");
    expect(isTelegramMiniApp()).toBe(true);
    window.history.replaceState(null, "", "/");
    markTelegramMiniApp();
    expect(sessionStorage.getItem(TG_SESSION_FLAG)).toBe("1");
    expect(isTelegramMiniApp()).toBe(true);
  });
  it("getWebApp ignores an SDK without initData (opened outside Telegram)", () => {
    (window as { Telegram?: unknown }).Telegram = { WebApp: { initData: "" } };
    expect(getWebApp()).toBeNull();
  });
  it("cloudGet resolves null when CloudStorage is missing", async () => {
    expect(await cloudGet("k")).toBeNull();
  });
  it("haptics are silent no-ops outside Telegram", () => {
    expect(() => {
      haptics.select();
      haptics.result(true);
      haptics.impact();
    }).not.toThrow();
  });
});
```

`telegram-provider.test.tsx`:

```tsx
import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { TelegramProvider } from "./telegram-provider";
import { TELEGRAM_SDK_URL, markTelegramMiniApp } from "@/lib/telegram/web-app";

afterEach(() => {
  sessionStorage.clear();
  document.head.querySelectorAll("script").forEach((s) => s.remove());
});

describe("TelegramProvider", () => {
  it("adds no script and renders children on the website", () => {
    render(<TelegramProvider><p>site</p></TelegramProvider>);
    expect(screen.getByText("site")).toBeInTheDocument();
    expect(document.querySelector(`script[src="${TELEGRAM_SDK_URL}"]`)).toBeNull();
  });
  it("injects the SDK once inside the Mini App", () => {
    markTelegramMiniApp();
    const { rerender } = render(<TelegramProvider><p>tg</p></TelegramProvider>);
    rerender(<TelegramProvider><p>tg</p></TelegramProvider>);
    expect(document.querySelectorAll(`script[src="${TELEGRAM_SDK_URL}"]`)).toHaveLength(1);
  });
});
```

- [ ] **Step 2: Run** `npx vitest run src/lib/telegram src/components/telegram` → FAIL.

- [ ] **Step 3: Implement.**

`web-app.ts` — the interface above plus:

```ts
export const TELEGRAM_SDK_URL = "https://telegram.org/js/telegram-web-app.js";
export const TG_SESSION_FLAG = "tg-webapp";
export const AUTOLOGIN_OFF_KEY = "autologin_off";

declare global {
  interface Window {
    Telegram?: { WebApp?: TelegramWebApp };
  }
}

/** Remembers, for this webview's lifetime, that we were launched by Telegram. */
export function markTelegramMiniApp(): void {
  try {
    sessionStorage.setItem(TG_SESSION_FLAG, "1");
  } catch {
    /* storage blocked: the hash check below still works on the launch page */
  }
}

export function isTelegramMiniApp(): boolean {
  if (typeof window === "undefined") return false;
  try {
    if (sessionStorage.getItem(TG_SESSION_FLAG) === "1") return true;
  } catch {
    /* fall through */
  }
  return window.location.hash.includes("tgWebAppData=");
}

/** The SDK object, but only when Telegram actually launched us. */
export function getWebApp(): TelegramWebApp | null {
  if (typeof window === "undefined") return null;
  const webApp = window.Telegram?.WebApp;
  return webApp && webApp.initData ? webApp : null;
}

export function cloudGet(key: string): Promise<string | null> {
  const storage = getWebApp()?.CloudStorage;
  if (!storage) return Promise.resolve(null);
  return new Promise((resolve) => {
    try {
      storage.getItem(key, (err, value) => resolve(err ? null : value || null));
    } catch {
      resolve(null);
    }
  });
}

export function cloudSet(key: string, value: string): Promise<void> {
  const storage = getWebApp()?.CloudStorage;
  if (!storage) return Promise.resolve();
  return new Promise((resolve) => {
    try {
      storage.setItem(key, value, () => resolve());
    } catch {
      resolve();
    }
  });
}

export function cloudRemove(key: string): Promise<void> {
  const storage = getWebApp()?.CloudStorage;
  if (!storage) return Promise.resolve();
  return new Promise((resolve) => {
    try {
      storage.removeItem(key, () => resolve());
    } catch {
      resolve();
    }
  });
}
```

`haptics.ts`:

```ts
import { getWebApp } from "./web-app";

// Every call is a no-op outside Telegram, so shared components can call these
// unconditionally.
export const haptics = {
  select() {
    getWebApp()?.HapticFeedback.selectionChanged();
  },
  result(correct: boolean) {
    getWebApp()?.HapticFeedback.notificationOccurred(correct ? "success" : "error");
  },
  impact() {
    getWebApp()?.HapticFeedback.impactOccurred("light");
  },
};
```

`telegram-provider.tsx`:

```tsx
"use client";

import { createContext, useContext, useEffect, useState } from "react";
import { getWebApp, isTelegramMiniApp, TELEGRAM_SDK_URL, type TelegramWebApp } from "@/lib/telegram/web-app";

const TelegramContext = createContext<TelegramWebApp | null>(null);

export function useTelegram(): TelegramWebApp | null {
  return useContext(TelegramContext);
}

/**
 * Loads Telegram's SDK only when Telegram launched us. On the website this
 * renders its children and does nothing else — no script request, no work.
 */
export function TelegramProvider({ children }: { children: React.ReactNode }) {
  const [webApp, setWebApp] = useState<TelegramWebApp | null>(null);

  useEffect(() => {
    if (!isTelegramMiniApp()) return;
    const existing = getWebApp();
    if (existing) {
      setWebApp(existing);
      return;
    }
    let script = document.querySelector<HTMLScriptElement>(`script[src="${TELEGRAM_SDK_URL}"]`);
    if (!script) {
      script = document.createElement("script");
      script.src = TELEGRAM_SDK_URL;
      script.async = true;
      document.head.appendChild(script);
    }
    const onLoad = () => setWebApp(getWebApp());
    script.addEventListener("load", onLoad);
    return () => script?.removeEventListener("load", onLoad);
  }, []);

  useEffect(() => {
    if (!webApp) return;
    document.documentElement.classList.add("tg-webapp");
    webApp.ready();
    webApp.expand();
    if (webApp.isVersionAtLeast("7.7")) webApp.disableVerticalSwipes?.();
  }, [webApp]);

  return <TelegramContext.Provider value={webApp}>{children}</TelegramContext.Provider>;
}
```

`providers.tsx`: wrap inside `ThemeProvider`: `<TelegramProvider><InitSentry /><RegisterServiceWorker />{children}</TelegramProvider>`.

`register-sw.tsx`: in the effect, before registering: `if (isTelegramMiniApp()) return;` with a comment (the Mini App lives in Telegram's webview; an SW there only adds a stale-cache risk and an install prompt nobody can use).

`use-session-engine.ts`: import `haptics` and, inside `if (response.recorded) {`, first line: `if (response.correct !== undefined) haptics.result(response.correct);`.

- [ ] **Step 4: Run, verify PASS** — targeted, then full `npx vitest run`.
- [ ] **Step 5: Commit** — `feat(telegram): load the Mini App SDK only inside Telegram`.

---

### Task 7: `/tg` entry screen

**Files:**
- Create: `frontend/src/app/[locale]/(auth)/tg/page.tsx`
- Create: `frontend/src/components/telegram/telegram-entry.tsx`
- Create: `frontend/src/lib/telegram/safe-next.ts`
- Modify: `frontend/src/i18n/namespaces.ts` (`"TelegramApp"` in `AUTH_EXTRA`)
- Modify: `frontend/messages/{uz-Latn,uz-Cyrl,ru}.json` (namespace `TelegramApp`)
- Test: `frontend/src/lib/telegram/safe-next.test.ts`, `frontend/src/components/telegram/telegram-entry.test.tsx`

`/tg` lives in the existing public `(auth)` group, so the proxy guard and its route-group test need no change; `tg` must NOT be added to `PROTECTED_SEGMENTS`.

**Interfaces:**
- Consumes: Task 6 (`useTelegram`, `markTelegramMiniApp`, `cloudGet`, `cloudRemove`, `AUTOLOGIN_OFF_KEY`), Task 4 (`POST /api/auth/telegram`).
- Produces: `export function safeNextPath(raw: string | null, locale: string): string` (returns `/${locale}/dashboard` unless `raw` starts with `/${locale}/`, has no `//`, no `\\`, no scheme).

- [ ] **Step 1: Failing tests.**

`safe-next.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { safeNextPath } from "./safe-next";

describe("safeNextPath", () => {
  it.each([
    [null, "/uz-Latn/dashboard"],
    ["/uz-Latn/tickets", "/uz-Latn/tickets"],
    ["/uz-Latn/session/abc?x=1", "/uz-Latn/session/abc?x=1"],
    ["//evil.example", "/uz-Latn/dashboard"],
    ["/uz-Latn//evil.example", "/uz-Latn/dashboard"],
    ["https://evil.example", "/uz-Latn/dashboard"],
    ["/ru/tickets", "/uz-Latn/dashboard"],
    ["/uz-Latn/tg", "/uz-Latn/dashboard"],
    ["/uz-Latn\\evil", "/uz-Latn/dashboard"],
  ])("%s → %s", (raw, want) => {
    expect(safeNextPath(raw, "uz-Latn")).toBe(want);
  });
});
```

`telegram-entry.test.tsx` — render `<TelegramEntry />` inside `NextIntlClientProvider` with the uz-Latn `TelegramApp` messages, mock `next/navigation` (`useRouter` → `{ replace: vi.fn() }`) and `@/components/telegram/telegram-provider` (`useTelegram` returns a fake WebApp with `initData: "signed"`, `initDataUnsafe.user.first_name: "Ali"`, `CloudStorage` stub), and `global.fetch`:
  1. telegram route returns tokens and `GET /api/proxy/me` 200 → `replace("/uz-Latn/dashboard")`.
  2. returns `{data:{need_phone:true, first_name:"Ali"}}` → shows greeting containing "Ali" and links to `/uz-Latn/login` and `/uz-Latn/register`.
  3. returns 401 `invalid_init_data` → shows the "open from the bot" text.
  4. CloudStorage `autologin_off = "1"` → no fetch to `/api/auth/telegram` until the "continue as Ali" button is clicked; clicking it calls the route and removes the key.
  5. tokens OK but `/api/proxy/me` 401 → shows the "open in the phone app" text.
  6. `useTelegram()` null for 3 s (use fake timers) → shows the "open from the bot" text.

- [ ] **Step 2: Run** → FAIL.

- [ ] **Step 3: Implement.**

`safe-next.ts`:

```ts
/**
 * Where /tg may send the learner after sign-in. Only same-locale app paths:
 * anything else (other origins, protocol-relative, backslash tricks, /tg
 * itself) falls back to the dashboard.
 */
export function safeNextPath(raw: string | null, locale: string): string {
  const fallback = `/${locale}/dashboard`;
  if (!raw) return fallback;
  if (!raw.startsWith(`/${locale}/`) || raw.includes("//") || raw.includes("\\")) return fallback;
  if (raw === `/${locale}/tg` || raw.startsWith(`/${locale}/tg?`) || raw.startsWith(`/${locale}/tg/`)) return fallback;
  return raw;
}
```

`tg/page.tsx`:

```tsx
import { TelegramEntry } from "@/components/telegram/telegram-entry";

export default function TelegramEntryPage() {
  return <TelegramEntry />;
}
```

`telegram-entry.tsx` — a client component with a state machine
`"loading" | "welcome" | "outside" | "cookie_blocked" | "unavailable" | "error"`:

```tsx
"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
import { Loader2, Send } from "lucide-react";
import { BrandLogo } from "@/components/brand/brand-logo";
import { Button } from "@/components/ui/button";
import { useTelegram } from "@/components/telegram/telegram-provider";
import { AUTOLOGIN_OFF_KEY, cloudGet, cloudRemove, markTelegramMiniApp } from "@/lib/telegram/web-app";
import { safeNextPath } from "@/lib/telegram/safe-next";

type Phase = "loading" | "welcome" | "outside" | "cookie_blocked" | "unavailable" | "error";

// The SDK script normally loads in well under a second; past this we assume
// the page was opened as a plain URL, not from Telegram.
const SDK_WAIT_MS = 3000;

export function TelegramEntry() {
  const t = useTranslations("TelegramApp");
  const locale = useLocale();
  const router = useRouter();
  const webApp = useTelegram();
  const [phase, setPhase] = useState<Phase>("loading");
  const [firstName, setFirstName] = useState("");
  const [canContinue, setCanContinue] = useState(false);
  const started = useRef(false);

  useEffect(() => {
    markTelegramMiniApp();
  }, []);

  const signIn = useCallback(async () => {
    if (!webApp) return;
    setPhase("loading");
    let res: Response;
    try {
      res = await fetch("/api/auth/telegram", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ init_data: webApp.initData }),
      });
    } catch {
      setPhase("error");
      return;
    }
    const json = (await res.json().catch(() => null)) as
      | { data?: { need_phone?: boolean; first_name?: string; must_change_password?: boolean }; error?: { code?: string } }
      | null;
    if (res.ok && json?.data?.need_phone) {
      setFirstName(json.data.first_name || webApp.initDataUnsafe.user?.first_name || "");
      setCanContinue(false);
      setPhase("welcome");
      return;
    }
    if (!res.ok) {
      const code = json?.error?.code;
      setPhase(code === "invalid_init_data" ? "outside" : code === "telegram_bot_unconfigured" ? "unavailable" : "error");
      return;
    }
    // Safari on web.telegram.org refuses third-party cookies even when
    // partitioned; prove the cookie stuck before handing over to the app.
    const probe = await fetch("/api/proxy/me").catch(() => null);
    if (!probe || probe.status === 401) {
      setPhase("cookie_blocked");
      return;
    }
    await cloudRemove(AUTOLOGIN_OFF_KEY);
    const next = new URLSearchParams(window.location.search).get("next");
    router.replace(json?.data?.must_change_password ? `/${locale}/change-password` : safeNextPath(next, locale));
  }, [locale, router, webApp]);

  useEffect(() => {
    if (webApp) return;
    const timer = window.setTimeout(() => setPhase((p) => (p === "loading" ? "outside" : p)), SDK_WAIT_MS);
    return () => window.clearTimeout(timer);
  }, [webApp]);

  useEffect(() => {
    if (!webApp || started.current) return;
    started.current = true;
    void (async () => {
      if ((await cloudGet(AUTOLOGIN_OFF_KEY)) === "1") {
        // Signed out on purpose last time: offer, never force, the way back.
        setFirstName(webApp.initDataUnsafe.user?.first_name || "");
        setCanContinue(true);
        setPhase("welcome");
        return;
      }
      await signIn();
    })();
  }, [signIn, webApp]);

  return (
    <main className="flex min-h-[100dvh] flex-col items-center justify-center gap-6 bg-background p-6 text-center">
      <BrandLogo size={72} className="h-18 w-18 rounded-3xl" />
      {phase === "loading" && (
        <p role="status" className="flex items-center gap-2 text-sm font-semibold text-muted-foreground">
          <Loader2 aria-hidden="true" className="h-4 w-4 animate-spin" /> {t("loading")}
        </p>
      )}
      {phase === "welcome" && (
        <div className="w-full max-w-sm space-y-4">
          <h1 className="font-display text-2xl font-extrabold">
            {firstName ? t("welcomeNamed", { name: firstName }) : t("welcome")}
          </h1>
          <p className="text-sm text-muted-foreground">{t("welcomeHint")}</p>
          {canContinue && (
            <Button variant="game" size="lg" className="w-full" onClick={() => void signIn()}>
              <Send aria-hidden="true" className="mr-2 h-4 w-4" /> {t("continueAs", { name: firstName })}
            </Button>
          )}
          <Link href={`/${locale}/login`} className="block">
            <Button as="span" variant={canContinue ? "outline" : "game"} size="lg" className="w-full">{t("login")}</Button>
          </Link>
          <Link href={`/${locale}/register`} className="block">
            <Button as="span" variant="outline" size="lg" className="w-full">{t("register")}</Button>
          </Link>
        </div>
      )}
      {phase === "outside" && <Notice title={t("outsideTitle")} body={t("outsideBody")} />}
      {phase === "cookie_blocked" && <Notice title={t("cookieTitle")} body={t("cookieBody")} />}
      {phase === "unavailable" && <Notice title={t("unavailableTitle")} body={t("unavailableBody")} />}
      {phase === "error" && (
        <div className="space-y-4">
          <Notice title={t("errorTitle")} body={t("errorBody")} />
          <Button variant="outline" onClick={() => void signIn()}>{t("retry")}</Button>
        </div>
      )}
    </main>
  );
}

function Notice({ title, body }: { title: string; body: string }) {
  return (
    <div role="alert" className="max-w-sm space-y-2">
      <h1 className="font-display text-xl font-extrabold">{title}</h1>
      <p className="text-sm text-muted-foreground">{body}</p>
    </div>
  );
}
```

  Check `BrandLogo`'s props and `Button`'s `as` prop against their definitions and adjust class names to existing tokens (`h-18` may not exist in the Tailwind config — use `h-16 w-16` if not).

  Locale on first open (spec §4.2 step 2): inside the `webApp` effect, before `cloudGet`, if `document.cookie` has no `NEXT_LOCALE=` and `webApp.initDataUnsafe.user?.language_code === "ru"` and `locale !== "ru"`, `router.replace("/ru/tg" + window.location.search)` and return (no prefetch — NEXT_LOCALE trap).

  Messages — `TelegramApp` namespace, all three files:

  | key | uz-Latn | uz-Cyrl | ru |
  |---|---|---|---|
  | loading | Kirilmoqda… | Кирилмоқда… | Входим… |
  | welcome | Xush kelibsiz! | Хуш келибсиз! | Добро пожаловать! |
  | welcomeNamed | Xush kelibsiz, {name}! | Хуш келибсиз, {name}! | Добро пожаловать, {name}! |
  | welcomeHint | Telefon raqamingiz bilan kiring yoki ro'yxatdan o'ting. Keyingi safar ilova o'zi ochiladi. | Телефон рақамингиз билан киринг ёки рўйхатдан ўтинг. Кейинги сафар илова ўзи очилади. | Войдите или зарегистрируйтесь по номеру телефона. В следующий раз приложение откроется сразу. |
  | continueAs | {name} sifatida davom etish | {name} сифатида давом этиш | Продолжить как {name} |
  | login | Kirish | Кириш | Войти |
  | register | Ro'yxatdan o'tish | Рўйхатдан ўтиш | Зарегистрироваться |
  | outsideTitle | Botdan oching | Ботдан очинг | Откройте через бота |
  | outsideBody | Bu sahifa Telegram'dagi DriverGo boti orqali ochiladi. Botni qayta oching. | Бу саҳифа Telegram'даги DriverGo боти орқали очилади. Ботни қайта очинг. | Эта страница открывается из бота DriverGo в Telegram. Откройте бота заново. |
  | cookieTitle | Telefoningizda oching | Телефонингизда очинг | Откройте на телефоне |
  | cookieBody | Bu brauzer kirishni saqlay olmadi. Telegram ilovasida (telefon yoki kompyuter) oching. | Бу браузер киришни сақлай олмади. Telegram иловасида (телефон ёки компьютер) очинг. | Этот браузер не сохранил вход. Откройте в приложении Telegram (телефон или компьютер). |
  | unavailableTitle | Vaqtincha mavjud emas | Вақтинча мавжуд эмас | Временно недоступно |
  | unavailableBody | Birozdan so'ng qayta urinib ko'ring yoki drivergo.uz saytidan foydalaning. | Бироздан сўнг қайта уриниб кўринг ёки drivergo.uz сайтидан фойдаланинг. | Попробуйте позже или воспользуйтесь сайтом drivergo.uz. |
  | errorTitle | Ulanib bo'lmadi | Уланиб бўлмади | Не удалось подключиться |
  | errorBody | Internet aloqasini tekshirib, qayta urinib ko'ring. | Интернет алоқасини текшириб, қайта уриниб кўринг. | Проверьте интернет и попробуйте снова. |
  | retry | Qayta urinish | Қайта уриниш | Повторить |
  | sharePhone | Raqamni Telegram'dan olish | Рақамни Telegram'дан олиш | Взять номер из Telegram |
  | linkedStatus | Telegram ulangan | Telegram уланган | Telegram подключён |

  (`sharePhone` and `linkedStatus` are used by Tasks 8–9; `TelegramApp` must also be added to `APP_EXTRA` for `linkedStatus`.)

- [ ] **Step 4: Run, verify PASS** — targeted tests, then `npx vitest run` (includes the i18n key-parity tests, if any, and `middleware.test.ts`).
- [ ] **Step 5: Commit** — `feat(telegram): /tg Mini App entry with silent sign-in`.

---

### Task 8: Login and register inside the Mini App

**Files:**
- Create: `frontend/src/components/telegram/telegram-phone-button.tsx`
- Create: `frontend/src/lib/telegram/auth-body.ts`
- Modify: `frontend/src/app/[locale]/(auth)/login/page.tsx`, `frontend/src/app/[locale]/(auth)/register/page.tsx`
- Test: `frontend/src/lib/telegram/auth-body.test.ts`, `frontend/src/components/telegram/telegram-phone-button.test.tsx`, extend `login/page.test.tsx`

**Interfaces:**
- Consumes: Task 6 (`useTelegram`, `cloudRemove`, `AUTOLOGIN_OFF_KEY`), Task 4 (`telegram_linked`).
- Produces:
  - `export function withTelegramInitData<T extends object>(body: T, webApp: TelegramWebApp | null): T & { tg_init_data?: string }`
  - `export async function afterTelegramAuth(linked: boolean): Promise<void>` (removes `AUTOLOGIN_OFF_KEY` when linked)
  - `<TelegramPhoneButton onPhone={(national9: string) => void} />` — renders nothing outside Telegram.

- [ ] **Step 1: Failing tests.**

`auth-body.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { withTelegramInitData } from "./auth-body";
import type { TelegramWebApp } from "./web-app";

describe("withTelegramInitData", () => {
  it("leaves the website body untouched", () => {
    expect(withTelegramInitData({ phone: "901234567" }, null)).toEqual({ phone: "901234567" });
  });
  it("adds the signed launch data inside Telegram", () => {
    const webApp = { initData: "signed" } as TelegramWebApp;
    expect(withTelegramInitData({ phone: "901234567" }, webApp)).toEqual({ phone: "901234567", tg_init_data: "signed" });
  });
});
```

`telegram-phone-button.test.tsx`: with `useTelegram` mocked to `null` → renders nothing; with a fake WebApp whose `requestContact` calls back `(true, { responseUnsafe: { contact: { phone_number: "998901234567" } } })` → clicking the button calls `onPhone("901234567")`; callback `(false)` → `onPhone` not called.

`login/page.test.tsx` addition: with `useTelegram` mocked to a fake WebApp, submitting sends a body containing `tg_init_data`; without it (existing tests) the body has no such key.

- [ ] **Step 2: Run** → FAIL.

- [ ] **Step 3: Implement.**

`auth-body.ts`:

```ts
import { AUTOLOGIN_OFF_KEY, cloudRemove, type TelegramWebApp } from "./web-app";

/** Inside the Mini App, phone sign-in also links this Telegram account. */
export function withTelegramInitData<T extends object>(body: T, webApp: TelegramWebApp | null): T & { tg_init_data?: string } {
  return webApp?.initData ? { ...body, tg_init_data: webApp.initData } : body;
}

/** A successful link means auto-login should work again next launch. */
export async function afterTelegramAuth(linked: boolean): Promise<void> {
  if (linked) await cloudRemove(AUTOLOGIN_OFF_KEY);
}
```

`telegram-phone-button.tsx`:

```tsx
"use client";

import { useTranslations } from "next-intl";
import { Send } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useTelegram } from "@/components/telegram/telegram-provider";
import { normalizeNationalPhone } from "@/lib/phone-format";

/**
 * Telegram's own "share your number" sheet. Only pre-fills the field: the
 * number still goes through the same validation and password check as typing
 * it (spec D3), so this adds no new trust.
 */
export function TelegramPhoneButton({ onPhone }: { onPhone: (national: string) => void }) {
  const t = useTranslations("TelegramApp");
  const webApp = useTelegram();
  if (!webApp) return null;
  return (
    <Button
      type="button"
      variant="outline"
      size="lg"
      className="w-full text-sm font-extrabold"
      onClick={() =>
        webApp.requestContact((shared, res) => {
          const raw = res?.responseUnsafe?.contact?.phone_number;
          if (shared && raw) onPhone(normalizeNationalPhone(raw));
        })
      }
    >
      <Send aria-hidden="true" className="mr-2 h-4 w-4" /> {t("sharePhone")}
    </Button>
  );
}
```

  Login page: `const webApp = useTelegram();`; body → `JSON.stringify(withTelegramInitData({ phone: localPhone, password }, webApp))`; read `json.data?.telegram_linked === true` and `await afterTelegramAuth(linked)` before `finishAuth`; render `<TelegramPhoneButton onPhone={setPhone} />` directly above the phone field label. Register page: same three changes (body built where `JSON.stringify({ phone: localPhone, ...})` is today). `AUTH_NAMESPACES` already has `TelegramApp` from Task 7.

- [ ] **Step 4: Run, verify PASS** — targeted + full vitest.
- [ ] **Step 5: Commit** — `feat(telegram): phone sign-in inside the Mini App links Telegram`.

---

### Task 9: Telegram chrome — theme, BackButton, safe area, closing guard, links, logout

**Files:**
- Create: `frontend/src/components/telegram/telegram-chrome.tsx`
- Create: `frontend/src/lib/telegram/routes.ts`
- Create: `frontend/src/lib/telegram/logout.ts`
- Modify: `frontend/src/components/telegram/telegram-provider.tsx` (render `<TelegramChrome webApp={webApp} />` when present)
- Modify: `frontend/src/app/globals.css` (`html.tg-webapp` safe-area rules)
- Modify: `frontend/src/components/auth/session-expired-gate.tsx`, `frontend/src/app/[locale]/(app)/profile/page.tsx` (`handleLogout`), `frontend/src/components/profile/mobile/profile-mobile.tsx` (its logout handler), `frontend/src/components/profile/telegram-link-card.tsx` and `mobile/mobile-telegram.tsx` (linked status in Mini App)
- Test: `frontend/src/lib/telegram/routes.test.ts`, `frontend/src/components/telegram/telegram-chrome.test.tsx`, extend `session-expired-gate` test if one exists (`ls src/components/auth`)

**Interfaces:**
- Consumes: Task 6, Task 4 (`/api/auth/logout` clears telegram-mode cookies via `tgp`).
- Produces:
  - `export function isTabRoot(pathname: string): boolean` — `/<locale>/(dashboard|tickets|practice|exam|profile)` exactly, or `/<locale>` / `/<locale>/tg`
  - `export function needsClosingGuard(pathname: string): boolean` — `/<locale>/session/<id>` (not `/session/start`) and `/<locale>/practice/memorize...`
  - `export function postLogoutPath(locale: string, fallback: string): string` → `/${locale}/tg` inside Telegram, else `fallback`
  - `export async function markTelegramLogout(): Promise<void>` → `cloudSet(AUTOLOGIN_OFF_KEY, "1")`

- [ ] **Step 1: Failing tests.**

`routes.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { isTabRoot, needsClosingGuard } from "./routes";

describe("telegram routes", () => {
  it.each(["/uz-Latn/dashboard", "/ru/tickets", "/uz-Cyrl/practice", "/uz-Latn/exam", "/uz-Latn/profile", "/uz-Latn/tg", "/uz-Latn"])(
    "%s is a tab root",
    (p) => expect(isTabRoot(p)).toBe(true)
  );
  it.each(["/uz-Latn/tickets/12", "/uz-Latn/signs", "/uz-Latn/session/abc", "/uz-Latn/premium"])(
    "%s shows Back",
    (p) => expect(isTabRoot(p)).toBe(false)
  );
  it("guards closing only inside a running test", () => {
    expect(needsClosingGuard("/uz-Latn/session/abc")).toBe(true);
    expect(needsClosingGuard("/uz-Latn/practice/memorize/7")).toBe(true);
    expect(needsClosingGuard("/uz-Latn/session/start")).toBe(false);
    expect(needsClosingGuard("/uz-Latn/dashboard")).toBe(false);
  });
});
```

`telegram-chrome.test.tsx`: mock `next/navigation` (`usePathname`, `useRouter`) and `next-themes` (`useTheme` → `{ setTheme: vi.fn() }`); with a fake WebApp:
  - pathname `/uz-Latn/signs` → `BackButton.show` called; clicking the registered handler calls `router.back()`; on `/uz-Latn/dashboard` → `BackButton.hide`.
  - `colorScheme: "light"` → `setTheme("light")`; firing the registered `themeChanged` callback after switching `colorScheme` to `"dark"` → `setTheme("dark")`.
  - pathname `/uz-Latn/session/x` → `enableClosingConfirmation`; leaving to `/uz-Latn/dashboard` → `disableClosingConfirmation`.
  - clicking a rendered `<a href="https://payme.uz/x">` inside the document → `openLink("https://payme.uz/x")` and default prevented; `<a href="https://t.me/x">` → `openTelegramLink`; same-origin link untouched.

- [ ] **Step 2: Run** → FAIL.

- [ ] **Step 3: Implement.**

`routes.ts`:

```ts
const TAB_ROOTS = new Set(["dashboard", "tickets", "practice", "exam", "profile", "tg"]);

function segments(pathname: string): string[] {
  return pathname.split("/").filter(Boolean); // [locale, ...rest]
}

/** Telegram's BackButton is hidden on the bottom-tab roots, shown elsewhere. */
export function isTabRoot(pathname: string): boolean {
  const [, first, second] = segments(pathname);
  if (!first) return true;
  return TAB_ROOTS.has(first) && second === undefined;
}

/** A swipe-down or tap on X mid-test would lose the attempt; ask first. */
export function needsClosingGuard(pathname: string): boolean {
  const [, first, second] = segments(pathname);
  if (first === "session") return second !== undefined && second !== "start";
  return first === "practice" && second === "memorize";
}
```

`logout.ts`:

```ts
import { AUTOLOGIN_OFF_KEY, cloudSet, isTelegramMiniApp } from "./web-app";

/**
 * Signing out inside the Mini App keeps the Telegram link (bot digests and
 * password reset depend on it, spec D6) and instead switches auto-login off
 * for this Telegram user, so /tg shows the welcome screen next time.
 */
export async function markTelegramLogout(): Promise<void> {
  await cloudSet(AUTOLOGIN_OFF_KEY, "1");
}

/** Where to land after logout or session expiry. */
export function postLogoutPath(locale: string, fallback: string): string {
  return isTelegramMiniApp() ? `/${locale}/tg` : fallback;
}
```
`telegram-chrome.tsx`:

```tsx
"use client";

import { useEffect } from "react";
import { usePathname, useRouter } from "next/navigation";
import { useTheme } from "next-themes";
import type { TelegramWebApp } from "@/lib/telegram/web-app";
import { isTabRoot, needsClosingGuard } from "@/lib/telegram/routes";

function cssColor(variable: string): string | null {
  // Tokens are stored as "H S% L%" triplets (shadcn convention).
  const raw = getComputedStyle(document.documentElement).getPropertyValue(variable).trim();
  return raw ? `hsl(${raw.replace(/ /g, ", ")})` : null;
}

function toHex(color: string): string {
  const ctx = document.createElement("canvas").getContext("2d");
  if (!ctx) return color;
  ctx.fillStyle = color;
  return ctx.fillStyle; // canvas normalises any CSS colour to #rrggbb
}

export function TelegramChrome({ webApp }: { webApp: TelegramWebApp }) {
  const pathname = usePathname();
  const router = useRouter();
  const { setTheme, resolvedTheme } = useTheme();

  // Follow Telegram's light/dark scheme (spec D4) and keep following it.
  useEffect(() => {
    const apply = () => setTheme(webApp.colorScheme);
    apply();
    webApp.onEvent("themeChanged", apply);
    return () => webApp.offEvent("themeChanged", apply);
  }, [setTheme, webApp]);

  // Paint Telegram's header/background with our page colour so the frame
  // and the page read as one surface.
  useEffect(() => {
    const bg = cssColor("--background");
    if (!bg) return;
    const hex = toHex(bg);
    webApp.setHeaderColor(hex);
    webApp.setBackgroundColor(hex);
    if (webApp.isVersionAtLeast("7.10")) webApp.setBottomBarColor?.(hex);
  }, [resolvedTheme, webApp]);

  useEffect(() => {
    if (isTabRoot(pathname)) {
      webApp.BackButton.hide();
      return;
    }
    const onBack = () => {
      if (window.history.length > 1) router.back();
      else router.replace(`/${pathname.split("/")[1]}/dashboard`);
    };
    webApp.BackButton.onClick(onBack);
    webApp.BackButton.show();
    return () => webApp.BackButton.offClick(onBack);
  }, [pathname, router, webApp]);

  useEffect(() => {
    if (needsClosingGuard(pathname)) webApp.enableClosingConfirmation();
    else webApp.disableClosingConfirmation();
  }, [pathname, webApp]);

  // Inside Telegram, a plain external <a> would navigate the webview away from
  // the app (and payment pages refuse to be framed on Telegram Web).
  useEffect(() => {
    const onClick = (event: MouseEvent) => {
      if (event.defaultPrevented || event.button !== 0) return;
      const anchor = (event.target as Element | null)?.closest?.("a[href]") as HTMLAnchorElement | null;
      if (!anchor) return;
      let url: URL;
      try {
        url = new URL(anchor.href, window.location.href);
      } catch {
        return;
      }
      if (url.origin === window.location.origin || !/^https?:$/.test(url.protocol)) return;
      event.preventDefault();
      if (url.hostname === "t.me") webApp.openTelegramLink(url.href);
      else webApp.openLink(url.href);
    };
    document.addEventListener("click", onClick, true);
    return () => document.removeEventListener("click", onClick, true);
  }, [webApp]);

  return null;
}
```

  Also patch `window.open` usage for external targets: grep `window.open(` in `src/components` and `src/app` (`TelegramLinkCard`, `mobile-telegram.tsx`, checkout/Payme hand-off). For the checkout/payment hand-off (`grep -rn "paycom\|click.uz\|window.location.href = " src/components/checkout src/app/[locale]/\(app\)/checkout`), when `getWebApp()` is non-null call `getWebApp()!.openLink(url)` instead of navigating; leave website behaviour unchanged. Verify the form-POST Payme flow (`form-action https://checkout.paycom.uz`): if checkout submits an HTML form, in the Mini App build the GET URL equivalent only if Payme supports it — otherwise keep the form submit (mobile webviews navigate top-level fine) and note it in the final report. Do not change website behaviour.

  `telegram-provider.tsx`: `return <TelegramContext.Provider value={webApp}>{webApp && <TelegramChrome webApp={webApp} />}{children}</TelegramContext.Provider>;`

  `globals.css` (append; check how the top bar/tab bar/session runner apply `env(safe-area-inset-*)` first — `grep -n "safe-area-inset" src/app/globals.css src/components/layout/sidebar.tsx`):

```css
/* Telegram Mini App: Telegram's own header/controls overlay the content area
   on newer clients; its SDK publishes the inset as CSS variables. */
html.tg-webapp body {
  padding-top: var(--tg-content-safe-area-inset-top, 0px);
}
html.tg-webapp .app-bottom-nav {
  padding-bottom: max(env(safe-area-inset-bottom), var(--tg-safe-area-inset-bottom, 0px));
}
```

  (Adjust to the real selectors; the goal is no element hidden behind Telegram chrome at 390×844 — verified in Task 10's e2e.)

  Logout + expiry:
  - `profile/page.tsx` `handleLogout` and the mobile profile logout handler: before the fetch, `if (isTelegramMiniApp()) await markTelegramLogout();`; redirect to `postLogoutPath(currentLocale, \`/${currentLocale}/login\`)`.
  - `session-expired-gate.tsx`: replace `router.replace(\`/${locale}/login?expired=1\`)` with
    ```ts
    // In the Mini App the launch data can sign the learner straight back in;
    // /tg does that silently and returns them to where they were.
    router.replace(
      isTelegramMiniApp()
        ? `/${locale}/tg?next=${encodeURIComponent(window.location.pathname + window.location.search)}`
        : `/${locale}/login?expired=1`
    );
    ```
  - `telegram-link-card.tsx` and `mobile-telegram.tsx`: when `useTelegram()` is non-null and `status.linked`, render the card with `t("linkedStatus")` from `TelegramApp` and no link/unlink actions (the Mini App is itself the link). Outside Telegram unchanged.

- [ ] **Step 4: Run, verify PASS** — targeted + full vitest.
- [ ] **Step 5: Commit** — `feat(telegram): native chrome inside the Mini App`.

---

### Task 10: End-to-end proof, full gates, docs

**Files:**
- Create: `frontend/e2e/telegram-mini-app.spec.ts`
- Modify: `deploy/README.md` or the prod env doc that lists `TELEGRAM_BOT_*` (add `TELEGRAM_WEBAPP_URL`)

- [ ] **Step 1: Write the e2e spec** (no backend: stub everything with `page.route`, inject a fake Telegram SDK so the real script is never needed):

```ts
import { test, expect, type Page } from "@playwright/test";

const fakeTelegram = (opts: { autologinOff?: boolean }) => `
  (() => {
    const handlers = {};
    const store = ${opts.autologinOff ? `{ autologin_off: "1" }` : `{}`};
    window.__tg = { calls: [] };
    const rec = (name) => (...args) => window.__tg.calls.push([name, ...args]);
    sessionStorage.setItem("tg-webapp", "1");
    window.Telegram = { WebApp: {
      initData: "query_id=x&user=%7B%22id%22%3A1%7D&auth_date=1&hash=00",
      initDataUnsafe: { user: { id: 1, first_name: "Ali", language_code: "uz" } },
      colorScheme: "dark", version: "8.0", platform: "android",
      ready: rec("ready"), expand: rec("expand"), isVersionAtLeast: () => true,
      disableVerticalSwipes: rec("disableVerticalSwipes"),
      enableClosingConfirmation: rec("enableClosingConfirmation"),
      disableClosingConfirmation: rec("disableClosingConfirmation"),
      setHeaderColor: rec("setHeaderColor"), setBackgroundColor: rec("setBackgroundColor"), setBottomBarColor: rec("setBottomBarColor"),
      onEvent: (e, cb) => { handlers[e] = cb; }, offEvent: () => {},
      openLink: rec("openLink"), openTelegramLink: rec("openTelegramLink"),
      requestContact: (cb) => cb(true, { responseUnsafe: { contact: { phone_number: "998901234567" } } }),
      BackButton: { show: rec("back.show"), hide: rec("back.hide"), onClick: () => {}, offClick: () => {} },
      HapticFeedback: { impactOccurred: rec("impact"), notificationOccurred: rec("notify"), selectionChanged: rec("select") },
      CloudStorage: {
        getItem: (k, cb) => cb(null, store[k] || ""),
        setItem: (k, v, cb) => { store[k] = v; cb && cb(null, true); },
        removeItem: (k, cb) => { delete store[k]; cb && cb(null, true); },
      },
    } };
  })();
`;

async function stubSdk(page: Page) {
  await page.route("https://telegram.org/js/telegram-web-app.js", (r) => r.fulfill({ contentType: "text/javascript", body: "" }));
}

test.describe("Telegram Mini App", () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test("unlinked user sees the welcome screen and reaches the phone login with Telegram's number", async ({ page }) => {
    await stubSdk(page);
    await page.addInitScript(fakeTelegram({}));
    await page.route("**/api/auth/telegram", (r) => r.fulfill({ json: { data: { need_phone: true, first_name: "Ali" } } }));
    await page.goto("/uz-Latn/tg");
    await expect(page.getByRole("heading", { name: /Ali/ })).toBeVisible();
    await page.getByRole("link", { name: "Kirish" }).click();
    await expect(page).toHaveURL(/\/uz-Latn\/login/);
    await page.getByRole("button", { name: "Raqamni Telegram'dan olish" }).click();
    await expect(page.getByLabel(/telefon/i).first()).toHaveValue("90 123 45 67");
  });

  test("signed-out-on-purpose user is offered, not forced, the way back", async ({ page }) => {
    await stubSdk(page);
    await page.addInitScript(fakeTelegram({ autologinOff: true }));
    let calls = 0;
    await page.route("**/api/auth/telegram", (r) => { calls++; return r.fulfill({ json: { data: { need_phone: true, first_name: "Ali" } } }); });
    await page.goto("/uz-Latn/tg");
    await expect(page.getByRole("button", { name: /Ali sifatida davom etish/ })).toBeVisible();
    expect(calls).toBe(0);
  });

  test("opened as a plain URL explains how to open it", async ({ page }) => {
    await page.route("https://telegram.org/js/telegram-web-app.js", (r) => r.fulfill({ contentType: "text/javascript", body: "" }));
    await page.addInitScript(() => sessionStorage.setItem("tg-webapp", "1"));
    await page.goto("/uz-Latn/tg");
    await expect(page.getByRole("alert")).toContainText("Botdan oching", { timeout: 6000 });
  });

  test("website never requests the Telegram SDK", async ({ page }) => {
    const sdk: string[] = [];
    page.on("request", (r) => { if (r.url().includes("telegram-web-app.js")) sdk.push(r.url()); });
    await page.goto("/uz-Latn/login");
    await page.waitForLoadState("networkidle");
    expect(sdk).toEqual([]);
  });

  test("framing headers: learner pages allow Telegram Web, admin does not", async ({ request }) => {
    const learner = await request.get("/uz-Latn/login");
    expect(learner.headers()["content-security-policy"]).toContain("frame-ancestors 'self' https://web.telegram.org");
    expect(learner.headers()["x-frame-options"]).toBeUndefined();
    const admin = await request.get("/uz-Latn/admin/login", { maxRedirects: 0 });
    expect(admin.headers()["x-frame-options"]).toBe("DENY");
  });
});
```

  Adjust the label selector to the login page's real `aria-label` (`t("phoneLabel")` value in `uz-Latn.json`) and the admin login path (`ls src/app/[locale]/admin`).

- [ ] **Step 2: Run** — `cd frontend && CI=true PORT=3112 npx playwright test e2e/telegram-mini-app.spec.ts` → PASS (fix the app, not the test, on real failures).

- [ ] **Step 3: Full gates**
  - `cd backend && go test -p 1 ./... -count=1 && golangci-lint run ./...`
  - `cd frontend && rm -rf .next && npx tsc --noEmit && npm run lint && npx vitest run && CI=true PORT=3112 npx playwright test`
  - `git diff main~10 --stat` audit: no debug code, no unrelated edits.

- [ ] **Step 4: Docs** — add `TELEGRAM_WEBAPP_URL` (purpose, example `https://drivergo.uz/uz-Latn/tg`, kill switch = empty + restart) to the deploy env documentation next to `TELEGRAM_BOT_MODE`.

- [ ] **Step 5: Commit** — `test(e2e): Telegram Mini App entry, chrome and framing`.
