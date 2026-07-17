# M1 Plan 02 — Auth (Phone + Telegram OTP), Profile, Entitlement

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Working authentication (UZ phone number + OTP over pluggable channels: sandbox now, Telegram Gateway ready), JWT sessions with rotating refresh + reuse detection, profile endpoints, and VIP entitlement checks — unblocking every user-scoped feature (sessions, learning, billing).

**Architecture:** `internal/auth` owns OTP lifecycle (salted-hash codes, TTL 5min, ≤5 attempts, Redis rate limits), JWT issue/verify (HS256 access 15min) and refresh rotation (random 256-bit, sha256-stored, 30d, reuse → revoke-all). `internal/billing` owns entitlement math (active pass = max ends_at > now; passes stack). `internal/account` owns /me endpoints. Senders are an interface: `SandboxSender` (dev), `TelegramSender` (Gateway API, config-gated); SMS (Eskiz) deferred — channel `sms` errors `not_configured`.

**Tech Stack:** adds `github.com/golang-jwt/jwt/v5`, `github.com/redis/go-redis/v9`. Everything else from Plan 01.

## Global Constraints

- All Plan 01 conventions hold (envelope, locales, commit style, `-p 1` tests, `export PATH=$HOME/.local/go/bin:$HOME/go/bin:$PATH`).
- Phones: Uzbekistan E.164 only — normalized to `+998XXXXXXXXX` (9 digits after +998); anything else → `invalid_phone`.
- OTP: 6 digits; storage format `salt$sha256(salt||code)` (hex, 16-byte random salt); TTL 300s; max 5 attempts; per-phone resend cooldown 60s; per-phone 5/hour; per-IP 20/hour. Limits via Redis fixed windows.
- `debug_code` appears in the OTP-request response ONLY when `cfg.Env == "dev"` AND channel is `sandbox` — never otherwise; a test asserts absence.
- JWT: HS256, secret `JWT_SECRET` (dev default `dev-secret-change-me`), access TTL 15min, claims `sub` (profile uuid), `role`, `jti`. Refresh tokens are opaque (32 random bytes, base64url), stored hashed (sha256 hex), TTL 30d, single-use (rotation); presenting a revoked token revokes ALL of that profile's refresh tokens.
- Redis tests use `TEST_REDIS_URL` (default `redis://localhost:6379/1`) and flush that DB; dev uses DB 0.
- New config keys (env): `JWT_SECRET`, `OTP_CHANNEL` (`sandbox`|`telegram`|`sms`, default `sandbox`), `TELEGRAM_GATEWAY_TOKEN` (optional), `TELEGRAM_GATEWAY_URL` (default `https://gatewayapi.telegram.org`).
- Referral code: 8 chars, Crockford base32 (no I/L/O/U), generated at profile creation, unique with 5 retries.

## File Structure (new/modified)

```
backend/
  internal/auth/
    phone.go phone_test.go        # NormalizePhone
    code.go code_test.go          # GenerateCode, HashCode, VerifyCode, NewReferralCode
    jwt.go jwt_test.go            # IssueAccess, ParseAccess, NewRefreshToken, HashToken
    ratelimit.go ratelimit_test.go# Limiter (Redis fixed window + cooldown)
    sender.go                     # Sender iface, SandboxSender, senderFor(cfg)
    telegram.go telegram_test.go  # TelegramSender (Gateway API)
    service.go service_test.go    # RequestOTP/VerifyOTP/Refresh/Logout
    middleware.go middleware_test.go
    handlers.go handlers_test.go
  internal/billing/entitlement.go entitlement_test.go
  internal/account/handlers.go handlers_test.go
  internal/redisx/redisx.go testhelper.go
  internal/db/queries/auth.sql   # + sqlc generate
  cmd/grantvip/main.go
  internal/config/config.go      # +JWTSecret/OTPChannel/Telegram* (modify)
  internal/server/server.go      # +Redis dep, mount auth & me routes (modify)
  cmd/api/main.go                # +redis client (modify)
```

---

### Task 1: Pure utils — phone, OTP code, referral, JWT, refresh tokens

**Files:** create `internal/auth/phone.go|phone_test.go|code.go|code_test.go|jwt.go|jwt_test.go`

