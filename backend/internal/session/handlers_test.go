package session_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/blob"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/fixture"
	"avtotest.uz/backend/internal/importer"
	"avtotest.uz/backend/internal/session"
	"avtotest.uz/backend/internal/testdb"
)

const handlerSecret = "test-secret"

func setupServer(t *testing.T) (*httptest.Server, string, *sqlc.Queries) {
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
	tok, err := auth.IssueAccess([]byte(handlerSecret), profile.ID, "user", time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	svc := session.NewService(q, billing.Service{Q: q})
	r := chi.NewRouter()
	h := &session.Handler{Svc: svc}
	h.Routes(r.With(auth.Required([]byte(handlerSecret))))

	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)
	return ts, tok, q
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

func TestFullVariantSessionOverHTTP(t *testing.T) {
	ts, tok, q := setupServer(t)
	v, err := q.GetVariantByNumber(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}

	body, _ := json.Marshal(map[string]any{"mode": "variant", "variant_id": v.ID, "locale": "uz-Latn"})
	status, env := doReq(t, ts, http.MethodPost, "/sessions", tok, body)
	if status != http.StatusCreated {
		t.Fatalf("create session status=%d body=%s", status, env.Data)
	}
	var created struct {
		ID          string   `json:"id"`
		QuestionIDs []string `json:"question_ids"`
		Total       int      `json:"total"`
	}
	if err := json.Unmarshal(env.Data, &created); err != nil {
		t.Fatal(err)
	}
	if created.Total != 20 || len(created.QuestionIDs) != 20 {
		t.Fatalf("expected 20 questions: %+v", created)
	}

	ansBody, _ := json.Marshal(map[string]any{"question_id": created.QuestionIDs[0], "answer_id": "00000000-0000-0000-0000-000000000000"})
	status, env = doReq(t, ts, http.MethodPost, "/sessions/"+created.ID+"/answers", tok, ansBody)
	if status != http.StatusBadRequest || env.Error == nil || env.Error.Code != "invalid_answer" {
		t.Fatalf("expected 400 invalid_answer for a made-up answer id, got status=%d env=%+v", status, env)
	}

	status, env = doReq(t, ts, http.MethodGet, "/me/variants", tok, nil)
	if status != http.StatusOK {
		t.Fatalf("me/variants status=%d", status)
	}
	var statuses []struct {
		Number   int32 `json:"number"`
		Unlocked bool  `json:"unlocked"`
	}
	if err := json.Unmarshal(env.Data, &statuses); err != nil {
		t.Fatal(err)
	}
	if !statuses[0].Unlocked || statuses[1].Unlocked {
		t.Fatalf("expected only variant 1 unlocked initially: %+v", statuses)
	}
}

func TestSessionsRequireAuth(t *testing.T) {
	ts, _, _ := setupServer(t)
	resp, err := ts.Client().Get(ts.URL + "/me/sessions")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", resp.StatusCode)
	}
}
