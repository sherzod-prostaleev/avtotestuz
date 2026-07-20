package demo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/blob"
	"avtotest.uz/backend/internal/config"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/fixture"
	"avtotest.uz/backend/internal/importer"
	"avtotest.uz/backend/internal/redisx"
	"avtotest.uz/backend/internal/server"
	"avtotest.uz/backend/internal/testdb"
)

type envelope struct {
	Data  json.RawMessage `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func setupDemoServer(t *testing.T, seed bool) (*httptest.Server, *sqlc.Queries, *pgxpool.Pool) {
	t.Helper()
	pool := testdb.New(t)
	if seed {
		ds, images := fixture.Sample()
		if _, err := importer.Store(context.Background(), pool, blob.NewLocalDir(t.TempDir()), ds,
			importer.StoreOptions{MarkVerified: true, Images: images, Source: "fixture"}); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	rc := redisx.NewTest(t)
	h := server.New(config.Config{Env: "test", MediaBaseURL: "http://media.test"},
		server.Deps{Queries: sqlc.New(pool), Pool: pool, Redis: rc})
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return ts, sqlc.New(pool), pool
}

func doJSON(t *testing.T, ts *httptest.Server, method, path string, body []byte) (int, envelope) {
	t.Helper()
	req, err := http.NewRequest(method, ts.URL+path, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var env envelope
	_ = json.NewDecoder(resp.Body).Decode(&env)
	return resp.StatusCode, env
}

func TestDemoQuestionOverHTTP(t *testing.T) {
	ts, _, _ := setupDemoServer(t, true)

	status, env := doJSON(t, ts, http.MethodGet, "/api/v1/demo/question?locale=ru", nil)
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s err=%+v", status, env.Data, env.Error)
	}

	var q struct {
		ID      string `json:"id"`
		Text    string `json:"text"`
		Answers []struct {
			ID   string `json:"id"`
			Text string `json:"text"`
		} `json:"answers"`
	}
	if err := json.Unmarshal(env.Data, &q); err != nil {
		t.Fatal(err)
	}
	if q.ID == "" || len(q.Answers) == 0 {
		t.Fatalf("expected a real question with answers, got %+v", q)
	}
	if !strings.HasPrefix(q.Text, "[ОБРАЗЕЦ]") {
		t.Fatalf("expected ru fixture text, got %q", q.Text)
	}

	// Anti-cheat: assert on the raw serialized bytes, not just the struct —
	// a correctness field could exist in the JSON without a matching Go tag
	// above and this would still (wrongly) pass if we only checked the struct.
	raw := string(env.Data)
	if strings.Contains(raw, "is_correct") || strings.Contains(raw, "correct_answer") {
		t.Fatalf("correctness leaked into demo question payload: %s", raw)
	}
}

func TestDemoQuestionEmptyDBOverHTTP(t *testing.T) {
	ts, _, _ := setupDemoServer(t, false)

	status, env := doJSON(t, ts, http.MethodGet, "/api/v1/demo/question", nil)
	if status != http.StatusNotFound || env.Error == nil || env.Error.Code != "not_found" {
		t.Fatalf("status=%d env=%+v want 404 not_found", status, env)
	}
}

func TestDemoAnswerWhitelistEnforcementOverHTTP(t *testing.T) {
	ts, q, _ := setupDemoServer(t, true)
	ctx := context.Background()

	v, err := q.GetVariantByNumber(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := q.ListVariantQuestionIDsOrdered(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) <= 2 {
		t.Fatal("fixture must have more than 2 questions in variant 1")
	}
	outsideID := ids[2]
	correctID, err := q.GetCorrectAnswerID(ctx, outsideID)
	if err != nil {
		t.Fatal(err)
	}

	body, _ := json.Marshal(map[string]any{"question_id": outsideID.String(), "answer_id": correctID.String()})
	status, env := doJSON(t, ts, http.MethodPost, "/api/v1/demo/answer", body)
	if status != http.StatusNotFound || env.Error == nil || env.Error.Code != "not_found" {
		t.Fatalf("status=%d env=%+v want 404 not_found for a non-whitelisted (but real) question", status, env)
	}
}

func TestDemoAnswerInvalidAnswerOverHTTP(t *testing.T) {
	ts, q, _ := setupDemoServer(t, true)
	ctx := context.Background()

	v, err := q.GetVariantByNumber(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := q.ListVariantQuestionIDsOrdered(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}

	body, _ := json.Marshal(map[string]any{"question_id": ids[0].String(), "answer_id": uuid.New().String()})
	status, env := doJSON(t, ts, http.MethodPost, "/api/v1/demo/answer", body)
	if status != http.StatusBadRequest || env.Error == nil || env.Error.Code != "invalid_answer" {
		t.Fatalf("status=%d env=%+v want 400 invalid_answer", status, env)
	}
}

func TestDemoAnswerGradingOverHTTP(t *testing.T) {
	ts, q, _ := setupDemoServer(t, true)
	ctx := context.Background()

	v, err := q.GetVariantByNumber(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := q.ListVariantQuestionIDsOrdered(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	qid := ids[0]
	correctID, err := q.GetCorrectAnswerID(ctx, qid)
	if err != nil {
		t.Fatal(err)
	}

	body, _ := json.Marshal(map[string]any{"question_id": qid.String(), "answer_id": correctID.String()})
	status, env := doJSON(t, ts, http.MethodPost, "/api/v1/demo/answer", body)
	if status != http.StatusOK {
		t.Fatalf("status=%d env=%+v", status, env)
	}
	var res struct {
		Correct         bool   `json:"correct"`
		CorrectAnswerID string `json:"correct_answer_id"`
	}
	if err := json.Unmarshal(env.Data, &res); err != nil {
		t.Fatal(err)
	}
	if !res.Correct || res.CorrectAnswerID != correctID.String() {
		t.Fatalf("res=%+v want correct=true correct_answer_id=%s", res, correctID)
	}
}

func TestDemoAnswerRequiresBothWhitelistAndOwnership(t *testing.T) {
	// Not in brief as a separate scenario, but guards the two checks aren't
	// accidentally coupled: an answer_id that belongs to a *different*
	// whitelisted question must still be rejected as invalid_answer, not
	// silently graded against the wrong question.
	ts, q, _ := setupDemoServer(t, true)
	ctx := context.Background()

	v, err := q.GetVariantByNumber(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := q.ListVariantQuestionIDsOrdered(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	otherCorrectID, err := q.GetCorrectAnswerID(ctx, ids[1])
	if err != nil {
		t.Fatal(err)
	}

	body, _ := json.Marshal(map[string]any{"question_id": ids[0].String(), "answer_id": otherCorrectID.String()})
	status, env := doJSON(t, ts, http.MethodPost, "/api/v1/demo/answer", body)
	if status != http.StatusBadRequest || env.Error == nil || env.Error.Code != "invalid_answer" {
		t.Fatalf("status=%d env=%+v want 400 invalid_answer for cross-question answer_id", status, env)
	}
}
