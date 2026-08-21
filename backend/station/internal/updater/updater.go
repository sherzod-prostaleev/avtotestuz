// Package updater keeps an installed classroom agent up to date on its own.
//
// Before this existed, shipping a fix meant asking every school to open the
// admin panel, download the .exe again and run it on every PC by hand. That is
// not something a driving school does, so in practice a fix reached the
// machines that someone happened to revisit and no others -- and there was no
// way to tell which was which.
//
// How it works. The API image already carries the plain, school-agnostic
// Windows agent it serves from the admin panel. Two unauthenticated endpoints
// expose it: a manifest with the version, size and SHA-256, and the bytes
// themselves. This package polls the manifest, and when it names a version
// other than the running one it downloads the base binary, checks the digest,
// appends the trailer its own installed copy is already carrying (see
// embedcfg.RawTrailer -- that is where the school's installer key lives, and
// it never leaves the machine), and swaps the result into place.
//
// What it deliberately does NOT do is restart the agent mid-lesson. Windows
// will not let a running image be overwritten but will let it be renamed, so
// the swap is safe while the current process keeps running from the renamed
// file; the new binary takes over at the next start. A classroom PC is
// switched off every evening, so "next start" is tomorrow morning -- and for
// the impatient case the agent restarts itself once the kiosk has been idle
// long enough that nobody can be mid-exam.
package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"avtotest.uz/station/internal/embedcfg"
	"avtotest.uz/station/internal/netclient"
)

// Manifest is what GET /api/v1/b2b/stations/agent-manifest answers.
type Manifest struct {
	Version string `json:"version"`
	SHA256  string `json:"sha256"`
	Size    int64  `json:"size"`
}

const (
	manifestPath = "/api/v1/b2b/stations/agent-manifest"
	downloadPath = "/api/v1/b2b/stations/agent"

	// maxAgentBytes bounds what will be pulled down and hashed. The agent is
	// ~6 MB; 64 MB is generous headroom that still refuses to fill a
	// classroom PC's disk if the endpoint ever serves something unexpected.
	maxAgentBytes = 64 << 20

	// stagedName holds the version already written into place, so a restart
	// can tell "the swap worked" from "the swap is still pending".
	stagedName = "update-staged.json"
)

// Config is one updater.
type Config struct {
	// APIBase is the backend origin, e.g. https://drivergo.uz.
	APIBase string
	// Version is what this process is running.
	Version string
	// Target is the installed copy to replace -- selfinstall.Target(stateDir).
	Target string
	// StateDir is where the staged-version marker is written.
	StateDir string
	// Report receives a short Uzbek sentence whenever the update state
	// changes, for the kiosk status page. May be nil.
	Report func(string)
	// Idle reports how long the kiosk has been making no API calls. The agent
	// only restarts itself when this is comfortably longer than an exam, so a
	// student is never interrupted. May be nil, in which case the agent never
	// restarts itself and the update lands at the next boot.
	Idle func() time.Duration
	// Restart is called when it is safe to hand over to the new binary. May
	// be nil.
	Restart func()

	// CheckEvery and IdleBefore default to sensible values when zero.
	CheckEvery time.Duration
	IdleBefore time.Duration
	// HTTP is for tests.
	HTTP *http.Client
}

// staged records the version written to Target but not yet running.
type staged struct {
	Version string `json:"version"`
	At      string `json:"at"`
}

// normalize fills in the defaults. Every entry point calls it, rather than
// only Run: a zero IdleBefore would otherwise make maybeRestart treat a busy
// classroom as idle and cut a student's exam short.
func (c Config) normalize() Config {
	if c.CheckEvery <= 0 {
		// Six hours: a classroom PC is on for about eight, so an update
		// published in the morning is picked up the same day, while a fleet
		// of thirty PCs still costs the origin only a handful of requests
		// per school per day.
		c.CheckEvery = 6 * time.Hour
	}
	if c.IdleBefore <= 0 {
		// A full exam is 20 questions in 25 minutes. Thirty minutes of no
		// API traffic at all means the room is empty, not that someone is
		// thinking hard.
		c.IdleBefore = 30 * time.Minute
	}
	if c.Report == nil {
		c.Report = func(string) {}
	}
	return c
}

