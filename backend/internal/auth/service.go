package auth

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/db/sqlc"
)

var (
	ErrRateLimited     = errors.New("rate limited")
	ErrInvalidPhone    = errors.New("invalid phone")
	ErrInvalidCode     = errors.New("invalid code")
	ErrExpiredCode     = errors.New("expired code")
	ErrTooManyAttempts = errors.New("too many attempts")
	ErrInvalidRefresh  = errors.New("invalid refresh token")
	ErrReusedRefresh   = errors.New("refresh token reused")
	// ErrAccountBlocked is returned when profile.status is banned (admin block).
	ErrAccountBlocked = errors.New("account blocked")
)

const profileStatusBanned = "banned"

func assertProfileActive(profile sqlc.Profile) error {
	if profile.Status == profileStatusBanned {
		return ErrAccountBlocked
	}
	return nil
}

const maxOTPAttempts = 5

// Service owns the OTP request/verify lifecycle and session issuance.
type Service struct {
	Q      *sqlc.Queries
	Pool   *pgxpool.Pool
	Lim    Limiter
	Sender Sender
	Secret []byte
	Env    string
	// DebugEcho returns the generated OTP in the request response. Explicit
	// opt-in (config.OTPDebugEcho), never inferred from Env — a host left on
	// ENV=dev by mistake must not hand out every account.
	DebugEcho bool

	AccessTTL  time.Duration
	RefreshTTL time.Duration
	CodeTTL    time.Duration
}

// NewService builds a Service with production-default TTLs.
func NewService(q *sqlc.Queries, pool *pgxpool.Pool, lim Limiter, sender Sender, secret []byte, env string) *Service {
	return &Service{
		Q:      q,
		Pool:   pool,
		Lim:    lim,
		Sender: sender,
		Secret: secret,
		Env:    env,

		AccessTTL:  15 * time.Minute,
		RefreshTTL: 30 * 24 * time.Hour,
		CodeTTL:    5 * time.Minute,
	}
}

type OTPRequestResult struct {
	Channel   string
	DebugCode string // set only when DebugEcho is on AND channel=="sandbox"
}

// RequestOTP enforces cooldown/rate limits, then generates and sends a code.
func (s *Service) RequestOTP(ctx context.Context, rawPhone, ip string) (OTPRequestResult, error) {
	if s.Sender.Channel() == "off" {
		return OTPRequestResult{}, ErrOTPDisabled
	}
	phone, err := NormalizePhone(rawPhone)
	if err != nil {
		return OTPRequestResult{}, ErrInvalidPhone
	}

	if ok, err := s.Lim.Cooldown(ctx, "otp:cooldown:"+phone, 60*time.Second); err != nil {
		return OTPRequestResult{}, err
	} else if !ok {
		return OTPRequestResult{}, ErrRateLimited
	}
	if ok, err := s.Lim.Allow(ctx, "otp:phone:"+phone, 5, time.Hour); err != nil {
		return OTPRequestResult{}, err
	} else if !ok {
		return OTPRequestResult{}, ErrRateLimited
	}
	if ip != "" {
		if ok, err := s.Lim.Allow(ctx, "otp:ip:"+ip, 20, time.Hour); err != nil {
			return OTPRequestResult{}, err
		} else if !ok {
			return OTPRequestResult{}, ErrRateLimited
		}
	}

	code, err := GenerateCode()
	if err != nil {
		return OTPRequestResult{}, err
	}
	codeHash, err := HashCode(code)
	if err != nil {
		return OTPRequestResult{}, err
	}
	_, err = s.Q.CreateOTPChallenge(ctx, sqlc.CreateOTPChallengeParams{
		Phone:     phone,
		CodeHash:  codeHash,
		Channel:   s.Sender.Channel(),
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(s.CodeTTL), Valid: true},
	})
	if err != nil {
		return OTPRequestResult{}, err
	}

	if err := s.Sender.Send(ctx, phone, code); err != nil {
		return OTPRequestResult{}, err
	}

	res := OTPRequestResult{Channel: s.Sender.Channel()}
	if s.DebugEcho && s.Sender.Channel() == "sandbox" {
		res.DebugCode = code
	}
	return res, nil
}

type Tokens struct {
	Access  string
	Refresh string
}

type VerifyResult struct {
	Tokens
	Profile sqlc.Profile
	Created bool
}