**Interfaces (produced):**
- `auth.NormalizePhone(raw string) (string, error)` → `+998XXXXXXXXX` or error
- `auth.GenerateCode() string` (6 digits, crypto/rand); `auth.HashCode(code string) string` (returns `salt$hash`); `auth.VerifyCode(stored, code string) bool` (constant-time)
- `auth.NewReferralCode() string` (8-char Crockford)
- `auth.IssueAccess(secret []byte, profileID uuid.UUID, role string, ttl time.Duration) (string, error)`; `auth.ParseAccess(secret []byte, token string) (Claims{ProfileID uuid.UUID; Role string}, error)`
- `auth.NewRefreshToken() (raw string)`; `auth.HashToken(raw string) string` (sha256 hex)

- [ ] Write tests: normalize table (`"+998 90 123-45-67"`→`+998901234567`; `998901234567`→ok; `901234567`→ok (prepend); `+7...`→err; short→err). Code: generated is 6 digits; Verify(Hash(c), c)=true; wrong code false; tampered stored false. Referral: len 8, charset check, 1000 gen → unique-ish (no dup in run). JWT: issue→parse roundtrip (sub/role match); wrong secret → err; expired ttl=-1s → err. Refresh: raw len ≥ 43 (32B base64url), HashToken deterministic 64 hex.
- [ ] Run → FAIL; implement:

`phone.go`:
```go
package auth

import (
	"fmt"
	"strings"
)

// NormalizePhone accepts UZ numbers in loose formats and returns +998XXXXXXXXX.
func NormalizePhone(raw string) (string, error) {
	var digits strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	d := digits.String()
	switch {
	case len(d) == 12 && strings.HasPrefix(d, "998"):
	case len(d) == 9:
		d = "998" + d
	default:
		return "", fmt.Errorf("invalid uz phone: %q", raw)
	}
	return "+" + d, nil
}
```

`code.go`:
```go
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
)

func GenerateCode() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(1000000))
	return fmt.Sprintf("%06d", n.Int64())
}

func HashCode(code string) string {
	salt := make([]byte, 16)
	_, _ = rand.Read(salt)
	sum := sha256.Sum256(append(salt, []byte(code)...))
	return hex.EncodeToString(salt) + "$" + hex.EncodeToString(sum[:])
}

func VerifyCode(stored, code string) bool {
	parts := strings.SplitN(stored, "$", 2)
	if len(parts) != 2 {
		return false
	}
	salt, err := hex.DecodeString(parts[0])
	if err != nil {
		return false
	}
	sum := sha256.Sum256(append(salt, []byte(code)...))
	return subtle.ConstantTimeCompare([]byte(hex.EncodeToString(sum[:])), []byte(parts[1])) == 1
}

const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

func NewReferralCode() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	out := make([]byte, 8)
	for i, v := range b {
		out[i] = crockford[int(v)%len(crockford)]
	}
	return string(out)
}
```

`jwt.go`:
```go
package auth

import (
	"fmt"
	"time"

	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	ProfileID uuid.UUID
	Role      string
}

func IssueAccess(secret []byte, profileID uuid.UUID, role string, ttl time.Duration) (string, error) {
	now := time.Now()
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  profileID.String(),
		"role": role,
		"iat":  now.Unix(),
		"exp":  now.Add(ttl).Unix(),
		"jti":  uuid.NewString(),
	})
	return t.SignedString(secret)
}

func ParseAccess(secret []byte, token string) (Claims, error) {
	parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected alg %v", t.Header["alg"])
		}
		return secret, nil
	})
	if err != nil || !parsed.Valid {
		return Claims{}, fmt.Errorf("invalid token: %w", err)
	}
	mc, _ := parsed.Claims.(jwt.MapClaims)
	sub, _ := mc["sub"].(string)
	id, err := uuid.Parse(sub)
	if err != nil {
		return Claims{}, fmt.Errorf("invalid sub")
	}
	role, _ := mc["role"].(string)
	return Claims{ProfileID: id, Role: role}, nil
}

func NewRefreshToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
```
- [ ] `go get github.com/golang-jwt/jwt/v5` → tests PASS → commit `feat(backend): auth primitives — phone, otp code, referral, jwt, refresh`

### Task 2: Redis client + rate limiter

**Files:** create `internal/redisx/redisx.go|testhelper.go`, `internal/auth/ratelimit.go|ratelimit_test.go`

