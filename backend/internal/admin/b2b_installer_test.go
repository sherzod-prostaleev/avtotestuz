package admin

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"avtotest.uz/backend/internal/b2b"
	"avtotest.uz/backend/internal/testdb"
)

type installerKeyEnv struct {
	Data *installerKeyDTO `json:"data"`
}

type installerErrEnv struct {
	Error struct {
		Code string `json:"code"`
	} `json:"error"`
}

// createInstallerTestOrg creates an org via the admin HTTP API — same helpers
// as b2b_test.go — and, when seats/days are positive, attaches a license.
func createInstallerTestOrg(t *testing.T, r chi.Router, access, name string, seats, days int) uuid.UUID {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/admin/v1/b2b/orgs",
		bytes.NewBufferString(fmt.Sprintf(`{"name":%q}`, name)))
	req.Header.Set("Authorization", "Bearer "+access)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("create org status=%d body=%s", w.Code, w.Body.String())
	}
	var env struct {
		Data B2BOrgRow `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}

	if seats > 0 {
		req = httptest.NewRequest(http.MethodPost, "/admin/v1/b2b/orgs/"+env.Data.ID.String()+"/licenses",
			bytes.NewBufferString(fmt.Sprintf(`{"seats":%d,"days":%d,"note":"installer test"}`, seats, days)))
		req.Header.Set("Authorization", "Bearer "+access)
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("license status=%d body=%s", w.Code, w.Body.String())
		}
	}
	return env.Data.ID
}

func newInstallerTestHandler(t *testing.T, email string) (*Handler, chi.Router, string) {
	t.Helper()
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := Store{Pool: pool}
	secret := []byte("test-admin-secret-at-least-32-bytes!!")
	if _, err := store.EnsureSuperadmin(t.Context(), email, "password123", "Ops"); err != nil {
		t.Fatal(err)
	}
	h := &Handler{Svc: Service{Store: store, Secret: secret}, Pool: pool, Secret: secret}
	r := chi.NewRouter()
	r.Route("/admin/v1", h.Routes)
	access := loginAccess(t, r, email, "password123")
	return h, r, access
}

// TestInstallerKeyEndpoints asserts the idempotence the download flow depends
// on: an admin installing 30 PCs over several days must be able to fetch the
// installer repeatedly without invalidating the copies already handed out.
func TestInstallerKeyEndpoints(t *testing.T) {
	_, r, access := newInstallerTestHandler(t, "installer-ops@example.uz")
	orgID := createInstallerTestOrg(t, r, access, "Installer School", 30, 365)

	// 1. GET /installer -> 200, data == null
	req := httptest.NewRequest(http.MethodGet, "/admin/v1/b2b/orgs/"+orgID.String()+"/installer", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("get installer status=%d body=%s", w.Code, w.Body.String())
	}
	var env installerKeyEnv
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Data != nil {
		t.Fatalf("expected data == null before any key is opened, got %+v", env.Data)
	}

	// 2. POST /installer -> 200, code non-empty, max_uses == 30
	req = httptest.NewRequest(http.MethodPost, "/admin/v1/b2b/orgs/"+orgID.String()+"/installer", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("open installer status=%d body=%s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Data == nil || env.Data.Code == "" {
		t.Fatalf("expected a minted code, got %+v", env.Data)
	}
	if env.Data.MaxUses != 30 {
		t.Fatalf("max_uses=%d, want 30", env.Data.MaxUses)
	}
	firstCode := env.Data.Code

	// 3. POST /installer again -> 200, SAME code
	req = httptest.NewRequest(http.MethodPost, "/admin/v1/b2b/orgs/"+orgID.String()+"/installer", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("second open installer status=%d body=%s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Data == nil || env.Data.Code != firstCode {
		t.Fatalf("second POST minted a different code: %+v, want %q", env.Data, firstCode)
	}

	// 4. GET /installer -> 200, same code, used_count == 0
	req = httptest.NewRequest(http.MethodGet, "/admin/v1/b2b/orgs/"+orgID.String()+"/installer", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("get installer status=%d body=%s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Data == nil || env.Data.Code != firstCode || env.Data.UsedCount != 0 {
		t.Fatalf("get after open = %+v, want code=%q used_count=0", env.Data, firstCode)
	}

	// 5. POST /installer/rotate -> 200, DIFFERENT code
	req = httptest.NewRequest(http.MethodPost, "/admin/v1/b2b/orgs/"+orgID.String()+"/installer/rotate", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("rotate installer status=%d body=%s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Data == nil || env.Data.Code == "" || env.Data.Code == firstCode {
		t.Fatalf("rotate = %+v, want a fresh non-empty code (old was %q)", env.Data, firstCode)
	}
	rotatedCode := env.Data.Code

	// 6. GET /installer -> 200, the rotated code
	req = httptest.NewRequest(http.MethodGet, "/admin/v1/b2b/orgs/"+orgID.String()+"/installer", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("get installer status=%d body=%s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Data == nil || env.Data.Code != rotatedCode {
		t.Fatalf("get after rotate = %+v, want code=%q", env.Data, rotatedCode)
	}
}

// TestInstallerExeCarriesTheConfig proves the download is a real binary with a
// readable trailer, not just a 200.
func TestInstallerExeCarriesTheConfig(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := Store{Pool: pool}
	secret := []byte("test-admin-secret-at-least-32-bytes!!")
	if _, err := store.EnsureSuperadmin(t.Context(), "installer-exe@example.uz", "password123", "Ops"); err != nil {
		t.Fatal(err)
	}

	baseBytes := []byte("FAKE-EXE-BASE-BYTES-FOR-INSTALLER-TEST")
	basePath := filepath.Join(t.TempDir(), "avtotest-station.exe")
	if err := os.WriteFile(basePath, baseBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	h := &Handler{
		Svc: Service{Store: store, Secret: secret}, Pool: pool, Secret: secret,
		StationBinaryPath: basePath,
		PublicBaseURL:     "https://api.drivergo.uz",
	}
	r := chi.NewRouter()
	r.Route("/admin/v1", h.Routes)
	access := loginAccess(t, r, "installer-exe@example.uz", "password123")

	orgID := createInstallerTestOrg(t, r, access, "Exe School", 5, 30)

	req := httptest.NewRequest(http.MethodPost, "/admin/v1/b2b/orgs/"+orgID.String()+"/installer", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("open installer status=%d body=%s", w.Code, w.Body.String())
	}
	var env installerKeyEnv
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	code := env.Data.Code

	req = httptest.NewRequest(http.MethodGet, "/admin/v1/b2b/orgs/"+orgID.String()+"/installer.exe?locale=ru", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("download installer status=%d body=%s", w.Code, w.Body.String())
	}

	wantDisposition := `attachment; filename="` + b2b.InstallerFilename("Exe School", orgID) + `"`
	if got := w.Header().Get("Content-Disposition"); got != wantDisposition {
		t.Fatalf("Content-Disposition=%q, want %q", got, wantDisposition)
	}

	body := w.Body.Bytes()
	if !bytes.HasPrefix(body, baseBytes) {
		t.Fatalf("body does not start with the fake base bytes: % x", body[:min(len(body), 64)])
	}

	// Parse the trailer with the same layout b2b.AppendConfig writes:
	// [base][json][uint32 BE len][16-byte magic "AVTOSTATIONCFG01"].
	const magic = "AVTOSTATIONCFG01"
	if got := string(body[len(body)-16:]); got != magic {
		t.Fatalf("trailer magic=%q, want %q", got, magic)
	}
	n := binary.BigEndian.Uint32(body[len(body)-20 : len(body)-16])
	jsonStart := len(body) - 20 - int(n)
	if jsonStart != len(baseBytes) {
		t.Fatalf("json starts at %d, want %d (right after the base bytes)", jsonStart, len(baseBytes))
	}
	var cfg b2b.InstallerConfig
	if err := json.Unmarshal(body[jsonStart:len(body)-20], &cfg); err != nil {
		t.Fatalf("trailer json did not parse: %v", err)
	}
	want := b2b.InstallerConfig{
		Code:     code,
		API:      "https://api.drivergo.uz",
		Frontend: "https://api.drivergo.uz",
		Org:      "Exe School",
		Locale:   "ru",
	}
	if cfg != want {
		t.Fatalf("trailer config = %+v, want %+v", cfg, want)
	}
}

// TestInstallerExeWithoutAKey asserts the download refuses rather than minting
// one as a side effect of a GET.
func TestInstallerExeWithoutAKey(t *testing.T) {
	_, r, access := newInstallerTestHandler(t, "installer-nokey@example.uz")
	orgID := createInstallerTestOrg(t, r, access, "No Key School", 5, 30)

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/b2b/orgs/"+orgID.String()+"/installer.exe", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s, want 409", w.Code, w.Body.String())
	}
	var env installerErrEnv
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Error.Code != "no_installer_key" {
		t.Fatalf("error code=%q, want no_installer_key", env.Error.Code)
	}
}

// TestInstallerRefusesOrgWithoutLicense
func TestInstallerRefusesOrgWithoutLicense(t *testing.T) {
	_, r, access := newInstallerTestHandler(t, "installer-nolicense@example.uz")
	orgID := createInstallerTestOrg(t, r, access, "No License School", 0, 0)

	req := httptest.NewRequest(http.MethodPost, "/admin/v1/b2b/orgs/"+orgID.String()+"/installer", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s, want 409", w.Code, w.Body.String())
	}
	var env installerErrEnv
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Error.Code != "no_license" {
		t.Fatalf("error code=%q, want no_license", env.Error.Code)
	}
}

// TestGetInstallerNoKeyBodyIsExplicitNull proves the "no key yet" response is
// the documented {"data":null}, not {} with the key dropped. This can only be
// caught at the raw-byte level: unmarshalling either shape into a Go struct
// field yields the same nil pointer, so a struct-based assertion here would
// pass whether or not the handler regressed to a bare untyped nil.
func TestGetInstallerNoKeyBodyIsExplicitNull(t *testing.T) {
	_, r, access := newInstallerTestHandler(t, "installer-null-body@example.uz")
	orgID := createInstallerTestOrg(t, r, access, "Null Body School", 5, 30)

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/b2b/orgs/"+orgID.String()+"/installer", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if body := strings.TrimSpace(w.Body.String()); body != `{"data":null}` {
		t.Fatalf("body=%q, want exactly {\"data\":null}", body)
	}
}

// TestGetInstallerUnknownOrgIs404 asserts a nonexistent org id 404s rather
// than reading as "org exists, no key yet" — ActiveInstallerKey alone cannot
// tell those apart, since it returns (nil, nil) for both.
func TestGetInstallerUnknownOrgIs404(t *testing.T) {
	_, r, access := newInstallerTestHandler(t, "installer-unknown-org@example.uz")

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/b2b/orgs/"+uuid.New().String()+"/installer", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s, want 404", w.Code, w.Body.String())
	}
	var env installerErrEnv
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Error.Code != "not_found" {
		t.Fatalf("error code=%q, want not_found", env.Error.Code)
	}
}

// TestInstallerDownloadOrgLookupTransientErrorIs500 asserts that a genuine DB
// error on the org-name lookup — a connection failure or timeout, not "no
// rows" — reports 500, not the misleading "org not found" an admin would
// otherwise see immediately after ActiveInstallerKey succeeded against the
// same org id. An ACCESS EXCLUSIVE lock on b2b_org from a second connection
// blocks only the raw `SELECT name FROM b2b_org` in the download handler
// (ActiveInstallerKey reads a different table, b2b_org_enroll_code, and is
// unaffected); a short request context timeout then turns that block into a
// real non-ErrNoRows pgx error deterministically, without touching the pool.
func TestInstallerDownloadOrgLookupTransientErrorIs500(t *testing.T) {
	h, r, access := newInstallerTestHandler(t, "installer-lock@example.uz")
	orgID := createInstallerTestOrg(t, r, access, "Lock School", 5, 30)

	openReq := httptest.NewRequest(http.MethodPost, "/admin/v1/b2b/orgs/"+orgID.String()+"/installer", nil)
	openReq.Header.Set("Authorization", "Bearer "+access)
	openW := httptest.NewRecorder()
	r.ServeHTTP(openW, openReq)
	if openW.Code != http.StatusOK {
		t.Fatalf("open installer status=%d body=%s", openW.Code, openW.Body.String())
	}

	lockTx, err := h.Pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lockTx.Rollback(context.Background()) }()
	if _, err := lockTx.Exec(context.Background(), "LOCK TABLE b2b_org IN ACCESS EXCLUSIVE MODE"); err != nil {
		t.Fatal(err)
	}

	h.StationBinaryPath = filepath.Join(t.TempDir(), "unused.exe")
	if err := os.WriteFile(h.StationBinaryPath, []byte("BASE"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/admin/v1/b2b/orgs/"+orgID.String()+"/installer.exe", nil).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer "+access)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s, want 500 (transient DB error must not read as not_found)", w.Code, w.Body.String())
	}
	var env installerErrEnv
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Error.Code != "internal" {
		t.Fatalf("error code=%q, want internal (not not_found)", env.Error.Code)
	}
}
