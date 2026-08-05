package b2b

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/httpx"
)

// PublicRoutes are the three unauthenticated station endpoints. They are
// public by necessity — a classroom PC has no session until it has proved it
// holds a station key — so each one is rate limited two ways: by caller
// identity (station id, or enrollment code for the one endpoint that has no
// station id yet) and, secondarily, by client IP.
func (h *Handler) PublicRoutes(r chi.Router) {
	r.Post("/b2b/stations/enroll", h.enrollStation)
	r.Post("/b2b/stations/challenge", h.stationChallenge)
	r.Post("/b2b/stations/token", h.stationToken)
}

func (h *Handler) stationAuth() StationAuth {
	return StationAuth{Pool: h.Pool, Redis: h.Redis, Secret: h.Secret}
}

// Per-action limits. identity is the real abuse control: nginx proxies to
// this service over loopback and does not yet send the signed trusted-proxy
// assertion h.ClientIPs needs to recover the real caller, so until that is
// configured `allow` skips the IP dimension entirely rather than collapsing
// every caller on the platform into one shared bucket (see allow's doc
// comment for why). It activates automatically, with no code change, the
// moment nginx starts sending the assertion.
//
// challenge issues a bearer-less nonce that proves nothing about the
// caller — every syntactically valid station id "succeeds" whether or not
// that station exists — so there is no failed/successful split to key on;
// its identity ceiling only has to bound raw resource use, not act as a
// per-station lockout. A live station needs roughly 5 challenge+token pairs
// an hour (the access token's TTL is 15 minutes); challengeIdentityLimit
// gives ~40x that for clock drift, retries and reconnects, which is still
// far too low to be a meaningful DoS budget for an attacker.
//
// token verifies a real signature, so unlike challenge a wrong guess is
// distinguishable from the station's own traffic. tokenIdentityLimit is a
// coarse pre-work volume cap only — raised the same way as challenge's, and
// it still counts every attempt including a station's own successful ones,
// so it must never be the sole gate or the same lockout challenge had would
// just move here. tokenFailedIdentityLimit is the control that actually
// matters: it only grows on a failed verification, so a legitimate
// station's own successful traffic can never push it over the edge, while
// an attacker who cannot produce a valid signature for a station id it does
// not own (a station id is not a credential — it sits in agent config and
// teacher-facing station lists) is bounded to a small number of guesses per
// hour.
const (
	enrollIdentityLimit = 20
	enrollIPLimit       = 60

	challengeIdentityLimit = 200
	challengeIPLimit       = 600

	tokenIdentityLimit = 200
	// tokenFailedIdentityLimit gates how many failed signature checks a
	// single station id can accumulate in an hour before token issuance
	// locks out for that id (see recordTokenFailure). It was 20 in the
	// previous round, sized to also leave headroom for a station's own
	// legitimate traffic sharing the bucket. That headroom concern is gone:
	// a station's own successful renewals never touch this bucket, and a
	// successful issue now clears it outright (see the reset in
	// stationToken), so nothing legitimate accumulates here anymore -- only
	// sustained failure does. What is left to size for is purely how
	// expensive it is for someone who merely knows a station's id (not a
	// secret -- it sits in agent config and teacher-facing station lists)
	// to lock that station out of obtaining tokens for the rest of the
	// window. Raised to 100: still small enough to fail closed quickly
	// against a sustained attacker (an attacker without the station's
	// private key can never produce a passing signature no matter how many
	// tries it gets, so a higher ceiling does not buy it anything but
	// wasted requests), while making a deliberate lockout cost 5x what it
	// did before.
	tokenFailedIdentityLimit = 100
	tokenIPLimit             = 600
)

// hashIdentity turns a caller-supplied rate-limit identity (an enrollment
// code or a station id) into a fixed-length, non-reversible key segment. The
// enrollment code in particular is a short-lived shared secret; without this
// it would appear verbatim in Redis MONITOR output and the slowlog for the
// life of the enrollment window. Applied uniformly to every identity
// dimension so the keying stays consistent regardless of what the identity
// actually is.
func hashIdentity(identity string) string {
	sum := sha256.Sum256([]byte(identity))
	return hex.EncodeToString(sum[:16])
}

// clientIP returns the caller's address only when h.ClientIPs verified a
// signed trusted-proxy assertion for this request -- the same asserted
// check allow uses for its own IP dimension (see allow's doc comment for
// why an unasserted value cannot be trusted as the caller's IP). An
// unasserted result is not "no IP available": h.ClientIPs.Resolve would
// happily return one, but in this deployment that is nginx's own loopback
// peer address for every request until nginx is configured to send the
// assertion, so writing it as-is would populate b2b_station.last_ip with
// the same wrong value for every station on every login. The result is
// written through a ::inet cast via NULLIF($3,'')::inet, and a malformed
// value there fails that cast, which silently drops the rest of the
// telemetry UPDATE (last_seen_at, agent_version) along with it, so an
// unasserted or otherwise-unparseable IP yields an empty string -- turned
// into a NULL an operator can interpret, not a loopback address that looks
// like real data.
func (h *Handler) clientIP(r *http.Request) string {
	ip, asserted := h.ClientIPs.ResolveAsserted(r)
	if !asserted || net.ParseIP(ip) == nil {
		return ""
	}
	return ip
}

