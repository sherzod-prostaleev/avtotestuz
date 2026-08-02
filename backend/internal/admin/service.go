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
)

// Service handles staff login/session lifecycle.
type Service struct {
	Store Store
	// Secret signs admin JWTs — access tokens, TOTP challenges and TOTP
	// enrollment tokens. Signing only: rotating it must not cost data.
	Secret []byte
	// DataKey derives the AES-GCM key that seals admin TOTP secrets at rest
	// (DATA_ENCRYPTION_KEY). Empty falls back to Secret, which is how every
	// already-stored secret was sealed — see dataKEK.
	DataKey []byte
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

// LoginResult is returned by Login — tokens, a TOTP challenge, or (when
// enforcement is on and this account has no authenticator) an enrollment
// token and no session at all.
type LoginResult struct {
	RequiresTOTP   bool
	ChallengeToken string
	// RequiresTOTPSetup means ADMIN_TOTP_ENFORCE denied a session because the
	// account has no authenticator. Pair is deliberately zero; EnrollToken is
	// the only thing issued, and it opens nothing but TOTP enrollment.
	RequiresTOTPSetup bool
	EnrollToken       string
	Pair              TokenPair
	Me                MeResponse
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
	//
	// It must not brick the account either. The enrollment endpoints sit
	// behind an admin session, so refusing outright left every admin created
	// after ADMIN_TOTP_ENFORCE=true (and every non-superadmin, whose /me never
	// even reported the requirement) permanently locked out with no in-product
	// recovery. Issue a token that authorizes enrollment and nothing else.
	if TOTPEnforce() && !u.TOTPEnabled() {
		enroll, err := IssueTOTPEnroll(s.Secret, u.ID, u.Email)
		if err != nil {
			return LoginResult{}, err
		}
		me, err := s.meFromUser(ctx, u)
		if err != nil {
			return LoginResult{}, err
		}
		return LoginResult{RequiresTOTPSetup: true, EnrollToken: enroll, Me: me}, nil
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
	plain, err := decryptSecret(s.dataKEK(), u.TotpSecretEnc)
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

// BeginTOTPEnroll returns the secret this server derives for adminID in the
// current enrollment window, plus its otpauth URL. Nothing is persisted yet —
// ConfirmTOTPEnroll re-derives the same value rather than taking the client's
// word for it, so the secret needs no server-side storage between the calls.
func (s Service) BeginTOTPEnroll(ctx context.Context, adminID uuid.UUID) (secret, otpauth string, err error) {
	u, err := s.Store.GetUserByID(ctx, adminID)
	if err != nil {
		return "", "", err
	}
	key, err := generateTOTPKey(u.Email, s.enrollSecretBytes(u.ID, enrollWindowAt(time.Now())))
	if err != nil {
		return "", "", err
	}
	return key.Secret(), key.URL(), nil
}

// ConfirmTOTPEnroll stores the pending secret once the admin proves possession
// of it with a live code.
//
// The secret is re-derived here (see enrollSecret), never read from the
// request: the endpoint used to persist whatever secret the caller posted, so
// the enrolled authenticator was not provably the one this server issued —
// anyone able to reach the endpoint could bind a secret of their own choosing.
func (s Service) ConfirmTOTPEnroll(ctx context.Context, adminID uuid.UUID, code string) error {
	u, err := s.Store.GetUserByID(ctx, adminID)
	if err != nil {
		return err
	}
	secret, ok := s.pendingTOTPSecret(u.ID, code)
	if !ok {
		return ErrTOTPInvalid
	}
	enc, err := encryptSecret(s.dataKEK(), []byte(secret))
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
	plain, err := decryptSecret(s.dataKEK(), u.TotpSecretEnc)
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
	access, err := IssueAccess(s.Secret, u.ID, u.Email, accessTTL)
	if err != nil {
		return TokenPair{}, err
	}
	newRaw, err := NewRefreshToken()
	if err != nil {
		return TokenPair{}, err
	}
	newHash := HashToken(newRaw)
	expires := time.Now().Add(refreshTTL)
	rotated, err := s.Store.RotateSession(ctx, sess.ID, hash, newHash, expires)
	if err != nil {
		return TokenPair{}, err
	}
	if !rotated {
		// Another request already consumed this one-time refresh token.
		return TokenPair{}, ErrInvalidCreds
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
	// Every role, not just superadmin. Login refuses a session to any admin
	// without an authenticator while enforcement is on, so reporting the
	// requirement to only one role left the others staring at a sign-in that
	// failed for no stated reason — and at an admin UI that never surfaced the
	// enrollment screen.
	setupRequired := TOTPEnforce() && !u.TOTPEnabled()
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
	raw, err := NewRefreshToken()
	if err != nil {
		return TokenPair{}, err
	}
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
