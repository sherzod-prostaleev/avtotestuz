package admin

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"avtotest.uz/backend/internal/auth"
)

// Claims are staff JWT claims. Typ must be "admin".
type Claims struct {
	AdminUserID uuid.UUID
	Email       string
}

// IssueAccess mints a short-lived admin access token.
func IssueAccess(secret []byte, adminUserID uuid.UUID, email string, ttl time.Duration) (string, error) {
	now := time.Now()
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   adminUserID.String(),
		"email": email,
		"typ":   tokenTyp,
		"iat":   now.Unix(),
		"exp":   now.Add(ttl).Unix(),
		"jti":   uuid.NewString(),
	})
	return t.SignedString(secret)
}

// ParseAccess validates an admin access token (rejects learner JWTs).
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
	typ, _ := mc["typ"].(string)
	if typ != tokenTyp {
		return Claims{}, fmt.Errorf("not an admin token")
	}
	sub, _ := mc["sub"].(string)
	id, err := uuid.Parse(sub)
	if err != nil {
		return Claims{}, fmt.Errorf("invalid sub")
	}
	email, _ := mc["email"].(string)
	return Claims{AdminUserID: id, Email: email}, nil
}

// NewRefreshToken / HashToken reuse learner crypto helpers.
func NewRefreshToken() string { return auth.NewRefreshToken() }
func HashToken(raw string) string { return auth.HashToken(raw) }
