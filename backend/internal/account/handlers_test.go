package account_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/account"
	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

const testSecret = "test-secret"

// setup returns the pool alongside the server/profile so tests that need to
// seed data sqlc has no query for yet (e.g. tariff/tariff_translation rows,
// see payments_test.go) can do so directly, without opening a second pool
// (testdb.New truncates on every call, which would wipe the profile/rows
// already created via the first pool).
func setup(t *testing.T) (*httptest.Server, sqlc.Profile, *pgxpool.Pool) {
	t.Helper()
	pool := testdb.New(t)
	q := sqlc.New(pool)
	profile, err := q.CreateProfile(context.Background(), sqlc.CreateProfileParams{
		Phone:        "+998901234567",
		ReferralCode: pgtype.Text{String: "ABCD1234", Valid: true},
	})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}

	r := chi.NewRouter()
	h := &account.Handler{Q: q, Billing: billing.Service{Q: q}}
	h.Routes(r.With(auth.Required([]byte(testSecret))))

	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)
	return ts, profile, pool
}

type respEnvelope struct {
	Data  json.RawMessage `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func doReq(t *testing.T, ts *httptest.Server, method, path, token string, body []byte) (int, respEnvelope) {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	} else {
		reader = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, ts.URL+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var env respEnvelope
	_ = json.NewDecoder(resp.Body).Decode(&env)
	return resp.StatusCode, env
}

func TestMeRequiresAuth(t *testing.T) {
	ts, _, _ := setup(t)
	status, _ := doReq(t, ts, http.MethodGet, "/me", "", nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", status)
	}
}

func TestMeGetPatchRoundtrip(t *testing.T) {
	ts, profile, _ := setup(t)
	tok, err := auth.IssueAccess([]byte(testSecret), profile.ID, "user", time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	status, env := doReq(t, ts, http.MethodGet, "/me", tok, nil)
	if status != http.StatusOK {
		t.Fatalf("get /me status=%d", status)
	}
	var meOut struct {
		Profile struct {
			Name         string `json:"name"`
			ReferralCode string `json:"referral_code"`
		} `json:"profile"`
		VIP struct {
			Active bool `json:"active"`
		} `json:"vip"`
	}
	if err := json.Unmarshal(env.Data, &meOut); err != nil {
		t.Fatal(err)
	}
	if meOut.VIP.Active {
		t.Fatal("fresh profile should not be VIP")
	}
	if meOut.Profile.ReferralCode != "ABCD1234" {
		t.Fatalf("referral_code = %q", meOut.Profile.ReferralCode)
	}

	body, _ := json.Marshal(map[string]any{"name": "Alisher", "birth_date": "2000-05-10"})
	status, env = doReq(t, ts, http.MethodPatch, "/me", tok, body)
	if status != http.StatusOK {
		t.Fatalf("patch /me status=%d", status)
	}
	var patched struct {
		Name      string  `json:"name"`
		BirthDate *string `json:"birth_date"`
	}
	if err := json.Unmarshal(env.Data, &patched); err != nil {
		t.Fatal(err)
	}
	if patched.Name != "Alisher" {
		t.Fatalf("name = %q, want Alisher", patched.Name)
	}
	if patched.BirthDate == nil || *patched.BirthDate != "2000-05-10" {
		t.Fatalf("birth_date = %v, want 2000-05-10", patched.BirthDate)
	}

	status, env = doReq(t, ts, http.MethodGet, "/me/entitlement", tok, nil)
	if status != http.StatusOK {
		t.Fatalf("get /me/entitlement status=%d", status)
	}
	var ent struct {
		Active bool `json:"active"`
	}
	if err := json.Unmarshal(env.Data, &ent); err != nil {
		t.Fatal(err)
	}
	if ent.Active {
		t.Fatal("expected inactive entitlement")
	}
}

func TestPatchMeInvalidBirthDate(t *testing.T) {
	ts, profile, _ := setup(t)
	tok, err := auth.IssueAccess([]byte(testSecret), profile.ID, "user", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{"birth_date": "not-a-date"})
	status, _ := doReq(t, ts, http.MethodPatch, "/me", tok, body)
	if status != http.StatusBadRequest {
		t.Fatalf("status=%d want 400", status)
	}
}
