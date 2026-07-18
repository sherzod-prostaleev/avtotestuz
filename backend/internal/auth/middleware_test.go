package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
)

func probeHandler(secret []byte) http.Handler {
	return Required(secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := FromContext(r.Context())
		if !ok {
			http.Error(w, "no claims", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(claims.ProfileID.String()))
	}))
}

func TestRequiredRejectsMissingGarbageExpired(t *testing.T) {
	secret := []byte("test-secret")
	h := probeHandler(secret)

	cases := map[string]string{
		"missing": "",
		"garbage": "Bearer not-a-real-token",
	}
	for name, header := range cases {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if header != "" {
				req.Header.Set("Authorization", header)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", rec.Code)
			}
		})
	}

	t.Run("expired", func(t *testing.T) {
		tok, err := IssueAccess(secret, uuid.New(), "user", -time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("wrong secret", func(t *testing.T) {
		tok, err := IssueAccess([]byte("other-secret"), uuid.New(), "user", time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
	})
}

func TestRequiredAcceptsValidToken(t *testing.T) {
	secret := []byte("test-secret")
	id := uuid.New()
	tok, err := IssueAccess(secret, id, "user", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	probeHandler(secret).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != id.String() {
		t.Fatalf("body = %q, want %q", rec.Body.String(), id.String())
	}
}
