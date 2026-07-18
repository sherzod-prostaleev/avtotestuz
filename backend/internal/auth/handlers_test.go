package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/redisx"
	"avtotest.uz/backend/internal/testdb"
)

const handlerSecret = "test-secret"

func setupHandlerServer(t *testing.T) *httptest.Server {
	t.Helper()
	pool := testdb.New(t)
	c := redisx.NewTest(t)
	svc := NewService(sqlc.New(pool), pool, Limiter{R: c}, SandboxSender{Log: zap.NewNop()}, []byte(handlerSecret), "dev")

	r := chi.NewRouter()
	h := &Handler{Svc: svc}
	h.Routes(r)
	r.With(Required([]byte(handlerSecret))).Get("/probe", func(w http.ResponseWriter, req *http.Request) {
		claims, _ := FromContext(req.Context())
		_, _ = w.Write([]byte(claims.ProfileID.String()))
	})

	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)
	return ts
}

type respEnvelope struct {
	Data  json.RawMessage `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func postJSON(t *testing.T, ts *httptest.Server, path string, body any) (int, respEnvelope) {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := ts.Client().Post(ts.URL+path, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var env respEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, env
}

func TestOTPRequestVerifyProbeAndRefreshOverHTTP(t *testing.T) {
	ts := setupHandlerServer(t)

	status, env := postJSON(t, ts, "/auth/otp/request", map[string]string{"phone": "901234567"})
	if status != http.StatusOK {
		t.Fatalf("request status=%d", status)
	}
	var reqOut otpRequestResponse
	if err := json.Unmarshal(env.Data, &reqOut); err != nil {
		t.Fatal(err)
	}
	if reqOut.DebugCode == "" {
		t.Fatal("expected debug_code in dev+sandbox response")
	}

	status, env = postJSON(t, ts, "/auth/otp/verify", map[string]string{"phone": "901234567", "code": reqOut.DebugCode})
	if status != http.StatusOK {
		t.Fatalf("verify status=%d", status)
	}
	var toks tokensResponse
	if err := json.Unmarshal(env.Data, &toks); err != nil {
		t.Fatal(err)
	}
	if toks.AccessToken == "" || toks.RefreshToken == "" {
		t.Fatal("expected non-empty tokens")
	}

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/probe", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+toks.AccessToken)
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("probe status=%d, want 200", resp.StatusCode)
	}

	status, env = postJSON(t, ts, "/auth/refresh", map[string]string{"refresh_token": toks.RefreshToken})
	if status != http.StatusOK {
		t.Fatalf("refresh status=%d", status)
	}
	var toks2 tokensResponse
	if err := json.Unmarshal(env.Data, &toks2); err != nil {
		t.Fatal(err)
	}
	if toks2.RefreshToken == "" || toks2.RefreshToken == toks.RefreshToken {
		t.Fatal("expected a new, rotated refresh token")
	}

	// the old refresh token was rotated out; the immediate resend cooldown
	// also still applies, so a re-request must be rejected either way
	status, _ = postJSON(t, ts, "/auth/otp/request", map[string]string{"phone": "901234567"})
	if status != http.StatusTooManyRequests {
		t.Fatalf("re-request status=%d, want 429", status)
	}
}
