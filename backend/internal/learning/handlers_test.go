package learning_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/blob"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/fixture"
	"avtotest.uz/backend/internal/importer"
	"avtotest.uz/backend/internal/learning"
	"avtotest.uz/backend/internal/testdb"
)

const handlerSecret = "test-secret"

func setupHandlerServer(t *testing.T) (*httptest.Server, string, []string) {
	t.Helper()
	pool := testdb.New(t)
	ds, images := fixture.Sample()
	if _, err := importer.Store(context.Background(), pool, blob.NewLocalDir(t.TempDir()), ds,
		importer.StoreOptions{MarkVerified: true, Images: images, Source: "fixture"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	q := sqlc.New(pool)
	profile, err := q.CreateProfile(context.Background(), sqlc.CreateProfileParams{Phone: "+998901234567"})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}
	v, err := q.GetVariantByNumber(context.Background(), 1)
	if err != nil {
		t.Fatalf("get variant: %v", err)
	}
	qids, err := q.ListVariantQuestionIDsOrdered(context.Background(), v.ID)
	if err != nil || len(qids) == 0 {
		t.Fatalf("question ids: %v", err)
	}
	tok, err := auth.IssueAccess([]byte(handlerSecret), profile.ID, "user", time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	svc := learning.NewService(q)
	r := chi.NewRouter()
	h := &learning.Handler{Svc: svc}
	h.Routes(r.With(auth.Required([]byte(handlerSecret))))

	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)
	ids := make([]string, len(qids))
	for i, id := range qids {
		ids[i] = id.String()
	}
	return ts, tok, ids
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
	req, err := http.NewRequest(method, ts.URL+path, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
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

func TestPostLearnReviewAndStats(t *testing.T) {
	ts, tok, qids := setupHandlerServer(t)

	body, _ := json.Marshal(map[string]any{"question_id": qids[0], "rating": 3})
	status, env := doReq(t, ts, http.MethodPost, "/learn/review", tok, body)
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, env.Data)
	}
	var out struct {
		Stability float64 `json:"stability"`
		Reps      int     `json:"reps"`
	}
	if err := json.Unmarshal(env.Data, &out); err != nil {
		t.Fatal(err)
	}
	if out.Reps != 1 || out.Stability <= 0 {
		t.Fatalf("unexpected review response: %+v", out)
	}

	status, env = doReq(t, ts, http.MethodPost, "/learn/review", tok, []byte(`{"question_id":"`+qids[0]+`","rating":99}`))
	if status != http.StatusBadRequest || env.Error == nil || env.Error.Code != "invalid_rating" {
		t.Fatalf("expected 400 invalid_rating, got status=%d env=%+v", status, env)
	}

	status, env = doReq(t, ts, http.MethodGet, "/me/stats", tok, nil)
	if status != http.StatusOK {
		t.Fatalf("stats status=%d", status)
	}
	var stats struct {
		ReadinessPct int `json:"readiness_pct"`
		DueCount     int `json:"due_count"`
		Categories   []struct {
			CategoryCode string  `json:"category_code"`
			Mastery      float64 `json:"mastery"`
		} `json:"categories"`
	}
	if err := json.Unmarshal(env.Data, &stats); err != nil {
		t.Fatal(err)
	}
	if len(stats.Categories) != 4 {
		t.Fatalf("expected 4 categories, got %d", len(stats.Categories))
	}
}

func TestGetMistakeBankSummaryDistinguishesDueFromTotal(t *testing.T) {
	ts, tok, qids := setupHandlerServer(t)

	// FSRS deliberately schedules an Again review at least one day ahead.
	// The bank must therefore report one total mistake but zero due now,
	// together with the next due timestamp so the UI can explain the state.
	body, _ := json.Marshal(map[string]any{"question_id": qids[0], "rating": 1})
	status, env := doReq(t, ts, http.MethodPost, "/learn/review", tok, body)
	if status != http.StatusOK {
		t.Fatalf("review status=%d error=%+v", status, env.Error)
	}

	status, env = doReq(t, ts, http.MethodGet, "/me/mistakes", tok, nil)
	if status != http.StatusOK {
		t.Fatalf("mistakes status=%d error=%+v", status, env.Error)
	}
	var summary struct {
		DueCount       int     `json:"due_count"`
		TotalBankCount int     `json:"total_bank_count"`
		NextDueAt      *string `json:"next_due_at"`
	}
	if err := json.Unmarshal(env.Data, &summary); err != nil {
		t.Fatal(err)
	}
	if summary.DueCount != 0 || summary.TotalBankCount != 1 || summary.NextDueAt == nil {
		t.Fatalf("unexpected mistake summary: %+v", summary)
	}
	if _, err := time.Parse(time.RFC3339Nano, *summary.NextDueAt); err != nil {
		t.Fatalf("next_due_at=%q is not RFC3339: %v", *summary.NextDueAt, err)
	}
	if !strings.HasSuffix(*summary.NextDueAt, "Z") {
		t.Fatalf("next_due_at=%q must be normalized to UTC", *summary.NextDueAt)
	}

	otherToken, err := auth.IssueAccess([]byte(handlerSecret), uuid.New(), "user", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	status, env = doReq(t, ts, http.MethodGet, "/me/mistakes", otherToken, nil)
	if status != http.StatusOK {
		t.Fatalf("other profile mistakes status=%d error=%+v", status, env.Error)
	}
	var otherSummary struct {
		DueCount       int        `json:"due_count"`
		TotalBankCount int        `json:"total_bank_count"`
		NextDueAt      *time.Time `json:"next_due_at"`
	}
	if err := json.Unmarshal(env.Data, &otherSummary); err != nil {
		t.Fatal(err)
	}
	if otherSummary.DueCount != 0 || otherSummary.TotalBankCount != 0 || otherSummary.NextDueAt != nil {
		t.Fatalf("another profile can see the owner's mistake bank: %+v", otherSummary)
	}
}

func TestGetMistakeBankSummaryEmptyUsesExplicitNullNextDue(t *testing.T) {
	ts, tok, _ := setupHandlerServer(t)
	status, env := doReq(t, ts, http.MethodGet, "/me/mistakes", tok, nil)
	if status != http.StatusOK {
		t.Fatalf("mistakes status=%d error=%+v", status, env.Error)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(env.Data, &fields); err != nil {
		t.Fatal(err)
	}
	if string(fields["due_count"]) != "0" || string(fields["total_bank_count"]) != "0" {
		t.Fatalf("empty counts payload=%s", env.Data)
	}
	if next, ok := fields["next_due_at"]; !ok || string(next) != "null" {
		t.Fatalf("empty next_due_at must be explicit null, payload=%s", env.Data)
	}
}

func TestLearnRoutesRequireAuth(t *testing.T) {
	ts, _, _ := setupHandlerServer(t)
	resp, err := ts.Client().Get(ts.URL + "/me/stats")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", resp.StatusCode)
	}

	resp, err = ts.Client().Get(ts.URL + "/me/mistakes")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("mistakes status=%d want 401", resp.StatusCode)
	}
}
