package admin

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/auth"
)

var (
	ErrInvalidCreds = errors.New("invalid email or password")
	ErrDisabled     = errors.New("admin account disabled")
	// ErrLoginThrottled is returned when an account or source IP has spent
	// its attempt budget. Deliberately distinct from ErrInvalidCreds so the
	// handler can answer 429 instead of feeding a guesser a clean signal.
	ErrLoginThrottled = errors.New("too many login attempts")
	// ErrTOTPSetupRequired is returned when ADMIN_TOTP_ENFORCE is on and the
	// account has no authenticator enrolled. Enforcement used to be a
	// cosmetic flag on /me while login still handed out a full token pair.
	ErrTOTPSetupRequired = errors.New("totp enrollment required")
)

// Service handles staff login/session lifecycle.
type Service struct {
	Store  Store
	Secret []byte
	// Lim throttles password guessing. Nil disables throttling, which is
	// only appropriate in tests that are not exercising it.
	Lim auth.Limiter
}

const (
	// adminLoginIPWindow/Limit bound how fast one source can try accounts.
	adminLoginIPWindow = time.Hour
	adminLoginIPLimit  = 60
	// adminLoginFailWindow/Limit lock a single account after a run of wrong
	// passwords. Sized so a human who mistypes a few times is unaffected,
	// while an online guesser is stopped long before bcrypt cost alone would.
	adminLoginFailWindow = 15 * time.Minute
	adminLoginFailLimit  = 10
)

func adminLoginFailKey(email string) string {
	return "admin:login:fail:" + strings.ToLower(strings.TrimSpace(email))
}

// TokenPair is returned on login/refresh.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

// MeResponse is GET /admin/v1/me.
type MeResponse struct {
	ID                uuid.UUID `json:"id"`
	Email             string    `json:"email"`
	DisplayName       string    `json:"display_name"`
	Roles             []string  `json:"roles"`
	Permissions       []string  `json:"permissions"`
	TOTPEnabled       bool      `json:"totp_enabled"`
	TOTPSetupRequired bool      `json:"totp_setup_required"`
}

// LoginResult is returned by Login — either tokens or a TOTP challenge.
type LoginResult struct {
	RequiresTOTP   bool
	ChallengeToken string
	Pair           TokenPair
	Me             MeResponse
}

func (s Service) Login(ctx context.Context, email, password, ua string, ip *net.IP) (LoginResult, error) {
	failKey := adminLoginFailKey(email)
	throttled := s.Lim.R != nil

	// Check the lockout before doing any work: bcrypt at cost 12 is the only
	// thing that used to slow a guesser down, and it slowed the server too.
	if throttled {
		n, err := s.Lim.Count(ctx, failKey)
		if err != nil {
			return LoginResult{}, err
		}
		if n >= adminLoginFailLimit {
			return LoginResult{}, ErrLoginThrottled
		}
		if ip != nil {
			ok, err := s.Lim.Allow(ctx, "admin:login:ip:"+ip.String(), adminLoginIPLimit, adminLoginIPWindow)
			if err != nil {
				return LoginResult{}, err
			}
			if !ok {
				return LoginResult{}, ErrLoginThrottled
			}
		}
	}

	countFailure := func() {
		if throttled {
			_, _ = s.Lim.Allow(ctx, failKey, adminLoginFailLimit, adminLoginFailWindow)
		}
	}

	u, err := s.Store.GetUserByEmail(ctx, email)
	if err != nil {
		if IsNoRows(err) {
			// Counted too: without this an attacker could enumerate which
			// identifiers exist by watching only real accounts lock out.
			countFailure()
			return LoginResult{}, ErrInvalidCreds
		}
		return LoginResult{}, err
	}
	if u.Status != "active" {
		return LoginResult{}, ErrDisabled
	}
	if !auth.CheckPassword(u.PasswordHash, password) {
		countFailure()
		return LoginResult{}, ErrInvalidCreds
	}
	if throttled {
		_ = s.Lim.Reset(ctx, failKey)
	}
	// Enforcement must block the token pair, not just decorate /me: an
	// account without a second factor is exactly the one worth protecting.
	if TOTPEnforce() && !u.TOTPEnabled() {
		return LoginResult{}, ErrTOTPSetupRequired
	}
	if u.TOTPEnabled() {
		ch, err := IssueTOTPChallenge(s.Secret, u.ID, u.Email)
		if err != nil {
			return LoginResult{}, err
		}
		return LoginResult{RequiresTOTP: true, ChallengeToken: ch}, nil
	}
	pair, err := s.issuePair(ctx, u, ua, ip)
	if err != nil {
		return LoginResult{}, err
	}
	me, err := s.meFromUser(ctx, u)
	if err != nil {
		return LoginResult{}, err
	}
	return LoginResult{Pair: pair, Me: me}, nil
}

// VerifyTOTPLogin completes login after password + TOTP challenge.
func (s Service) VerifyTOTPLogin(ctx context.Context, challengeToken, code, ua string, ip *net.IP) (TokenPair, MeResponse, error) {
	claims, err := ParseTOTPChallenge(s.Secret, challengeToken)
	if err != nil {
		return TokenPair{}, MeResponse{}, ErrInvalidCreds
	}
	u, err := s.Store.GetUserByID(ctx, claims.AdminUserID)
	if err != nil || u.Status != "active" {
		return TokenPair{}, MeResponse{}, ErrInvalidCreds
	}
	if !u.TOTPEnabled() {
		return TokenPair{}, MeResponse{}, ErrTOTPInvalid
	}
	plain, err := decryptSecret(deriveKEK(s.Secret), u.TotpSecretEnc)
	if err != nil {
		return TokenPair{}, MeResponse{}, ErrTOTPInvalid
	}
	if !validateTOTP(string(plain), code) {
		return TokenPair{}, MeResponse{}, ErrTOTPInvalid
	}
	pair, err := s.issuePair(ctx, u, ua, ip)
	if err != nil {
		return TokenPair{}, MeResponse{}, err
	}
	me, err := s.meFromUser(ctx, u)
	return pair, me, err
}

