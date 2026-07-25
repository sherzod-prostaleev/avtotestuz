package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

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
		"typ":  "learner",
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
	mc, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return Claims{}, fmt.Errorf("invalid claims")
	}
	// Admin JWTs must never authenticate learner routes (blast-radius isolation).
	if typ, _ := mc["typ"].(string); typ == "admin" {
		return Claims{}, fmt.Errorf("admin token not allowed")
	}
	sub, _ := mc["sub"].(string)
	id, err := uuid.Parse(sub)
	if err != nil {
		return Claims{}, fmt.Errorf("invalid sub")
	}
	role, _ := mc["role"].(string)
	return Claims{ProfileID: id, Role: role}, nil
}

// NewRefreshToken returns an opaque 256-bit token (base64url, no padding).
func NewRefreshToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// HashToken is how refresh tokens are stored (sha256 hex) — never raw.
func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
