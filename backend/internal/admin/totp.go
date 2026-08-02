package admin

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"

	"avtotest.uz/backend/internal/httpx"
)

const (
	totpChallengeTyp = "admin_totp_challenge"
	totpChallengeTTL = 5 * time.Minute
	// totpEnrollTyp marks the enrollment token login hands an admin that
	// ADMIN_TOTP_ENFORCE refuses a session to. It authorizes the two TOTP
	// enrollment endpoints and nothing else: ParseAccess (behind every other
	// admin route) rejects it because that demands typ=admin.
	totpEnrollTyp = "admin_totp_enroll"
	totpEnrollTTL = 15 * time.Minute
	totpIssuer    = "Driver Go Admin"
	// totpEnrollWindow bounds how long a QR code handed out by
	// BeginTOTPEnroll stays confirmable — see enrollSecret.
	totpEnrollWindow = 10 * time.Minute
)

var (
	ErrTOTPRequired = errors.New("totp required")
	ErrTOTPInvalid  = errors.New("invalid totp code")
)

// b32NoPad is the encoding otpauth URLs use for the shared secret.
var b32NoPad = base32.StdEncoding.WithPadding(base32.NoPadding)

// TOTPEnforce reports whether admins must enroll TOTP (ADMIN_TOTP_ENFORCE=1|true).
func TOTPEnforce() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("ADMIN_TOTP_ENFORCE")))
	return v == "1" || v == "true" || v == "yes"
}

func deriveKEK(secret []byte) []byte {
	sum := sha256.Sum256(secret)
	return sum[:]
}

// dataKEK is the AES-GCM key protecting admin TOTP secrets at rest. It comes
// from DATA_ENCRYPTION_KEY when the deployment sets one and falls back to the
// JWT signing secret otherwise: every secret sealed before the two keys were
// split used the JWT secret, and re-keying them would mean re-enrolling every
// admin. The fallback is also what keeps this correct if Service.DataKey has
// not been wired yet.
func (s Service) dataKEK() []byte {
	if len(s.DataKey) > 0 {
		return deriveKEK(s.DataKey)
	}
	return deriveKEK(s.Secret)
}

func encryptSecret(kek, plaintext []byte) (string, error) {
	block, err := aes.NewCipher(kek)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	out := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(out), nil
}

func decryptSecret(kek []byte, enc string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(kek)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(raw) < gcm.NonceSize() {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ct := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ct, nil)
}

// issueScopedToken mints a short-lived token that is only good for the flow
// named by typ. The typ claim is the whole point: it is what stops one of
// these from being replayed at a route that expects a different one.
func issueScopedToken(secret []byte, typ string, ttl time.Duration, adminUserID uuid.UUID, email string) (string, error) {
	now := time.Now()
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   adminUserID.String(),
		"email": email,
		"typ":   typ,
		"iat":   now.Unix(),
		"exp":   now.Add(ttl).Unix(),
		"jti":   uuid.NewString(),
	})
	return t.SignedString(secret)
}

func parseScopedToken(secret []byte, typ, token string) (Claims, error) {
	parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected alg")
		}
		return secret, nil
	})
	if err != nil || !parsed.Valid {
		return Claims{}, fmt.Errorf("invalid %s token", typ)
	}
	mc, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return Claims{}, fmt.Errorf("invalid claims")
	}
	if got, _ := mc["typ"].(string); got != typ {
		return Claims{}, fmt.Errorf("not a %s token", typ)
	}
	sub, _ := mc["sub"].(string)
	id, err := uuid.Parse(sub)
	if err != nil {
		return Claims{}, err
	}
	email, _ := mc["email"].(string)
	return Claims{AdminUserID: id, Email: email}, nil
}

// IssueTOTPChallenge mints a short-lived challenge after password OK.
func IssueTOTPChallenge(secret []byte, adminUserID uuid.UUID, email string) (string, error) {
	return issueScopedToken(secret, totpChallengeTyp, totpChallengeTTL, adminUserID, email)
}

// ParseTOTPChallenge validates a challenge token.
func ParseTOTPChallenge(secret []byte, token string) (Claims, error) {
	return parseScopedToken(secret, totpChallengeTyp, token)
}

// IssueTOTPEnroll mints the enrollment token described on totpEnrollTyp.
func IssueTOTPEnroll(secret []byte, adminUserID uuid.UUID, email string) (string, error) {
	return issueScopedToken(secret, totpEnrollTyp, totpEnrollTTL, adminUserID, email)
}

// ParseTOTPEnroll validates an enrollment token.
func ParseTOTPEnroll(secret []byte, token string) (Claims, error) {
	return parseScopedToken(secret, totpEnrollTyp, token)
}