**Interfaces:** `redisx.New(url string) (*redis.Client, error)`; `redisx.NewTest(t) *redis.Client` (TEST_REDIS_URL default `redis://localhost:6379/1`, FlushDB, cleanup close); `auth.Limiter{R *redis.Client}` with `Allow(ctx, key string, limit int, window time.Duration) (bool, error)` (INCR; if 1 → EXPIRE window; count>limit → false) and `Cooldown(ctx, key string, d time.Duration) (bool, error)` (SET NX EX; false if exists).

- [ ] Tests: Allow 3 with limit 3 → true×3, 4th false; new key independent; Cooldown first true, second false. Implement; `go get github.com/redis/go-redis/v9`; PASS; commit `feat(backend): redis client and fixed-window rate limiter`.

`redisx.go`:
```go
// Package redisx builds redis clients from URLs.
package redisx

import "github.com/redis/go-redis/v9"

func New(url string) (*redis.Client, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}
	return redis.NewClient(opt), nil
}
```
`testhelper.go`:
```go
package redisx

import (
	"context"
	"os"
	"testing"
)

import "github.com/redis/go-redis/v9"

func NewTest(t *testing.T) *redis.Client {
	t.Helper()
	url := os.Getenv("TEST_REDIS_URL")
	if url == "" {
		url = "redis://localhost:6379/1"
	}
	c, err := New(url)
	if err != nil {
		t.Fatalf("redis: %v", err)
	}
	if err := c.FlushDB(context.Background()).Err(); err != nil {
		t.Fatalf("flushdb: %v (run `make up`)", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}
```
`ratelimit.go`:
```go
package auth

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type Limiter struct{ R *redis.Client }

// Allow implements a fixed-window counter: true while count ≤ limit.
func (l Limiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	n, err := l.R.Incr(ctx, key).Result()
	if err != nil {
		return false, err
	}
	if n == 1 {
		_ = l.R.Expire(ctx, key, window).Err()
	}
	return n <= int64(limit), nil
}

// Cooldown returns true if the key was free (and sets it for d).
func (l Limiter) Cooldown(ctx context.Context, key string, d time.Duration) (bool, error) {
	ok, err := l.R.SetNX(ctx, key, "1", d).Result()
	return ok, err
}
```

### Task 3: Config + auth queries + sqlc

**Files:** modify `internal/config/config.go` (+`JWTSecret, OTPChannel, TelegramGatewayToken, TelegramGatewayURL` via getenv defaults `dev-secret-change-me`, `sandbox`, ``, `https://gatewayapi.telegram.org`; extend config_test); create `internal/db/queries/auth.sql`; `sqlc generate`.

`auth.sql`:
```sql
-- name: CreateOTPChallenge :one
INSERT INTO otp_challenge (phone, code_hash, channel, expires_at)
VALUES ($1, $2, $3, $4) RETURNING id;

-- name: LatestOTPChallenge :one
SELECT * FROM otp_challenge
WHERE phone = $1 AND consumed = false
ORDER BY created_at DESC LIMIT 1;

-- name: IncrementOTPAttempts :exec
UPDATE otp_challenge SET attempts = attempts + 1 WHERE id = $1;

-- name: ConsumeOTP :exec
UPDATE otp_challenge SET consumed = true WHERE id = $1;

-- name: GetProfileByPhone :one
SELECT * FROM profile WHERE phone = $1;

-- name: GetProfileByID :one
SELECT * FROM profile WHERE id = $1;

-- name: CreateProfile :one
INSERT INTO profile (phone, referral_code) VALUES ($1, $2) RETURNING *;

-- name: UpdateProfileMe :one
UPDATE profile SET
  name = $2, region = $3, district = $4, birth_date = $5,
  locale_pref = $6, theme_pref = $7
WHERE id = $1 RETURNING *;

-- name: CreateRefreshToken :exec
INSERT INTO refresh_token (profile_id, token_hash, expires_at) VALUES ($1, $2, $3);

-- name: GetRefreshToken :one
SELECT * FROM refresh_token WHERE token_hash = $1;

-- name: RevokeRefreshToken :exec
UPDATE refresh_token SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL;

-- name: RevokeAllRefreshTokens :exec
UPDATE refresh_token SET revoked_at = now() WHERE profile_id = $1 AND revoked_at IS NULL;

-- name: ActiveEntitlementEnd :one
SELECT ends_at FROM entitlement
WHERE profile_id = $1 AND ends_at > now()
ORDER BY ends_at DESC LIMIT 1;

-- name: InsertEntitlement :one
INSERT INTO entitlement (profile_id, source, starts_at, ends_at, note, created_by)
VALUES ($1, $2, $3, $4, $5, $6) RETURNING id;
```
- [ ] generate; build; config tests updated (defaults + override) PASS; commit `feat(backend): auth/profile/entitlement queries and config keys`.