// allow applies a two-dimension fixed-window limit: identity first, IP
// second. identity is skipped when it is empty (no code/station id yet).
//
// The IP dimension is skipped unless h.ClientIPs verified a signed
// trusted-proxy assertion for this request (ResolveAsserted's asserted
// return). An *unasserted* IP is not "no IP" — h.clientIP(r) will happily
// return the TCP peer address — but in production that peer is nginx's own
// loopback address for every request until nginx is configured to send the
// assertion, so keying a limit on it would share one bucket across every
// caller on the platform: an attacker rotating identities (a fresh
// station_id per request) never trips their own identity bucket while
// draining the shared IP bucket dry, locking out every real station until
// the window rolls. Gating on asserted rather than on "IP is non-empty" is
// what closes that: the dimension turns on by itself, correctly, the moment
// nginx starts sending the header, with no code change here.
//
// If neither dimension applied — identity empty and IP unasserted — the
// request would otherwise leave with no limit at all, breaking allow's own
// fail-closed contract (reachable today via e.g. an enroll call with
// `{"code":""}`). That case falls back to one coarse action-wide bucket,
// sized the same as the dormant IP bucket: it can't attribute the request
// to a caller, but it still bounds the total rate of unkeyable requests.
//
// A missing limiter (tests) allows. Fails closed: a limiter error denies.
func (h *Handler) allow(r *http.Request, action, identity string, identityLimit, ipLimit int, window time.Duration) bool {
	if h.Lim.R == nil {
		return true
	}
	applied := false
	if identity != "" {
		key := "station:" + action + ":id:" + hashIdentity(identity)
		ok, err := h.Lim.Allow(r.Context(), key, identityLimit, window)
		if err != nil || !ok {
			return false
		}
		applied = true
	}
	if ip, asserted := h.ClientIPs.ResolveAsserted(r); asserted {
		ok, err := h.Lim.Allow(r.Context(), "station:"+action+":ip:"+ip, ipLimit, window)
		if err != nil || !ok {
			return false
		}
		applied = true
	}
	if !applied {
		ok, err := h.Lim.Allow(r.Context(), "station:"+action+":unkeyed", ipLimit, window)
		if err != nil || !ok {
			return false
		}
	}
	return true
}

type enrollStationBody struct {
	Code         string `json:"code"`
	PublicKey    string `json:"public_key"`
	HWIDHash     string `json:"hwid_hash"`
	Label        string `json:"label"`
	AgentVersion string `json:"agent_version"`
}

