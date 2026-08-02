package admin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"

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

// loginEnvelope is the shape /auth/login answers with in every branch.
type loginEnvelope struct {
	Data struct {
		RequiresTOTP      bool   `json:"requires_totp"`
		ChallengeToken    string `json:"challenge_token"`
		TOTPSetupRequired bool   `json:"totp_setup_required"`
		EnrollmentToken   string `json:"enrollment_token"`
	} `json:"data"`
	Error struct {
		Code string `json:"code"`
	} `json:"error"`
}

func decodeLogin(t *testing.T, w *httptest.ResponseRecorder) loginEnvelope {
	t.Helper()
	var env loginEnvelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode %s: %v", w.Body.String(), err)
	}
	return env
}

func authedPost(t *testing.T, r chi.Router, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func authedGet(t *testing.T, r chi.Router, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
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
	env := decodeLogin(t, w)
	if env.Error.Code != "totp_setup_required" {
		t.Errorf("error code=%q, want totp_setup_required", env.Error.Code)
	}
	if bytes.Contains(w.Body.Bytes(), []byte("access_token")) {
		t.Error("a token pair was issued despite TOTP enforcement")
	}
	// ...but the reply must carry the way out. Refusing with nothing attached
	// is what bricked every admin created after enforcement was switched on.
	if !env.Data.TOTPSetupRequired || env.Data.EnrollmentToken == "" {
		t.Fatalf("no enrollment token offered: %s", w.Body.String())
	}
}

// TestEnrollmentTokenAuthorizesEnrollmentAndNothingElse is the AD-2 recovery
// path end to end: an admin enforcement locks out enrolls with the token
// login gave them, then signs in normally — while that token opens no other
// admin route at any point.
func TestEnrollmentTokenAuthorizesEnrollmentAndNothingElse(t *testing.T) {
	t.Setenv("ADMIN_TOTP_ENFORCE", "1")
	r, _ := setupLoginHarness(t)

	enrollToken := decodeLogin(t, postLogin(t, r, "ops@example.uz", "password123")).Data.EnrollmentToken
	if enrollToken == "" {
		t.Fatal("login issued no enrollment token")
	}

	// Scope: everything except enroll/confirm must reject it outright.
	for _, path := range []string{"/admin/v1/me", "/admin/v1/ping", "/admin/v1/users", "/admin/v1/security/audit"} {
		if got := authedGet(t, r, path, enrollToken).Code; got != http.StatusUnauthorized {
			t.Errorf("GET %s with enrollment token = %d, want 401", path, got)
		}
	}
	if got := authedPost(t, r, "/admin/v1/security/totp/disable", enrollToken, `{"code":"000000"}`).Code; got != http.StatusUnauthorized {
		t.Errorf("totp/disable with enrollment token = %d, want 401", got)
	}

	// Enroll: the secret is generated and shown by the server.
	w := authedPost(t, r, "/admin/v1/security/totp/enroll", enrollToken, `{}`)
	if w.Code != http.StatusOK {
		t.Fatalf("enroll status=%d body=%s", w.Code, w.Body.String())
	}
	var enrolled struct {
		Data struct {
			Secret     string `json:"secret"`
			OtpauthURL string `json:"otpauth_url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &enrolled); err != nil {
		t.Fatal(err)
	}
	if enrolled.Data.Secret == "" || enrolled.Data.OtpauthURL == "" {
		t.Fatalf("enroll returned nothing usable: %s", w.Body.String())
	}

	// Confirm with a live code — and note the request carries no secret at
	// all, because the server re-derives the one it issued (AD-4).
	code, err := totp.GenerateCode(enrolled.Data.Secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if got := authedPost(t, r, "/admin/v1/security/totp/confirm", enrollToken, `{"code":"`+code+`"}`); got.Code != http.StatusOK {
		t.Fatalf("confirm status=%d body=%s", got.Code, got.Body.String())
	}

	// The lockout is over: login now offers the ordinary TOTP challenge.
	env := decodeLogin(t, postLogin(t, r, "ops@example.uz", "password123"))
	if !env.Data.RequiresTOTP || env.Data.ChallengeToken == "" {
		t.Fatalf("login after enrollment did not challenge: %+v", env.Data)
	}

	// And the enrollment token is spent — a stolen copy must not be able to
	// swap in a different authenticator for the rest of its TTL.
	if got := authedPost(t, r, "/admin/v1/security/totp/enroll", enrollToken, `{}`).Code; got != http.StatusUnauthorized {
		t.Errorf("replayed enrollment token = %d, want 401", got)
	}
}

// TestConfirmRejectsAClientSuppliedSecret is AD-4 at the wire: the endpoint
// used to store whatever secret the body carried, so an enrolled
// authenticator was never provably the one this server issued.
func TestConfirmRejectsAClientSuppliedSecret(t *testing.T) {
	t.Setenv("ADMIN_TOTP_ENFORCE", "1")
	r, store := setupLoginHarness(t)

	enrollToken := decodeLogin(t, postLogin(t, r, "ops@example.uz", "password123")).Data.EnrollmentToken
	rogue, err := generateTOTPKey("attacker@example.uz", nil)
	if err != nil {
		t.Fatal(err)
	}
	code, err := totp.GenerateCode(rogue.Secret(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	body := `{"secret":"` + rogue.Secret() + `","code":"` + code + `"}`
	if got := authedPost(t, r, "/admin/v1/security/totp/confirm", enrollToken, body); got.Code != http.StatusBadRequest {
		t.Fatalf("confirm with attacker secret status=%d body=%s, want 400", got.Code, got.Body.String())
	}
	u, err := store.GetUserByEmail(t.Context(), "ops@example.uz")
	if err != nil {
		t.Fatal(err)
	}
	if u.TOTPEnabled() {
		t.Fatal("a secret of the caller's choosing was enrolled")
	}
}

// TestMeReportsTOTPSetupRequiredForEveryRole: login refuses every role
// without an authenticator, so /me must say so for every role. Reporting it
// only to superadmin is what left the others with no explanation and no UI.
func TestMeReportsTOTPSetupRequiredForEveryRole(t *testing.T) {
	t.Setenv("ADMIN_TOTP_ENFORCE", "1")
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := Store{Pool: pool}
	svc := Service{Store: store, Secret: []byte("test-admin-secret-at-least-32-bytes!!")}

	for _, role := range []string{"superadmin", "admin", "support", "editor", "finance"} {
		t.Run(role, func(t *testing.T) {
			id := seedAdminWithRole(t, store, role+"@example.uz", role)
			me, err := svc.Me(t.Context(), id)
			if err != nil {
				t.Fatal(err)
			}
			if !me.TOTPSetupRequired {
				t.Errorf("role %s: totp_setup_required=false while login refuses it a session", role)
			}
		})
	}
}

func seedAdminWithRole(t *testing.T, store Store, email, role string) uuid.UUID {
	t.Helper()
	hash, err := auth.HashPassword("password123")
	if err != nil {
		t.Fatal(err)
	}
	var id uuid.UUID
	if err := store.Pool.QueryRow(t.Context(), `
		INSERT INTO admin_user (email, display_name, password_hash, status)
		VALUES ($1, $2, $3, 'active') RETURNING id`, email, role, hash).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Pool.Exec(t.Context(), `
		INSERT INTO admin_user_role (admin_user_id, role_id)
		SELECT $1, r.id FROM admin_role r WHERE r.code = $2`, id, role); err != nil {
		t.Fatal(err)
	}
	return id
}
