package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/stationctx"
)

func TestStationTokenCarriesStationID(t *testing.T) {
	secret := []byte("test-secret-that-is-long-enough-000000")
	stationID, profileID := uuid.New(), uuid.New()

	token, err := auth.IssueStationAccess(secret, stationID, profileID, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := auth.ParseAccess(secret, token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.ProfileID != profileID {
		t.Fatalf("sub=%v, want shadow profile %v", claims.ProfileID, profileID)
	}
	if claims.StationID != stationID {
		t.Fatalf("sid=%v, want %v", claims.StationID, stationID)
	}
	if claims.Role != "station" {
		t.Fatalf("role=%q, want station", claims.Role)
	}
}

func TestRequiredPutsStationOnContext(t *testing.T) {
	secret := []byte("test-secret-that-is-long-enough-000000")
	stationID, profileID := uuid.New(), uuid.New()
	token, err := auth.IssueStationAccess(secret, stationID, profileID, time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	var seen uuid.UUID
	var ok bool
	h := auth.Required(secret)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen, ok = stationctx.FromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	h.ServeHTTP(httptest.NewRecorder(), req)

	if !ok || seen != stationID {
		t.Fatalf("station on ctx = %v (ok=%v), want %v", seen, ok, stationID)
	}
}

func TestRequiredLeavesLearnerContextStationless(t *testing.T) {
	secret := []byte("test-secret-that-is-long-enough-000000")
	token, err := auth.IssueAccess(secret, uuid.New(), "user", time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	var ok bool
	h := auth.Required(secret)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, ok = stationctx.FromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	h.ServeHTTP(httptest.NewRecorder(), req)

	if ok {
		t.Fatal("a learner token must not put a station on the context")
	}
}

// TestParseAccessRejectsStationTokenWithBadSid guards against silent
// privilege confusion: a station-typed token whose sid is missing, empty, or
// unparseable must never come back as valid-looking Claims with
// StationID == uuid.Nil, because that would reach handlers as an
// authenticated principal that silently degraded into a learner.
func TestParseAccessRejectsStationTokenWithBadSid(t *testing.T) {
	secret := []byte("test-secret-that-is-long-enough-000000")

	cases := []struct {
		name    string
		haveSid bool
		sid     string
	}{
		{name: "sid absent", haveSid: false},
		{name: "sid empty string", haveSid: true, sid: ""},
		{name: "sid not a uuid", haveSid: true, sid: "not-a-uuid"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now()
			mc := jwt.MapClaims{
				"sub":  uuid.New().String(),
				"role": "station",
				"typ":  "station",
				"iat":  now.Unix(),
				"exp":  now.Add(15 * time.Minute).Unix(),
				"jti":  uuid.NewString(),
			}
			if tc.haveSid {
				mc["sid"] = tc.sid
			}
			token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, mc).SignedString(secret)
			if err != nil {
				t.Fatal(err)
			}

			claims, err := auth.ParseAccess(secret, token)
			if err == nil {
				t.Fatal("want error for malformed sid, got nil")
			}
			if claims != (auth.Claims{}) {
				t.Fatalf("claims must be zero value on error, got %+v", claims)
			}
			if claims.ProfileID != uuid.Nil || claims.StationID != uuid.Nil {
				t.Fatalf("claims must not carry any id on error, got %+v", claims)
			}
		})
	}
}