// VerifyOTP checks the latest challenge for phone, provisions a profile on
// first sign-in, and issues an access+refresh token pair.
func (s *Service) VerifyOTP(ctx context.Context, rawPhone, code string) (VerifyResult, error) {
	// Also refused here, not just in RequestOTP: a challenge created while the
	// channel was live must not stay redeemable after OTP is turned off.
	if s.Sender.Channel() == "off" {
		return VerifyResult{}, ErrOTPDisabled
	}
	phone, err := NormalizePhone(rawPhone)
	if err != nil {
		return VerifyResult{}, ErrInvalidPhone
	}

	challenge, err := s.Q.LatestOTPChallenge(ctx, phone)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return VerifyResult{}, ErrInvalidCode
		}
		return VerifyResult{}, err
	}

	if challenge.Attempts >= maxOTPAttempts {
		return VerifyResult{}, ErrTooManyAttempts
	}
	if time.Now().After(challenge.ExpiresAt.Time) {
		return VerifyResult{}, ErrExpiredCode
	}
	if !VerifyCode(challenge.CodeHash, code) {
		if err := s.Q.IncrementOTPAttempts(ctx, challenge.ID); err != nil {
			return VerifyResult{}, err
		}
		return VerifyResult{}, ErrInvalidCode
	}
	if err := s.Q.ConsumeOTP(ctx, challenge.ID); err != nil {
		return VerifyResult{}, err
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return VerifyResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := sqlc.New(tx)

	created := false
	profile, err := q.GetProfileByPhone(ctx, phone)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return VerifyResult{}, err
		}
		profile, err = createProfileWithReferral(ctx, q, phone, "", "")
		if err != nil {
			return VerifyResult{}, err
		}
		if err := grantSignupTrial(ctx, q, profile.ID); err != nil {
			return VerifyResult{}, err
		}
		created = true
	}
	if err := assertProfileActive(profile); err != nil {
		return VerifyResult{}, err
	}

	access, err := IssueAccess(s.Secret, profile.ID, profile.Role, s.AccessTTL)
	if err != nil {
		return VerifyResult{}, err
	}
	refresh, err := NewRefreshToken()
	if err != nil {
		return VerifyResult{}, err
	}
	if err := q.CreateRefreshToken(ctx, sqlc.CreateRefreshTokenParams{
		ProfileID: profile.ID,
		TokenHash: HashToken(refresh),
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(s.RefreshTTL), Valid: true},
	}); err != nil {
		return VerifyResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return VerifyResult{}, err
	}

	return VerifyResult{
		Tokens:  Tokens{Access: access, Refresh: refresh},
		Profile: profile,
		Created: created,
	}, nil
}

type RegisterInput struct {
	Phone    string
	Password string
	Name     string
	IP       string
}

// Register creates a profile with phone + password and issues a session.
// SMS OTP is intentionally not required until an OTP provider is wired —
// learners sign up with phone and password only.
func (s *Service) Register(ctx context.Context, in RegisterInput) (VerifyResult, error) {
	phone, err := NormalizePhone(in.Phone)
	if err != nil {
		return VerifyResult{}, ErrInvalidPhone
	}
	if err := s.rateLimitAuth(ctx, "register", phone, in.IP); err != nil {
		return VerifyResult{}, err
	}

	hash, err := HashPassword(in.Password)
	if err != nil {
		return VerifyResult{}, err
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return VerifyResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := sqlc.New(tx)

	if _, err := q.GetProfileByPhone(ctx, phone); err == nil {
		return VerifyResult{}, ErrPhoneTaken
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return VerifyResult{}, err
	}

	name := strings.TrimSpace(in.Name)
	profile, err := createProfileWithReferral(ctx, q, phone, hash, name)
	if err != nil {
		if isUniqueViolation(err) {
			return VerifyResult{}, ErrPhoneTaken
		}
		return VerifyResult{}, err
	}
	if err := grantSignupTrial(ctx, q, profile.ID); err != nil {
		return VerifyResult{}, err
	}

	toks, err := s.issueSession(ctx, q, profile)
	if err != nil {
		return VerifyResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return VerifyResult{}, err
	}
	return VerifyResult{Tokens: toks, Profile: profile, Created: true}, nil
}

type LoginInput struct {
	Phone    string
	Password string
	IP       string
}

// Login authenticates phone + password and issues a session.
func (s *Service) Login(ctx context.Context, in LoginInput) (VerifyResult, error) {
	phone, err := NormalizePhone(in.Phone)
	if err != nil {
		return VerifyResult{}, ErrInvalidPhone
	}
	if err := s.rateLimitAuth(ctx, "login", phone, in.IP); err != nil {
		return VerifyResult{}, err
	}

	profile, err := s.Q.GetProfileByPhone(ctx, phone)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return VerifyResult{}, ErrInvalidCreds
		}
		return VerifyResult{}, err
	}
	if !profile.PasswordHash.Valid || profile.PasswordHash.String == "" {
		return VerifyResult{}, ErrPasswordNotSet
	}
	if !CheckPassword(profile.PasswordHash.String, in.Password) {
		return VerifyResult{}, ErrInvalidCreds
	}
	if err := assertProfileActive(profile); err != nil {
		return VerifyResult{}, err
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return VerifyResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := sqlc.New(tx)

	toks, err := s.issueSession(ctx, q, profile)
	if err != nil {
		return VerifyResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return VerifyResult{}, err
	}
	return VerifyResult{Tokens: toks, Profile: profile, Created: false}, nil
}

