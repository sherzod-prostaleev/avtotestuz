package push

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
	"avtotest.uz/backend/internal/testdb"
)

const handlerTestSecret = "test-secret-for-push-handlers-32b"

func setupPushServer(t *testing.T, configured bool) (*httptest.Server, string, *Service) {
	t.Helper()
	pool := testdb.New(t)
	q := sqlc.New(pool)
	profile, err := q.CreateProfile(context.Background(), sqlc.CreateProfileParams{Phone: "+998901150001"})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}
	tok, err := auth.IssueAccess([]byte(handlerTestSecret), profile.ID, "user", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{}
	if configured {
		cfg = Config{PublicKey: "pub", PrivateKey: "priv", Subject: "mailto:t@example.com"}
	}
	fake := &FakeSender{}
	svc := NewService(pool, q, cfg, fake)
	r := chi.NewRouter()
	h := &Handler{Svc: svc}
	h.AuthedRoutes(r.With(auth.Required([]byte(handlerTestSecret))))
	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)
	return ts, tok, svc
}

func doJSON(t *testing.T, ts *httptest.Server, method, path, token string, body any) (int, []byte) {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, ts.URL+path, rdr)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
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

func TestPushStatusRequiresAuth(t *testing.T) {
	ts, _, _ := setupPushServer(t, true)
	status, _ := doJSON(t, ts, http.MethodGet, "/me/push", "", nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("status = %d", status)
	}
}

func TestSubscribeUnconfigured(t *testing.T) {
	ts, tok, _ := setupPushServer(t, false)
	status, body := doJSON(t, ts, http.MethodPost, "/me/push/subscribe", tok, map[string]any{
		"endpoint": "https://push.example/x",
		"keys":     map[string]string{"p256dh": "p", "auth": "a"},
	})
	if status != http.StatusServiceUnavailable {
		t.Fatalf("status = %d body=%s", status, body)
	}
	var env struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatal(err)
	}
	if env.Error.Code != "web_push_unconfigured" {
		t.Fatalf("code = %q", env.Error.Code)
	}
}

func TestSubscribeAndGetStatus(t *testing.T) {
	ts, tok, _ := setupPushServer(t, true)
	status, body := doJSON(t, ts, http.MethodPost, "/me/push/subscribe", tok, map[string]any{
		"endpoint": "https://push.example/ok",
		"keys":     map[string]string{"p256dh": "p", "auth": "a"},
	})
	if status != http.StatusOK {
		t.Fatalf("subscribe status = %d body=%s", status, body)
	}
	status, body = doJSON(t, ts, http.MethodGet, "/me/push", tok, nil)
	if status != http.StatusOK {
		t.Fatalf("get status = %d", status)
	}
	var env struct {
		Data Status `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatal(err)
	}
	if !env.Data.Configured || !env.Data.Subscribed {
		t.Fatalf("data = %+v", env.Data)
	}
}
