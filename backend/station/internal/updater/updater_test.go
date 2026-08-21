package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"avtotest.uz/station/internal/embedcfg"
)

// writeConfigured builds a file shaped like an installed agent: a base binary
// with the admin panel's config trailer appended. It mirrors
// backend/internal/b2b.AppendConfig's layout, which embedcfg documents.
func writeConfigured(t *testing.T, path string, base []byte, cfg embedcfg.Config) {
	t.Helper()
	payload, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out := append([]byte(nil), base...)
	out = append(out, payload...)
	n := len(payload)
	out = append(out, byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
	out = append(out, []byte("AVTOSTATIONCFG01")...)
	if err := os.WriteFile(path, out, 0o755); err != nil {
		t.Fatal(err)
	}
}

func serveAgent(t *testing.T, version string, base []byte) *httptest.Server {
	t.Helper()
	sum := sha256.Sum256(base)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case manifestPath:
			_ = json.NewEncoder(w).Encode(map[string]any{"data": Manifest{
				Version: version, SHA256: hex.EncodeToString(sum[:]), Size: int64(len(base)),
			}})
		case downloadPath:
			_, _ = w.Write(base)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestStageKeepsTheSchoolConfiguration is the property the whole design rests
// on: the server serves one school-agnostic binary, and each PC re-attaches
// the trailer it is already carrying. Losing it would leave a classroom with
// an agent that has no installer key and belongs to no school.
func TestStageKeepsTheSchoolConfiguration(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "avtotest-station.exe")
	want := embedcfg.Config{
		Code: "AVTO-KEEP-THIS", API: "https://drivergo.uz",
		Frontend: "https://drivergo.uz", Org: "Test Avtomaktab",
	}
	writeConfigured(t, target, []byte("OLD-BINARY-BYTES"), want)

	newBase := []byte("NEW-BINARY-BYTES-THAT-ARE-LONGER")
	srv := serveAgent(t, "1.1.0", newBase)

	cfg := Config{APIBase: srv.URL, Version: "1.0.9", Target: target, StateDir: dir, HTTP: srv.Client()}
	if err := checkOnce(context.Background(), cfg); err != nil {
		t.Fatalf("checkOnce() = %v", err)
	}

	got, err := embedcfg.Read(target)
	if err != nil {
		t.Fatalf("the updated agent no longer carries a readable school config: %v", err)
	}
	if got != want {
		t.Fatalf("school config = %+v, want %+v", got, want)
	}
	body, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(body[:len(newBase)]) != string(newBase) {
		t.Fatal("the new binary bytes were not written")
	}
	if _, err := os.Stat(target + ".prev"); err != nil {
		t.Fatalf("the previous binary was not kept: %v", err)
	}

	var s staged
	b, err := os.ReadFile(filepath.Join(dir, stagedName))
	if err != nil {
		t.Fatalf("no staged marker: %v", err)
	}
	if json.Unmarshal(b, &s) != nil || s.Version != "1.1.0" {
		t.Fatalf("staged marker = %s, want version 1.1.0", b)
	}
}

// TestStageRefusesAWrongDigest covers the only integrity control this
// unsigned binary has. Whoever can serve the update owns every classroom PC,
// so a mismatch must leave the working agent exactly where it was.
func TestStageRefusesAWrongDigest(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "avtotest-station.exe")
	writeConfigured(t, target, []byte("OLD-BINARY-BYTES"), embedcfg.Config{Code: "AVTO-X", Org: "Org"})
	before, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case manifestPath:
			_ = json.NewEncoder(w).Encode(map[string]any{"data": Manifest{
				Version: "1.1.0",
				SHA256:  "00000000000000000000000000000000000000000000000000000000deadbeef",
				Size:    int64(len("TAMPERED-BYTES")),
			}})
		case downloadPath:
			_, _ = w.Write([]byte("TAMPERED-BYTES"))
		}
	}))
	t.Cleanup(srv.Close)

	cfg := Config{APIBase: srv.URL, Version: "1.0.9", Target: target, StateDir: dir, HTTP: srv.Client()}
	if err := checkOnce(context.Background(), cfg); err == nil {
		t.Fatal("checkOnce() = nil, want a digest mismatch to be refused")
	}

	after, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("the installed agent was modified despite a failed digest check")
	}
	if _, err := os.Stat(filepath.Join(dir, stagedName)); !os.IsNotExist(err) {
		t.Fatal("a staged marker was written for an update that never happened")
	}
}

// TestNoUpdateWhenVersionsMatch keeps the steady state cheap: thirty PCs in a
// school ask for a manifest, see their own version and stop.
func TestNoUpdateWhenVersionsMatch(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "avtotest-station.exe")
	writeConfigured(t, target, []byte("BINARY"), embedcfg.Config{Code: "AVTO-X", Org: "Org"})

	var downloads int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == downloadPath {
			downloads++
		}
		if r.URL.Path == manifestPath {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": Manifest{
				Version: "1.0.9", SHA256: "x", Size: 1,
			}})
		}
	}))
	t.Cleanup(srv.Close)

	cfg := Config{APIBase: srv.URL, Version: "1.0.9", Target: target, StateDir: dir, HTTP: srv.Client()}
	if err := checkOnce(context.Background(), cfg); err != nil {
		t.Fatalf("checkOnce() = %v", err)
	}
	if downloads != 0 {
		t.Fatalf("downloaded the agent %d times while already up to date", downloads)
	}
}

// TestRestartWaitsForAnIdleKiosk is what keeps an update from ending a
// student's exam. The binary is swapped either way -- it lands at the next
// boot -- but the running process only hands over when nothing has called the
// API for a while.
func TestRestartWaitsForAnIdleKiosk(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "avtotest-station.exe")
	writeConfigured(t, target, []byte("OLD"), embedcfg.Config{Code: "AVTO-X", Org: "Org"})
	srv := serveAgent(t, "1.1.0", []byte("NEW-BINARY"))

	restarted := false
	cfg := Config{
		APIBase: srv.URL, Version: "1.0.9", Target: target, StateDir: dir, HTTP: srv.Client(),
		IdleBefore: 30 * time.Minute,
		Idle:       func() time.Duration { return time.Minute }, // a lesson is running
		Restart:    func() { restarted = true },
	}
	if err := checkOnce(context.Background(), cfg); err != nil {
		t.Fatalf("checkOnce() = %v", err)
	}
	if restarted {
		t.Fatal("restarted while the kiosk was busy; a student would have lost their exam")
	}
	if _, err := embedcfg.Read(target); err != nil {
		t.Fatalf("the update should still be staged on disk: %v", err)
	}

	cfg.Idle = func() time.Duration { return time.Hour } // empty classroom
	if err := checkOnce(context.Background(), cfg); err != nil {
		t.Fatalf("second checkOnce() = %v", err)
	}
	if !restarted {
		t.Fatal("did not hand over to the new binary even though the kiosk was idle")
	}
}
