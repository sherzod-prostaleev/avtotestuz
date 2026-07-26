package admin

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

const (
	totpChallengeTyp = "admin_totp_challenge"
	totpChallengeTTL = 5 * time.Minute
	totpIssuer       = "Driver Go Admin"
)

var (
	ErrTOTPRequired = errors.New("totp required")
	ErrTOTPInvalid  = errors.New("invalid totp code")
)

// TOTPEnforce reports whether superadmin must enroll TOTP (ADMIN_TOTP_ENFORCE=1|true).
func TOTPEnforce() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("ADMIN_TOTP_ENFORCE")))
	return v == "1" || v == "true" || v == "yes"
}

func deriveKEK(secret []byte) []byte {
	sum := sha256.Sum256(secret)
	return sum[:]
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

// IssueTOTPChallenge mints a short-lived challenge after password OK.
func IssueTOTPChallenge(secret []byte, adminUserID uuid.UUID, email string) (string, error) {
	now := time.Now()
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   adminUserID.String(),
		"email": email,
		"typ":   totpChallengeTyp,
		"iat":   now.Unix(),
		"exp":   now.Add(totpChallengeTTL).Unix(),
		"jti":   uuid.NewString(),
	})
	return t.SignedString(secret)
}

// ParseTOTPChallenge validates a challenge token.
func ParseTOTPChallenge(secret []byte, token string) (Claims, error) {
	parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected alg")
		}
		return secret, nil
	})
	if err != nil || !parsed.Valid {
		return Claims{}, fmt.Errorf("invalid challenge")
	}
	mc, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return Claims{}, fmt.Errorf("invalid claims")
	}
	if typ, _ := mc["typ"].(string); typ != totpChallengeTyp {
		return Claims{}, fmt.Errorf("not a totp challenge")
	}
	sub, _ := mc["sub"].(string)
	id, err := uuid.Parse(sub)
	if err != nil {
		return Claims{}, err
	}
	email, _ := mc["email"].(string)
	return Claims{AdminUserID: id, Email: email}, nil
}

func generateTOTPKey(email string) (*otp.Key, error) {
	return totp.Generate(totp.GenerateOpts{
		Issuer:      totpIssuer,
		AccountName: email,
		Period:      30,
		Digits:      otp.DigitsSix,
		Algorithm:   otp.AlgorithmSHA1,
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