// BeginTOTPEnroll returns a one-time plaintext secret + otpauth URL (not yet persisted).
func (s Service) BeginTOTPEnroll(ctx context.Context, adminID uuid.UUID) (secret, otpauth string, err error) {
	u, err := s.Store.GetUserByID(ctx, adminID)
	if err != nil {
		return "", "", err
	}
	key, err := generateTOTPKey(u.Email)
	if err != nil {
		return "", "", err
	}
	return key.Secret(), key.URL(), nil
}

// ConfirmTOTPEnroll verifies code against secret and stores encrypted secret.
func (s Service) ConfirmTOTPEnroll(ctx context.Context, adminID uuid.UUID, secret, code string) error {
	secret = strings.TrimSpace(secret)
	if secret == "" || !validateTOTP(secret, code) {
		return ErrTOTPInvalid
	}
	enc, err := encryptSecret(deriveKEK(s.Secret), []byte(secret))
	if err != nil {
		return err
	}
	return s.Store.SetUserTOTPSecret(ctx, adminID, enc)
}

// DisableTOTP clears enrolled TOTP after verifying a current code.
func (s Service) DisableTOTP(ctx context.Context, adminID uuid.UUID, code string) error {
	u, err := s.Store.GetUserByID(ctx, adminID)
	if err != nil {
		return err
	}
	if !u.TOTPEnabled() {
		return nil
	}
	plain, err := decryptSecret(deriveKEK(s.Secret), u.TotpSecretEnc)
	if err != nil {
		return ErrTOTPInvalid
	}
	if !validateTOTP(string(plain), code) {
		return ErrTOTPInvalid
	}
	return s.Store.SetUserTOTPSecret(ctx, adminID, "")
}

func (s Service) Refresh(ctx context.Context, refreshRaw, ua string, ip *net.IP) (TokenPair, error) {
	hash := HashToken(refreshRaw)
	sess, err := s.Store.FindSessionByRefreshHash(ctx, hash)
	if err != nil {
		return TokenPair{}, ErrInvalidCreds
	}
	if time.Now().After(sess.ExpiresAt) {
		_ = s.Store.RevokeSession(ctx, sess.ID)
		return TokenPair{}, ErrInvalidCreds
	}
	u, err := s.Store.GetUserByID(ctx, sess.UserID)
	if err != nil || u.Status != "active" {
		return TokenPair{}, ErrDisabled
	}
	newRaw := NewRefreshToken()
	newHash := HashToken(newRaw)
	expires := time.Now().Add(refreshTTL)
	if err := s.Store.RotateSession(ctx, sess.ID, newHash, expires); err != nil {
		return TokenPair{}, err
	}
	access, err := IssueAccess(s.Secret, u.ID, u.Email, accessTTL)
	if err != nil {
		return TokenPair{}, err
	}
	_ = ua
	_ = ip
	return TokenPair{
		AccessToken:  access,
		RefreshToken: newRaw,
		ExpiresIn:    int(accessTTL.Seconds()),
	}, nil
}

func (s Service) Logout(ctx context.Context, refreshRaw string) error {
	if strings.TrimSpace(refreshRaw) == "" {
		return nil
	}
	sess, err := s.Store.FindSessionByRefreshHash(ctx, HashToken(refreshRaw))
	if err != nil {
		return nil
	}
	return s.Store.RevokeSession(ctx, sess.ID)
}

func (s Service) Me(ctx context.Context, id uuid.UUID) (MeResponse, error) {
	u, err := s.Store.GetUserByID(ctx, id)
	if err != nil {
		return MeResponse{}, err
	}
	return s.meFromUser(ctx, u)
}

func (s Service) meFromUser(ctx context.Context, u User) (MeResponse, error) {
	roles, err := s.Store.ListRoles(ctx, u.ID)
	if err != nil {
		return MeResponse{}, err
	}
	perms, err := s.Store.ListPermissions(ctx, u.ID)
	if err != nil {
		return MeResponse{}, err
	}
	if roles == nil {
		roles = []string{}
	}
	if perms == nil {
		perms = []string{}
	}
	setupRequired := false
	if TOTPEnforce() && !u.TOTPEnabled() {
		for _, r := range roles {
			if r == "superadmin" {
				setupRequired = true
				break
			}
		}
	}
	return MeResponse{
		ID:                u.ID,
		Email:             u.Email,
		DisplayName:       u.DisplayName,
		Roles:             roles,
		Permissions:       perms,
		TOTPEnabled:       u.TOTPEnabled(),
		TOTPSetupRequired: setupRequired,
	}, nil
}

func (s Service) issuePair(ctx context.Context, u User, ua string, ip *net.IP) (TokenPair, error) {
	raw := NewRefreshToken()
	hash := HashToken(raw)
	expires := time.Now().Add(refreshTTL)
	if _, err := s.Store.CreateSession(ctx, u.ID, hash, ua, ip, expires); err != nil {
		return TokenPair{}, err
	}
	access, err := IssueAccess(s.Secret, u.ID, u.Email, accessTTL)
	if err != nil {
		return TokenPair{}, err
	}
	return TokenPair{
		AccessToken:  access,
		RefreshToken: raw,
		ExpiresIn:    int(accessTTL.Seconds()),
	}, nil
}