### Task 4: Senders (sandbox + Telegram Gateway)

**Files:** create `internal/auth/sender.go`, `internal/auth/telegram.go|telegram_test.go`.

**Interfaces:** `type Sender interface{ Send(ctx, phone, code string) error; Channel() string }`; `SandboxSender{Log *zap.Logger}` (logs, never fails, Channel "sandbox"); `NewTelegramSender(baseURL, token string, hc *http.Client) *TelegramSender` (POST `{base}/sendVerificationMessage` JSON `{phone_number, code}`, `Authorization: Bearer {token}`; non-200 → error; Channel "telegram"); `SenderFor(cfg config.Config, log *zap.Logger) (Sender, error)` — sandbox default; telegram requires token; `sms` → error `sms sender not configured (Plan 05+)`.

- [ ] telegram_test: httptest server asserting path/auth-header/body, returns 200 → Send ok; 500 → error. Implement; PASS; commit `feat(backend): otp senders — sandbox and telegram gateway`.

### Task 5: Auth service (request/verify) — DB+Redis integration

**Files:** create `internal/auth/service.go|service_test.go`.

**Interfaces:**
```go
type Service struct {
	Q       *sqlc.Queries
	Pool    *pgxpool.Pool // tx for verify+profile-create
	Lim     Limiter
	Sender  Sender
	Secret  []byte
	Env     string
	AccessTTL  time.Duration // 15m default via NewService
	RefreshTTL time.Duration // 30d
	CodeTTL    time.Duration // 5m
}
func NewService(...) *Service // fills defaults
type OTPRequestResult struct{ Channel, DebugCode string } // DebugCode "" unless dev+sandbox
func (s *Service) RequestOTP(ctx, rawPhone, ip string) (OTPRequestResult, error)
type Tokens struct{ Access, Refresh string }
type VerifyResult struct{ Tokens; Profile sqlc.Profile; Created bool }
func (s *Service) VerifyOTP(ctx, rawPhone, code string) (VerifyResult, error)
```
Error sentinels: `ErrRateLimited, ErrInvalidPhone, ErrInvalidCode, ErrExpiredCode, ErrTooManyAttempts` (errors.New; handlers map to HTTP).
Logic per Global Constraints; profile create wraps in tx: GetProfileByPhone → if ErrNoRows create with referral retry (unique violation → regenerate, ≤5); issue access+refresh, store hashed refresh.
- [ ] service_test (testdb+redisx.NewTest): happy path request→(read code_hash? No—use sandbox sender capture) — capture code via test Sender stub `captureSender{last string}`; verify → tokens non-empty, profile created with referral_code len 8, Created true; second verify same phone new code → Created false; wrong code ×5 → ErrTooManyAttempts on 6th even with right code; expired (CodeTTL=-1s service) → ErrExpiredCode; cooldown: immediate second RequestOTP → ErrRateLimited. PASS; commit `feat(backend): otp auth service with rate limits and profile provisioning`.

### Task 6: Refresh rotation + reuse detection + logout

**Files:** modify `internal/auth/service.go` (+`Refresh(ctx, raw string) (Tokens, error)`, `Logout(ctx, raw string) error`), extend service_test.

Logic: hash → GetRefreshToken; ErrNoRows → ErrInvalidRefresh; revoked_at set → RevokeAllRefreshTokens(profile) + ErrReusedRefresh; expired → ErrInvalidRefresh; else revoke old, insert new, issue access. Logout: revoke if exists (nil on missing).
- [ ] tests: refresh returns new pair, old raw now fails with ErrReusedRefresh AND second valid token (from before reuse) also revoked (revoke-all proof); logout then refresh → ErrInvalidRefresh. PASS; commit `feat(backend): refresh rotation with reuse detection and logout`.

### Task 7: Middleware + auth handlers + wiring

