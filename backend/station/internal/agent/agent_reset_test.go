package agent_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"avtotest.uz/station/internal/agent"
	"avtotest.uz/station/internal/keystore"
)

// A school that is deleted and recreated in the admin panel leaves every
// classroom PC holding a station id the backend no longer knows. The agent
// used to read that as an ordinary error and retry forever, because only a
// missing station id produced ErrNotEnrolled -- so the PC never recovered and
// the console said nothing a school could act on.
func TestTokenReportsStationUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{
				"code":    "station_unauthorized",
				"message": "station authentication failed",
			},
		})
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "station.json"),
		[]byte(`{"station_id":"11111111-1111-1111-1111-111111111111","org_id":"o","label":"PC-1"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	keys, err := keystore.Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	a := &agent.Agent{APIBase: srv.URL, StateDir: dir, Keys: keys, HWID: "hw", Version: "test"}
	_, err = a.Token(context.Background())
	if !errors.Is(err, agent.ErrStationUnauthorized) {
		t.Fatalf("Token() error = %v, want ErrStationUnauthorized", err)
	}
	if errors.Is(err, agent.ErrNotEnrolled) {
		t.Fatal("a rejected station must not look like one that was never enrolled")
	}
}

// ResetEnrollment is the explicit escape hatch for that PC. It must remove
// both halves of the identity: leaving station.key behind would re-seal a new
// enrollment onto a key the backend has already seen.
func TestResetEnrollmentRemovesBothHalvesAndIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	keys, err := keystore.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := keys.Load(); err != nil { // creates station.key
		t.Fatal(err)
	}
	statePath := filepath.Join(dir, "station.json")
	if err := os.WriteFile(statePath, []byte(`{"station_id":"abc"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	a := &agent.Agent{APIBase: "http://127.0.0.1:1", StateDir: dir, Keys: keys, HWID: "hw", Version: "test"}
	if err := a.ResetEnrollment(); err != nil {
		t.Fatalf("ResetEnrollment: %v", err)
	}
	for _, path := range []string{statePath, keystore.KeyPath(dir)} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s still exists after reset (err=%v)", filepath.Base(path), err)
		}
	}

	// Running -reenroll on a PC that was never enrolled must not fail.
	if err := a.ResetEnrollment(); err != nil {
		t.Fatalf("second ResetEnrollment: %v", err)
	}

	// And the agent must now look brand new, not merely tokenless.
	if _, err := a.Token(context.Background()); !errors.Is(err, agent.ErrNotEnrolled) {
		t.Fatalf("Token() after reset = %v, want ErrNotEnrolled", err)
	}
}
