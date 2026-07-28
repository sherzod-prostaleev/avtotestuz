package admin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/redisx"
	"avtotest.uz/backend/internal/testdb"
)

// setupLoginHarness wires the admin router with a real Redis-backed limiter,
// which is what production uses. Tests that leave Lim zero-valued keep the
// old unthrottled behaviour, so existing cases are unaffected.
func setupLoginHarness(t *testing.T) (chi.Router, Store) {
	t.Helper()
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := Store{Pool: pool}
	secret := []byte("test-admin-secret-at-least-32-bytes!!")
	if _, err := store.EnsureSuperadmin(t.Context(), "ops@example.uz", "password123", "Ops"); err != nil {
		t.Fatal(err)
	}
	svc := Service{Store: store, Secret: secret, Lim: auth.Limiter{R: redisx.NewTest(t)}}
	h := &Handler{Svc: svc, Pool: pool, Secret: secret}
	r := chi.NewRouter()
	r.Route("/admin/v1", h.Routes)
	return r, store
}

func postLogin(t *testing.T, r chi.Router, email, password string) *httptest.ResponseRecorder {
	t.Helper()
	body := fmt.Sprintf(`{"email":%q,"password":%q}`, email, password)
	req := httptest.NewRequest(http.MethodPost, "/admin/v1/auth/login", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestAdminLoginLocksOutAfterRepeatedFailures pins the fix for an
// internet-reachable password oracle: /admin/v1/auth/login had no limiter,
// no lockout and no audit of failures, so bcrypt cost was the only thing
// between a guesser and full control of payouts and user data.
func TestAdminLoginLocksOutAfterRepeatedFailures(t *testing.T) {
	r, store := setupLoginHarness(t)

	for i := 0; i < adminLoginFailLimit; i++ {
		if got := postLogin(t, r, "ops@example.uz", "wrong").Code; got != http.StatusUnauthorized {
			t.Fatalf("attempt %d status=%d, want 401", i, got)
		}
	}

	// Budget spent: further attempts are refused outright, and crucially the
	// CORRECT password is refused too — otherwise the lockout would not
	// actually stop a guesser who eventually hits it.
	if got := postLogin(t, r, "ops@example.uz", "wrong").Code; got != http.StatusTooManyRequests {
		t.Errorf("after lockout status=%d, want 429", got)
	}
	if got := postLogin(t, r, "ops@example.uz", "password123").Code; got != http.StatusTooManyRequests {
		t.Errorf("correct password during lockout status=%d, want 429", got)
	}

	var failures int
	if err := store.Pool.QueryRow(t.Context(),
		`SELECT count(*) FROM admin_audit_log WHERE action = 'admin.login.failed'`,
	).Scan(&failures); err != nil {
		t.Fatal(err)
	}
	if failures == 0 {
		t.Error("no admin.login.failed audit rows: a brute-force run must be visible to security.audit.read")
	}
}

// TestAdminLoginSucceedsAndClearsFailureBudget keeps the everyday path
// honest: a few typos must not lock a legitimate admin out once they get it
// right.
func TestAdminLoginSucceedsAndClearsFailureBudget(t *testing.T) {
	r, _ := setupLoginHarness(t)

	for i := 0; i < adminLoginFailLimit-1; i++ {
		postLogin(t, r, "ops@example.uz", "wrong")
	}
	if got := postLogin(t, r, "ops@example.uz", "password123").Code; got != http.StatusOK {
		t.Fatalf("login after typos status=%d, want 200", got)
	}

	// The successful login reset the counter, so the next wrong password is
	// an ordinary 401 rather than an immediate lockout.
	if got := postLogin(t, r, "ops@example.uz", "wrong").Code; got != http.StatusUnauthorized {
		t.Errorf("status after successful reset=%d, want 401", got)
	}
}

// TestAdminLoginRefusesWhenTOTPEnforcedButNotEnrolled pins the other half:
// ADMIN_TOTP_ENFORCE used to only set a cosmetic flag on /me while login
// still issued a full token pair, so "enforcement" enforced nothing.
func TestAdminLoginRefusesWhenTOTPEnforcedButNotEnrolled(t *testing.T) {
	t.Setenv("ADMIN_TOTP_ENFORCE", "1")
	r, _ := setupLoginHarness(t)

	w := postLogin(t, r, "ops@example.uz", "password123")
	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want 403 (body=%s)", w.Code, w.Body.String())
	}
	var env struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Error.Code != "totp_setup_required" {
		t.Errorf("error code=%q, want totp_setup_required", env.Error.Code)
	}
	if bytes.Contains(w.Body.Bytes(), []byte("access_token")) {
		t.Error("a token pair was issued despite TOTP enforcement")
	}
}
