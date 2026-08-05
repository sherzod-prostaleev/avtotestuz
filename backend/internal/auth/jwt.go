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

// learnerTyp is the `typ` claim every learner access token carries. Tokens
// minted for other flows (admin sessions, admin TOTP challenge/enrollment)
// are signed with the same secret, so ParseAccess matches on this exact value
// rather than trying to enumerate the types it should refuse.
const learnerTyp = "learner"

// stationTyp is the `typ` claim on a classroom-station access token. A station
// authenticates as a machine, not a person: `sub` is its shadow profile so
// every profile-keyed subsystem keeps working, and `sid` is the station row
// that entitlement checks read.
const stationTyp = "station"

// stationRole is the `role` claim on a classroom-station access token.
const stationRole = "station"

type Claims struct {
	ProfileID uuid.UUID
	Role      string
	// StationID is set only for station tokens; uuid.Nil for learners.
	StationID uuid.UUID
}

func IssueAccess(secret []byte, profileID uuid.UUID, role string, ttl time.Duration) (string, error) {
	now := time.Now()
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  profileID.String(),
		"role": role,
		"typ":  learnerTyp,
		"iat":  now.Unix(),
		"exp":  now.Add(ttl).Unix(),
		"jti":  uuid.NewString(),
	})
	return t.SignedString(secret)
}

// IssueStationAccess mints a short-lived access token for a bound station.
func IssueStationAccess(secret []byte, stationID, profileID uuid.UUID, ttl time.Duration) (string, error) {
	now := time.Now()
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  profileID.String(),
		"sid":  stationID.String(),
		"role": stationRole,
		"typ":  stationTyp,
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
	// Only a learner or station token authenticates these routes
	// (blast-radius isolation). This is an allowlist, not a "reject admin"
	// denylist: the admin package also mints scoped tokens
	// (admin_totp_challenge, admin_totp_enroll) that are signed with the same
	// secret, so a denylist naming only "admin" let those parse here as a
	// learner whose ProfileID is really an admin_user id. RejectBanned
	// happens to refuse them today because no profile row has that id, but
	// that is an accident of the route stack, not a guarantee — every future
	// token type is refused here unless it is explicitly allowlisted.
	typ, _ := mc["typ"].(string)
	if typ != learnerTyp && typ != stationTyp {
		return Claims{}, fmt.Errorf("token type %q not accepted: only learner and station tokens are allowed here", typ)
	}
	sub, _ := mc["sub"].(string)
	id, err := uuid.Parse(sub)
	if err != nil {
		return Claims{}, fmt.Errorf("invalid sub")
	}
	role, _ := mc["role"].(string)
	out := Claims{ProfileID: id, Role: role}
	if typ == stationTyp {
		sid, _ := mc["sid"].(string)
		stationID, err := uuid.Parse(sid)
		if err != nil {
			return Claims{}, fmt.Errorf("invalid sid")
		}
		out.StationID = stationID
	}
	return out, nil
}

// NewRefreshToken returns an opaque 256-bit token (base64url, no padding).
func NewRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// HashToken is how refresh tokens are stored (sha256 hex) — never raw.
func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