func (s *Service) rateLimitAuth(ctx context.Context, action, phone, ip string) error {
	if ok, err := s.Lim.Allow(ctx, action+":phone:"+phone, 10, time.Hour); err != nil {
		return err
	} else if !ok {
		return ErrRateLimited
	}
	if ip != "" {
		if ok, err := s.Lim.Allow(ctx, action+":ip:"+ip, 30, time.Hour); err != nil {
			return err
		} else if !ok {
			return ErrRateLimited
		}
	}
	return nil
}

func (s *Service) issueSession(ctx context.Context, q *sqlc.Queries, profile sqlc.Profile) (Tokens, error) {
	if err := assertProfileActive(profile); err != nil {
		return Tokens{}, err
	}
	access, err := IssueAccess(s.Secret, profile.ID, profile.Role, s.AccessTTL)
	if err != nil {
		return Tokens{}, err
	}
	refresh, err := NewRefreshToken()
	if err != nil {
		return Tokens{}, err
	}
	if err := q.CreateRefreshToken(ctx, sqlc.CreateRefreshTokenParams{
		ProfileID: profile.ID,
		TokenHash: HashToken(refresh),
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(s.RefreshTTL), Valid: true},
	}); err != nil {
		return Tokens{}, err
	}
	return Tokens{Access: access, Refresh: refresh}, nil
}

// refreshGraceTTL is how long a just-rotated refresh token may be presented
// again and still receive the same successor pair. Parallel BFF/proxy calls
// (and multi-tab) often race with the old cookie after rotation; treating that
// as theft + revoke-all caused intermittent production logouts. Past this
// window, reuse still triggers revoke-all (compromise signal).
const refreshGraceTTL = 45 * time.Second

func refreshGraceKey(raw string) string {
	return "auth:rtgrace:" + HashToken(raw)
}

// storedRefreshPair is the grace-window payload.
//
// ProfileID is carried alongside the tokens so the grace path can enforce the
// account-status check (see gracePair) with one primary-key read, instead of
// first resolving the refresh-token row to learn whose session this is.
type storedRefreshPair struct {
	Access    string    `json:"a"`
	Refresh   string    `json:"r"`
	ProfileID uuid.UUID `json:"p"`
}

// graceSealInfo is the HKDF label separating this key derivation from any
// other use of Service.Secret (JWT signing today, anything added later).
const graceSealInfo = "avtotest.uz/auth/refresh-grace/v1"

