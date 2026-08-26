// Package agent talks to the AvtoTest backend on behalf of one classroom PC.
package agent

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"avtotest.uz/station/internal/keystore"
	"avtotest.uz/station/internal/netclient"
)

// tokenRenewMargin renews before expiry so a lesson never stalls on a
// round-trip that could have happened a minute earlier.
const tokenRenewMargin = 2 * time.Minute

// tokenFailTTL and tokenThrottledTTL are how long a failed token attempt is
// remembered before the agent will go back on the wire for another one.
//
// Nothing remembered a failure before, and a token is not fetched on a
// schedule: proxy.New asks for one on EVERY proxied API request, and the kiosk
// page re-probes /me for as long as it is unhappy. So while a station was
// failing, each of those turned straight back into a fresh challenge+token
// pair upstream -- one per image on the page, one per retry, per PC. On
// 2026-08-26 a 55-PC school produced 4,584 challenge calls and 2,446 token
// calls in about two hours against a budget of 60 a minute, which is why the
// rate limiter it had tripped could not drain while the class was sitting
// there. The retry rate has to be a property of this agent, not of how many
// requests the page in front of it happens to make.
//
// Two values because the two cases want opposite things. An ordinary failure
// is usually a few seconds of network coming up, and a classroom must recover
// from that in seconds: 5s is short enough to be invisible, long enough that a
// page full of requests costs one attempt instead of thirty. A 429 is the
// server saying in as many words that the caller is going too fast, and the
// only useful response is to stop asking for a while -- long enough that a
// throttled school actually drains its bucket instead of holding it open.
const (
	tokenFailTTL      = 5 * time.Second
	tokenThrottledTTL = 60 * time.Second
)

// tokenFailCooldown picks how long to sit out after err.
func tokenFailCooldown(err error) time.Duration {
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusTooManyRequests {
		return tokenThrottledTTL
	}
	return tokenFailTTL
}

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
	// tokenErr is the last token failure, served back to callers until
	// tokenErrTill so that a page full of API calls costs one attempt
	// upstream rather than one per call. See tokenFailTTL.
	tokenErr     error
	tokenErrTill time.Time
	state        State
	loaded       bool
	clockOff     int64
	clockSeen    bool
}

// ClockOffset returns how many seconds this PC's clock is ahead of the
// backend's, measured from the Date header of the last response, and whether
// any measurement has been taken yet.
//
// It exists because the backend rejects a signature stamped more than two
// minutes from its own clock (stationClockSkew) with the same opaque
// station_unauthorized it uses for a revoked station. Without this the agent
// tells a school with a dead CMOS battery to re-enrol, which spends a licence
// seat and leaves the clock exactly as wrong as it was.
func (a *Agent) ClockOffset() (seconds int64, known bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.clockOff, a.clockSeen
}

// State returns this PC's saved enrollment identity.
func (a *Agent) State() State {
	a.mu.Lock()
	defer a.mu.Unlock()
	_ = a.loadState()
	return a.state
}

// State is what survives a restart.
//
// Org and CodeHash were added after a school ran a second school's installer
// on a PC that was already enrolled: the agent kept the first school silently,
// because nothing in the saved state described which school the running binary
// had been built for. Both are optional -- state written by an older agent
// simply leaves them empty and the comparison is skipped.
type State struct {
	StationID string `json:"station_id"`
	OrgID     string `json:"org_id"`
	Label     string `json:"label"`
	// Org is the school name that was baked into the installer this PC
	// enrolled with, for display only.
	Org string `json:"org,omitempty"`
	// CodeHash identifies the installer key without storing it: a key is a
	// bearer credential that can enrol further PCs, and station.json is a
	// plain file. Comparing hashes is enough to notice a different school's
	// installer, which is all this is for.
	CodeHash string `json:"code_hash,omitempty"`
}