func (h *Handler) enrollStation(w http.ResponseWriter, r *http.Request) {
	var body enrollStationBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "invalid json")
		return
	}
	// Decoding the body is pure CPU work, no DB or Redis touched yet, so the
	// limiter still runs before any I/O. The enrollment code is the identity
	// dimension here — there is no station id until enrollment succeeds.
	// Canonicalize it the same way Store.EnrollStation does before it becomes
	// a rate-limit key: otherwise "AVTO-ABCD-EFGH", "avto-abcd-efgh" and
	// " AVTO-ABCD-EFGH " are three independent buckets for what is really one
	// leaked code, and the limit is evaded by changing case or whitespace.
	identity := strings.ToUpper(strings.TrimSpace(body.Code))
	if !h.allow(r, "enroll", identity, enrollIdentityLimit, enrollIPLimit, time.Hour) {
		httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "too many enrollment attempts")
		return
	}
	pub, err := base64.StdEncoding.DecodeString(body.PublicKey)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "public_key must be base64")
		return
	}
	out, err := h.store().EnrollStation(r.Context(), EnrollInput{
		Code:         body.Code,
		PublicKey:    pub,
		HWIDHash:     body.HWIDHash,
		Label:        body.Label,
		AgentVersion: body.AgentVersion,
	})
	if err != nil {
		writeStoreErr(w, err, "enrollment failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

type stationChallengeBody struct {
	StationID string `json:"station_id"`
}

func (h *Handler) stationChallenge(w http.ResponseWriter, r *http.Request) {
	var body stationChallengeBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "invalid json")
		return
	}
	stationID, err := uuid.Parse(body.StationID)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid station id")
		return
	}
	// Decoding the body and parsing the UUID are both pure CPU work, no DB
	// or Redis touched yet, so the limiter still runs before any I/O. The
	// station id is the identity dimension.
	if !h.allow(r, "challenge", stationID.String(), challengeIdentityLimit, challengeIPLimit, time.Hour) {
		httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "too many challenge requests")
		return
	}
	out, err := h.stationAuth().Challenge(r.Context(), stationID)
	if err != nil {
		if errors.Is(err, ErrStationAuth) {
			httpx.Error(w, http.StatusUnauthorized, "station_unauthorized", "station authentication failed")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "server_error", "challenge failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

type stationTokenBody struct {
	StationID    string `json:"station_id"`
	Nonce        string `json:"nonce"`
	TS           int64  `json:"ts"`
	Sig          string `json:"sig"`
	HWIDHash     string `json:"hwid_hash"`
	AgentVersion string `json:"agent_version"`
}

func (h *Handler) stationToken(w http.ResponseWriter, r *http.Request) {
	var body stationTokenBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "invalid json")
		return
	}
	stationID, err := uuid.Parse(body.StationID)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid station id")
		return
	}
	// Decoding the body and parsing the UUID are both pure CPU work, no DB
	// or Redis touched yet, so the pre-work limiter still runs before any
	// I/O. This is a coarse volume cap only -- it counts every attempt,
	// including the station's own successful ones -- so it must not be the
	// only gate; see the constants' doc comment.
	if !h.allow(r, "token", stationID.String(), tokenIdentityLimit, tokenIPLimit, time.Hour) {
		httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "too many token requests")
		return
	}
	// The failed-attempt bucket is what actually protects one station from
	// being locked out by someone who merely knows its id: it only grows
	// when a call turns out to be bogus, so a legitimate station's own
	// successful renewals can never fill it. Checked before doing any
	// verification work so an already-bounded attacker doesn't get a free
	// signature check per request.
	failKey := "station:token:fail:" + hashIdentity(stationID.String())
	if h.Lim.R != nil {
		if n, err := h.Lim.Count(r.Context(), failKey); err != nil || n >= tokenFailedIdentityLimit {
			httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "too many failed token attempts")
			return
		}
	}
	sig, err := base64.StdEncoding.DecodeString(body.Sig)
	if err != nil {
		h.recordTokenFailure(r, failKey)
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "sig must be base64")
		return
	}
	out, err := h.stationAuth().Token(r.Context(), TokenInput{
		StationID:    stationID,
		Nonce:        body.Nonce,
		TS:           body.TS,
		Sig:          sig,
		HWIDHash:     body.HWIDHash,
		AgentVersion: body.AgentVersion,
		IP:           h.clientIP(r),
	})
	if err != nil {
		if errors.Is(err, ErrStationAuth) {
			h.recordTokenFailure(r, failKey)
			httpx.Error(w, http.StatusUnauthorized, "station_unauthorized", "station authentication failed")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "server_error", "token failed")
		return
	}
	// A successful Token call proves in.Sig verified against this station's
	// real key, so whatever is already sitting in failKey was noise --
	// clock drift, an expired nonce, a restart mid-handshake -- and not an
	// attacker grinding it up: nothing lacking the station's private key
	// can ever reach this line, so an attacker can never trigger this reset
	// to launder its own failed attempts. Clearing it here is what lets a
	// station that recovers keep working for the rest of the hour instead
	// of staying locked out by failures that happened before it recovered.
	// Best effort, like recordTokenFailure: the response is already a 200
	// by the time this runs, so a Redis error here does not need to change
	// it.
	if h.Lim.R != nil {
		_ = h.Lim.Reset(r.Context(), failKey)
	}
	httpx.Data(w, http.StatusOK, out)
}

// recordTokenFailure grows the failed-attempt bucket that gates further
// stationToken calls for this station id. Best effort: the response for
// this request is already decided (401 or 400) by the time this runs, so a
// limiter error here does not need to change it -- the pre-work bucket in
// stationToken already fails closed on every subsequent call regardless.
func (h *Handler) recordTokenFailure(r *http.Request, failKey string) {
	if h.Lim.R == nil {
		return
	}
	_, _ = h.Lim.Allow(r.Context(), failKey, tokenFailedIdentityLimit, time.Hour)
}

type enrollWindowBody struct {
	TTLMinutes int `json:"ttl_minutes"`
}

func (h *Handler) openEnrollWindow(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing claims")
		return
	}
	orgID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid org id")
		return
	}
	var body enrollWindowBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		// An empty body keeps meaning "use the default": Decode returns EOF
		// when there is nothing to read. Any other error (malformed JSON, a
		// string ttl_minutes) must not silently fall back to the 2-hour
		// default with no signal to the teacher who asked for something else.
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "invalid json")
		return
	}
	ttl := time.Duration(body.TTLMinutes) * time.Minute
	out, err := h.store().OpenEnrollWindowAsTeacher(r.Context(), claims.ProfileID, orgID, ttl)
	if err != nil {
		writeStoreErr(w, err, "open enroll window failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) getEnrollWindow(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing claims")
		return
	}
	orgID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid org id")
		return
	}
	out, err := h.store().ActiveEnrollCodeAsTeacher(r.Context(), claims.ProfileID, orgID)
	if err != nil {
		writeStoreErr(w, err, "enroll window query failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) closeEnrollWindow(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing claims")
		return
	}
	orgID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid org id")
		return
	}
	codeID, err := uuid.Parse(chi.URLParam(r, "codeID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid code id")
		return
	}
	if err := h.store().CloseEnrollWindowAsTeacher(r.Context(), claims.ProfileID, orgID, codeID); err != nil {
		writeStoreErr(w, err, "close enroll window failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