// graceAEAD derives the AES-256-GCM key that seals one grace entry.
//
// Why encrypt at all: the grace path must hand the caller the *actual*
// successor access+refresh tokens, so — unlike every other token in this
// codebase — the stored value cannot be a one-way HashToken digest. It used to
// be stored as plaintext JSON, which made `redis-cli --scan` or a leaked RDB
// snapshot a source of live 15-minute sessions and 30-day refresh tokens for
// every account that had refreshed in the last 45 seconds.
//
// Why the key is derived from the raw token and not from Service.Secret alone:
// the Redis key is HashToken(raw), i.e. only a sha256 digest of the token, so
// mixing raw into the derivation makes the entry openable *only* by a caller
// who already presents the pre-rotation token. That is exactly the caller the
// grace window exists to serve, and it means an attacker holding a full Redis
// dump gets nothing even if the JWT secret leaks too — they would need a
// sha256 preimage. Salting with raw also binds each entry to its own key, so a
// blob copied to another key simply fails to open.
//
// The trade-offs this accepts: (1) the server cannot enumerate or inspect
// grace entries out of band — nothing needs to, they are write-once,
// read-by-the-holder, 45s-lived; (2) rotating Service.Secret orphans entries
// written before the rotation. They then behave exactly like an expired grace
// entry (the pre-rotation token falls through to reuse detection), which is
// already the fate of every access token at a secret rotation and is bounded
// by the 45-second TTL.
func (s *Service) graceAEAD(raw string) (cipher.AEAD, error) {
	key, err := hkdf.Key(sha256.New, s.Secret, []byte(raw), graceSealInfo, 32)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// sealRefreshGrace encrypts pair into the value stored under the grace key.
// Output is base64url(nonce || AES-GCM ciphertext).
func (s *Service) sealRefreshGrace(raw string, pair storedRefreshPair) (string, error) {
	payload, err := json.Marshal(pair)
	if err != nil {
		return "", err
	}
	aead, err := s.graceAEAD(raw)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(aead.Seal(nonce, nonce, payload, nil)), nil
}

// openRefreshGrace reverses sealRefreshGrace. Anything that does not decrypt
// and parse into a complete pair is reported as a miss: the caller then falls
// through to the DB path, which is the same behaviour a corrupt or truncated
// value had before.
func (s *Service) openRefreshGrace(raw, sealed string) (storedRefreshPair, bool) {
	blob, err := base64.RawURLEncoding.DecodeString(sealed)
	if err != nil {
		return storedRefreshPair{}, false
	}
	aead, err := s.graceAEAD(raw)
	if err != nil || len(blob) < aead.NonceSize() {
		return storedRefreshPair{}, false
	}
	plain, err := aead.Open(nil, blob[:aead.NonceSize()], blob[aead.NonceSize():], nil)
	if err != nil {
		return storedRefreshPair{}, false
	}
	var pair storedRefreshPair
	if err := json.Unmarshal(plain, &pair); err != nil {
		return storedRefreshPair{}, false
	}
	if pair.Access == "" || pair.Refresh == "" || pair.ProfileID == uuid.Nil {
		return storedRefreshPair{}, false
	}
	return pair, true
}

func (s *Service) readRefreshGrace(ctx context.Context, raw string) (storedRefreshPair, bool) {
	if s.Lim.R == nil {
		return storedRefreshPair{}, false
	}
	val, err := s.Lim.R.Get(ctx, refreshGraceKey(raw)).Result()
	if err != nil {
		return storedRefreshPair{}, false
	}
	return s.openRefreshGrace(raw, val)
}

// gracePair resolves the cached successor pair for raw and applies the same
// account-status gate the rotation path applies.
//
// The bug this closes: Refresh returned the cached pair before touching the
// database, so assertProfileActive never ran on the grace path. A profile
// banned during those 45 seconds still collected a fresh 15-minute access
// token — and since a client refreshes exactly when its access token is about
// to expire, the banned user's next request was very likely to land here.
//
// The status check costs one indexed primary-key SELECT on a grace hit. That
// is deliberately not free, and it is still an order of magnitude cheaper than
// the rotation it short-circuits (token lookup, profile lookup, then a
// BEGIN/UPDATE/INSERT/COMMIT), it is read-only, and grace hits are rare by
// construction — only late or parallel callers holding a token that was
// rotated in the last 45 seconds reach it. For scale: RejectBanned already
// runs this identical query on *every* authenticated learner request.
//
// The check lives here rather than in an "invalidate the grace entries when an
// admin bans someone" hook because the database is the single source of truth
// for account status: bans applied by a support script, a migration, or direct
// SQL — the way the existing test bans, and the way incidents actually get
// handled at 3am — carry no application hook to fire. An invalidation hook
// would be a latency optimisation on top of this check, never a replacement
// for it.
//
// A missing profile (deleted account) reports an invalid refresh token rather
// than a "reused" one: there is nobody left to revoke tokens for, and the
// revoke-all path would only spend writes on a dead profile id. Any other
// database error propagates, so the grace path fails closed instead of
// issuing tokens it could not authorise.
func (s *Service) gracePair(ctx context.Context, raw string) (Tokens, bool, error) {
	pair, ok := s.readRefreshGrace(ctx, raw)
	if !ok {
		return Tokens{}, false, nil
	}
	profile, err := s.Q.GetProfileByID(ctx, pair.ProfileID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Tokens{}, false, ErrInvalidRefresh
		}
		return Tokens{}, false, err
	}
	if err := assertProfileActive(profile); err != nil {
		return Tokens{}, false, err
	}
	return Tokens{Access: pair.Access, Refresh: pair.Refresh}, true, nil
}

