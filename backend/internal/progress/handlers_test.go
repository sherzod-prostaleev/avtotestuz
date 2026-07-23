package progress_test

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
	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/blob"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/fixture"
	"avtotest.uz/backend/internal/importer"
	"avtotest.uz/backend/internal/progress"
	"avtotest.uz/backend/internal/testdb"
)

const handlerSecret = "test-secret"

func setupHandlerServer(t *testing.T) (*httptest.Server, string, string, *pgxpool.Pool) {
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
		t.Fatalf("variant: %v", err)
	}
	qids, err := q.ListVariantQuestionIDsOrdered(context.Background(), v.ID)
	if err != nil || len(qids) == 0 {
		t.Fatalf("question ids: %v", err)
	}
	tok, err := auth.IssueAccess([]byte(handlerSecret), profile.ID, "user", time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	r := chi.NewRouter()
	h := &progress.Handler{Svc: progress.NewService(q)}
	h.Routes(r.With(auth.Required([]byte(handlerSecret))))

	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)
	return ts, tok, qids[0].String(), pool
}

func doReq(t *testing.T, ts *httptest.Server, method, path, token string, body []byte) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, ts.URL+path, bytes.NewReader(body))
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
	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, buf
}

func TestSavedQuestionsRoundtripOverHTTP(t *testing.T) {
	ts, tok, questionID, _ := setupHandlerServer(t)

	body, _ := json.Marshal(map[string]string{"question_id": questionID})
	status, _ := doReq(t, ts, http.MethodPost, "/me/saved", tok, body)
	if status != http.StatusOK {
		t.Fatalf("POST /me/saved status=%d", status)
	}

	status, respBody := doReq(t, ts, http.MethodGet, "/me/saved", tok, nil)
	if status != http.StatusOK {
		t.Fatalf("GET /me/saved status=%d", status)
	}
	var env struct {
		Data []struct {
			QuestionID string `json:"question_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Data) != 1 || env.Data[0].QuestionID != questionID {
		t.Fatalf("saved list = %+v", env.Data)
	}

	status, _ = doReq(t, ts, http.MethodDelete, "/me/saved/"+questionID, tok, nil)
	if status != http.StatusOK {
		t.Fatalf("DELETE status=%d", status)
	}
}

func TestSavedQuestionsListExcludesQuarantinedQuestions(t *testing.T) {
	ts, tok, questionID, pool := setupHandlerServer(t)

	body, _ := json.Marshal(map[string]string{"question_id": questionID})
	status, _ := doReq(t, ts, http.MethodPost, "/me/saved", tok, body)
	if status != http.StatusOK {
		t.Fatalf("POST /me/saved status=%d", status)
	}
	if _, err := pool.Exec(context.Background(),
		`UPDATE question SET validation_status = 'quarantined' WHERE id = $1`, questionID); err != nil {
		t.Fatalf("quarantine question: %v", err)
	}

	status, respBody := doReq(t, ts, http.MethodGet, "/me/saved", tok, nil)
	if status != http.StatusOK {
		t.Fatalf("GET /me/saved status=%d body=%s", status, respBody)
	}
	var env struct {
		Data []struct {
			QuestionID string `json:"question_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Data) != 0 {
		t.Fatalf("saved list contains quarantined question: %+v", env.Data)
	}
}

func TestStreakRequiresAuth(t *testing.T) {
	ts, _, _, _ := setupHandlerServer(t)
	resp, err := ts.Client().Get(ts.URL + "/me/streak")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", resp.StatusCode)
	}
}