// Run polls for a new build until ctx is done. It never returns an error: an
// agent that cannot update is still an agent that works, and a school must
// never lose its kiosk because a download failed.
func Run(ctx context.Context, cfg Config) {
	cfg = cfg.normalize()
	clearStaleMarker(cfg)

	// First check shortly after boot rather than immediately: the network is
	// often not up yet, and enrollment deserves the first round trip.
	timer := time.NewTimer(2 * time.Minute)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		if err := checkOnce(ctx, cfg); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("update check: %v", err)
		}
		timer.Reset(cfg.CheckEvery)
	}
}

// clearStaleMarker resolves the marker left by a previous swap. If the running
// version is the one that was staged, the update landed and the marker is
// removed. If a different version is running, the staged binary is still
// waiting for a restart and the marker stays so the kiosk can say so.
func clearStaleMarker(cfg Config) {
	path := filepath.Join(cfg.StateDir, stagedName)
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var s staged
	if json.Unmarshal(b, &s) != nil || s.Version == "" {
		_ = os.Remove(path)
		return
	}
	if s.Version == cfg.Version {
		log.Printf("update to %s is now running", s.Version)
		_ = os.Remove(path)
		return
	}
	cfg.Report(fmt.Sprintf("Yangilanish %s tayyor — kompyuter qayta yoqilganda o'rnatiladi.", s.Version))
}

func client(cfg Config) *http.Client {
	if cfg.HTTP != nil {
		return cfg.HTTP
	}
	// Generous next to the 15s used for control calls: this pulls ~6 MB over
	// whatever a driving school calls broadband.
	return netclient.New(10 * time.Minute)
}

func checkOnce(ctx context.Context, cfg Config) error {
	cfg = cfg.normalize()
	m, err := fetchManifest(ctx, cfg)
	if err != nil {
		return err
	}
	if m.Version == "" || m.SHA256 == "" {
		return fmt.Errorf("manifest is incomplete: %+v", m)
	}
	if m.Version == cfg.Version {
		cfg.Report("")
		return nil
	}
	// Already staged, just waiting for a restart.
	if s, err := readStaged(cfg); err == nil && s.Version == m.Version {
		maybeRestart(cfg, m.Version)
		return nil
	}

	log.Printf("update: %s available (running %s)", m.Version, cfg.Version)
	cfg.Report(fmt.Sprintf("Yangilanish %s yuklab olinmoqda…", m.Version))
	if err := stage(ctx, cfg, m); err != nil {
		cfg.Report("")
		return err
	}
	log.Printf("update: %s staged at %s", m.Version, cfg.Target)
	cfg.Report(fmt.Sprintf("Yangilanish %s tayyor — kompyuter qayta yoqilganda o'rnatiladi.", m.Version))
	maybeRestart(cfg, m.Version)
	return nil
}