func (s *Service) writeRefreshGrace(ctx context.Context, oldRaw string, profileID uuid.UUID, tokens Tokens) {
	if s.Lim.R == nil {
		return
	}
	sealed, err := s.sealRefreshGrace(oldRaw, storedRefreshPair{
		Access:    tokens.Access,
		Refresh:   tokens.Refresh,
		ProfileID: profileID,
	})
	if err != nil {
		return
	}
	// Best-effort: a Redis blip must not fail the successful rotation.
	_ = s.Lim.R.Set(ctx, refreshGraceKey(oldRaw), sealed, refreshGraceTTL).Err()
}

// claimRefreshRotation serializes concurrent rotations of the same raw token
// across API instances. Losers wait briefly for the winner's grace entry.
func (s *Service) claimRefreshRotation(ctx context.Context, raw string) bool {
	if s.Lim.R == nil {
		return true
	}
	ok, err := s.Lim.R.SetNX(ctx, refreshClaimKey(raw), "1", 15*time.Second).Result()
	if err != nil {
		// Fail open: prefer a rare dual-issue over blocking every refresh.
		return true
	}
	return ok
}

func refreshClaimKey(raw string) string {
	return "auth:rtclaim:" + HashToken(raw)
}

func (s *Service) refreshClaimHeld(ctx context.Context, raw string) bool {
	if s.Lim.R == nil {
		return false
	}
	n, err := s.Lim.R.Exists(ctx, refreshClaimKey(raw)).Result()
	return err == nil && n > 0
}

// waitRefreshGrace polls for the winner's grace entry. A non-nil error means
// the entry was found but must not be handed out (e.g. the profile is banned);
// the caller propagates it instead of continuing down the rotation path.
func (s *Service) waitRefreshGrace(ctx context.Context, raw string) (Tokens, bool, error) {
	deadline := time.Now().Add(2 * time.Second)
	for {
		cached, ok, err := s.gracePair(ctx, raw)
		if err != nil {
			return Tokens{}, false, err
		}
		if ok {
			return cached, true, nil
		}
		if time.Now().After(deadline) {
			return Tokens{}, false, nil
		}
		select {
		case <-ctx.Done():
			return Tokens{}, false, nil
		case <-time.After(25 * time.Millisecond):
		}
	}
}

