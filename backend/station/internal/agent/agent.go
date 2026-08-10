// Package agent talks to the AvtoTest backend on behalf of one classroom PC.
package agent

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"avtotest.uz/station/internal/keystore"
	"avtotest.uz/station/internal/netclient"
)

// tokenRenewMargin renews before expiry so a lesson never stalls on a
// round-trip that could have happened a minute earlier.
const tokenRenewMargin = 2 * time.Minute

// Agent holds one station's identity and its cached access token.
type Agent struct {
	APIBase  string
	StateDir string
	Keys     keystore.Store
	HWID     string
	Version  string
	HTTP     *http.Client

	mu        sync.Mutex
	token     string
	tokenTill time.Time
	state     State
	loaded    bool
}

// State is what survives a restart.
type State struct {
	StationID string `json:"station_id"`
	OrgID     string `json:"org_id"`
	Label     string `json:"label"`
}

// ErrNotEnrolled means this PC has never been bound to a school.
var ErrNotEnrolled = errors.New("station is not enrolled")

// ErrStationUnauthorized means the backend refused this station's identity:
// it no longer knows the station id, or the station was revoked, or the
// signature/HWID did not match. The server answers all of those with one
// opaque code on purpose -- an unauthenticated endpoint must not reveal
// whether a given station id exists -- so the agent cannot tell them apart
// either, and must not "recover" by silently enrolling again: that would
// undo an admin's revoke and spend a fresh seat. Callers surface it to
// whoever is standing at the PC instead.
var ErrStationUnauthorized = errors.New("station authentication failed")

// APIError carries the backend's error envelope so callers can match on the
// code rather than on message text.
type APIError struct {
	Path    string
	Code    string
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s: %s (%s)", e.Path, e.Message, e.Code)
}

// Is lets errors.Is(err, ErrStationUnauthorized) work without callers
// knowing the wire code.
func (e *APIError) Is(target error) bool {
	return target == ErrStationUnauthorized && e.Code == "station_unauthorized"
}

// ResetEnrollment discards this PC's station identity -- the saved station id
// and the sealed private key -- so the next start enrolls as a brand new
// station. Deliberately not automatic: see ErrStationUnauthorized.
func (a *Agent) ResetEnrollment() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.state = State{}
	a.loaded = true
	a.token = ""
	a.tokenTill = time.Time{}

	for _, path := range []string{a.statePath(), keystore.KeyPath(a.StateDir)} {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("reset enrollment: %w", err)
		}
	}
	return nil
}

func (a *Agent) client() *http.Client {
	if a.HTTP != nil {
		return a.HTTP
	}
	return netclient.New(15 * time.Second)
}

func (a *Agent) statePath() string { return filepath.Join(a.StateDir, "station.json") }

func (a *Agent) loadState() error {
	if a.loaded {
		return nil
	}
	b, err := os.ReadFile(a.statePath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			a.loaded = true
			return nil
		}
		return err
	}
	if err := json.Unmarshal(b, &a.state); err != nil {
		return err
	}
	a.loaded = true
	return nil
}

func (a *Agent) saveState() error {
	b, err := json.Marshal(a.state)
	if err != nil {
		return err
	}
	return os.WriteFile(a.statePath(), b, 0o600)
}

// post sends a JSON body and decodes the {"data": ...} envelope into out.
func (a *Agent) post(ctx context.Context, path string, in, out any) error {
	body, err := json.Marshal(in)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.APIBase+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.client().Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	var env struct {
		Data  json.RawMessage `json:"data"`
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return fmt.Errorf("%s: bad response (%d)", path, resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		if env.Error != nil {
			return &APIError{Path: path, Code: env.Error.Code, Message: env.Error.Message}
		}
		return fmt.Errorf("%s: status %d", path, resp.StatusCode)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(env.Data, out)
}

// Enroll binds this machine to a school using a one-time org code.
func (a *Agent) Enroll(ctx context.Context, code, label string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	priv, err := a.Keys.Load()
	if err != nil {
		return err
	}
	pub, ok := priv.Public().(ed25519.PublicKey)
	if !ok {
		return errors.New("station key is not ed25519")
	}

	var out struct {
		StationID string `json:"station_id"`
		OrgID     string `json:"org_id"`
		Label     string `json:"label"`
	}
	err = a.post(ctx, "/api/v1/b2b/stations/enroll", map[string]any{
		"code":          code,
		"public_key":    base64.StdEncoding.EncodeToString(pub),
		"hwid_hash":     a.HWID,
		"label":         label,
		"agent_version": a.Version,
	}, &out)
	if err != nil {
		return err
	}
	a.state = State{StationID: out.StationID, OrgID: out.OrgID, Label: out.Label}
	a.loaded = true
	return a.saveState()
}

// Token returns a live station access token, renewing it when it is close to
// expiring.
func (a *Agent) Token(ctx context.Context) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if err := a.loadState(); err != nil {
		return "", err
	}
	if a.state.StationID == "" {
		return "", ErrNotEnrolled
	}
	if a.token != "" && time.Until(a.tokenTill) > tokenRenewMargin {
		return a.token, nil
	}

	var ch struct {
		Nonce string `json:"nonce"`
	}
	if err := a.post(ctx, "/api/v1/b2b/stations/challenge",
		map[string]any{"station_id": a.state.StationID}, &ch); err != nil {
		return "", err
	}

	priv, err := a.Keys.Load()
	if err != nil {
		return "", err
	}
	ts := time.Now().Unix()
	msg := []byte("avtotest-station-v1|" + a.state.StationID + "|" + ch.Nonce + "|" + strconv.FormatInt(ts, 10))
	sig := ed25519.Sign(priv, msg)

	var tok struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	err = a.post(ctx, "/api/v1/b2b/stations/token", map[string]any{
		"station_id":    a.state.StationID,
		"nonce":         ch.Nonce,
		"ts":            ts,
		"sig":           base64.StdEncoding.EncodeToString(sig),
		"hwid_hash":     a.HWID,
		"agent_version": a.Version,
	}, &tok)
	if err != nil {
		return "", err
	}
	a.token = tok.AccessToken
	a.tokenTill = time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
	return a.token, nil
}