// TOTPEnrollAuth fronts ONLY POST /security/totp/{enroll,confirm}. It accepts
// a normal admin session (an admin enrolling voluntarily) or the scoped
// enrollment token, which exists because ADMIN_TOTP_ENFORCE denies a session
// to exactly the admins who still have to enroll — leaving them, before this,
// with no in-product way to ever sign in.
//
// It grants no RBAC: no permissions are put in the context, so any handler
// that reached it behind RequirePermission would be denied. Every other admin
// route stays behind Required, whose ParseAccess rejects an enrollment token
// outright (typ mismatch).
func TOTPEnrollAuth(secret []byte, store Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok || token == "" {
				httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing bearer token")
				return
			}
			enrollOnly := false
			claims, err := ParseAccess(secret, token)
			if err != nil {
				claims, err = ParseTOTPEnroll(secret, token)
				if err != nil {
					httpx.Error(w, http.StatusUnauthorized, "unauthorized", "invalid or expired admin token")
					return
				}
				enrollOnly = true
			}
			u, err := store.GetUserByID(r.Context(), claims.AdminUserID)
			if err != nil || u.Status != "active" {
				httpx.Error(w, http.StatusUnauthorized, "unauthorized", "admin account inactive")
				return
			}
			// An enrollment token is spent the moment an authenticator exists.
			// Otherwise a stolen copy could keep swapping in a fresh one for
			// the rest of its TTL, which is a full account takeover.
			if enrollOnly && u.TOTPEnabled() {
				httpx.Error(w, http.StatusUnauthorized, "unauthorized", "totp already enrolled; sign in normally")
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), claimsKey, claims)))
		})
	}
}

// enrollWindowAt maps a moment to the enrollment window it belongs to.
func enrollWindowAt(t time.Time) int64 {
	return t.Unix() / int64(totpEnrollWindow/time.Second)
}

// enrollSecret derives the TOTP secret this server hands adminID during a
// given window, as an HMAC under the data key.
//
// The point is that ConfirmTOTPEnroll can re-derive it instead of storing what
// the client posts back: the endpoint used to persist whatever secret arrived
// in the request body, so a confirmed authenticator was never provably the one
// the server issued. Deriving needs no pending-secret column and therefore no
// migration, and the window makes a handed-out QR expire on its own — a secret
// is confirmable for at most two windows (see pendingTOTPSecret).
//
// Determinism within a window is harmless: the value is already shown to that
// admin, it is bound to their id, and anyone who could derive another admin's
// secret would need the data key — with which they could simply decrypt every
// enrolled secret instead.
func (s Service) enrollSecretBytes(adminID uuid.UUID, window int64) []byte {
	mac := hmac.New(sha256.New, s.dataKEK())
	// hash.Hash writes never fail, hence the discards (same pattern as
	// billing.humoPushFingerprint).
	_, _ = fmt.Fprintf(mac, "drivergo:admin-totp-enroll:v1:%s:%d", adminID, window)
	// 20 bytes: the 160-bit shared secret RFC 4226 recommends, and the size
	// totp.Generate produces by default.
	return mac.Sum(nil)[:20]
}

func (s Service) enrollSecret(adminID uuid.UUID, window int64) string {
	return b32NoPad.EncodeToString(s.enrollSecretBytes(adminID, window))
}

// pendingTOTPSecret returns the enrollment secret whose live code the caller
// just proved possession of. The previous window is accepted as well so a QR
// scanned seconds before a boundary still confirms.
func (s Service) pendingTOTPSecret(adminID uuid.UUID, code string) (string, bool) {
	now := enrollWindowAt(time.Now())
	for _, window := range [...]int64{now, now - 1} {
		if secret := s.enrollSecret(adminID, window); validateTOTP(secret, code) {
			return secret, true
		}
	}
	return "", false
}

// generateTOTPKey builds the otpauth key/URL. A nil secret makes
// totp.Generate mint a random one.
func generateTOTPKey(email string, secret []byte) (*otp.Key, error) {
	return totp.Generate(totp.GenerateOpts{
		Issuer:      totpIssuer,
		AccountName: email,
		Period:      30,
		Digits:      otp.DigitsSix,
		Algorithm:   otp.AlgorithmSHA1,
		Secret:      secret,
	})
}

func validateTOTP(secret, code string) bool {
	ok, err := totp.ValidateCustom(strings.TrimSpace(code), secret, time.Now().UTC(), totp.ValidateOpts{
		Period:    30,
		Skew:      1,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	return err == nil && ok
}
