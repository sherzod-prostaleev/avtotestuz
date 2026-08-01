package events_test

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
	"github.com/google/uuid"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/events"
	"avtotest.uz/backend/internal/testdb"
)

const handlerSecret = "test-secret"

func setupHandlerServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	pool := testdb.New(t)
	q := sqlc.New(pool)
	profile, err := q.CreateProfile(context.Background(), sqlc.CreateProfileParams{Phone: "+998901234567"})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}
	tok, err := auth.IssueAccess([]byte(handlerSecret), profile.ID, "user", time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	r := chi.NewRouter()
	h := &events.Handler{Svc: events.NewService(q, pool)}
	h.Routes(r.With(auth.Required([]byte(handlerSecret))))

	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)
	return ts, tok
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

func TestLogBatchOverHTTP(t *testing.T) {
	ts, tok := setupHandlerServer(t)

	body, _ := json.Marshal(map[string]any{
		"idempotency_key": "9f444244-f00a-4a4e-a311-40ca820995a4",
		"events": []map[string]any{
			{"name": "view_question", "props": map[string]string{"question_id": "x"}},
			{"name": "session_finish"},
		},
	})
	status, respBody := doReq(t, ts, http.MethodPost, "/events", tok, body)
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, respBody)
	}

	var env struct {
		Data struct {
			OK    bool `json:"ok"`
			Count int  `json:"count"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &env); err != nil {
		t.Fatal(err)
	}
	if !env.Data.OK || env.Data.Count != 2 {
		t.Fatalf("data=%+v", env.Data)
	}
}

func TestLogBatchRejectsEmptyOverHTTP(t *testing.T) {
	ts, tok := setupHandlerServer(t)

	body, _ := json.Marshal(map[string]any{"idempotency_key": uuid.NewString(), "events": []map[string]any{}})
	status, respBody := doReq(t, ts, http.MethodPost, "/events", tok, body)
	if status != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", status, respBody)
	}
	var env struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBody, &env); err != nil {
		t.Fatal(err)
	}
	if env.Error.Code != "invalid_request" {
		t.Fatalf("error code=%q want invalid_request", env.Error.Code)
	}
}

func TestLogBatchRejectsOversizedOverHTTP(t *testing.T) {
	ts, tok := setupHandlerServer(t)

	evs := make([]map[string]any, 101)
	for i := range evs {
		evs[i] = map[string]any{"name": "x"}
	}
	body, _ := json.Marshal(map[string]any{"idempotency_key": uuid.NewString(), "events": evs})
	status, respBody := doReq(t, ts, http.MethodPost, "/events", tok, body)
	if status != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", status, respBody)
	}
	var env struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBody, &env); err != nil {
		t.Fatal(err)
	}
	if env.Error.Code != "invalid_request" {
		t.Fatalf("error code=%q want invalid_request", env.Error.Code)
	}
}

func TestLogBatchRequiresAuth(t *testing.T) {
	ts, _ := setupHandlerServer(t)

	body, _ := json.Marshal(map[string]any{"idempotency_key": uuid.NewString(), "events": []map[string]any{{"name": "x"}}})
	resp, err := ts.Client().Post(ts.URL+"/events", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", resp.StatusCode)
	}
}
