package leaderboard_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/leaderboard"
)

const handlerSecret = "test-secret"

// setupHandlerServer mirrors the established pattern in
// internal/progress/handlers_test.go: a real httptest.Server behind the
// real auth.Required middleware and a real auth.IssueAccess token, so the
// test exercises actual auth wiring rather than injecting claims directly.
func setupHandlerServer(t *testing.T) (*httptest.Server, string, *leaderboard.Service) {
	t.Helper()
	svc, q := newTestService(t)
	profile, err := q.CreateProfile(context.Background(), sqlc.CreateProfileParams{Phone: "+998901111199"})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}
	tok, err := auth.IssueAccess([]byte(handlerSecret), profile.ID, "user", time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	r := chi.NewRouter()
	h := &leaderboard.Handler{Svc: svc}
	h.Routes(r.With(auth.Required([]byte(handlerSecret))))

	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)
	return ts, tok, svc
}

func doReq(t *testing.T, ts *httptest.Server, token, path string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, ts.URL+path, bytes.NewReader(nil))
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, buf
}

func TestGetLeaderboardRequiresAuth(t *testing.T) {
	ts, _, _ := setupHandlerServer(t)
	status, _ := doReq(t, ts, "", "/leaderboard?period=daily")
	if status != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", status, http.StatusUnauthorized)
	}
}

func TestGetLeaderboardRejectsInvalidPeriod(t *testing.T) {
	ts, tok, _ := setupHandlerServer(t)
	status, body := doReq(t, ts, tok, "/leaderboard?period=yearly")
	if status != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body: %s", status, http.StatusBadRequest, body)
	}
}

func TestGetLeaderboardReturnsShapeForEachValidPeriod(t *testing.T) {
	ts, tok, svc := setupHandlerServer(t)
	claims, err := auth.ParseAccess([]byte(handlerSecret), tok)
	if err != nil {
		t.Fatalf("parse test token: %v", err)
	}
	if err := svc.RecordPoint(context.Background(), claims.ProfileID); err != nil {
		t.Fatalf("RecordPoint: %v", err)
	}

	for _, p := range leaderboard.AllPeriods {
		status, body := doReq(t, ts, tok, "/leaderboard?period="+string(p))
		if status != http.StatusOK {
			t.Fatalf("period=%s status = %d, want 200; body: %s", p, status, body)
		}
		var resp struct {
			Data struct {
				Period string `json:"period"`
				You    struct {
					Rank  *int   `json:"rank"`
					Score int    `json:"score"`
					Name  string `json:"name"`
				} `json:"you"`
				Top []struct {
					Rank  int    `json:"rank"`
					Name  string `json:"name"`
					Score int    `json:"score"`
				} `json:"top"`
				AroundYou []struct{} `json:"around_you"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			t.Fatalf("period=%s unmarshal: %v; body: %s", p, err, body)
		}
		if resp.Data.Period != string(p) {
			t.Errorf("period=%s Data.Period = %q, want %q", p, resp.Data.Period, string(p))
		}
		if resp.Data.You.Score != 1 {
			t.Errorf("period=%s You.Score = %d, want 1", p, resp.Data.You.Score)
		}
		if len(resp.Data.Top) != 1 {
			t.Errorf("period=%s len(Top) = %d, want 1", p, len(resp.Data.Top))
		}
	}
}