// HashCode is the fingerprint stored in State.CodeHash.
func HashCode(code string) string {
	if code == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(strings.ToUpper(strings.TrimSpace(code))))
	return hex.EncodeToString(sum[:8])
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
//
// Status is carried alongside Code because not every rejection comes with an
// envelope to read. nginx answers its own rate limiting itself, with an HTML
// body and no JSON, so Code is empty and the status line is the only thing
// that says what happened. That case used to degrade to a bare fmt.Errorf and
// reach a classroom as "Serverdan tushunarsiz javob keldi" -- "the server sent
// something we don't understand" -- which is what a school of 55 PCs was shown
// all morning on 2026-08-26 while the one real answer, 429, sat unread in the
// status line. Callers match on Status when Code is empty.
type APIError struct {
	Path    string
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string {
	if e.Code == "" {
		return fmt.Sprintf("%s: bad response (%d)", e.Path, e.Status)
	}
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
	// The cached failure belongs to the identity being discarded. Keeping it
	// would have the freshly re-enrolled station served the old station's
	// rejection for the rest of the cooldown -- which is the exact window an
	// operator who just ran -reenroll is standing there watching.
	a.tokenErr, a.tokenErrTill = nil, time.Time{}

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
	// Every response carries the origin's clock. Recording it costs nothing
	// and turns "station_unauthorized" from an unexplainable rejection into
	// "this PC's clock is N minutes off". Callers already hold a.mu, so this
	// writes the fields directly rather than re-locking.
	if served, dErr := http.ParseTime(resp.Header.Get("Date")); dErr == nil {
		a.clockOff = int64(time.Since(served).Seconds())
		a.clockSeen = true
	}
	var env struct {
		Data  json.RawMessage `json:"data"`
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		// Not JSON at all. This is the shape of every rejection written by
		// something in front of the API rather than by the API -- nginx's
		// 429 above all -- so it must keep the status code rather than
		// flattening to an untyped error.
		return &APIError{Path: path, Status: resp.StatusCode}
	}
	if resp.StatusCode != http.StatusOK {
		if env.Error != nil {
			return &APIError{Path: path, Status: resp.StatusCode, Code: env.Error.Code, Message: env.Error.Message}
		}
		return &APIError{Path: path, Status: resp.StatusCode}
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(env.Data, out)
}

// Enroll binds this machine to a school using the school's installer key.
// org is the school name baked into this installer, stored only so a later run
// can tell the operator which school this PC belongs to.
func (a *Agent) Enroll(ctx context.Context, code, label, org string) error {
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
	a.state = State{
		StationID: out.StationID, OrgID: out.OrgID, Label: out.Label,
		Org: org, CodeHash: HashCode(code),
	}
	a.loaded = true
	// Same reasoning as ResetEnrollment: a failure recorded against the
	// previous identity must not be replayed at the new one.
	a.tokenErr, a.tokenErrTill = nil, time.Time{}
	return a.saveState()
}

// Token returns a live station access token, renewing it when it is close to
// expiring.
//
// A failure from the last few seconds is served back from cache instead of
// being retried, because this is the call proxy.New makes on every proxied API
// request and the page in front of it can make dozens. Callers that pace
// themselves want Renew.
func (a *Agent) Token(ctx context.Context) (string, error) {
	return a.fetchToken(ctx, true)
}

// Renew is Token for the one caller that already paces itself: keepTokenWarm,
// whose loop backs off from 5 seconds to 2 minutes on its own.
//
// It skips the failure cache deliberately. That cache exists to stop *unpaced*
// callers from multiplying attempts; applying it here would stack a second
// delay on top of a backoff that was never the problem, and the only thing it
// would buy is a slower recovery for a classroom waiting to come back. Which
// classroom recovery matters: the stations reactivated after the 2026-08-26
// incident come back with nobody touching them because this loop keeps asking.
func (a *Agent) Renew(ctx context.Context) (string, error) {
	return a.fetchToken(ctx, false)
}

func (a *Agent) fetchToken(ctx context.Context, honourCooldown bool) (string, error) {
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
	// A failure this recent has already been reported upstream once; asking
	// again now would only deepen whatever caused it.
	if honourCooldown && a.tokenErr != nil && time.Now().Before(a.tokenErrTill) {
		return "", a.tokenErr
	}

	var ch struct {
		Nonce string `json:"nonce"`
	}
	if err := a.post(ctx, "/api/v1/b2b/stations/challenge",
		map[string]any{"station_id": a.state.StationID}, &ch); err != nil {
		return "", a.failToken(err)
	}

	priv, err := a.Keys.Load()
	if err != nil {
		return "", a.failToken(err)
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
		return "", a.failToken(err)
	}
	a.token = tok.AccessToken
	a.tokenTill = time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
	a.tokenErr, a.tokenErrTill = nil, time.Time{}
	return a.token, nil
}

// failToken records err as the current token failure and returns it unchanged,
// so callers keep matching on it with errors.Is exactly as before. Callers
// already hold a.mu.
func (a *Agent) failToken(err error) error {
	a.tokenErr = err
	a.tokenErrTill = time.Now().Add(tokenFailCooldown(err))
	return err
}