**Files:** create `internal/auth/middleware.go|middleware_test.go|handlers.go|handlers_test.go`; modify `internal/server/server.go` (Deps{Queries, Pool, Redis}; build auth service+handler; mount `/api/v1/auth/*`); modify `cmd/api/main.go` (redis client via redisx.New(cfg.RedisURL)).

**Interfaces:** `auth.Required(secret []byte) func(http.Handler) http.Handler` (Bearer → Claims in ctx; 401 `unauthorized`); `auth.FromContext(ctx) (Claims, bool)`; handler routes: `POST /auth/otp/request {phone}`; `POST /auth/otp/verify {phone, code}`; `POST /auth/refresh {refresh_token}`; `POST /auth/logout {refresh_token}`. Error mapping: ErrRateLimited→429 `rate_limited`; ErrInvalidPhone→400; ErrInvalidCode/Expired/TooMany→400 with codes `invalid_code|expired_code|too_many_attempts`; ErrInvalidRefresh/Reused→401 `invalid_refresh|refresh_reused`.
- [ ] handlers_test (httptest full server, sandbox+dev → use debug_code from response): request→verify→access works on protected probe route (test mounts `Required` around a dummy); refresh rotation over HTTP; 429 on immediate re-request; invalid locale-agnostic. middleware_test: missing/garbage/expired token → 401. PASS; commit `feat(backend): auth http endpoints and jwt middleware`.

### Task 8: Billing entitlement + account (/me) endpoints

**Files:** create `internal/billing/entitlement.go|entitlement_test.go`, `internal/account/handlers.go|handlers_test.go`; modify server (mount `/api/v1/me` with Required).

**Interfaces:** `billing.Service{Q *sqlc.Queries}`: `Status(ctx, profileID) (vip bool, until *time.Time, err)`; `GrantDays(ctx, profileID, days int, source, note string, by uuid.NullUUID) (until time.Time, err)` (start = max(now, ActiveEntitlementEnd)). Account routes: `GET /me` → `{profile:{id,phone,name,region,district,birth_date,locale_pref,theme_pref,referral_code,role,created_at}, vip:{active,until}}`; `PATCH /me` (partial body; birth_date `"2006-01-02"` or null) → updated profile; `GET /me/entitlement` → `{active, until}`.
- [ ] tests: Status false/nil on fresh profile; GrantDays 30 → active true until≈now+30d; second GrantDays 30 → until≈now+60d (stacking); HTTP: unauthenticated /me → 401; authenticated GET/PATCH roundtrip; PATCH invalid birth_date → 400. PASS; commit `feat(backend): entitlement service and /me endpoints`.

### Task 9: grantvip CLI

**Files:** create `cmd/grantvip/main.go` — flags `-phone -days -note`; loads config, connects db, finds profile by normalized phone (error if missing: "profile not found — user must sign in once first"), GrantDays source `admin`, prints `VIP until: <ts>`.
- [ ] build; smoke on dev DB (after a sandbox signup in Task 10 smoke); commit `feat(backend): grantvip admin CLI`.

### Task 10: Full verification + docs

- [ ] `make check` (lint + `-p 1` full suite) green.
- [ ] Live smoke (PORT=8090): otp/request (see debug_code) → verify → GET /me (Bearer) → PATCH /me → grantvip -phone ... -days 7 → GET /me shows vip.until. Record outputs.
- [ ] README: auth section (endpoints, env keys, sandbox flow, grantvip). Commit `docs: auth flow and env reference`.

## Self-Review

1. **Spec coverage:** §8 auth oqimi (telefon+TG kod, TTL/attempt/cooldown/rate-limit, JWT+rotating refresh) → T1–T7; anti-fraud device jadvali writes deferred (Plan 04 events bilan birga — spec §8.4 faqat yozish, blokловčи emas); §7.2 entitlement semantics (stacking) → T8; ichki VIP berish (CLI) → T9; Eskiz SMS ataylab keyinga (spec: zaxira kanal) — `SenderFor` xato beradi, hujjatlashtirilgan.
2. **Placeholders:** yo'q; har task kod yoki aniq xulq-shartnoma bilan.
3. **Type consistency:** `Tokens/Claims/Limiter/Sender` nomlari T1–T8 bo'ylab mos; sqlc nomlari generate'dan keyin moslashtiriladi (Plan 01 tajribasi).
