package payme

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func basicAuthHeader(login, pass string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(login+":"+pass))
}

// TestAuthAndTransport exercises the transport/auth layer of the webhook:
// method check, Basic-auth, JSON parsing, and unknown-method dispatch.
// Every response must be HTTP 200 — Payme errors travel in the JSON body,
// never in the status code.
func TestAuthAndTransport(t *testing.T) {
	const key = "test-cashbox-key"

	cases := []struct {
		name       string
		method     string
		authHeader string // empty = no Authorization header at all
		body       string
		wantCode   int
	}{
		{
			name:     "GET is rejected as not-POST",
			method:   http.MethodGet,
			wantCode: -32300,
		},
		{
			name:     "POST without auth header",
			method:   http.MethodPost,
			body:     `{"method":"CheckTransaction","params":{},"id":1}`,
			wantCode: -32504,
		},
		{
			name:       "POST with wrong key",
			method:     http.MethodPost,
			authHeader: basicAuthHeader("Paycom", "wrong-key"),
			body:       `{"method":"CheckTransaction","params":{},"id":1}`,
			wantCode:   -32504,
		},
		{
			name:       "POST valid auth, unknown method",
			method:     http.MethodPost,
			authHeader: basicAuthHeader("Paycom", key),
			body:       `{"method":"NoSuchMethod","params":{},"id":1}`,
			wantCode:   -32601,
		},
		{
			name:       "POST valid auth, malformed JSON",
			method:     http.MethodPost,
			authHeader: basicAuthHeader("Paycom", key),
			body:       `{not json`,
			wantCode:   -32700,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := &Handler{Key: key}

			req := httptest.NewRequest(tc.method, "/api/v1/billing/payme", strings.NewReader(tc.body))
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("HTTP status = %d, want 200 (Payme requires 200 on every response)", rec.Code)
			}

			var resp struct {
				Error *struct {
					Code int `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode response body: %v (body=%s)", err, rec.Body.String())
			}
			if resp.Error == nil {
				t.Fatalf("expected error object in body, got none (body=%s)", rec.Body.String())
			}
			if resp.Error.Code != tc.wantCode {
				t.Errorf("error.code = %d, want %d", resp.Error.Code, tc.wantCode)
			}
		})
	}
}

// TestAuthEmptyKeyAlwaysRejects: an empty Handler.Key (Payme not
// configured, e.g. dev without ENV set) must always reject with -32504,
// even with an otherwise well-formed Basic-auth header.
func TestAuthEmptyKeyAlwaysRejects(t *testing.T) {
	h := &Handler{Key: ""}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/billing/payme",
		strings.NewReader(`{"method":"CheckTransaction","params":{},"id":1}`))
	req.Header.Set("Authorization", basicAuthHeader("Paycom", ""))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200", rec.Code)
	}
	var resp struct {
		Error *struct {
			Code int `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response body: %v (body=%s)", err, rec.Body.String())
	}
	if resp.Error == nil || resp.Error.Code != -32504 {
		t.Fatalf("error = %+v, want code -32504", resp.Error)
	}
}
