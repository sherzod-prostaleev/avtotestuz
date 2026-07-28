package auth

import (
	"context"
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

	code := GenerateCode()
	_, err = s.Q.CreateOTPChallenge(ctx, sqlc.CreateOTPChallengeParams{
		Phone:     phone,
		CodeHash:  HashCode(code),
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
	refresh := NewRefreshToken()
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

// Register creates a profile with a password hash and issues a session.
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

type SetPasswordInput struct {
	Phone    string
	Password string
	IP       string
}

// SetPassword is a one-shot bootstrap for OTP-era accounts with a NULL hash.
// It refuses once a password is already present.
func (s *Service) SetPassword(ctx context.Context, in SetPasswordInput) (VerifyResult, error) {
	phone, err := NormalizePhone(in.Phone)
	if err != nil {
		return VerifyResult{}, ErrInvalidPhone
	}
	if err := s.rateLimitAuth(ctx, "set-password", phone, in.IP); err != nil {
		return VerifyResult{}, err
	}

	hash, err := HashPassword(in.Password)
	if err != nil {
		return VerifyResult{}, err
	}

	profile, err := s.Q.GetProfileByPhone(ctx, phone)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return VerifyResult{}, ErrInvalidCreds
		}
		return VerifyResult{}, err
	}
	if profile.PasswordHash.Valid && profile.PasswordHash.String != "" {
		return VerifyResult{}, ErrPasswordSet
	}

	n, err := s.Q.SetPasswordHashIfNull(ctx, sqlc.SetPasswordHashIfNullParams{
		Phone:        phone,
		PasswordHash: pgtype.Text{String: hash, Valid: true},
	})
	if err != nil {
		return VerifyResult{}, err
	}
	if n == 0 {
		return VerifyResult{}, ErrPasswordSet
	}

	profile.PasswordHash = pgtype.Text{String: hash, Valid: true}

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
	refresh := NewRefreshToken()
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

type storedRefreshPair struct {
	Access  string `json:"a"`
	Refresh string `json:"r"`
}

func (s *Service) readRefreshGrace(ctx context.Context, raw string) (Tokens, bool) {
	if s.Lim.R == nil {
		return Tokens{}, false
	}
	val, err := s.Lim.R.Get(ctx, refreshGraceKey(raw)).Result()
	if err != nil {
		return Tokens{}, false
	}
	var pair storedRefreshPair
	if err := json.Unmarshal([]byte(val), &pair); err != nil || pair.Access == "" || pair.Refresh == "" {
		return Tokens{}, false
	}
	return Tokens(pair), true
}

func (s *Service) writeRefreshGrace(ctx context.Context, oldRaw string, tokens Tokens) {
	if s.Lim.R == nil {
		return
	}
	payload, err := json.Marshal(storedRefreshPair(tokens))
	if err != nil {
		return
	}
	// Best-effort: a Redis blip must not fail the successful rotation.
	_ = s.Lim.R.Set(ctx, refreshGraceKey(oldRaw), payload, refreshGraceTTL).Err()
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

func (s *Service) waitRefreshGrace(ctx context.Context, raw string) (Tokens, bool) {
	deadline := time.Now().Add(2 * time.Second)
	for {
		if cached, ok := s.readRefreshGrace(ctx, raw); ok {
			return cached, true
		}
		if time.Now().After(deadline) {
			return Tokens{}, false
		}
		select {
		case <-ctx.Done():
			return Tokens{}, false
		case <-time.After(25 * time.Millisecond):
		}
	}
}

// Refresh rotates a refresh token: the presented raw token is single-use.
// Presenting an already-rotated (revoked) token outside the short grace
// window is treated as a compromise signal and revokes every refresh token
// belonging to that profile. Within the grace window (concurrent/late
// retries with the old cookie), the same successor pair is returned.
func (s *Service) Refresh(ctx context.Context, raw string) (Tokens, error) {
	// Fast path for late/parallel callers that still hold the pre-rotation cookie.
	if cached, ok := s.readRefreshGrace(ctx, raw); ok {
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
		if cached, ok := s.readRefreshGrace(ctx, raw); ok {
			return cached, nil
		}
		// Concurrent rotator still holding the claim — wait for its grace write.
		if s.refreshClaimHeld(ctx, raw) {
			if cached, ok := s.waitRefreshGrace(ctx, raw); ok {
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
		if cached, ok := s.waitRefreshGrace(ctx, raw); ok {
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
	if err := s.Q.RevokeRefreshToken(ctx, rt.ID); err != nil {
		return Tokens{}, err
	}

	access, err := IssueAccess(s.Secret, profile.ID, profile.Role, s.AccessTTL)
	if err != nil {
		return Tokens{}, err
	}
	newRaw := NewRefreshToken()
	if err := s.Q.CreateRefreshToken(ctx, sqlc.CreateRefreshTokenParams{
		ProfileID: profile.ID,
		TokenHash: HashToken(newRaw),
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(s.RefreshTTL), Valid: true},
	}); err != nil {
		return Tokens{}, err
	}

	tokens := Tokens{Access: access, Refresh: newRaw}
	s.writeRefreshGrace(ctx, raw, tokens)
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
		p, err := q.CreateProfile(ctx, sqlc.CreateProfileParams{
			Phone:        phone,
			ReferralCode: pgtype.Text{String: NewReferralCode(), Valid: true},
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
