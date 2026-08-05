package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