func fetchManifest(ctx context.Context, cfg Config) (Manifest, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.APIBase+manifestPath, nil)
	if err != nil {
		return Manifest{}, err
	}
	resp, err := client(cfg).Do(req)
	if err != nil {
		return Manifest{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return Manifest{}, fmt.Errorf("%s: status %d", manifestPath, resp.StatusCode)
	}
	var env struct {
		Data Manifest `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<10)).Decode(&env); err != nil {
		return Manifest{}, fmt.Errorf("%s: %w", manifestPath, err)
	}
	return env.Data, nil
}

// stage downloads the base binary, verifies it against the manifest, gives it
// this machine's own school trailer and swaps it into place.
//
// The digest is checked before the trailer is appended, because the digest is
// of the plain binary the server holds -- appending first would make every
// school's file hash differently and there would be nothing to compare against.
func stage(ctx context.Context, cfg Config, m Manifest) (err error) {
	trailer, err := embedcfg.RawTrailer(cfg.Target)
	if err != nil {
		// Without a trailer the replacement would come up with no school
		// configured and no installer key, i.e. a PC that has just
		// un-enrolled itself. Refuse rather than ship that.
		return fmt.Errorf("read this PC's school configuration: %w", err)
	}

	dir := filepath.Dir(cfg.Target)
	tmp, err := os.CreateTemp(dir, ".avtotest-update-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		if err != nil {
			_ = os.Remove(tmpPath)
		}
	}()

	// The version in the query is what makes the response safe to cache at
	// the edge: the manifest is never cached, so a PC always learns the new
	// version first and then asks for that exact build by name.
	req, rErr := http.NewRequestWithContext(ctx, http.MethodGet,
		cfg.APIBase+downloadPath+"?v="+url.QueryEscape(m.Version), nil)
	if rErr != nil {
		_ = tmp.Close()
		return rErr
	}
	resp, rErr := client(cfg).Do(req)
	if rErr != nil {
		_ = tmp.Close()
		return rErr
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		_ = tmp.Close()
		return fmt.Errorf("%s: status %d", downloadPath, resp.StatusCode)
	}

	sum := sha256.New()
	written, cErr := io.Copy(io.MultiWriter(tmp, sum), io.LimitReader(resp.Body, maxAgentBytes))
	if cErr != nil {
		_ = tmp.Close()
		return cErr
	}
	if written != m.Size {
		_ = tmp.Close()
		return fmt.Errorf("downloaded %d bytes, manifest says %d", written, m.Size)
	}
	if got := hex.EncodeToString(sum.Sum(nil)); !strings.EqualFold(got, m.SHA256) {
		_ = tmp.Close()
		return fmt.Errorf("sha256 mismatch: got %s, manifest says %s", got, m.SHA256)
	}
	if _, err = tmp.Write(trailer); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Chmod(0o755); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}

	// Prove the assembled file is readable as a configured agent before it
	// replaces a working one. A trailer written onto a truncated download
	// would otherwise only be discovered at the next boot, on a PC with no
	// working binary left.
	if _, err = embedcfg.Read(tmpPath); err != nil {
		return fmt.Errorf("assembled update is not a valid configured agent: %w", err)
	}

	if err = replace(tmpPath, cfg.Target); err != nil {
		return err
	}
	return writeStaged(cfg, m.Version)
}

// replace swaps src over dst, keeping the previous binary as dst+".prev".
//
// Rename first, then move into place: Windows refuses to open a running image
// for writing but allows renaming it, so this works even when dst is the copy
// this very process was started from. The old binary is kept rather than
// deleted so a human has something to put back by hand if a release turns out
// to be bad on a machine nobody can reach.
func replace(src, dst string) error {
	prev := dst + ".prev"
	_ = os.Remove(prev)
	if err := os.Rename(dst, prev); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("move the current agent aside: %w", err)
	}
	if err := os.Rename(src, dst); err != nil {
		// Put the working binary back rather than leave the PC with none.
		_ = os.Rename(prev, dst)
		return fmt.Errorf("move the new agent into place: %w", err)
	}
	return nil
}

func readStaged(cfg Config) (staged, error) {
	var s staged
	b, err := os.ReadFile(filepath.Join(cfg.StateDir, stagedName))
	if err != nil {
		return s, err
	}
	return s, json.Unmarshal(b, &s)
}

func writeStaged(cfg Config, version string) error {
	b, err := json.Marshal(staged{Version: version, At: time.Now().UTC().Format(time.RFC3339)})
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(cfg.StateDir, stagedName), b, 0o644)
}

// maybeRestart hands over to the newly staged binary, but only when the kiosk
// has been completely idle long enough that no student can be mid-exam. When
// it is not safe, nothing happens and the update lands at the next boot, which
// on a classroom PC is the following morning.
func maybeRestart(cfg Config, version string) {
	if cfg.Restart == nil || cfg.Idle == nil {
		return
	}
	if cfg.Idle() < cfg.IdleBefore {
		return
	}
	log.Printf("update: kiosk idle, restarting into %s", version)
	cfg.Restart()
}