// Refresh rotates a refresh token: the presented raw token is single-use.
// Presenting an already-rotated (revoked) token outside the short grace
// window is treated as a compromise signal and revokes every refresh token
// belonging to that profile. Within the grace window (concurrent/late
// retries with the old cookie), the same successor pair is returned — but
// only to a profile that is still active; see gracePair.
func (s *Service) Refresh(ctx context.Context, raw string) (Tokens, error) {
	// Fast path for late/parallel callers that still hold the pre-rotation cookie.
	if cached, ok, err := s.gracePair(ctx, raw); err != nil {
		return Tokens{}, err
	} else if ok {
		return cached, nil
	}

	rt, err := s.Q.GetRefreshToken(ctx, HashToken(raw))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Tokens{}, ErrInvalidRefresh
		}
		return Tokens{}, err
	}
	if rt.RevokedAt.Valid {
		// Winner may have written grace between our miss and this Get.
		if cached, ok, err := s.gracePair(ctx, raw); err != nil {
			return Tokens{}, err
		} else if ok {
			return cached, nil
		}
		// Concurrent rotator still holding the claim — wait for its grace write.
		if s.refreshClaimHeld(ctx, raw) {
			cached, ok, err := s.waitRefreshGrace(ctx, raw)
			if err != nil {
				return Tokens{}, err
			}
			if ok {
				return cached, nil
			}
		}
		if err := s.Q.RevokeAllRefreshTokens(ctx, rt.ProfileID); err != nil {
			return Tokens{}, err
		}
		return Tokens{}, ErrReusedRefresh
	}
	if time.Now().After(rt.ExpiresAt.Time) {
		return Tokens{}, ErrInvalidRefresh
	}

	if !s.claimRefreshRotation(ctx, raw) {
		cached, ok, err := s.waitRefreshGrace(ctx, raw)
		if err != nil {
			return Tokens{}, err
		}
		if ok {
			return cached, nil
		}
		// Winner vanished without writing grace — soft-fail without revoke-all.
		return Tokens{}, ErrInvalidRefresh
	}

	profile, err := s.Q.GetProfileByID(ctx, rt.ProfileID)
	if err != nil {
		return Tokens{}, err
	}
	if err := assertProfileActive(profile); err != nil {
		return Tokens{}, err
	}
	access, err := IssueAccess(s.Secret, profile.ID, profile.Role, s.AccessTTL)
	if err != nil {
		return Tokens{}, err
	}
	newRaw, err := NewRefreshToken()
	if err != nil {
		return Tokens{}, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Tokens{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tag, err := tx.Exec(ctx, `
		UPDATE refresh_token SET revoked_at=now()
		WHERE id=$1 AND token_hash=$2 AND revoked_at IS NULL AND expires_at > now()`, rt.ID, HashToken(raw))
	if err != nil {
		return Tokens{}, err
	}
	if tag.RowsAffected() != 1 {
		cached, ok, waitErr := s.waitRefreshGrace(ctx, raw)
		if waitErr != nil {
			return Tokens{}, waitErr
		}
		if ok {
			return cached, nil
		}
		return Tokens{}, ErrInvalidRefresh
	}
	q := sqlc.New(tx)
	if err := q.CreateRefreshToken(ctx, sqlc.CreateRefreshTokenParams{
		ProfileID: profile.ID,
		TokenHash: HashToken(newRaw),
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(s.RefreshTTL), Valid: true},
	}); err != nil {
		return Tokens{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Tokens{}, err
	}

	tokens := Tokens{Access: access, Refresh: newRaw}
	s.writeRefreshGrace(ctx, raw, profile.ID, tokens)
	return tokens, nil
}

// Logout deletes the refresh token if present; missing tokens are a no-op.
func (s *Service) Logout(ctx context.Context, raw string) error {
	rt, err := s.Q.GetRefreshToken(ctx, HashToken(raw))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	return s.Q.DeleteRefreshToken(ctx, rt.ID)
}

// SignupTrialDuration is how much full access a brand-new profile receives.
// The paywall is a worse first impression than the product is, so the limits
// only start applying once the user has seen what they are paying for.
const SignupTrialDuration = 24 * time.Hour

// grantSignupTrial writes the welcome entitlement inside the caller's
// transaction rather than going through billing.GrantDays: it must commit or
// roll back together with the profile row, and a profile created moments ago
// has no existing entitlement for GrantDays' stacking logic to consider.
func grantSignupTrial(ctx context.Context, q *sqlc.Queries, profileID uuid.UUID) error {
	now := time.Now()
	_, err := q.InsertEntitlement(ctx, sqlc.InsertEntitlementParams{
		ProfileID: profileID,
		Source:    "trial",
		StartsAt:  pgtype.Timestamptz{Time: now, Valid: true},
		EndsAt:    pgtype.Timestamptz{Time: now.Add(SignupTrialDuration), Valid: true},
		Note:      "signup welcome trial",
	})
	return err
}

func createProfileWithReferral(ctx context.Context, q *sqlc.Queries, phone, passwordHash, name string) (sqlc.Profile, error) {
	const maxRetries = 5
	var passwordParam pgtype.Text
	if passwordHash != "" {
		passwordParam = pgtype.Text{String: passwordHash, Valid: true}
	}
	for i := 0; i < maxRetries; i++ {
		referralCode, err := NewReferralCode()
		if err != nil {
			return sqlc.Profile{}, err
		}
		p, err := q.CreateProfile(ctx, sqlc.CreateProfileParams{
			Phone:        phone,
			ReferralCode: pgtype.Text{String: referralCode, Valid: true},
			PasswordHash: passwordParam,
			Name:         name,
		})
		if err == nil {
			return p, nil
		}
		if isUniqueViolation(err) {
			// Phone collisions are permanent; only retry referral_code races.
			if phoneTaken(err) {
				return sqlc.Profile{}, err
			}
			continue
		}
		return sqlc.Profile{}, err
	}
	return sqlc.Profile{}, fmt.Errorf("could not generate unique referral code after %d retries", maxRetries)
}

func phoneTaken(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return false
	}
	return pgErr.ConstraintName == "profile_phone_key"
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
