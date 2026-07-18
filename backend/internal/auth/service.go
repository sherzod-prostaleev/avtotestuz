package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

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
)

const maxOTPAttempts = 5

// Service owns the OTP request/verify lifecycle and session issuance.
type Service struct {
	Q      *sqlc.Queries
	Pool   *pgxpool.Pool
	Lim    Limiter
	Sender Sender
	Secret []byte
	Env    string

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
	DebugCode string // set only when Env=="dev" and channel=="sandbox"
}

// RequestOTP enforces cooldown/rate limits, then generates and sends a code.
func (s *Service) RequestOTP(ctx context.Context, rawPhone, ip string) (OTPRequestResult, error) {
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
	if s.Env == "dev" && s.Sender.Channel() == "sandbox" {
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
		profile, err = createProfileWithReferral(ctx, q, phone)
		if err != nil {
			return VerifyResult{}, err
		}
		created = true
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

func createProfileWithReferral(ctx context.Context, q *sqlc.Queries, phone string) (sqlc.Profile, error) {
	const maxRetries = 5
	for i := 0; i < maxRetries; i++ {
		p, err := q.CreateProfile(ctx, sqlc.CreateProfileParams{
			Phone:        phone,
			ReferralCode: pgtype.Text{String: NewReferralCode(), Valid: true},
		})
		if err == nil {
			return p, nil
		}
		if isUniqueViolation(err) {
			continue
		}
		return sqlc.Profile{}, err
	}
	return sqlc.Profile{}, fmt.Errorf("could not generate unique referral code after %d retries", maxRetries)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
